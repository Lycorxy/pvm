package shim

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
)

// RuntimeShims 定义每个 runtime 默认需要生成哪些 shim 命令。
// 遵循"稳定子集"原则：只 shim 稳定会存在的可执行文件。
var RuntimeShims = map[string][]string{
	"node": {
		"node", "npm", "npx", "corepack",
		// yarn/yarnpkg 由独立的 yarn runtime 管理（如已安装），但 corepack 也能管它
		"yarn", "yarnpkg",
	},
	"python": {
		"python", "python3", "pip", "pip3",
	},
	"go": {
		"go", "gofmt",
	},
	"pnpm": {
		"pnpm", "pnpx",
	},
	"yarn": {
		"yarn", "yarnpkg",
	},
	"git": {
		// Git for Windows 核心命令
		"git", "git-lfs",
		"git-askpass", "git-askyesno", "git-credential-helper-selector",
		"git-http-fetch", "git-http-push", "git-receive-pack",
		"gitk", "git-gui", "git-upload-pack",
		// Git Bash - VSCode 等 IDE 需要这些来运行 Git Bash 终端
		"bash", "sh",
	},
}

// Reshim 重建 shims 目录：
//  1. 清理旧 shims（含迁移期残留的 .cmd/.ps1/.shim 旧格式）
//  2. 把 pvm-shim 二进制复制（Windows）或符号链接（Unix）为每个命令名
//
// 所有命令统一走 pvm-shim → pvm shim-exec 动态解析版本，
// 不再区分"工具型直接 shim"与"动态 shim"的双轨制。
func Reshim() error {
	shimsDir := config.ShimsDir()
	if err := config.EnsureDir(shimsDir); err != nil {
		return err
	}

	// 定位 pvm-shim 二进制源（reshim 的前提）
	shimSrc := GetPvmShimPath()
	if shimSrc == "" {
		return fmt.Errorf("pvm-shim binary not found — run `pvm setup` first, or build with: go build -o %%~/.pvm/bin/pvm-shim ./cmd/shim")
	}

	// 清理目录中所有旧 shim（只删文件，不删目录）
	// 被其他进程占用的文件跳过，不影响后续生成新 shim
	entries, _ := os.ReadDir(shimsDir)
	removedCount := 0
	skippedCount := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		err := os.Remove(filepath.Join(shimsDir, e.Name()))
		if err != nil {
			skippedCount++
		} else {
			removedCount++
		}
	}
	if removedCount > 0 || skippedCount > 0 {
		logger.Verbose("  → Cleaned %d old shims (%d skipped due to file lock)", removedCount, skippedCount)
	}

	// 收集所有 runtime + 所有已安装版本中出现过的可执行文件名
	commandSet := make(map[string]struct{})

	for _, rt := range config.SupportedRuntimes {
		// 默认 shim
		for _, c := range RuntimeShims[rt] {
			commandSet[c] = struct{}{}
		}
		// 扫描该 runtime 下每个已安装版本的 bin 目录，收集所有可执行文件
		rtDir := filepath.Join(config.InstallsDir(), rt)
		versions, _ := os.ReadDir(rtDir)
		for _, v := range versions {
			if !v.IsDir() || v.Name() == "current" {
				continue
			}
			scanBinDir(config.BinDir(rt, v.Name()), commandSet)
			// Git for Windows: bash.exe/sh.exe 等在 bin/ 和 usr/bin/ 而非 cmd/
			if rt == "git" && runtime.GOOS == "windows" {
				installDir := config.InstallDir(rt, v.Name())
				scanBinDir(filepath.Join(installDir, "bin"), commandSet)
				scanBinDir(filepath.Join(installDir, "usr", "bin"), commandSet)
			}
		}
	}

	// 排序后生成
	cmds := make([]string, 0, len(commandSet))
	for c := range commandSet {
		cmds = append(cmds, c)
	}
	sort.Strings(cmds)

	// 生成 shim，收集被占用的警告，不因单个失败而中断
	var warnings []string
	for _, c := range cmds {
		if err := writeShim(shimsDir, c, shimSrc); err != nil {
			var shimErr *ShimInUseError
			if errors.As(err, &shimErr) {
				warnings = append(warnings, shimErr.Error())
			} else {
				return fmt.Errorf("write shim %s: %w", c, err)
			}
		}
	}
	if len(warnings) > 0 {
		return &ReshimWarning{Messages: warnings}
	}
	return nil
}

