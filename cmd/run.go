package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/installer"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/shim"
)

// runRun 执行 pvm.yaml 中声明的流水线
//
// 用法：
//
//	pvm run <name>          # 执行一条流水线（前置自动安装+激活 .pvmrc 声明的运行时版本）
//	pvm run --list          # 列出所有流水线
//	pvm run <name> --dry-run # 展开打印执行计划，不执行
//
// 语义：
//   - pvm.yaml 从当前目录向上查找（monorepo：根目录一个，子目录直接跑，就近优先）
//   - 命令原样执行；整条流水线在同一个 shell 会话中运行（工作目录 = pvm.yaml 所在目录，cd 效果保留）
//   - 顺序执行，任一步失败即停止并报告失败步骤
//   - @名字 引用其他流水线，递归展开，循环引用报错
func runRun(args []string) error {
	args, listFlag := hasFlag(args, "--list")
	args, dryRun := hasFlag(args, "--dry-run")

	cwd, _ := os.Getwd()
	pf, err := config.FindPipelineFile(cwd)
	if err != nil {
		return err
	}
	if pf == nil {
		return fmt.Errorf("no pvm.yaml found from %s\n"+
			"  Create pvm.yaml in the project root, e.g.:\n"+
			"    build:\n"+
			"      dev:\n"+
			"        - cd web\n"+
			"        - pnpm i\n"+
			"        - npm run build:dev", cwd)
	}
	if len(pf.Names()) == 0 {
		return fmt.Errorf("no pipelines declared in %s", pf.Path)
	}

	// --list 或无参数：展示可用流水线
	if listFlag || len(args) == 0 {
		printPipelineList(pf)
		return nil
	}
	if len(args) > 1 {
		return fmt.Errorf("usage: pvm run <name> [--dry-run]  |  pvm run --list")
	}
	name := args[0]

	cmds, err := pf.ExpandPipeline(name)
	if err != nil {
		return err
	}
	if len(cmds) == 0 {
		logger.Info("pipeline %s is empty", name)
		return nil
	}

	if dryRun {
		fmt.Printf("pvm run %s --dry-run (%d steps, from %s)\n", name, len(cmds), pf.Path)
		for i, c := range cmds {
			fmt.Printf("  [%d/%d] %s\n", i+1, len(cmds), c)
		}
		return nil
	}

	// 前置 1：确保 .pvmrc 声明的运行时版本已安装（shim 执行时按声明自动解析版本）
	if vf, err := config.FindVersionFile(cwd); err == nil && vf != nil {
		if err := ensureDeclaredVersionsInstalled(vf); err != nil {
			return err
		}
	}

	// 前置 2：shims 未生效时给出提示（不阻断；流水线可能不依赖 shim）
	if ok, msg := checkShimsInPath(); !ok {
		logger.Info("  ⚠ pvm shims not effective: %s", msg)
		logger.Info("    run `pvm setup` to fix PATH before relying on managed runtimes")
	}

	if pf.Name != "" {
		fmt.Printf("pvm run %s — %d steps (project: %s, from %s)\n", name, len(cmds), pf.Name, pf.Path)
	} else {
		fmt.Printf("pvm run %s — %d steps (from %s)\n", name, len(cmds), pf.Path)
	}
	return executePipelineSteps(cmds, filepath.Dir(pf.Path), pf)
}

// printPipelineList 按首段分组展示项目名与全部流水线
func printPipelineList(pf *config.PipelineFile) {
	if pf.Name != "" {
		fmt.Printf("project: %s\n", pf.Name)
	}
	fmt.Printf("pipelines in %s:\n", pf.Path)
	for _, g := range pf.ListGroups() {
		fmt.Printf("  %s\n", g.Prefix)
		for _, n := range g.Names {
			fmt.Printf("    %-28s %d steps\n", n, len(pf.Commands[n]))
		}
	}
	fmt.Println()
	fmt.Println("usage: pvm run <name> [--dry-run]")
}

