//go:build !windows
// +build !windows

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/config"
)

// unixOps 实现 Unix/macOS 平台的卸载操作
type unixOps struct{}

func newPlatformOps() uninstallOps {
	return &unixOps{}
}

// killProcesses 检测并终止 pvm 相关进程，返回 true 表示可以继续卸载
func (u *unixOps) killProcesses() bool {
	currentPid := os.Getpid()
	pvmHome := config.PvmHome()

	// 使用 pgrep 查找从 ~/.pvm/ 目录运行的进程
	type runningProc struct {
		name string
		pid  string
	}
	var runningProcs []runningProc
	seen := make(map[string]bool)

	// 方法一：使用 pgrep 查找进程路径包含 pvmHome 的进程
	processNames := []string{
		"node", "go", "python", "python3",
		"pnpm", "npm", "npx", "yarn", "bun",
		"bash", "sh", "zsh", "fish",
		"pvm",
	}

	for _, procName := range processNames {
		// pgrep 按名称查找
		out, err := exec.Command("pgrep", "-a", procName).Output()
		if err != nil {
			continue // pgrep 返回非零表示没有匹配
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// pgrep -a 输出格式: "PID command args"
			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 2 {
				continue
			}
			pid := parts[0]
			cmdLine := parts[1]

			// 跳过自身
			if pid == fmt.Sprintf("%d", currentPid) {
				continue
			}

			// 只关注从 pvmHome 目录运行的进程
			if !strings.Contains(cmdLine, pvmHome) && !strings.Contains(cmdLine, ".pvm/") {
				continue
			}

			key := procName + ":" + pid
			if !seen[key] {
				seen[key] = true
				runningProcs = append(runningProcs, runningProc{name: procName, pid: pid})
			}
		}
	}

	// 方法二：使用 ps aux 查找
	out, err := exec.Command("ps", "aux").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "USER") {
				continue
			}
			// 跳过自身
			if strings.Contains(line, fmt.Sprintf(" %d ", currentPid)) {
				continue
			}
			// 查找包含 pvmHome 或 .pvm/ 的进程行
			if !strings.Contains(line, pvmHome) && !strings.Contains(line, ".pvm/") {
				continue
			}
			// 提取 PID（ps aux 第二列）
			fields := strings.Fields(line)
			if len(fields) < 11 {
				continue
			}
			pid := fields[1]
			procName := filepath.Base(fields[10])
			key := procName + ":" + pid
			if !seen[key] {
				seen[key] = true
				runningProcs = append(runningProcs, runningProc{name: procName, pid: pid})
			}
		}
	}

	// 没有运行中的进程，直接返回 true
	if len(runningProcs) == 0 {
		fmt.Println("  ✓ No pvm or runtime processes running")
		return true
	}

	// 按进程名分组展示
	type groupedProc struct {
		name string
		pids []string
	}
	groupMap := make(map[string]*groupedProc)
	var groupOrder []string
	for _, rp := range runningProcs {
		if _, ok := groupMap[rp.name]; !ok {
			groupMap[rp.name] = &groupedProc{name: rp.name}
			groupOrder = append(groupOrder, rp.name)
		}
		groupMap[rp.name].pids = append(groupMap[rp.name].pids, rp.pid)
	}

	fmt.Println()
	fmt.Println("  ⚠ The following processes are running from pvm directories and may block uninstall:")
	fmt.Println()
	for _, name := range groupOrder {
		gp := groupMap[name]
		fmt.Printf("    - %-24s (PID: %s)\n", gp.name, strings.Join(gp.pids, ", "))
	}
	fmt.Println()
	fmt.Println("  These processes need to be terminated to complete uninstall.")
	fmt.Print("  Terminate these processes and continue? [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println()
		fmt.Println("  Aborted. Please close the listed processes manually and run uninstall again.")
		return false
	}
	fmt.Println()

	// 终止所有检测到的进程
	fmt.Println("  → Terminating processes...")
	terminatedAny := false
	for _, rp := range runningProcs {
		// 先发送 SIGTERM
		cmd := exec.Command("kill", rp.pid)
		if err := cmd.Run(); err != nil {
			// SIGTERM 失败，尝试 SIGKILL
			cmd2 := exec.Command("kill", "-9", rp.pid)
			if err2 := cmd2.Run(); err2 != nil {
				fmt.Printf("    ⚠ Failed to kill %s (PID %s): %v\n", rp.name, rp.pid, err2)
				continue
			}
		}
		terminatedAny = true
		fmt.Printf("    ✓ Killed %s (PID %s)\n", rp.name, rp.pid)
	}

	if terminatedAny {
		fmt.Println("  → Waiting for processes to fully exit...")
		time.Sleep(1500 * time.Millisecond)
		fmt.Println("  ✓ All blocking processes terminated")
	}

	return true
}