// ShimInUseError 表示 shim 文件被其他进程占用，无法覆盖
type ShimInUseError struct {
	File string
	Err  error
}

func (e *ShimInUseError) Error() string {
	return fmt.Sprintf("write shim %s: %v", e.File, e.Err)
}

func (e *ShimInUseError) Unwrap() error { return e.Err }

// ReshimWarning 表示 reshim 完成但有部分 shim 因文件占用被跳过
type ReshimWarning struct {
	Messages []string
}

func (w *ReshimWarning) Error() string {
	return strings.Join(w.Messages, "; ")
}

// IsReshimWarning 判断错误是否为 reshim 警告（非致命）
func IsReshimWarning(err error) bool {
	var w *ReshimWarning
	return errors.As(err, &w)
}

// writeShim 为单个命令生成 shim：把 pvm-shim 二进制复制（Windows）或
// 符号链接（Unix）为 <cmdName>。
//
// Windows 策略（参考 Proto crates/shim/src/windows.rs）：
//   - .exe 是真正的可执行文件，被 VSCode 等 IDE 识别（.cmd 不会被识别）
//   - 覆盖正在运行的 exe：先把旧 exe 重命名为 .previous，再写新 exe，再删 .previous
//   - 文件被占用时返回 ShimInUseError（非致命，下次 reshim 会重试）
//
// Unix 策略：
//   - 符号链接 <cmdName> → pvm-shim，零磁盘占用，升级 pvm-shim 后所有 shim 自动生效
//   - 用 tmp + rename 原子化（避免部分写入）
func writeShim(shimsDir, cmdName, shimSrc string) error {
	if runtime.GOOS == "windows" {
		return writeShimWindows(shimsDir, cmdName, shimSrc)
	}
	return writeShimUnix(shimsDir, cmdName, shimSrc)
}

// writeShimWindows 在 Windows 上把 pvm-shim.exe 复制为 <cmdName>.exe
func writeShimWindows(shimsDir, cmdName, shimSrc string) error {
	target := filepath.Join(shimsDir, cmdName+".exe")

	// 读取 pvm-shim 二进制内容
	data, err := os.ReadFile(shimSrc)
	if err != nil {
		return fmt.Errorf("read pvm-shim source: %w", err)
	}

	// 先写到临时文件，再原子替换目标
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0755); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write tmp shim: %w", err)
	}

	// 尝试直接 rename 覆盖目标
	if err := os.Rename(tmp, target); err == nil {
		return nil
	}

	// rename 失败：目标可能正在运行（Windows 不允许覆盖运行中的 exe）
	// 采用 Proto 方案：先把旧目标重命名为 .previous.exe，再写入新文件
	previous := target + ".previous"
	_ = os.Remove(previous) // 清理上次残留的 .previous
	if renameErr := os.Rename(target, previous); renameErr != nil {
		// 旧文件也无法重命名 → 真正被占用，返回非致命错误
		_ = os.Remove(tmp)
		if isFileBusy(renameErr) {
			return &ShimInUseError{File: target, Err: renameErr}
		}
		return fmt.Errorf("cannot replace shim %s: %w", target, renameErr)
	}

	// 旧文件已让位，写入新文件
	if err := os.Rename(tmp, target); err != nil {
		// 回滚：把旧文件放回去
		_ = os.Rename(previous, target)
		_ = os.Remove(tmp)
		return fmt.Errorf("install new shim: %w", err)
	}

	// 尽力删除 .previous（可能仍被占用，留待下次清理）
	_ = os.Remove(previous)
	return nil
}

