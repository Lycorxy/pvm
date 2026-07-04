package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/config"
)

// uninstallOps 定义平台相关的卸载操作接口
// 主流程通过此接口调用，避免在业务逻辑中散布 runtime.GOOS 判断
// 具体实现由 uninstall_windows.go / uninstall_unix.go 提供
type uninstallOps interface {
	// killProcesses 检测并终止 pvm 相关进程，返回 true 表示可以继续卸载
	killProcesses() bool
	// removeFromPath 从 PATH 中移除 pvm 相关条目
	removeFromPath(binHome, shimsDir string) error
	// removePvmHome 清除 PVM_HOME 环境变量
	removePvmHome()
	// cleanPlatformSpecific 清理平台特有的残留物（注册表、开始菜单、MSI 等）
	cleanPlatformSpecific()
	// findExtraInstallDirs 查找额外可能存在的 pvm 安装目录
	findExtraInstallDirs(pvmHome string) []string
	// cleanupExtraInstallDir 清理额外的安装目录（可能需要延迟删除）
	cleanupExtraInstallDir(dir, currentExe string)
	// scheduleDelayedCleanup 处理当前 exe 在安装目录内时的延迟删除
	scheduleDelayedCleanup(currentExe, pvmHome string)
	// displayRemovalPlan 显示平台特有的卸载计划信息
	displayRemovalPlan(pvmHome string, extraDirs []string)
	// displayFinalMessage 显示平台特有的最终提示
	displayFinalMessage()
}

