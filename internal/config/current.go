package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SetCurrent 在 ~/.pvm/bin 中为指定 runtime 创建可执行文件。
//
// 策略（单二进制方案）：
//   - Windows: 对所有运行时都创建 .exe（VSCode 等 IDE 只认 .exe）
//   - Git: pvm.exe 硬链接副本（git/bash/sh 等按文件名自分发到 pvm shim-exec）+ .cmd 直接指向
//     （ssh/curl/vim 等非 RuntimeShims 命令，避免 pvm 转发循环）
//   - Python/Go: 直接复制 exe
//   - Unix: 符号链接或复制
//
// ~/.pvm/bin 在 setup 时已加入 PATH 且永不变化，IDE 通过 bin 中的 exe 稳定可达。
// 关键：bash.exe 让 VSCode 能识别 Git Bash 终端 profile。
func SetCurrent(rt, version string) error {
	target := InstallDir(rt, version)
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("install dir not found: %s", target)
	}

	binDir := BinHome()
	if err := EnsureDir(binDir); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	switch rt {
	case "git":
		if runtime.GOOS == "windows" {
			return setCurrentGitWindows(target, binDir)
		}
		// Unix: 直接复制或符号链接
		return copyOrSymlink(filepath.Join(target, "bin", "git"), filepath.Join(binDir, "git"))
	case "python":
		if runtime.GOOS == "windows" {
			for _, name := range []string{"python.exe", "python3.exe"} {
				src := filepath.Join(target, name)
				if _, err := os.Stat(src); err == nil {
					if err := copyFile(src, filepath.Join(binDir, name)); err != nil {
						return fmt.Errorf("copy %s: %w", name, err)
					}
				}
			}
		} else {
			if err := copyOrSymlink(filepath.Join(target, "bin", "python3"), filepath.Join(binDir, "python3")); err != nil {
				return err
			}
		}
	case "go":
		if runtime.GOOS == "windows" {
			for _, name := range []string{"go.exe", "gofmt.exe"} {
				src := filepath.Join(target, "go", "bin", name)
				if _, err := os.Stat(src); err == nil {
					if err := copyFile(src, filepath.Join(binDir, name)); err != nil {
						return fmt.Errorf("copy %s: %w", name, err)
					}
				}
			}
		} else {
			for _, name := range []string{"go", "gofmt"} {
				src := filepath.Join(target, "go", "bin", name)
				if _, err := os.Stat(src); err == nil {
					if err := copyOrSymlink(src, filepath.Join(binDir, name)); err != nil {
						return err
					}
				}
			}
		}
	default:
		return fmt.Errorf("unsupported runtime for bin setup: %s", rt)
	}

	// 写入版本标记文件 bin/.pvm-<rt>-version
	versionFile := filepath.Join(binDir, fmt.Sprintf(".pvm-%s-version", rt))
	if err := os.WriteFile(versionFile, []byte(version), 0644); err != nil {
		return fmt.Errorf("write version marker: %w", err)
	}

	return nil
}

// gitPvmExeCommands 是通过 pvm.exe 硬链接转发的 Git 命令（在 RuntimeShims["git"] 里，
// pvm 的 FindRuntimeForCommand 能识别并正确解析 git 版本）。
// 这些命令硬链接为 <name>.exe（pvm.exe 副本），运行时按文件名自分发到 pvm shim-exec。
var gitPvmExeCommands = []string{
	"git", "git-lfs",
	"gitk", "git-gui",
	"git-askpass", "git-askyesno", "git-credential-helper-selector",
	"git-http-fetch", "git-http-push", "git-receive-pack", "git-upload-pack",
	// bash/sh：VSCode Git Bash 终端需要 bash.exe，通过 pvm.exe 硬链接转发到 git/bin/bash.exe
	"bash", "sh",
}