// writeShimUnix 在 Unix 上创建符号链接 <cmdName> → pvm-shim
func writeShimUnix(shimsDir, cmdName, shimSrc string) error {
	target := filepath.Join(shimsDir, cmdName)

	// 先写临时 symlink 再 rename，保证原子性
	tmp := target + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(shimSrc, tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("create symlink: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		if isFileBusy(err) {
			return &ShimInUseError{File: target, Err: err}
		}
		return fmt.Errorf("install shim: %w", err)
	}
	return nil
}

// scanBinDir 扫描目录中的可执行文件，将命令名加入 commandSet
func scanBinDir(dir string, commandSet map[string]struct{}) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if runtime.GOOS == "windows" {
			lower := strings.ToLower(name)
			switch {
			case strings.HasSuffix(lower, ".exe"):
				name = name[:len(name)-4]
			case strings.HasSuffix(lower, ".cmd"):
				name = name[:len(name)-4]
			case strings.HasSuffix(lower, ".bat"):
				name = name[:len(name)-4]
			default:
				continue
			}
		} else {
			info, err := f.Info()
			if err != nil {
				continue
			}
			if info.Mode()&0111 == 0 {
				continue
			}
		}
		commandSet[name] = struct{}{}
	}
}

// GetPvmShimPath 定位 pvm-shim 二进制文件的路径。
// 查找顺序：
//  1. ~/.pvm/bin/pvm-shim(.exe) —— 标准安装位置
//  2. 当前 pvm 可执行文件同目录 —— 开发模式 / MSI 安装目录
//  3. 项目 dist 目录 —— 开发构建产物
//  4. 项目 cmd/shim 源码目录的编译产物 —— go build 后的默认位置
func GetPvmShimPath() string {
	ext := config.ExeExt()
	name := "pvm-shim" + ext

	var candidates []string

	// 1. ~/.pvm/bin/pvm-shim
	candidates = append(candidates, filepath.Join(config.BinHome(), name))

	// 2. 当前 exe 同目录
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}

	// 3. dist 目录（开发构建产物）
	candidates = append(candidates, filepath.Join("dist", name))

	// 4. cmd/shim 同目录（go build -o ./cmd/shim/ 的场景）
	candidates = append(candidates, filepath.Join("cmd", "shim", name))

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// isFileBusy 判断错误是否为文件被其他进程占用
func isFileBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "being used by another process") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "the process cannot access")
}

// FindRuntimeForCommand 根据命令名反查属于哪个 runtime
// 例如 "npm" -> "node", "pip" -> "python"
// 策略：
//  1. 先查 RuntimeShims 显式定义（按 SupportedRuntimes 顺序，确保 pnpm 优先于 node）
//  2. 再遍历已安装版本的 bin 目录查找谁提供了该命令
func FindRuntimeForCommand(cmdName string) string {
	// 1) 显式清单：按 SupportedRuntimes 顺序遍历，避免 map 随机顺序导致 pnpm 被误判为 node
	for _, rt := range config.SupportedRuntimes {
		cmds, ok := RuntimeShims[rt]
		if !ok {
			continue
		}
		for _, c := range cmds {
			if c == cmdName {
				return rt
			}
		}
	}

	// 2) 扫描 installs
	for _, rt := range config.SupportedRuntimes {
		rtDir := filepath.Join(config.InstallsDir(), rt)
		versions, _ := os.ReadDir(rtDir)
		for _, v := range versions {
			if !v.IsDir() || v.Name() == "current" {
				continue
			}
			bin := config.BinDir(rt, v.Name())
			if hasExecutable(bin, cmdName) {
				return rt
			}
		}
	}
	return ""
}