// runSelfUninstall 彻底卸载 pvm 自身及其管理的所有内容：
//  1. 列出所有已安装的 runtime 和版本、用户配置
//  2. 检测并终止可能阻塞卸载的进程
//  3. 从 PATH 中移除 pvm 条目
//  4. 清除 PVM_HOME 环境变量
//  5. 卸载 MSI 注册的产品（Windows）
//  6. 删除所有 pvm 安装目录
//  7. 清理平台特有的残留物（注册表、开始菜单等）
//  8. 提示用户手动清理项目 .pvmrc 文件（可选）
func runSelfUninstall(args []string) error {
	// 检查 --yes / -y 跳过确认
	_, yes := hasFlag(args, "--yes", "-y")
	// 检查 --purge-rc：扫描并删除磁盘上所有 .pvmrc 项目配置
	_, purgeRC := hasFlag(args, "--purge-rc")

	ops := newPlatformOps()
	pvmHome := config.PvmHome()
	binHome := config.BinHome()
	shimsDir := config.ShimsDir()

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║           pvm uninstall                  ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()

	// ── 列出所有已安装的 runtime 版本 ──
	installedRuntimes := listInstalledRuntimes()
	if len(installedRuntimes) > 0 {
		fmt.Println("  Installed runtimes to be removed:")
		for _, item := range installedRuntimes {
			fmt.Printf("    • %s\n", item)
		}
		fmt.Println()
	}

	// ── 列出用户全局配置内容 ──
	globalVersions := listGlobalVersions()
	if len(globalVersions) > 0 {
		fmt.Println("  User config (~/.pvm/versions) to be removed:")
		for _, item := range globalVersions {
			fmt.Printf("    • %s\n", item)
		}
		fmt.Println()
	}

	// ── 扫描磁盘上的 .pvmrc 文件（仅在 --purge-rc 时列出） ──
	var rcFiles []string
	if purgeRC {
		rcFiles = scanProjectRcFiles()
		if len(rcFiles) > 0 {
			fmt.Println("  Project .pvmrc files to be removed (--purge-rc):")
			for _, f := range rcFiles {
				fmt.Printf("    • %s\n", f)
			}
			fmt.Println()
		}
	}

	// ── 检测额外的安装目录 ──
	extraDirs := ops.findExtraInstallDirs(pvmHome)

	fmt.Printf("  This will remove:\n")
	fmt.Printf("    • %s\n", pvmHome)
	fmt.Printf("      ├─ bin/        (pvm executable)\n")
	fmt.Printf("      ├─ shims/      (shim scripts for node, go, python, pnpm...)\n")
	fmt.Printf("      ├─ installs/   (all installed runtimes)\n")
	fmt.Printf("      ├─ versions    (user default version config)\n")
	fmt.Printf("      ├─ cache/      (download cache)\n")
	fmt.Printf("      └─ tmp/        (temporary files)\n")
	ops.displayRemovalPlan(pvmHome, extraDirs)
	fmt.Printf("    • PVM_HOME environment variable\n")
	if purgeRC && len(rcFiles) > 0 {
		fmt.Printf("    • %d project .pvmrc file(s)\n", len(rcFiles))
	}
	fmt.Println()
	if !purgeRC {
		fmt.Println("  Note: Project-level .pvmrc files are NOT removed.")
		fmt.Println("        To also remove them, re-run with: pvm uninstall --purge-rc")
		fmt.Println()
	}

	if !yes {
		fmt.Print("  Are you sure? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("  Aborted.")
			return nil
		}
		fmt.Println()
	}

	// ---- 1. 检测并终止可能阻塞卸载的进程 ----
	fmt.Println("  → Checking for running pvm processes...")
	if !ops.killProcesses() {
		return nil
	}
	time.Sleep(500 * time.Millisecond)

	// ---- 2. 从 PATH 中移除 pvm 条目 ----
	fmt.Println("  → Removing pvm from PATH...")
	if err := ops.removeFromPath(binHome, shimsDir); err != nil {
		fmt.Printf("  ⚠ Could not update PATH: %v\n", err)
	} else {
		fmt.Println("  ✓ Removed from PATH")
	}

	// ---- 3. 清理平台特有内容（MSI 产品、注册表等）----
	ops.cleanPlatformSpecific()

	// ---- 4. 清除 PVM_HOME 环境变量 ----
	fmt.Println("  → Clearing PVM_HOME...")
	ops.removePvmHome()

	// ---- 5. 删除额外的安装目录 ----
	currentExe, _ := os.Executable()
	currentExe = filepath.Clean(currentExe)
	for _, dir := range extraDirs {
		if strings.EqualFold(dir, pvmHome) {
			continue
		}
		ops.cleanupExtraInstallDir(dir, currentExe)
	}

	// ---- 6. 删除 ~/.pvm 目录 ----
	fmt.Printf("  → Removing %s ...\n", pvmHome)

	currentExeUnderPvmHome := strings.HasPrefix(
		strings.ToLower(currentExe)+string(filepath.Separator),
		strings.ToLower(pvmHome)+string(filepath.Separator),
	)

	if currentExeUnderPvmHome {
		// 当前 exe 在 pvmHome 内，删除除当前 exe 之外的所有内容
		partialErrs := removeExceptBestEffort(pvmHome, currentExe)
		if len(partialErrs) > 0 {
			fmt.Println("  ⚠ Some files are in use and will be removed after exit:")
			for _, e := range partialErrs {
				fmt.Printf("    - %v\n", e)
			}
		} else {
			fmt.Println("  ✓ Removed all pvm data (runtimes, shims, cache)")
		}

		// 注册延迟删除：等当前进程退出后删除 exe 及整个 pvmHome
		ops.scheduleDelayedCleanup(currentExe, pvmHome)

		fmt.Println()
		fmt.Println("  ✓ pvm.exe will be automatically deleted after this process exits.")
	} else {
		// 当前 exe 不在 pvmHome 内，直接删除整个目录
		partialErrs := removeAllBestEffort(pvmHome)
		if len(partialErrs) > 0 {
			fmt.Println("  ⚠ Some files are in use and will be removed after exit:")
			for _, e := range partialErrs {
				fmt.Printf("    - %v\n", e)
			}
			// 启动延迟清理
			ops.scheduleDelayedCleanup("", pvmHome)
		} else {
			fmt.Printf("  ✓ Removed %s (runtimes, shims, cache, tmp)\n", pvmHome)
		}
	}

	// ---- 7. 删除项目级 .pvmrc（仅当 --purge-rc）----
	if purgeRC && len(rcFiles) > 0 {
		fmt.Printf("  → Removing %d project .pvmrc file(s)...\n", len(rcFiles))
		removed := 0
		for _, f := range rcFiles {
			if err := os.Remove(f); err != nil {
				fmt.Printf("    ⚠ %s: %v\n", f, err)
			} else {
				removed++
			}
		}
		fmt.Printf("  ✓ Removed %d/%d .pvmrc file(s)\n", removed, len(rcFiles))
	}

	fmt.Println()
	fmt.Println("  ════════════════════════════════════════════")
	fmt.Println("  ✓ pvm has been uninstalled.")
	fmt.Println("  ════════════════════════════════════════════")
	fmt.Println()
	ops.displayFinalMessage()
	fmt.Println()
	fmt.Println("  Note: If 'node -v' or other commands still work,")
	fmt.Println("  it means the runtime was installed system-wide (not by pvm).")
	fmt.Println()
	return nil
}

// removePvmBlock 从 shell rc 内容中移除 pvm 配置块
// 精确匹配 setup.go 写入的注释 "# pvm (Polyglot Version Manager)"，
// 避免误伤用户自己写的 # pvm 相关注释
// 此函数被所有平台共享使用
func removePvmBlock(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 精确匹配 pvm 配置块的起始注释
		if strings.HasPrefix(trimmed, "# pvm (Polyglot Version Manager)") {
			skip = true
		}
		if skip {
			// 空行表示块结束
			if trimmed == "" {
				skip = false
				// 不输出这个空行（吸收掉块后的空行）
				continue
			}
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// scanProjectRcFiles 扫描用户主目录下常见的项目目录，列出所有 .pvmrc 文件
// 仅扫描有限的几层深度，避免遍历整个磁盘
func scanProjectRcFiles() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	roots := []string{
		homeDir,
		filepath.Join(homeDir, "Documents"),
		filepath.Join(homeDir, "Desktop"),
		filepath.Join(homeDir, "Projects"),
		filepath.Join(homeDir, "workspace"),
		filepath.Join(homeDir, "code"),
		filepath.Join(homeDir, "src"),
	}
	const maxDepth = 4

	seen := make(map[string]bool)
	var found []string
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		walkRcFiles(root, root, 0, maxDepth, seen, &found)
	}
	return found
}