// ensureDeclaredVersionsInstalled 安装 .pvmrc 中声明但尚未安装的运行时版本。
// 全局唯一运行时（go/git/rust）按用户全局解析，跳过；system 声明跳过。
func ensureDeclaredVersionsInstalled(vf *config.VersionFile) error {
	installedAny := false
	for _, rt := range config.SupportedRuntimes {
		ver, ok := vf.Versions[rt]
		if !ok || ver == "" || ver == config.SystemVersion || config.IsGlobalOnly(rt) {
			continue
		}
		already := false
		for _, v := range config.ListInstalledVersions(rt) {
			if v == ver {
				already = true
				break
			}
		}
		if already {
			continue
		}
		logger.Info("  → installing declared %s@%s ...", rt, ver)
		if _, err := installer.Install(rt, ver, true, false); err != nil {
			return fmt.Errorf("install declared %s@%s: %w", rt, ver, err)
		}
		installedAny = true
	}
	if installedAny {
		if err := shim.Reshim(); err != nil && !shim.IsReshimWarning(err) {
			logger.Info("  ! reshim: %v", err)
		}
	}
	return nil
}

// bgJob 一个后台作业：命令列表 + 进程句柄
type bgJob struct {
	label string
	cmds  []string
	exec  *exec.Cmd
}

// executePipelineSteps 执行展开后的步骤。
//   - "&" 前缀的步骤为后台作业：先并行启动（如 dev 服务器），
//     引用形式 "&@name" 展开为该流水线的全部命令，作业内按顺序执行
//   - 其余步骤在同一个 shell 会话中顺序执行（cd 效果保留，失败即停）
//   - 工作目录为 pvm.yaml 所在目录：根配置的相对路径从任何子目录执行都确定
//   - 最后等待全部后台作业；任一非零退出即为失败
//
// 中断（Ctrl+C）：终止前台进程与全部后台作业的进程树后退出。
func executePipelineSteps(cmds []string, workDir string, pf *config.PipelineFile) error {
	total := len(cmds)
	var fg []string
	var jobs []bgJob
	for _, c := range cmds {
		if !strings.HasPrefix(c, "&") {
			fg = append(fg, c)
			continue
		}
		body := strings.TrimSpace(c[1:])
		jobCmds := []string{body}
		if strings.HasPrefix(body, "@") {
			expanded, err := pf.ExpandPipeline(strings.TrimPrefix(body, "@"))
			if err != nil {
				return err
			}
			jobCmds, err = flattenJobCmds(expanded, pf)
			if err != nil {
				return err
			}
		}
		jobs = append(jobs, bgJob{label: body, cmds: jobCmds})
	}

	// 中断清理：杀掉前台进程与所有后台作业的进程树
	var fgCmd *exec.Cmd
	stopAll := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		select {
		case <-sig:
		case <-stopAll:
			return
		}
		for _, j := range jobs {
			if j.exec != nil && j.exec.Process != nil {
				killProcessTree(j.exec.Process.Pid)
			}
		}
		if fgCmd != nil && fgCmd.Process != nil {
			killProcessTree(fgCmd.Process.Pid)
		}
		os.Exit(1)
	}()
	defer close(stopAll)

	// 1. 先启动全部后台作业（服务器先起）
	for i := range jobs {
		fmt.Printf("==> background [%d/%d]: %s\n", i+1, len(jobs), jobs[i].label)
		cmd, cleanup, err := spawnScript(jobs[i].cmds, workDir)
		if err != nil {
			killStarted(jobs[:i])
			return err
		}
		defer cleanup()
		jobs[i].exec = cmd
		if err := cmd.Start(); err != nil {
			killStarted(jobs[:i])
			return fmt.Errorf("start background job %q: %w", jobs[i].label, err)
		}
	}

	// 2. 前台步骤顺序执行
	if len(fg) > 0 {
		fmt.Printf("==> running %d foreground steps\n", len(fg))
		cmd, cleanup, err := spawnScript(fg, workDir)
		if err != nil {
			killStarted(jobs)
			return err
		}
		defer cleanup()
		fgCmd = cmd
		if err := cmd.Run(); err != nil {
			killStarted(jobs)
			return fmt.Errorf("pipeline failed: %w", err)
		}
	}

	// 3. 等待全部后台作业
	var failed []string
	for _, j := range jobs {
		if err := j.exec.Wait(); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", j.label, err))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("background job(s) failed: %s", strings.Join(failed, "; "))
	}

	fmt.Printf("==> all %d steps passed (%d background)\n", total, len(jobs))
	return nil
}