// gitDirectCmdCommands 是直接用 .cmd 指向真实 exe 的 Git 工具命令
// （不在 RuntimeShims 里，不能走 pvm 转发，否则会循环或失败）。
// 每项格式：binName -> 相对于 git 安装目录的路径
var gitDirectCmdCommands = map[string]string{
	// Git Bash 启动器
	"git-bash": "git-bash.exe",
	// SSH / 远程操作（usr/bin）
	"ssh":        filepath.Join("usr", "bin", "ssh.exe"),
	"scp":        filepath.Join("usr", "bin", "scp.exe"),
	"sftp":       filepath.Join("usr", "bin", "sftp.exe"),
	"ssh-keygen": filepath.Join("usr", "bin", "ssh-keygen.exe"),
	"ssh-agent":  filepath.Join("usr", "bin", "ssh-agent.exe"),
	"ssh-add":    filepath.Join("usr", "bin", "ssh-add.exe"),
	// 网络工具
	"curl": filepath.Join("usr", "bin", "curl.exe"),
	// 文本处理
	"grep":  filepath.Join("usr", "bin", "grep.exe"),
	"egrep": filepath.Join("usr", "bin", "egrep.exe"),
	"fgrep": filepath.Join("usr", "bin", "fgrep.exe"),
	"sed":   filepath.Join("usr", "bin", "sed.exe"),
	"awk":   filepath.Join("usr", "bin", "awk.exe"),
	"diff":  filepath.Join("usr", "bin", "diff.exe"),
	"sort":  filepath.Join("usr", "bin", "sort.exe"),
	"uniq":  filepath.Join("usr", "bin", "uniq.exe"),
	"wc":    filepath.Join("usr", "bin", "wc.exe"),
	"head":  filepath.Join("usr", "bin", "head.exe"),
	"tail":  filepath.Join("usr", "bin", "tail.exe"),
	"cut":   filepath.Join("usr", "bin", "cut.exe"),
	"tr":    filepath.Join("usr", "bin", "tr.exe"),
	"xargs": filepath.Join("usr", "bin", "xargs.exe"),
	// 文件操作
	"find":  filepath.Join("usr", "bin", "find.exe"),
	"tar":   filepath.Join("usr", "bin", "tar.exe"),
	"zip":   filepath.Join("usr", "bin", "zip.exe"),
	"unzip": filepath.Join("usr", "bin", "unzip.exe"),
	"cat":   filepath.Join("usr", "bin", "cat.exe"),
	"less":  filepath.Join("usr", "bin", "less.exe"),
	// 编辑器
	"vim":  filepath.Join("usr", "bin", "vim.exe"),
	"vi":   filepath.Join("usr", "bin", "vi.exe"),
	"nano": filepath.Join("usr", "bin", "nano.exe"),
	// Unix 兼容工具
	"ls":       filepath.Join("usr", "bin", "ls.exe"),
	"cp":       filepath.Join("usr", "bin", "cp.exe"),
	"mv":       filepath.Join("usr", "bin", "mv.exe"),
	"rm":       filepath.Join("usr", "bin", "rm.exe"),
	"mkdir":    filepath.Join("usr", "bin", "mkdir.exe"),
	"chmod":    filepath.Join("usr", "bin", "chmod.exe"),
	"chown":    filepath.Join("usr", "bin", "chown.exe"),
	"pwd":      filepath.Join("usr", "bin", "pwd.exe"),
	"which":    filepath.Join("usr", "bin", "which.exe"),
	"env":      filepath.Join("usr", "bin", "env.exe"),
	"basename": filepath.Join("usr", "bin", "basename.exe"),
	"dirname":  filepath.Join("usr", "bin", "dirname.exe"),
	"date":     filepath.Join("usr", "bin", "date.exe"),
	"whoami":   filepath.Join("usr", "bin", "whoami.exe"),
}

