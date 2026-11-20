package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/installer"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/pipeline"
	"github.com/pvm/pvm/internal/shim"
)

const runUsage = `usage: pvm run <name> [--dry-run] [-C <dir>]
       pvm run --list`

const pipelineExample = `  Create pvm.yaml in the project root, e.g.:

    install: pnpm i
    build: pnpm build
    release:pro:
      - install
      - build`

// runRun 执行 pvm.yaml 中的流水线（本地流水线的唯一入口）。
//
//	pvm run <名字>            执行（先自动安装 .pvmrc 声明的运行时版本）
//	pvm run                   列出全部流水线（等同 --list）
//	pvm run <名字> --dry-run  只打印执行计划
//	pvm run <名字> -C <目录>  在指定目录查找 pvm.yaml（monorepo 子项目）
//
// pvm.yaml 从当前目录向上查找；命令的工作目录是 pvm.yaml 所在目录。
func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	var list, dryRun bool
	var dir string
	fs.BoolVar(&list, "list", false, "")
	fs.BoolVar(&list, "l", false, "")
	fs.BoolVar(&dryRun, "dry-run", false, "")
	fs.StringVar(&dir, "C", "", "")

	pos, err := parseFlags(fs, args)
	if err != nil {
		return fmt.Errorf("%v\n%s", err, runUsage)
	}
	if len(pos) > 1 {
		return fmt.Errorf("%s", runUsage)
	}

	cwd := dir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	pf, err := config.FindPipelineFile(cwd)
	if err != nil {
		return err
	}
	if pf == nil {
		return fmt.Errorf("no pvm.yaml found from %s\n%s", cwd, pipelineExample)
	}

	if list || len(pos) == 0 {
		printPipelineList(pf)
		// 列表也做一次引用校验：循环/未知引用在这里暴露，而不是等执行时才发现
		if errs := pipeline.Validate(pf); len(errs) > 0 {
			fmt.Fprintln(os.Stderr)
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  ✗ %v\n", e)
			}
			return fmt.Errorf("%s: %d pipeline error(s)", pf.Path, len(errs))
		}
		return nil
	}

	name := pos[0]
	root, err := pipeline.Compile(pf, name)
	if err != nil {
		return err
	}

	if dryRun {
		root.Render(os.Stdout)
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

	// Ctrl+C / SIGTERM：取消后 pipeline 终止全部子进程树；再按一次则立即退出
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()

	return pipeline.Run(ctx, root, pipeline.Options{Dir: filepath.Dir(pf.Path)})
}

// printPipelineList 展示全部流水线及其一行概要
func printPipelineList(pf *config.PipelineFile) {
	title := pf.Path
	if pf.Name != "" {
		title = pf.Name + "  (" + pf.Path + ")"
	}
	fmt.Println(title)
	fmt.Println()

	names := pf.Names()
	if len(names) == 0 {
		fmt.Println("  (no pipelines declared)")
		return
	}
	width := 0
	for _, n := range names {
		if len(n) > width {
			width = len(n)
		}
	}
	for _, n := range names {
		fmt.Printf("  %-*s  %s\n", width, n, pipeline.Summary(pf, n))
	}
	fmt.Println()
	fmt.Println("run: pvm run <name>   preview: pvm run <name> --dry-run")
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