// hasExecutable 判断 bin 目录下是否存在 name 命令（考虑 Windows .exe 等后缀）
func hasExecutable(dir, name string) bool {
	candidates := []string{name}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, name+".exe", name+".cmd", name+".bat")
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			return true
		}
	}
	return false
}

// ResolveBinary 根据 runtime + 版本 + 命令名，解析出真实二进制的绝对路径
func ResolveBinary(rt, version, cmdName string) (string, error) {
	bin := config.BinDir(rt, version)

	// 构建搜索目录列表
	searchDirs := []string{bin}

	// Git for Windows 特殊处理：bash.exe、sh.exe 等在 bin/ 目录而非 cmd/ 目录
	if rt == "git" && runtime.GOOS == "windows" {
		installDir := config.InstallDir(rt, version)
		gitBin := filepath.Join(installDir, "bin")
		if gitBin != bin {
			searchDirs = append(searchDirs, gitBin)
		}
		gitUsrBin := filepath.Join(installDir, "usr", "bin")
		searchDirs = append(searchDirs, gitUsrBin)
	}

	candidates := []string{cmdName}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, cmdName+".exe", cmdName+".cmd", cmdName+".bat")
	}

	for _, dir := range searchDirs {
		for _, c := range candidates {
			p := filepath.Join(dir, c)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("command %q not found in %s %s (looked in %v)", cmdName, rt, version, searchDirs)
}

// CleanupOrphanedShims 清理孤立的 shim 文件（对应的 runtime 版本已卸载）
func CleanupOrphanedShims() error {
	shimsDir := config.ShimsDir()
	entries, err := os.ReadDir(shimsDir)
	if err != nil {
		return err
	}

	cleaned := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !isShimNeeded(e.Name()) {
			shimPath := filepath.Join(shimsDir, e.Name())
			if err := os.Remove(shimPath); err == nil {
				cleaned++
			}
		}
	}
	if cleaned > 0 {
		logger.Verbose("  → Cleaned %d orphaned shims", cleaned)
	}
	return nil
}

// isShimNeeded 检查某个 shim 是否还有对应的已安装版本
func isShimNeeded(shimName string) bool {
	cmdName := shimName
	if runtime.GOOS == "windows" {
		cmdName = strings.TrimSuffix(cmdName, ".exe")
		cmdName = strings.TrimSuffix(cmdName, ".previous")
		cmdName = strings.TrimSuffix(cmdName, ".tmp")
		cmdName = strings.TrimSuffix(cmdName, ".cmd")
		cmdName = strings.TrimSuffix(cmdName, ".bat")
	}

	// 遍历所有已安装的 runtime 版本，检查是否有这个命令
	for _, rt := range config.SupportedRuntimes {
		rtDir := filepath.Join(config.InstallsDir(), rt)
		versions, err := os.ReadDir(rtDir)
		if err != nil {
			continue
		}
		for _, v := range versions {
			if !v.IsDir() || v.Name() == "current" {
				continue
			}
			binDir := config.BinDir(rt, v.Name())
			if runtime.GOOS == "windows" {
				if _, err := os.Stat(filepath.Join(binDir, cmdName+".exe")); err == nil {
					return true
				}
				if _, err := os.Stat(filepath.Join(binDir, cmdName+".cmd")); err == nil {
					return true
				}
			} else {
				if _, err := os.Stat(filepath.Join(binDir, cmdName)); err == nil {
					return true
				}
			}
			// Git for Windows: bash.exe/sh.exe 等在 bin/ 和 usr/bin/ 而非 cmd/
			if rt == "git" && runtime.GOOS == "windows" {
				installDir := config.InstallDir(rt, v.Name())
				extraDirs := []string{
					filepath.Join(installDir, "bin"),
					filepath.Join(installDir, "usr", "bin"),
				}
				for _, d := range extraDirs {
					if _, err := os.Stat(filepath.Join(d, cmdName+".exe")); err == nil {
						return true
					}
				}
			}
		}
	}
	return false
}