// walkRcFiles 受限深度遍历查找 .pvmrc 文件
func walkRcFiles(root, dir string, depth, maxDepth int, seen map[string]bool, out *[]string) {
	if depth > maxDepth {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			lower := strings.ToLower(name)
			if lower == "node_modules" || lower == ".git" || lower == ".pvm" ||
				lower == "dist" || lower == "build" || lower == "target" ||
				lower == "vendor" || (strings.HasPrefix(name, ".") && lower != ".config") {
				continue
			}
			walkRcFiles(root, filepath.Join(dir, name), depth+1, maxDepth, seen, out)
			continue
		}
		if name == ".pvmrc" {
			full := filepath.Join(dir, name)
			if !seen[full] {
				seen[full] = true
				*out = append(*out, full)
			}
		}
	}
}

// removeExceptBestEffort 删除 root 目录下除 exceptFile 所在目录链之外的所有内容。
// 采用尽力删除策略：
//  1. 先尝试整体删除（快速路径）
//  2. 遇到被占用的文件/目录，递归进入逐文件删除，跳过被占用的，继续删其他
//  3. 返回所有无法删除的文件路径（供调用方提示用户）
func removeExceptBestEffort(root, exceptFile string) []error {
	var errs []error
	entries, err := os.ReadDir(root)
	if err != nil {
		return []error{err}
	}
	for _, entry := range entries {
		fullPath := filepath.Join(root, entry.Name())
		clean := filepath.Clean(fullPath)
		exceptClean := filepath.Clean(exceptFile)
		if strings.EqualFold(clean, exceptClean) {
			continue
		}
		if strings.HasPrefix(strings.ToLower(exceptClean+string(filepath.Separator)),
			strings.ToLower(clean+string(filepath.Separator))) {
			continue
		}
		if err := os.RemoveAll(fullPath); err != nil {
			subErrs := removeAllBestEffort(fullPath)
			errs = append(errs, subErrs...)
		}
	}
	return errs
}

// removeAllBestEffort 递归删除目录，跳过被占用的文件，返回所有无法删除的文件错误
func removeAllBestEffort(path string) []error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []error{fmt.Errorf("unlinkat %s: %w", path, err)}
	}

	if !info.IsDir() {
		if err := os.Remove(path); err != nil {
			return []error{fmt.Errorf("unlinkat %s: %w", path, err)}
		}
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return []error{fmt.Errorf("unlinkat %s: %w", path, err)}
	}

	var errs []error
	for _, entry := range entries {
		subErrs := removeAllBestEffort(filepath.Join(path, entry.Name()))
		errs = append(errs, subErrs...)
	}

	if len(errs) == 0 {
		if err := os.Remove(path); err != nil {
			errs = append(errs, fmt.Errorf("unlinkat %s: %w", path, err))
		}
	}
	return errs
}

// listInstalledRuntimes 列出 ~/.pvm/installs/ 下所有已安装的 runtime 和版本
func listInstalledRuntimes() []string {
	installsDir := config.InstallsDir()
	rtDirs, err := os.ReadDir(installsDir)
	if err != nil {
		return nil
	}

	var items []string
	for _, rtDir := range rtDirs {
		if !rtDir.IsDir() {
			continue
		}
		rt := rtDir.Name()
		verDirs, err := os.ReadDir(filepath.Join(installsDir, rt))
		if err != nil {
			continue
		}
		for _, verDir := range verDirs {
			if !verDir.IsDir() {
				continue
			}
			if strings.HasSuffix(verDir.Name(), ".tmp") {
				continue
			}
			items = append(items, fmt.Sprintf("%s@%s", rt, verDir.Name()))
		}
	}
	return items
}

// listGlobalVersions 读取 ~/.pvm/versions 中的用户全局默认版本配置
func listGlobalVersions() []string {
	globalFile := config.GlobalVersionsFile()
	vf, err := config.LoadVersionFile(globalFile)
	if err != nil || vf == nil {
		return nil
	}

	var items []string
	for _, rt := range config.SupportedRuntimes {
		if v, ok := vf.Versions[rt]; ok && v != "" {
			items = append(items, fmt.Sprintf("%s → %s", rt, v))
		}
	}
	return items
}