// removeFromPath 从 PATH 中移除 pvm 相关条目
// 清理所有 shell rc 文件（不仅仅当前 shell），因为用户可能有多个 shell 配置
func (u *unixOps) removeFromPath(binHome, shimsDir string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}

	// 检测所有可能存在的 shell rc 文件（不仅仅是当前 shell）
	// 覆盖 bash、zsh、fish、csh/tcsh 等所有常见 shell
	rcFiles := []struct {
		relPath string
		shell   string // "bash", "zsh", "fish", "csh", or "generic"
	}{
		// bash
		{".bashrc", "bash"},
		{".bash_profile", "bash"},
		// zsh
		{".zshrc", "zsh"},
		{".zprofile", "zsh"},
		{".zshenv", "zsh"},
		// fish
		{".config/fish/config.fish", "fish"},
		// csh/tcsh
		{".cshrc", "csh"},
		{".tcshrc", "csh"},
		// 通用
		{".profile", "generic"},
	}

	for _, rc := range rcFiles {
		rcPath := filepath.Join(homeDir, rc.relPath)
		data, err := os.ReadFile(rcPath)
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(content, "PVM_HOME") && !strings.Contains(content, "pvm") {
			continue
		}

		// 移除 pvm 配置块（使用 uninstall_self.go 中的 removePvmBlock）
		cleaned := removePvmBlock(content)
		// 同时清理独立 PATH 条目
		cleaned = removePvmPathEntries(cleaned)
		// 清理独立 PVM_HOME export 行
		cleaned = removePvmHomeExports(cleaned)

		if cleaned != content {
			if err := os.WriteFile(rcPath, []byte(cleaned), 0644); err != nil {
				fmt.Printf("  ⚠ Cannot update %s: %v\n", rcPath, err)
				continue
			}
			fmt.Printf("  ✓ Cleaned ~/%s\n", rc.relPath)
		}
	}

	return nil
}

// removePvmHome 清除 PVM_HOME 环境变量
func (u *unixOps) removePvmHome() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}

	// 清理所有 shell rc 文件中的 PVM_HOME（removeFromPath 已处理大部分，
	// 这里处理残留的独立 export 行和 profile 级别配置）

	// 清理 ~/.profile
	profilePath := filepath.Join(homeDir, ".profile")
	if data, err := os.ReadFile(profilePath); err == nil {
		content := string(data)
		cleaned := removePvmHomeExports(content)
		if cleaned != content {
			os.WriteFile(profilePath, []byte(cleaned), 0644)
		}
	}

	// 清理 /etc/profile.d/pvm.sh（需要 sudo，尽力尝试）
	pvmSh := "/etc/profile.d/pvm.sh"
	if _, err := os.Stat(pvmSh); err == nil {
		err := os.Remove(pvmSh)
		if err != nil {
			fmt.Printf("  ⚠ Cannot remove %s (may require sudo): %v\n", pvmSh, err)
			fmt.Printf("    Run: sudo rm %s\n", pvmSh)
		} else {
			fmt.Printf("  ✓ Removed %s\n", pvmSh)
		}
	}

	// macOS: 清理 launchd plist（如果 pvm 曾安装过 plist）
	if runtime.GOOS == "darwin" {
		plistPaths := []string{
			filepath.Join(homeDir, "Library/LaunchAgents/com.pvm.plist"),
			filepath.Join(homeDir, "Library/LaunchAgents/io.pvm.plist"),
		}
		for _, plist := range plistPaths {
			if _, err := os.Stat(plist); err == nil {
				// 先 unload
				exec.Command("launchctl", "unload", plist).Run()
				err := os.Remove(plist)
				if err != nil {
					fmt.Printf("  ⚠ Cannot remove %s: %v\n", plist, err)
				} else {
					fmt.Printf("  ✓ Removed %s\n", plist)
				}
			}
		}
	}
}