// flattenJobCmds 展平后台作业内的步骤：作业内 "&@ref" 按前台顺序展开（后台语义只在顶层步骤生效）
func flattenJobCmds(cmds []string, pf *config.PipelineFile) ([]string, error) {
	var out []string
	for _, c := range cmds {
		c = strings.TrimPrefix(c, "&")
		if strings.HasPrefix(c, "@") {
			sub, err := pf.ExpandPipeline(strings.TrimPrefix(c, "@"))
			if err != nil {
				return nil, err
			}
			flat, err := flattenJobCmds(sub, pf)
			if err != nil {
				return nil, err
			}
			out = append(out, flat...)
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// killStarted 终止已启动的后台作业（出错路径清理）
func killStarted(jobs []bgJob) {
	for _, j := range jobs {
		if j.exec != nil && j.exec.Process != nil {
			killProcessTree(j.exec.Process.Pid)
		}
	}
}

// spawnScript 把命令列表写入临时脚本并构造执行命令；返回 cleanup 移除脚本文件
func spawnScript(cmds []string, workDir string) (*exec.Cmd, func(), error) {
	content := buildScriptContent(cmds)

	suffix := ".sh"
	if runtime.GOOS == "windows" {
		suffix = ".ps1"
	}
	tmp, err := os.CreateTemp("", "pvm-run-*"+suffix)
	if err != nil {
		return nil, nil, fmt.Errorf("create temp script: %w", err)
	}
	cleanup := func() { os.Remove(tmp.Name()) }
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		cleanup()
		return nil, nil, fmt.Errorf("write temp script: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		shell, legacy := windowsPipelineShell()
		if legacy {
			if step, ok := findUnsupportedChainOperator(cmds); ok {
				cleanup()
				return nil, nil, fmt.Errorf(
					"step %q uses '&&' or '||', which Windows PowerShell 5.1 does not support\n"+
						"  Fix any of:\n"+
						"    1. Install PowerShell 7 (winget install Microsoft.PowerShell) — pvm will use it automatically\n"+
						"    2. Split the command into multiple steps in pvm.yaml (one command per line)\n"+
						"    3. Chain with ';' (no failure propagation)",
					step)
			}
		}
		cmd = exec.Command(shell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmp.Name())
	} else {
		shell := "sh"
		if _, err := exec.LookPath("bash"); err == nil {
			shell = "bash"
		}
		cmd = exec.Command(shell, tmp.Name())
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = workDir
	return cmd, cleanup, nil
}

// windowsPipelineShell 返回 Windows 流水线使用的 PowerShell 可执行文件。
// 优先 pwsh（PowerShell 7+，原生支持 && / || 串联运算符）；
// 仅当未安装 pwsh 时回退 Windows PowerShell 5.1（legacy，不支持 && / ||）。
func windowsPipelineShell() (shell string, legacy bool) {
	if _, err := exec.LookPath("pwsh"); err == nil {
		return "pwsh", false
	}
	return "powershell", true
}

// findUnsupportedChainOperator 查找 PowerShell 5.1 不支持的串联运算符
// （&& / ||），返回第一个命中的命令。简单子串匹配，可能对引号内/URL 中
// 的 && 误报——此时按报错提示改写写法即可。
func findUnsupportedChainOperator(cmds []string) (string, bool) {
	for _, c := range cmds {
		if strings.Contains(c, "&&") || strings.Contains(c, "||") {
			return c, true
		}
	}
	return "", false
}

// buildScriptContent 生成平台脚本：每步前打印 [i/N] 标记；任一步非零退出即终止
func buildScriptContent(cmds []string) string {
	total := len(cmds)
	var b strings.Builder

	if runtime.GOOS == "windows" {
		// PowerShell：$ErrorActionPreference 让 cmdlet 失败即停；
		// 原生命令需显式检查 $LASTEXITCODE（每步前清零避免误判）
		b.WriteString("$ErrorActionPreference = 'Stop'\n")
		for i, c := range cmds {
			fmt.Fprintf(&b, "Write-Host '==> [%d/%d] %s'\n", i+1, total, psSingleQuote(c))
			b.WriteString("$global:LASTEXITCODE = 0\n")
			b.WriteString(c)
			b.WriteString("\n")
			fmt.Fprintf(&b, "if ($LASTEXITCODE -ne 0) { Write-Host '==> step [%d/%d] failed'; exit $LASTEXITCODE }\n", i+1, total)
		}
	} else {
		// sh：set -e 失败即停
		b.WriteString("set -e\n")
		for i, c := range cmds {
			fmt.Fprintf(&b, "echo %s\n", shSingleQuote(fmt.Sprintf("==> [%d/%d] %s", i+1, total, c)))
			b.WriteString(c)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// psSingleQuote 为 PowerShell 单引号字符串转义（' → ”）
func psSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// shSingleQuote 为 sh 单引号字符串转义（' → '\”）
func shSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
