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
	"github.com/pvm/pvm/internal/installer"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/shim"
)

// runRemove 彻底卸载某个已安装的版本：
//   - 删除物理安装目录（~/.pvm/installs/<runtime>/<version>）
//   - 清理用户全局配置（~/.pvm/versions）中的引用
//   - 清理当前项目配置（.pvmrc）中的引用
//   - 清理对应的下载缓存
//
// 用法：
//
//	pvm remove node@20.11.0              # 彻底卸载指定版本
//	pvm remove node --all                # 卸载 node 的所有已安装版本
//	pvm remove node@20.11.0 --force      # 跳过"当前激活"保护
//	pvm remove git                       # 对于全局唯一 runtime（git/mvn），卸载所有已安装版本
//	pvm remove node --user               # 仅从用户全局 ~/.pvm/versions 中移除 node 配置
//	pvm remove node --local              # 仅从当前目录 .pvmrc 中移除 node 配置
//	pvm remove git --force-kill          # 检测到占用进程时自动强制终止，不询问
func runRemove(args []string) error {
	args, force := hasFlag(args, "--force", "-f", "--yes", "-y")
	args, forceKill := hasFlag(args, "--force-kill", "-k")
	args, user := hasFlag(args, "--user", "-u")
	args, local := hasFlag(args, "--local", "-l")
	args, removeAll := hasFlag(args, "--all", "-a")

	if len(args) == 0 {
		return fmt.Errorf("usage:\n" +
			"  pvm remove <runtime@version>          # 彻底卸载（安装目录 + 配置引用 + 缓存）\n" +
			"  pvm remove <runtime> --user            # 仅从用户全局配置移除版本引用\n" +
			"  pvm remove <runtime> --local           # 仅从项目 .pvmrc 移除版本引用")
	}

	// --user / --local 模式：只移除配置引用，不删除安装目录
	if user || local {
		return runRemoveConfig(args, user)
	}

	// 默认模式：彻底卸载（物理安装 + 所有配置 + 缓存）
	cwd, _ := os.Getwd()

	for _, arg := range args {
		rt, ver, err := parseRuntimeArg(arg)
		if err != nil {
			logger.Error("  ✗ %s", err)
			continue
		}

		// Git/Go 走工具型卸载（从 ~/.pvm/installs 删除）
		if installer.IsToolRuntime(rt) {
			logger.Info("  → Uninstalling %s...", rt)
			if err := installer.UninstallTool(rt); err != nil {
				logger.Error("  ✗ %s: %v", rt, err)
			} else {
				// 卸载后重建 shims
				if err := shim.Reshim(); err != nil {
					if shim.IsReshimWarning(err) {
						logger.Info("  ! reshim: %v", err)
					} else {
						logger.Error("  ! reshim: %v", err)
					}
				}
				logger.Info("  ✓ %s uninstalled", rt)
			}
			continue
		}

		// 确定要删除的版本列表
		versionsToRemove := []string{ver}
		installs := getAllInstalledVersions(rt)

		if ver == "latest" {
			// 用户只指定了 runtime，没有指定具体版本
			if config.IsGlobalOnly(rt) {
				// 对于全局唯一 runtime（如 git）：自动卸载所有版本
				if len(installs) > 0 {
					versionsToRemove = installs
					logger.Info("  ℹ  %s is a user-only runtime. Found %d installed version(s):", rt, len(installs))
					for _, v := range installs {
						logger.Info("    - %s@%s", rt, v)
					}
				}
			} else if removeAll {
				// 用户指定了 --all，删除所有版本
				if len(installs) > 0 {
					versionsToRemove = installs
					logger.Info("  ℹ  Found %d installed version(s) of %s:", len(installs), rt)
					for _, v := range installs {
						logger.Info("    - %s@%s", rt, v)
					}
				} else {
					logger.Info("  ℹ  No installed versions of %s found", rt)
					continue
				}
			} else {
				// 用户没有指定版本也没有 --all，需要提示
				if len(installs) > 0 {
					logger.Info("  ℹ  Multiple versions of %s installed:", rt)
					for _, v := range installs {
						logger.Info("    - %s@%s", rt, v)
					}
					logger.Info("")
					logger.Info("  Please specify which version to remove:")
					logger.Info("    pvm remove %s@<version>     # remove a specific version", rt)
					logger.Info("    pvm remove %s --all         # remove all versions", rt)
					continue
				}
				// 没有已安装版本，尝试清理配置
				versionsToRemove = []string{ver}
			}
		}

	for _, targetVer := range versionsToRemove {
		// system 版本不是 pvm 管理的，无法卸载
		if targetVer == config.SystemVersion {
			logger.Error("  ! %s@system is managed by your OS/package manager, not pvm", rt)
			continue
		}

		installDir := config.InstallDir(rt, targetVer)
		installed := false
		if _, err := os.Stat(installDir); err == nil {
			installed = true
		}

		// 检查是否有任何配置引用这个版本
		hasGlobalConfig := false
		hasProjectConfig := false
		globalFile := config.GlobalVersionsFile()
		if vf, err := config.LoadVersionFile(globalFile); err == nil && vf != nil {
			if gv, ok := vf.Versions[rt]; ok && gv == targetVer {
				hasGlobalConfig = true
			}
		}
		if vf, _ := config.FindVersionFile(cwd); vf != nil {
			if pv, ok := vf.Versions[rt]; ok && pv == targetVer {
				hasProjectConfig = true
			}
		}

		// 如果既没有安装目录，也没有配置引用，则没有什么需要删除的
		if !installed && !hasGlobalConfig && !hasProjectConfig && targetVer == "latest" {
			logger.Info("  ℹ  %s is not managed by pvm (no installed versions found)", rt)
			// 检查是否有系统版本
			if sysVer, _ := config.ResolveVersion(rt, cwd); sysVer == config.SystemVersion {
				logger.Info("  ℹ  Detected system %s - this version is managed by your OS, not pvm", rt)
			}
			continue
		}

		// 如果当前正在使用这个版本，拒绝（除非 --force）
		active, _ := config.ResolveVersion(rt, cwd)
		if active == targetVer && !force {
			logger.Error("  ! %s@%s is currently active. Use --force to remove anyway.", rt, targetVer)
			continue
		}

		// 跟踪是否有任何操作被执行
		anyActionTaken := false

		// ── 0. 检测并终止占用进程 ──
		if installed {
			if err := terminateBlockingProcesses(installDir, targetVer, forceKill); err != nil {
				logger.Error("  ✗ %s", err)
				continue
			}
		}

		// ── 1. 删除物理安装目录 ──
		if installed {
			// 使用带重试的删除，处理文件占用问题
			if err := removeWithRetry(installDir); err != nil {
				logger.Error("  ✗ remove install dir: %v", err)
				continue
			}
			logger.Info("  ✓ removed install: %s", installDir)
			anyActionTaken = true

			// 尝试清理空的 runtime 根目录
			rtRoot := filepath.Join(config.InstallsDir(), rt)
			if entries, _ := os.ReadDir(rtRoot); len(entries) == 0 {
				os.Remove(rtRoot)
			}
		} else if targetVer != "latest" {
			// 只有指定了具体版本时才显示 "not installed"
			logger.Info("  ℹ  %s@%s is not installed (skipping install dir)", rt, targetVer)
		}

		// ── 2. 清理用户全局配置 ~/.pvm/versions ──
		if hasGlobalConfig {
			if err := config.RemoveGlobalVersion(rt); err == nil {
				logger.Info("  ✓ removed from user config: %s", globalFile)
				anyActionTaken = true
			} else {
				logger.Error("  ✗ remove from user config: %v", err)
			}
		}

		// ── 3. 清理当前项目配置 .pvmrc ──
		if hasProjectConfig {
			if path, err := config.RemoveProjectVersion(cwd, rt); err == nil && path != "" {
				logger.Info("  ✓ removed from project config: %s", path)
				anyActionTaken = true
			} else if err != nil {
				logger.Error("  ✗ remove from project config: %v", err)
			}
		}

		// ── 4. 清理下载缓存 ──
		cleanVersionCache(rt, targetVer)

		// ── 5. 立即重建 shims（在循环内执行，确保每卸载一个版本就清理一次）──
		logger.Info("  → Refreshing shims...")
		if err := shim.Reshim(); err != nil {
			if shim.IsReshimWarning(err) {
				logger.Info("  ! reshim: %v (some files in use, will update on next run)", err)
			} else {
				logger.Error("  ! reshim failed: %v", err)
			}
		} else {
			logger.Info("  ✓ shims refreshed")
		}

		// 只有在实际执行了某些操作时才显示 "fully removed"
		if anyActionTaken {
			logger.Info("  ✓ %s@%s fully removed", rt, targetVer)
		}
	}
	}

	// 最终再次重建 shims，确保完全清理
	logger.Info("")
	logger.Info("→ Final shim cleanup...")
	if err := shim.Reshim(); err != nil {
		if shim.IsReshimWarning(err) {
			logger.Info("  ! shim cleanup: %v", err)
		}
	}

	// 清理可能残留的 shim 文件（针对被卸载的 runtime）
	logger.Info("→ Cleaning orphaned shims...")
	if err := shim.CleanupOrphanedShims(); err != nil {
		logger.Verbose("  orphaned shim cleanup: %v", err)
	}
	logger.Info("  ✓ Done")
	return nil
}