// cleanPlatformSpecific 清理平台特有的残留物
func (u *unixOps) cleanPlatformSpecific() {
	// Linux: 清理 /etc/profile.d/pvm.sh
	if runtime.GOOS == "linux" {
		pvmSh := "/etc/profile.d/pvm.sh"
		if _, err := os.Stat(pvmSh); err == nil {
			err := os.Remove(pvmSh)
			if err != nil {
				fmt.Printf("  ⚠ Cannot remove %s (may require sudo): %v\n", pvmSh, err)
			} else {
				fmt.Printf("  ✓ Removed %s\n", pvmSh)
			}
		}
	}

	// macOS: 清理 launchd plist
	if runtime.GOOS == "darwin" {
		homeDir, _ := os.UserHomeDir()
		if homeDir == "" {
			homeDir = os.Getenv("HOME")
		}
		plistPaths := []string{
			filepath.Join(homeDir, "Library/LaunchAgents/com.pvm.plist"),
			filepath.Join(homeDir, "Library/LaunchAgents/io.pvm.plist"),
		}
		for _, plist := range plistPaths {
			if _, err := os.Stat(plist); err == nil {
				exec.Command("launchctl", "unload", plist).Run()
				os.Remove(plist)
			}
		}
	}
}

// findExtraInstallDirs 查找额外可能存在的 pvm 安装目录
func (u *unixOps) findExtraInstallDirs(pvmHome string) []string {
	var dirs []string

	// 检查常见 Unix 安装位置
	checkPaths := []string{
		"/usr/local/pvm",
		"/opt/pvm",
		"/usr/local/bin/pvm", // 如果 pvm 二进制作为 symlink 在这里
	}

	// 检查 /etc/paths.d/ 中包含 pvm 的条目（macOS）
	pathsDEntries, _ := os.ReadDir("/etc/paths.d")
	for _, entry := range pathsDEntries {
		if strings.Contains(strings.ToLower(entry.Name()), "pvm") {
			fullPath := filepath.Join("/etc/paths.d", entry.Name())
			dirs = append(dirs, fullPath)
		}
	}

	for _, p := range checkPaths {
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		// 如果是符号链接，解析目标
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err == nil {
				// 如果符号链接指向 pvmHome 内
				if strings.Contains(target, pvmHome) || strings.Contains(target, ".pvm") {
					dirs = append(dirs, p)
					continue
				}
			}
		}
		// 如果是目录或文件，加入列表
		if _, err := os.Stat(p); err == nil {
			// 跳过 pvmHome 本身
			if p != pvmHome {
				dirs = append(dirs, p)
			}
		}
	}

	// 去重
	return uniqueStrings(dirs)
}

// cleanupExtraInstallDir 清理额外的安装目录
func (u *unixOps) cleanupExtraInstallDir(dir, currentExe string) {
	info, err := os.Lstat(dir)
	if err != nil {
		return
	}

	// 如果是符号链接，直接删除
	if info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(dir); err != nil {
			fmt.Printf("  ⚠ Cannot remove symlink %s: %v\n", dir, err)
		} else {
			fmt.Printf("  ✓ Removed symlink %s\n", dir)
		}
		return
	}

	// 如果是文件（如 /etc/paths.d/pvm），直接删除
	if !info.IsDir() {
		if err := os.Remove(dir); err != nil {
			fmt.Printf("  ⚠ Cannot remove %s: %v\n", dir, err)
		} else {
			fmt.Printf("  ✓ Removed %s\n", dir)
		}
		return
	}

	// 如果是目录，检查当前 exe 是否在其中
	absCurrentExe, _ := filepath.Abs(currentExe)
	absDir, _ := filepath.Abs(dir)
	if strings.HasPrefix(absCurrentExe+string(filepath.Separator), absDir+string(filepath.Separator)) {
		// 当前 exe 在目录内，需要延迟删除
		u.scheduleDelayedCleanup(currentExe, dir)
		return
	}

	// 直接删除目录
	errs := removeAllBestEffort(dir)
	if len(errs) > 0 {
		fmt.Printf("  ⚠ Some files in %s could not be removed:\n", dir)
		for _, e := range errs {
			fmt.Printf("    - %v\n", e)
		}
	} else {
		fmt.Printf("  ✓ Removed %s\n", dir)
	}
}

// scheduleDelayedCleanup 处理当前 exe 在安装目录内时的延迟删除
func (u *unixOps) scheduleDelayedCleanup(currentExe, pvmHome string) {
	// 使用后台 shell 进程在当前进程退出后删除
	// (sleep 0.5; rm -f /path/to/exe; rm -rf /path/to/pvmHome) &
	script := fmt.Sprintf("(sleep 0.5; rm -f '%s'; rm -rf '%s') &",
		strings.ReplaceAll(currentExe, "'", "'\\''"),
		strings.ReplaceAll(pvmHome, "'", "'\\''"),
	)
	exec.Command("sh", "-c", script).Start()
	fmt.Println("  ✓ pvm files will be automatically deleted after this process exits.")
}