// setCurrentGitWindows 在 Windows 上为 git 创建 ~/.pvm/bin 下的可执行文件。
//   - pvm.exe 硬链接（git/bash/sh 等）：按文件名自分发到 pvm shim-exec，由 pvm 解析版本并执行
//   - .cmd 直接指向（ssh/curl/vim 等）：避免 pvm 转发循环，直接运行真实 exe
//
// 关键：bash.exe（pvm.exe 硬链接）让 VSCode 能识别 Git Bash 终端。
func setCurrentGitWindows(gitInstallDir, binDir string) error {
	currentDir := filepath.Join(InstallsDir(), "git", "current")
	// current junction 不存在时回退到 gitInstallDir（兼容旧安装）
	if _, err := os.Stat(currentDir); err != nil {
		currentDir = gitInstallDir
	}

	// 1. pvm 副本命令（git/bash/sh 等，硬链接 pvm.exe 自分发转发到 pvm shim-exec）
	shimSrc := resolvePvmExePath()
	if shimSrc != "" {
		for _, cmd := range gitPvmExeCommands {
			// bash/sh 在 bin/，git 在 cmd/，gitk/git-gui 在 cmd/
			// 硬链接 pvm.exe 为 <cmd>.exe，启动时按文件名转发（由 pvm ResolveBinary 查找真实 exe）
			binExe := filepath.Join(binDir, cmd+".exe")
			_ = os.Remove(binExe)
			if err := os.Link(shimSrc, binExe); err != nil {
				// 硬链接失败（跨文件系统等）→ 回退到复制
				_ = copyFile(shimSrc, binExe)
			}
		}
	} else {
		// pvm.exe 不可用，回退到 .cmd 直接指向（至少 git 可用）
		for _, cmd := range gitPvmExeCommands {
			realExe := findGitExe(currentDir, cmd)
			if realExe == "" {
				continue
			}
			content := fmt.Sprintf("@\"%s\" %%*\r\n", realExe)
			_ = os.WriteFile(filepath.Join(binDir, cmd+".cmd"), []byte(content), 0755)
		}
	}

	// 2. .cmd 直接指向命令（ssh/curl/vim 等，不经过 pvm 避免循环）
	for cmd, relPath := range gitDirectCmdCommands {
		realExe := filepath.Join(currentDir, relPath)
		if _, err := os.Stat(realExe); err != nil {
			continue
		}
		// 先尝试清理旧的 .shim 文件（迁移期）
		_ = os.Remove(filepath.Join(binDir, cmd+".shim"))
		content := fmt.Sprintf("@\"%s\" %%*\r\n", realExe)
		_ = os.WriteFile(filepath.Join(binDir, cmd+".cmd"), []byte(content), 0755)
	}

	return nil
}

// findGitExe 在 git 安装目录中查找命令的真实 exe 路径（cmd/ → bin/ → usr/bin/）
func findGitExe(gitDir, cmd string) string {
	candidates := []string{
		filepath.Join(gitDir, "cmd", cmd+".exe"),
		filepath.Join(gitDir, "bin", cmd+".exe"),
		filepath.Join(gitDir, "usr", "bin", cmd+".exe"),
		filepath.Join(gitDir, "mingw64", "bin", cmd+".exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// resolvePvmExePath 获取 pvm 主程序（pvm.exe）的路径（内联实现避免循环依赖）。
// 单二进制方案：git/bash/sh 等命令通过硬链接 pvm.exe 到 ~/.pvm/bin/ 实现自分发转发
// （pvm 启动按自身文件名识别命令，见 cmd.shimExeNameFromArgv0）。
// 优先级: ~/.pvm/bin/pvm.exe → 当前运行的 exe → dist/pvm.exe
func resolvePvmExePath() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	// 1. ~/.pvm/bin/pvm.exe
	pvmBin := filepath.Join(BinHome(), "pvm"+ext)
	if _, err := os.Stat(pvmBin); err == nil {
		return pvmBin
	}
	// 2. 当前运行的 exe（pvm 本体）
	if exe, err := os.Executable(); err == nil {
		if _, err := os.Stat(exe); err == nil {
			return exe
		}
	}
	// 3. dist 目录（开发构建产物）
	devExe := filepath.Join("dist", "pvm"+ext)
	if _, err := os.Stat(devExe); err == nil {
		if abs, err := filepath.Abs(devExe); err == nil {
			return abs
		}
	}
	return ""
}

// copyFile 复制文件（保留权限）
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Windows: 目标可能正在运行，先写 .tmp 再 rename
	tmp := dst + ".tmp"
	dstFile, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		dstFile.Close()
		os.Remove(tmp)
		return err
	}
	dstFile.Close()

	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		// rename 失败，尝试直接写
		os.Remove(tmp)
		dstFile2, err := os.Create(dst)
		if err != nil {
			return err
		}
		srcFile2, err := os.Open(src)
		if err != nil {
			dstFile2.Close()
			return err
		}
		defer srcFile2.Close()
		if _, err := dstFile2.ReadFrom(srcFile2); err != nil {
			dstFile2.Close()
			return err
		}
		dstFile2.Close()
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// copyOrSymlink 优先创建符号链接（Unix），失败则复制文件
func copyOrSymlink(src, dst string) error {
	os.Remove(dst)
	if err := os.Symlink(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

// CurrentVersion 返回 bin 中当前激活的版本号（从版本标记文件读取）
func CurrentVersion(rt string) string {
	versionFile := filepath.Join(BinHome(), fmt.Sprintf(".pvm-%s-version", rt))
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
