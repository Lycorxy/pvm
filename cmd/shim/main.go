// Package main 实现 pvm-shim：一个极简的命令转发器。
//
// 设计参考 Volta / Proto：把 pvm-shim 二进制复制（Windows）或符号链接（Unix）
// 为各个命令名（node、npm、git、go、pnpm...），启动后通过自身可执行文件名
// 识别命令，再委托 pvm 主程序解析版本并执行真实二进制。
//
// 这样做的优势：
//   - Windows 上 shim 是真正的 .exe，被 VSCode 等 IDE 识别（.cmd 不会被识别）
//   - 不受 PowerShell 执行策略影响（.ps1 会被拦截）
//   - 跨平台统一逻辑，支持项目级版本自动切换（symlink/junction 方案做不到）
//   - shim 极简，仅依赖标准库，体积小、启动快
//
// 编译：go build -o pvm-shim ./cmd/shim
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func main() {
	// 1. 从自身可执行文件路径取命令名（argv[0] 在 Windows 上不可靠，用 os.Executable）
	exePath, err := os.Executable()
	if err != nil {
		fatal("cannot determine executable path: %v", err)
	}

	cmdName := commandNameFromExe(exePath)
	if cmdName == "" || cmdName == "pvm-shim" || cmdName == "pvm" {
		// 直接运行 pvm-shim 本身：给出用法提示
		fmt.Fprintln(os.Stderr, "pvm-shim is an internal shim dispatcher and should not be run directly.")
		fmt.Fprintln(os.Stderr, "It is copied/symlinked as command names (node, npm, git, go, ...) and")
		fmt.Fprintln(os.Stderr, "forwards execution to the pvm version manager.")
		os.Exit(1)
	}

	// 2. 定位 pvm 主程序
	pvmExe, err := findPvmExe(exePath)
	if err != nil {
		fatal("%v", err)
	}

	// 3. 构造转发参数：pvm shim-exec <cmdName> <原参数...>
	passthrough := os.Args[1:]
	argv := append([]string{pvmExe, "shim-exec", cmdName}, passthrough...)

	// 4. 执行
	// Unix：用 syscall.Exec 替换当前进程，退出码与信号天然传递，零开销
	if runtime.GOOS != "windows" {
		if err := syscall.Exec(pvmExe, argv, os.Environ()); err != nil {
			fatal("exec pvm: %v", err)
		}
		// 不会到达这里
		return
	}

	// Windows：无法真 exec，用子进程 + 退出码透传 + 信号转发
	cmd := exec.Command(pvmExe, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	// 拦截 Ctrl+C / SIGTERM 并转发给子进程，保证交互式命令（node REPL、git push 等）正常退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	if err := cmd.Start(); err != nil {
		fatal("start pvm: %v", err)
	}

	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		fatal("pvm exited: %v", err)
	}
}

// commandNameFromExe 从可执行文件路径提取命令名。
// 例：C:\Users\x\.pvm\shims\node.exe → "node"
//     /home/x/.pvm/shims/npm         → "npm"
func commandNameFromExe(exePath string) string {
	base := filepath.Base(exePath)
	// 去掉 .exe 后缀（Windows）
	if runtime.GOOS == "windows" {
		base = strings.TrimSuffix(base, ".exe")
	}
	return strings.ToLower(base)
}

// findPvmExe 定位 pvm 主程序可执行文件。
// 查找顺序：
//  1. shim 所在目录的上一级 bin/ 目录（~/.pvm/shims/ → ~/.pvm/bin/pvm）—— 最常见
//  2. ~/.pvm/bin/pvm —— 标准安装位置
//  3. 当前可执行文件同目录 —— 开发模式 / MSI 安装目录
//  4. 系统 PATH —— 兜底
func findPvmExe(shimPath string) (string, error) {
	ext := exeExt()
	pvmName := "pvm" + ext

	var candidates []string

	// 1. shims 的父目录下的 bin/（~/.pvm/shims/x → ~/.pvm/bin/pvm）
	shimDir := filepath.Dir(shimPath)
	parentDir := filepath.Dir(shimDir)
	candidates = append(candidates, filepath.Join(parentDir, "bin", pvmName))

	// 2. ~/.pvm/bin/pvm
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".pvm", "bin", pvmName))
	}

	// 3. shim 同目录（开发模式或扁平安装）
	candidates = append(candidates, filepath.Join(shimDir, pvmName))

	// 4. 系统 PATH
	if p, err := exec.LookPath(pvmName); err == nil {
		candidates = append(candidates, p)
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}

	return "", fmt.Errorf("pvm-shim: cannot locate pvm executable (looked in: %s)", strings.Join(candidates, ", "))
}

func exeExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "pvm-shim: "+format+"\n", args...)
	os.Exit(1)
}