// displayRemovalPlan 显示平台特有的卸载计划信息
func (u *unixOps) displayRemovalPlan(pvmHome string, extraDirs []string) {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = os.Getenv("HOME")
	}

	fmt.Println("  Shell rc files that will be cleaned:")
	rcFiles := []string{
		".bashrc",
		".bash_profile",
		".zshrc",
		".zprofile",
		".zshenv",
		".config/fish/config.fish",
		".cshrc",
		".tcshrc",
		".profile",
	}
	for _, rc := range rcFiles {
		rcPath := filepath.Join(homeDir, rc)
		data, err := os.ReadFile(rcPath)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "PVM_HOME") || strings.Contains(content, "pvm") {
			fmt.Printf("    • ~/%s\n", rc)
		}
	}

	// 显示系统级配置文件
	pvmSh := "/etc/profile.d/pvm.sh"
	if _, err := os.Stat(pvmSh); err == nil {
		fmt.Printf("    • %s\n", pvmSh)
	}

	// macOS launchd plist
	if runtime.GOOS == "darwin" {
		plistPaths := []string{
			filepath.Join(homeDir, "Library/LaunchAgents/com.pvm.plist"),
			filepath.Join(homeDir, "Library/LaunchAgents/io.pvm.plist"),
		}
		for _, plist := range plistPaths {
			if _, err := os.Stat(plist); err == nil {
				fmt.Printf("    • %s\n", plist)
			}
		}
	}

	// 显示额外安装目录
	if len(extraDirs) > 0 {
		fmt.Println("  Extra pvm install locations:")
		for _, dir := range extraDirs {
			fmt.Printf("    • %s\n", dir)
		}
	}
}

// displayFinalMessage 显示平台特有的最终提示
func (u *unixOps) displayFinalMessage() {
	fmt.Println("  Please restart your terminal to complete removal.")
	fmt.Println()
	fmt.Println("  To verify pvm is gone:")
	fmt.Println("    1. Open a NEW terminal")
	fmt.Println("    2. Run: pvm")
	fmt.Println("    3. It should say 'command not found' or 'pvm: not found'")
	fmt.Println()
	fmt.Println("  If pvm is still found, check your shell rc files for")
	fmt.Println("  any remaining pvm entries and remove them manually:")
	fmt.Println("    ~/.bashrc, ~/.zshrc, ~/.config/fish/config.fish, ~/.profile, ~/.zprofile, ~/.cshrc")
	fmt.Println()
	fmt.Println("  Note: If 'node -v' or other commands still work,")
	fmt.Println("  it means the runtime was installed system-wide (not by pvm).")
}

// ──────────────────────────────────────────────────────────────────────────────
//  Unix 特有的辅助函数
// ──────────────────────────────────────────────────────────────────────────────

// removePvmPathEntries 移除 shell rc 内容中独立的包含 pvm 路径的 PATH 条目行
// 增强处理：即使没有 pvm 配置块标记，也能清理独立 PATH 行
func removePvmPathEntries(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 保留空行和注释
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
			continue
		}
		// 检查 export PATH= 行中是否包含 .pvm 或 /pvm/
		if strings.HasPrefix(trimmed, "export PATH=") {
			if strings.Contains(trimmed, ".pvm") || strings.Contains(trimmed, "/pvm/") {
				continue
			}
		}
		// 检查 fish set -gx PATH 行
		if strings.HasPrefix(trimmed, "set -gx PATH") {
			if strings.Contains(trimmed, ".pvm") || strings.Contains(trimmed, "/pvm/") {
				continue
			}
		}
		// 检查 PATH= 赋值行（不含 export）
		if strings.HasPrefix(trimmed, "PATH=") {
			if strings.Contains(trimmed, ".pvm") || strings.Contains(trimmed, "/pvm/") {
				continue
			}
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// removePvmHomeExports 移除 shell rc 内容中独立的 PVM_HOME export/set 行
// 增强处理：即使没有 pvm 配置块标记，也能清理独立的 PVM_HOME 行
func removePvmHomeExports(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 保留空行
		if trimmed == "" {
			result = append(result, line)
			continue
		}
		// 移除独立的 export PVM_HOME=... 行
		if strings.HasPrefix(trimmed, "export PVM_HOME=") {
			continue
		}
		// 移除 fish 的 set -gx PVM_HOME ... 行
		if strings.HasPrefix(trimmed, "set -gx PVM_HOME") {
			continue
		}
		// 移除 setenv PVM_HOME ... 行（csh/tcsh）
		if strings.HasPrefix(trimmed, "setenv PVM_HOME") {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// uniqueStrings 对字符串切片去重，保持原始顺序
func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
