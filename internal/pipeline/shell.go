package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// newShellCmd 把一组命令写入临时脚本，返回在 dir 下执行它的 *exec.Cmd。
// 同一脚本内的命令共用一个 shell 会话（cd、环境变量会保留），任一条失败即停止。
// cleanup 用于删除临时脚本。
func newShellCmd(cmds []string, dir string) (*exec.Cmd, func(), error) {
	suffix := ".sh"
	if runtime.GOOS == "windows" {
		suffix = ".ps1"
	}
	tmp, err := os.CreateTemp("", "pvm-run-*"+suffix)
	if err != nil {
		return nil, nil, fmt.Errorf("create temp script: %w", err)
	}
	cleanup := func() { os.Remove(tmp.Name()) }
	if _, err := tmp.WriteString(buildScript(cmds)); err != nil {
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
		shell, legacy := windowsShell()
		if legacy {
			if step, ok := findChainOperator(cmds); ok {
				cleanup()
				return nil, nil, fmt.Errorf(
					"step %q uses '&&' or '||', which Windows PowerShell 5.1 does not support\n"+
						"  Fix any of:\n"+
						"    1. Install PowerShell 7 (winget install Microsoft.PowerShell) — pvm will use it automatically\n"+
						"    2. Split the command into separate steps in pvm.yaml\n"+
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
	cmd.Dir = dir
	// 被取消后管道可能被孙进程占着，限时放弃等待，避免 Wait 卡死
	cmd.WaitDelay = 3 * time.Second
	return cmd, cleanup, nil
}

// windowsShell 优先 pwsh（7+，支持 && / ||），否则回退 Windows PowerShell 5.1。
func windowsShell() (shell string, legacy bool) {
	if _, err := exec.LookPath("pwsh"); err == nil {
		return "pwsh", false
	}
	return "powershell", true
}

// findChainOperator 查找 PowerShell 5.1 不支持的 && / ||（简单子串匹配）。
func findChainOperator(cmds []string) (string, bool) {
	for _, c := range cmds {
		if strings.Contains(c, "&&") || strings.Contains(c, "||") {
			return c, true
		}
	}
	return "", false
}

// buildScript 生成平台脚本：每步前打印 "==> 命令"，任一步非零退出即终止并指出是哪一步。
// 脚本里的提示文字保持 ASCII，避免 PowerShell 5.1 对无 BOM 文件的编码误判。
func buildScript(cmds []string) string {
	var b strings.Builder
	if runtime.GOOS == "windows" {
		b.WriteString("$ErrorActionPreference = 'Stop'\n")
		for _, c := range cmds {
			q := psQuote(c)
			fmt.Fprintf(&b, "Write-Host '==> %s'\n", q)
			b.WriteString("$global:LASTEXITCODE = 0\n")
			b.WriteString(c)
			b.WriteString("\n")
			fmt.Fprintf(&b, "if ($LASTEXITCODE -ne 0) { $c = $LASTEXITCODE; Write-Host ('x failed (exit ' + $c + '): ' + '%s'); exit $c }\n", q)
		}
		return b.String()
	}
	b.WriteString("set -e\n")
	b.WriteString("__pvm_step=''\n")
	b.WriteString(`trap 'c=$?; if [ $c -ne 0 ]; then echo "x failed (exit $c): $__pvm_step" >&2; fi' EXIT` + "\n")
	for _, c := range cmds {
		fmt.Fprintf(&b, "__pvm_step=%s\n", shQuote(c))
		fmt.Fprintf(&b, "echo %s\n", shQuote("==> "+c))
		b.WriteString(c)
		b.WriteString("\n")
	}
	return b.String()
}

// psQuote 转义 PowerShell 单引号字符串内容（单引号写两遍）
func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

// shQuote 生成 sh 单引号字符串
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