// terminateBlockingProcesses 检测占用目标安装目录的进程，并终止它们
// forceKill: 为 true 时直接终止，不询问用户
func terminateBlockingProcesses(installDir, version string, forceKill bool) error {
	// 先尝试多次检测，确保捕获所有相关进程
	var procs []processInfo
	for i := 0; i < 3; i++ {
		procs = detectProcessesUsingDir(installDir)
		if len(procs) > 0 || i == 2 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if len(procs) == 0 {
		return nil
	}

	// 按进程名分组展示
	type groupedProc struct {
		name string
		pids []string
	}
	groupMap := make(map[string]*groupedProc)
	var groupOrder []string
	seen := make(map[string]bool)
	for _, p := range procs {
		key := p.name
		if !seen[key] {
			seen[key] = true
			groupMap[key] = &groupedProc{name: p.name}
			groupOrder = append(groupOrder, key)
		}
		groupMap[key].pids = append(groupMap[key].pids, p.pid)
	}

	logger.Info("  ⚠ The following processes are using %s and may block removal:", version)
	for _, key := range groupOrder {
		gp := groupMap[key]
		displayName := strings.TrimSuffix(gp.name, ".exe")
		logger.Info("    - %s (PID: %s)", displayName, strings.Join(gp.pids, ", "))
	}

	if !forceKill {
		logger.Info("")
		logger.Info("  These processes need to be terminated to complete removal.")
		fmt.Print("  Terminate these processes and continue? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("aborted by user (processes still running)")
		}
	}

	logger.Info("  → Terminating processes...")
	terminatedAny := false
	failedPIDs := []string{}
	for _, p := range procs {
		// 先尝试优雅终止，再强制杀死
		cmd := exec.Command("taskkill", "/PID", p.pid)
		output, err := cmd.CombinedOutput()
		if err != nil {
			// 优雅终止失败，尝试强制终止
			cmd2 := exec.Command("taskkill", "/F", "/PID", p.pid)
			output2, err2 := cmd2.CombinedOutput()
			if err2 != nil {
				displayName := strings.TrimSuffix(p.name, ".exe")
				logger.Error("    ⚠ Failed to kill %s (PID %s): %v", displayName, p.pid, err2)
				failedPIDs = append(failedPIDs, p.pid)
				continue
			}
			_ = output2
		}
		_ = output
		terminatedAny = true
		displayName := strings.TrimSuffix(p.name, ".exe")
		logger.Info("    ✓ Killed %s (PID %s)", displayName, p.pid)
	}

	if terminatedAny {
		// 等待更长时间让文件句柄完全释放
		logger.Info("  → Waiting for file handles to release...")
		time.Sleep(1500 * time.Millisecond)
		// 验证进程是否真的终止了
		stillRunning := []string{}
		for _, p := range procs {
			checkCmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %s", p.pid), "/NH", "/FO", "CSV")
			out, _ := checkCmd.Output()
			if strings.Contains(string(out), p.name) {
				stillRunning = append(stillRunning, p.pid)
			}
		}
		if len(stillRunning) > 0 {
			logger.Error("  ⚠ Some processes still running: %s", strings.Join(stillRunning, ", "))
			return fmt.Errorf("some processes could not be terminated: %s", strings.Join(stillRunning, ", "))
		}
		logger.Info("  ✓ All blocking processes terminated")
	}

	// 二次验证：等待文件句柄完全释放后再检查一次
	time.Sleep(2 * time.Second)
	finalCheck := detectProcessesUsingDir(installDir)
	if len(finalCheck) > 0 {
		// 还有进程占用，尝试再次终止
		for _, p := range finalCheck {
			exec.Command("taskkill", "/F", "/PID", p.pid).Run()
		}
		time.Sleep(1 * time.Second)
		logger.Info("  ✓ Secondary process termination completed")
	}
	return nil
}

// processInfo 表示检测到的占用进程
type processInfo struct {
	name string
	pid  string
}

// detectProcessesUsingDir 检测正在使用指定目录的进程
// 使用多种方法确保捕获所有相关进程
func detectProcessesUsingDir(dir string) []processInfo {
	// 获取当前进程 PID，避免杀死自己
	currentPid := fmt.Sprintf("%d", os.Getpid())
	seen := make(map[string]bool)
	var procs []processInfo

	// 方法一：检测已知的运行时进程
	processNames := []string{
		"pvm.exe",
		"node.exe", "npm.exe", "npx.exe",
		"go.exe", "gofmt.exe",
		"python.exe", "pythonw.exe", "pip.exe",
		"pnpm.exe", "yarn.exe",
		"git.exe", "git-gui.exe", "gitk.exe",
		"git-bash.exe", "bash.exe", "sh.exe",
		"mingw32-make.exe",
		"ssh.exe", "ssh-agent.exe",
	}

	for _, procName := range processNames {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", procName),
			"/FI", fmt.Sprintf("PID ne %s", currentPid), "/NH", "/FO", "CSV")
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		outputStr := strings.TrimSpace(string(output))
		if outputStr == "" || strings.Contains(outputStr, "没有") || strings.Contains(outputStr, "No tasks") {
			continue
		}
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Split(line, ",")
			if len(fields) >= 2 {
				pid := strings.Trim(fields[1], `"`)
				key := procName + ":" + pid
				if !seen[key] {
					seen[key] = true
					procs = append(procs, processInfo{name: procName, pid: pid})
				}
			}
		}
	}

	// 方法二：用 PowerShell 查询加载了目标目录下模块的进程
	dirEsc := strings.ReplaceAll(dir, `'`, `''`)
	psQuery := fmt.Sprintf(
		`Get-Process | Where-Object { $_.Modules -ne $null } | ForEach-Object { `+
			`$p = $_; try { $p.Modules | Where-Object { $_.FileName -like '%s*' } | `+
			`ForEach-Object { "$($p.Id),$($p.ProcessName)" } } catch {} } | Sort-Object -Unique`,
		dirEsc,
	)
	psOut, psErr := exec.Command("powershell", "-NoProfile", "-Command", psQuery).Output()
	if psErr == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(psOut)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ",", 2)
			if len(parts) != 2 {
				continue
			}
			pid, procName := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])+".exe"
			if pid == currentPid {
				continue
			}
			key := procName + ":" + pid
			if !seen[key] {
				seen[key] = true
				procs = append(procs, processInfo{name: procName, pid: pid})
			}
		}
	}

	// 方法三：使用 WMIC 查询进程命令行参数（检测是否使用了目标目录）
	// 注意：Windows 10 21H1+ WMIC 已弃用，这里作为补充方法
	psQuery2 := fmt.Sprintf(
		`Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like '*%s*' } | `+
			`ForEach-Object { "$($_.ProcessId),$($_.ProcessName)" } | Sort-Object -Unique`,
		strings.ReplaceAll(dir, `\`, `\\`),
	)
	psOut2, psErr2 := exec.Command("powershell", "-NoProfile", "-Command", psQuery2).Output()
	if psErr2 == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(psOut2)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ",", 2)
			if len(parts) != 2 {
				continue
			}
			pid, procName := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if pid == currentPid {
				continue
			}
			// 确保 procName 有 .exe 后缀
			if !strings.HasSuffix(procName, ".exe") {
				procName += ".exe"
			}
			key := procName + ":" + pid
			if !seen[key] {
				seen[key] = true
				procs = append(procs, processInfo{name: procName, pid: pid})
			}
		}
	}

	return procs
}

// getAllInstalledVersions 获取某个 runtime 的所有已安装版本
func getAllInstalledVersions(rt string) []string {
	rtRoot := filepath.Join(config.InstallsDir(), rt)
	entries, err := os.ReadDir(rtRoot)
	if err != nil {
		return nil
	}

	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	return versions
}

// cleanVersionCache 清理某版本对应的下载缓存文件
func cleanVersionCache(rt, ver string) {
	cacheDir := config.CacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return // 缓存目录不存在或无法读取，忽略
	}

	// 缓存文件命名格式：<runtime>-<version>.<ext>
	// 例如：node-20.11.0.tar.gz、python-3.12.0.zip
	prefix := rt + "-" + ver + "."
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) {
			cachePath := filepath.Join(cacheDir, e.Name())
			if err := os.Remove(cachePath); err == nil {
				logger.Info("  ✓ removed cache: %s", e.Name())
			} else {
				logger.Verbose("  ! remove cache %s: %v", e.Name(), err)
			}
		}
	}
}

// runRemoveConfig 从用户全局或项目配置中移除 runtime 的版本引用（不删除安装目录）
func runRemoveConfig(args []string, isUser bool) error {
	for _, arg := range args {
		// 支持 "node" 或 "node@version" 两种格式，只取 runtime 部分
		rt := arg
		if idx := len(arg); idx > 0 {
			for i, c := range arg {
				if c == '@' {
					rt = arg[:i]
					break
				}
			}
		}
		rt = strings.ToLower(rt)
		// 别名
		switch rt {
		case "nodejs":
			rt = "node"
		case "golang":
			rt = "go"
		case "py":
			rt = "python"
		}
		if !config.IsSupportedRuntime(rt) {
			logger.Error("  ✗ unsupported runtime: %s", rt)
			continue
		}

		if isUser {
			if err := config.RemoveGlobalVersion(rt); err != nil {
				logger.Error("  ✗ %v", err)
				continue
			}
			logger.Info("  ✓ removed %s from user config (%s)", rt, config.GlobalVersionsFile())
		} else {
			cwd, _ := os.Getwd()
			path, err := config.RemoveProjectVersion(cwd, rt)
			if err != nil {
				logger.Error("  ✗ %v", err)
				continue
			}
			if path == "" {
				logger.Info("  ℹ  no .pvmrc found in current directory")
			} else {
				logger.Info("  ✓ removed %s from %s", rt, path)
			}
		}
	}
	return nil
}

// removeWithRetry 尝试删除目录，处理文件占用问题
// 使用重试机制 + 逐文件清理 + 更长的等待时间 + 重启后删除兜底
func removeWithRetry(path string) error {
	// 先尝试直接删除
	if err := os.RemoveAll(path); err == nil {
		return nil
	}

	// 删除失败，使用逐文件清理
	logger.Info("  → Directory busy, trying file-by-file cleanup...")
	removeDirBestEffort(path)

	// 多次重试删除（增加到 8 次，总等待时间约 20 秒）
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(path); err != nil {
			return nil
		}
		wait := time.Duration(500*(i+1)) * time.Millisecond
		if i > 3 {
			wait = 3 * time.Second
		}
		time.Sleep(wait)
		os.RemoveAll(path)
	}

	if _, err := os.Stat(path); err == nil {
		// 最后手段：标记为重启后删除（Windows）
		if runtime.GOOS == "windows" {
			if err := markForDeletionOnReboot(path); err == nil {
				logger.Info("  ⚠ Could not fully remove %s, marked for deletion on next reboot", path)
				return nil
			}
		}
		return fmt.Errorf("cannot fully remove %s (some files may be in use)", path)
	}
	return nil
}

// markForDeletionOnReboot 在 Windows 下标记文件/目录为重启后删除
// 使用 MoveFileEx API 和 MOVEFILE_DELAY_UNTIL_REBOOT 标志
func markForDeletionOnReboot(path string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("not supported on this platform")
	}

	// 使用 PowerShell 调用 MoveFileEx
	psScript := fmt.Sprintf(`
$path = '%s'
$signature = @'
[DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
public static extern bool MoveFileEx(string lpExistingFileName, string lpNewFileName, int dwFlags);
'@
$type = Add-Type -MemberDefinition $signature -Name "Win32MoveFileEx" -PassThru
# MOVEFILE_DELAY_UNTIL_REBOOT = 0x4
$result = $type::MoveFileEx($path, $null, 0x4)
if (-not $result) {
    Write-Output "FAILED: $([System.Runtime.InteropServices.Marshal]::GetLastWin32Error())"
} else {
    Write-Output "OK"
}
`, strings.ReplaceAll(path, `'`, `''`))

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(output), "OK") {
		return fmt.Errorf("MoveFileEx failed: %s", string(output))
	}
	return nil
}

// removeDirBestEffort 递归删除目录，跳过被占用的文件
// 复制自 internal/installer/helpers.go 的 removeDirBestEffort 逻辑
func removeDirBestEffort(path string) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}
	if !info.IsDir() {
		_ = os.Remove(path)
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		removeDirBestEffort(filepath.Join(path, entry.Name()))
	}
	_ = os.Remove(path)
}
