//go:build windows
// +build windows

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/config"
)

// windowsOps 实现 Windows 平台的卸载操作
type windowsOps struct{}

// newPlatformOps 返回 Windows 平台的 uninstallOps 实现
func newPlatformOps() uninstallOps {
	return &windowsOps{}
}

// killProcesses 检测并终止 pvm 相关进程，返回 true 表示可以继续卸载
func (w *windowsOps) killProcesses() bool {
	return killPvmProcesses()
}

// removeFromPath 从 PATH 中移除 pvm 相关条目（用户级 + 系统级）
func (w *windowsOps) removeFromPath(binHome, shimsDir string) error {
	// 1a. 用户级 PATH（彻底清理所有 pvm 相关条目）
	if err := uninstallWindowsPath(binHome, shimsDir); err != nil {
		fmt.Printf("  ⚠ Could not update User PATH: %v\n", err)
		fmt.Println("    Please remove these entries manually from your User PATH:")
		fmt.Printf("      %s\n", shimsDir)
		fmt.Printf("      %s\n", binHome)
	} else {
		fmt.Println("  ✓ Removed from User PATH")
	}

	// 1b. 系统级 PATH（需管理员，失败提示但不阻断）
	if err := uninstallWindowsSystemPath(binHome, shimsDir); err != nil {
		fmt.Printf("  ⚠ System PATH not updated (admin may be required): %v\n", err)
	} else {
		fmt.Println("  ✓ Removed from System PATH")
	}

	// 1c. 清理所有已知 pvm 安装路径的 PATH 条目
	knownPvmDirs := findAllPvmInstallDirs()
	for _, dir := range knownPvmDirs {
		_ = uninstallWindowsPath(dir, filepath.Join(dir, "shims"))       // 忽略错误
		_ = uninstallWindowsSystemPath(dir, filepath.Join(dir, "shims")) // 忽略错误
	}

	// 1d. 清理 PowerShell Profile 中的 pvm 配置
	cleanPowerShellProfiles()

	// 1e. 清理 Git Bash / MSYS2 的 shell rc 文件
	cleanGitBashRcFiles()

	// 1f. 清理 WSL 内的 pvm 残留（如果检测到 WSL 安装）
	cleanWslPvmConfig()

	return nil
}

// removePvmHome 清除 PVM_HOME 环境变量（用户级 + 系统级）
func (w *windowsOps) removePvmHome() {
	// 用户级
	_ = exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::SetEnvironmentVariable('PVM_HOME',$null,'User')`).Run() // 忽略错误
	// 系统级（需要管理员，失败也不报错）
	_ = exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::SetEnvironmentVariable('PVM_HOME',$null,'Machine')`).Run() // 忽略错误
	fmt.Println("  ✓ Cleared PVM_HOME environment variable")
}

// cleanPlatformSpecific 清理平台特有的残留物（注册表、开始菜单、MSI 产品）
func (w *windowsOps) cleanPlatformSpecific() {
	fmt.Println("  → Cleaning registry...")
	cleanRegistry()

	fmt.Println("  → Removing Start Menu shortcuts...")
	removeStartMenuShortcuts()

	uninstallMsiProducts()
}

// findExtraInstallDirs 查找额外可能存在的 pvm 安装目录
func (w *windowsOps) findExtraInstallDirs(pvmHome string) []string {
	return findAllPvmInstalls(pvmHome)
}

// cleanupExtraInstallDir 清理额外的安装目录（可能需要延迟删除）
func (w *windowsOps) cleanupExtraInstallDir(dir, currentExe string) {
	pvmExe := filepath.Join(dir, "pvm.exe")
	if _, err := os.Stat(pvmExe); err != nil {
		return
	}

	fmt.Printf("  → Removing pvm install: %s\n", dir)
	errs := removeAllBestEffort(dir)
	if len(errs) > 0 {
		fmt.Printf("  ⚠ Some files in %s are in use, scheduling delayed removal:\n", dir)
		for _, e := range errs {
			fmt.Printf("    - %v\n", e)
		}
		scheduleRebootDelete(errs)
		scheduleRebootDeletePath(dir)

		// 延迟删除脚本
		dirEscaped := strings.ReplaceAll(dir, `'`, `''`)
		exeTargets := ""
		if strings.HasPrefix(strings.ToLower(currentExe), strings.ToLower(dir)+string(filepath.Separator)) {
			exeEscaped := strings.ReplaceAll(currentExe, `'`, `''`)
			exeTargets = fmt.Sprintf(`$targets = @('%s'); foreach ($t in $targets) { while ((Get-Date) -lt $deadline) { try { if (Test-Path $t) { Remove-Item -Force -ErrorAction Stop $t }; break } catch { Start-Sleep -Milliseconds 500 } } }; `, exeEscaped)
		}
		psScript := fmt.Sprintf(
			`Start-Sleep -Milliseconds 500; $deadline = (Get-Date).AddSeconds(30); %swhile ((Get-Date) -lt $deadline) { try { if (Test-Path '%s') { Remove-Item -Recurse -Force -ErrorAction Stop '%s' }; break } catch { Start-Sleep -Milliseconds 500 } }`,
			exeTargets,
			dirEscaped, dirEscaped,
		)
		_ = exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden",
			"-Command", psScript).Start() // 忽略错误，异步删除
		fmt.Printf("  ✓ %s will be automatically deleted after this process exits.\n", dir)
	} else {
		fmt.Printf("  ✓ Removed %s\n", dir)
	}
}

// scheduleDelayedCleanup 处理当前 exe 在安装目录内时的延迟删除
func (w *windowsOps) scheduleDelayedCleanup(currentExe, pvmHome string) {
	pvmHomeEscaped := strings.ReplaceAll(pvmHome, `'`, `''`)
	exeEscaped := strings.ReplaceAll(currentExe, `'`, `''`)
	// 同时删除可能存在的 bin/pvm.exe（即使当前 exe 不在那里）
	binHome := config.BinHome()
	targetExe := filepath.Clean(filepath.Join(binHome, "pvm"+config.ExeExt()))
	binExeEscaped := strings.ReplaceAll(targetExe, `'`, `''`)
	psScript := fmt.Sprintf(
		`Start-Sleep -Milliseconds 500; `+
			`$deadline = (Get-Date).AddSeconds(30); `+
			`$targets = @('%s', '%s'); `+
			`foreach ($t in $targets) { `+
			`  while ((Get-Date) -lt $deadline) { `+
			`    try { if (Test-Path $t) { Remove-Item -Force -ErrorAction Stop $t }; break } `+
			`    catch { Start-Sleep -Milliseconds 500 } `+
			`  } `+
			`}; `+
			`# 最终清理整个 pvmHome 目录 `+
			`while ((Get-Date) -lt $deadline) { `+
			`  try { if (Test-Path '%s') { Remove-Item -Recurse -Force -ErrorAction Stop '%s' }; break } `+
			`  catch { Start-Sleep -Milliseconds 500 } `+
			`}`,
		exeEscaped, binExeEscaped,
		pvmHomeEscaped, pvmHomeEscaped,
	)
	exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden",
		"-Command", psScript).Start()
}

// displayRemovalPlan 显示平台特有的卸载计划信息
func (w *windowsOps) displayRemovalPlan(pvmHome string, extraDirs []string) {
	fmt.Printf("    • pvm entries from User PATH\n")
	fmt.Printf("    • pvm entries from System PATH (requires admin; will be skipped if no permission)\n")
	fmt.Printf("    • MSI-registered pvm product (if installed via MSI)\n")
	for _, dir := range extraDirs {
		fmt.Printf("    • %s\n", dir)
	}
}

// displayFinalMessage 显示平台特有的最终提示
func (w *windowsOps) displayFinalMessage() {
	fmt.Println("  IMPORTANT - To complete removal:")
	fmt.Println("    1. Close ALL terminal windows (PowerShell, CMD, VS Code, etc.)")
	fmt.Println("    2. Open a NEW terminal — do not reuse the current one")
	fmt.Println("    3. Verify: run 'pvm' — it should say 'command not found'")
	fmt.Println()
	fmt.Println("  Why? The current terminal still has pvm paths in its PATH")
	fmt.Println("  environment variable (loaded at startup). Only a NEW terminal")
	fmt.Println("  will pick up the cleaned PATH from the registry.")
	fmt.Println()
	fmt.Println("  If pvm is still found in a new terminal:")
	fmt.Println("    • Check: System Properties → Environment Variables → PATH")
	fmt.Println("    • Remove any entries containing '.pvm'")
}

// ──────────────────────────────────────────────────────────────────────────────
//  私有辅助函数
// ──────────────────────────────────────────────────────────────────────────────

// killPvmProcesses 在 Windows 上检测正在运行的 pvm 相关进程，
// 列出后询问用户是否终止它们，返回 true 表示可以继续卸载，false 表示用户取消。
func killPvmProcesses() bool {
	currentPid := os.Getpid()
	pvmHome := config.PvmHome()

	type runningProc struct {
		name string
		pid  string
	}
	var runningProcs []runningProc

	// 尝试最多3次检测，确保捕获所有相关进程
	for attempt := 0; attempt < 3; attempt++ {
		seen := make(map[string]bool)
		runningProcs = nil

		// 方法一：用 tasklist 查询已知的运行时进程列表
		processNames := []string{
			"pvm.exe",
			"node.exe",
			"go.exe",
			"python.exe",
			"pythonw.exe",
			"pnpm.exe",
			"npm.exe",
			"npx.exe",
			"yarn.exe",
			"bun.exe",
			"git.exe",
			"git-gui.exe",
			"gitk.exe",
			"git-bash.exe",
			"bash.exe",
			"sh.exe",
			"mingw32-make.exe",
			"ssh.exe",
			"ssh-agent.exe",
		}

		for _, procName := range processNames {
			cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", procName),
				"/FI", fmt.Sprintf("PID ne %d", currentPid), "/NH", "/FO", "CSV")
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
						runningProcs = append(runningProcs, runningProc{name: procName, pid: pid})
					}
				}
			}
		}

		// 方法二：尝试用 PowerShell 查询占用 pvmHome 目录的其他进程
		pvmHomeEsc := strings.ReplaceAll(pvmHome, `'`, `''`)
		psQuery := fmt.Sprintf(
			`Get-Process | Where-Object { $_.Modules -ne $null } | ForEach-Object { `+
				`$p = $_; try { $p.Modules | Where-Object { $_.FileName -like '%s\*' } | `+
				`ForEach-Object { "$($p.Id),$($p.ProcessName)" } } catch {} } | Sort-Object -Unique`,
			pvmHomeEsc,
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
				if pid == fmt.Sprintf("%d", currentPid) {
					continue
				}
				key := procName + ":" + pid
				if !seen[key] {
					seen[key] = true
					runningProcs = append(runningProcs, runningProc{name: procName, pid: pid})
				}
			}
		}

		// 方法三：使用 CIM 查询命令行参数中包含 pvmHome 的进程
		psQuery2 := fmt.Sprintf(
			`Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like '*%s*' } | `+
				`ForEach-Object { "$($_.ProcessId),$($_.ProcessName)" } | Sort-Object -Unique`,
			strings.ReplaceAll(pvmHome, `\`, `\\`),
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
				if pid == fmt.Sprintf("%d", currentPid) {
					continue
				}
				if !strings.HasSuffix(procName, ".exe") {
					procName += ".exe"
				}
				key := procName + ":" + pid
				if !seen[key] {
					seen[key] = true
					runningProcs = append(runningProcs, runningProc{name: procName, pid: pid})
				}
			}
		}

		if len(runningProcs) > 0 || attempt == 2 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

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
	fmt.Println("  ⚠ The following processes are running and may block uninstall:")
	fmt.Println()
	for _, name := range groupOrder {
		gp := groupMap[name]
		displayName := strings.TrimSuffix(gp.name, ".exe")
		fmt.Printf("    - %-24s (PID: %s)\n", displayName, strings.Join(gp.pids, ", "))
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

	// 杀死所有检测到的进程
	fmt.Println("  → Terminating processes...")
	terminatedAny := false
	var failedPIDs []string
	for _, rp := range runningProcs {
		cmd := exec.Command("taskkill", "/PID", rp.pid)
		output, err := cmd.CombinedOutput()
		if err != nil {
			cmd2 := exec.Command("taskkill", "/F", "/PID", rp.pid)
			output2, err2 := cmd2.CombinedOutput()
			if err2 != nil {
				displayName := strings.TrimSuffix(rp.name, ".exe")
				fmt.Printf("    ⚠ Failed to kill %s (PID %s): %v\n", displayName, rp.pid, err2)
				failedPIDs = append(failedPIDs, rp.pid)
				continue
			}
			_ = output2
		}
		_ = output
		terminatedAny = true
		displayName := strings.TrimSuffix(rp.name, ".exe")
		fmt.Printf("    ✓ Killed %s (PID %s)\n", displayName, rp.pid)
	}

	if len(failedPIDs) > 0 {
		fmt.Printf("  ⚠ Failed to terminate %d processes: %s\n", len(failedPIDs), strings.Join(failedPIDs, ", "))
	}

	if terminatedAny {
		fmt.Println("  → Waiting for file handles to release...")
		time.Sleep(1500 * time.Millisecond)

		var stillRunning []string
		for _, rp := range runningProcs {
			checkCmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %s", rp.pid), "/NH", "/FO", "CSV")
			out, _ := checkCmd.Output()
			if strings.Contains(string(out), rp.name) {
				stillRunning = append(stillRunning, rp.pid)
			}
		}
		if len(stillRunning) > 0 {
			fmt.Printf("  ⚠ Some processes still running: %s\n", strings.Join(stillRunning, ", "))
			fmt.Print("  Continue anyway? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println()
				fmt.Println("  Aborted. Please close the listed processes manually and run uninstall again.")
				return false
			}
		} else {
			fmt.Println("  ✓ All blocking processes terminated")
		}
	}
	return true
}

// uninstallWindowsPath 从 Windows 用户 PATH 中移除 pvm 相关条目
func uninstallWindowsPath(binHome, shimsDir string) error {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('Path','User')`).Output()
	if err != nil {
		return fmt.Errorf("cannot read user PATH: %w", err)
	}
	pathStr := strings.TrimRight(string(out), "\r\n")

	entries := splitPath(pathStr)
	var filtered []string
	seen := make(map[string]bool)
	for _, e := range entries {
		if e == "" {
			continue
		}
		ce := filepath.Clean(e)
		if strings.EqualFold(ce, filepath.Clean(shimsDir)) ||
			strings.EqualFold(ce, filepath.Clean(binHome)) {
			continue
		}
		if strings.Contains(strings.ToLower(e), ".pvm") {
			continue
		}
		lower := strings.ToLower(ce)
		if strings.HasSuffix(lower, "\\pvm") || strings.HasSuffix(lower, "/pvm") {
			localAppData := strings.ToLower(filepath.Clean(os.Getenv("LOCALAPPDATA")))
			if strings.HasPrefix(lower, localAppData+string(filepath.Separator)) {
				continue
			}
			progFiles := strings.ToLower(filepath.Clean(os.Getenv("ProgramFiles")))
			progFiles86 := strings.ToLower(filepath.Clean(os.Getenv("ProgramFiles(x86)")))
			if strings.HasPrefix(lower, progFiles+string(filepath.Separator)) ||
				strings.HasPrefix(lower, progFiles86+string(filepath.Separator)) {
				continue
			}
		}
		key := strings.ToLower(ce)
		if seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, e)
	}

	newPath := joinPath(filtered)

	psScript := fmt.Sprintf(`$newPath = '%s'; [Environment]::SetEnvironmentVariable('Path', $newPath, 'User'); `+
		`Write-Host "  ✓ Removed pvm from PATH"; `+
		`Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; ' + `+
		`'public class Win32 { [DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Auto)] ' + `+
		`'public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, ' + `+
		`'uint fuFlags, uint uTimeout, out UIntPtr lpdwResult); }'; `+
		`$HWND_BROADCAST = [IntPtr]0xffff; $WM_SETTINGCHANGE = 0x001A; $result = [UIntPtr]::Zero; `+
		`'Win32'::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null`,
		strings.ReplaceAll(newPath, `'`, `''`))

	encodedCmd := encodePowerShellCommand(psScript)
	cmd := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encodedCmd)
	return cmd.Run()
}

// uninstallWindowsSystemPath 从 Windows 系统 PATH（Machine）中移除 pvm 相关条目
func uninstallWindowsSystemPath(binHome, shimsDir string) error {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('Path','Machine')`).Output()
	if err != nil {
		return fmt.Errorf("cannot read system PATH: %w", err)
	}
	pathStr := strings.TrimRight(string(out), "\r\n")

	entries := splitPath(pathStr)
	var filtered []string
	changed := false
	seen := make(map[string]bool)
	for _, e := range entries {
		if e == "" {
			continue
		}
		ce := filepath.Clean(e)
		if strings.EqualFold(ce, filepath.Clean(shimsDir)) ||
			strings.EqualFold(ce, filepath.Clean(binHome)) {
			changed = true
			continue
		}
		if strings.Contains(strings.ToLower(e), ".pvm") {
			changed = true
			continue
		}
		lower := strings.ToLower(ce)
		if strings.HasSuffix(lower, "\\pvm") || strings.HasSuffix(lower, "/pvm") {
			localAppData := strings.ToLower(filepath.Clean(os.Getenv("LOCALAPPDATA")))
			if strings.HasPrefix(lower, localAppData+string(filepath.Separator)) {
				changed = true
				continue
			}
			progFiles := strings.ToLower(filepath.Clean(os.Getenv("ProgramFiles")))
			progFiles86 := strings.ToLower(filepath.Clean(os.Getenv("ProgramFiles(x86)")))
			if strings.HasPrefix(lower, progFiles+string(filepath.Separator)) ||
				strings.HasPrefix(lower, progFiles86+string(filepath.Separator)) {
				changed = true
				continue
			}
		}
		key := strings.ToLower(ce)
		if seen[key] {
			changed = true
			continue
		}
		seen[key] = true
		filtered = append(filtered, e)
	}

	if !changed {
		return nil
	}

	newPath := joinPath(filtered)
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`[Environment]::SetEnvironmentVariable('Path','%s','Machine')`, newPath))
	return cmd.Run()
}

// cleanRegistry 清理 Windows 注册表中 pvm 相关的残留项
func cleanRegistry() {
	psScript := `
		# 清理 HKCU:\Software\pvm
		Remove-Item -Path 'HKCU:\Software\pvm' -Recurse -Force -ErrorAction SilentlyContinue
		# 清理可能的 App Paths 记录
		Remove-Item -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\App Paths\pvm.exe' -Recurse -Force -ErrorAction SilentlyContinue
		# 清理 Run / RunOnce 中可能存在的 pvm 启动项
		Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -ErrorAction SilentlyContinue |
			Where-Object { $_.PSObject.Properties.Name -like '*pvm*' } |
			ForEach-Object { Remove-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name $_.PSObject.Properties.Name -ErrorAction SilentlyContinue }
		# 清理"添加/删除程序"中残留的 pvm 条目
		$uninstallKeys = @(
			'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall',
			'HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall',
			'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall'
		)
		foreach ($key in $uninstallKeys) {
			if (!(Test-Path $key)) { continue }
			Get-ChildItem $key -ErrorAction SilentlyContinue |
				Where-Object {
					$displayName = $_.GetValue('DisplayName')
					$displayName -ne $null -and ($displayName -like '*pvm*' -or $displayName -like '*PVM*')
				} |
				ForEach-Object {
					$installLoc = $_.GetValue('InstallLocation')
					if ($installLoc -ne $null -and !(Test-Path $installLoc)) {
						Remove-Item $_.PSPath -Recurse -Force -ErrorAction SilentlyContinue
					}
				}
		}
	`
	encodedCmd := encodePowerShellCommand(psScript)
	cmd := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encodedCmd)
	_ = cmd.Run() // 忽略错误，尽力清理
	fmt.Println("  ✓ Registry cleaned")
}

// removeStartMenuShortcuts 删除开始菜单中的 pvm 快捷方式
func removeStartMenuShortcuts() {
	startMenuDirs := []string{
		filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs"),
		filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs"),
	}
	found := false
	for _, dir := range startMenuDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.Contains(strings.ToLower(info.Name()), "pvm") &&
				(strings.HasSuffix(strings.ToLower(info.Name()), ".lnk") ||
					strings.HasSuffix(strings.ToLower(info.Name()), ".url")) {
				if err := os.Remove(path); err == nil {
					fmt.Printf("    ✓ Removed: %s\n", info.Name())
					found = true
				}
			}
			return nil
		})
	}
	if !found {
		fmt.Println("  ✓ No Start Menu shortcuts found")
	}
}

// uninstallMsiProducts 查找并卸载所有 MSI 注册的 pvm 产品
func uninstallMsiProducts() {
	fmt.Println("  → Checking for MSI-registered pvm products...")

	psScript := `
		$products = Get-ItemProperty "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*" -ErrorAction SilentlyContinue
		$products += Get-ItemProperty "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*" -ErrorAction SilentlyContinue
		$products | Where-Object { $_.DisplayName -like "*PVM*" -or $_.DisplayName -like "*pvm*" } |
			ForEach-Object { "$($_.PSChildName)|$($_.DisplayName)" }
	`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", psScript).Output()
	if err != nil {
		fmt.Printf("  ⚠ Cannot query MSI products: %v\n", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		productCode := strings.TrimSpace(parts[0])
		displayName := strings.TrimSpace(parts[1])

		fmt.Printf("  → Uninstalling MSI product: %s\n", displayName)
		cmd := exec.Command("msiexec", "/x", productCode, "/qn")
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("  ⚠ Silent uninstall failed, trying with UI...\n")
			cmd2 := exec.Command("msiexec", "/x", productCode)
			_ = cmd2.Start() // 忽略错误，异步执行
			_ = output
		} else {
			fmt.Printf("  ✓ Uninstalled MSI product: %s\n", displayName)
		}
	}
}

// findAllPvmInstallDirs 返回系统中所有存在的 pvm 安装目录列表
func findAllPvmInstallDirs() []string {
	var dirs []string
	checkDirs := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "pvm"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "pvm"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "pvm"),
	}
	for _, dir := range checkDirs {
		if _, err := os.Stat(dir); err == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// findAllPvmInstalls 检测系统中所有 pvm 安装位置（用于展示卸载计划）
func findAllPvmInstalls(pvmHome string) []string {
	var installs []string
	checkDirs := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "pvm"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "pvm"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "pvm"),
	}
	for _, dir := range checkDirs {
		pvmExe := filepath.Join(dir, "pvm.exe")
		if _, err := os.Stat(pvmExe); err == nil {
			if !strings.EqualFold(dir, pvmHome) {
				installs = append(installs, dir)
			}
		}
	}
	return installs
}

// scheduleRebootDelete 从错误列表中提取文件路径，注册 Windows 重启后删除
func scheduleRebootDelete(errs []error) {
	for _, e := range errs {
		if e == nil {
			continue
		}
		msg := e.Error()
		path := extractPathFromUnlinkatError(msg)
		if path != "" {
			scheduleRebootDeletePath(path)
		}
	}
}

// scheduleRebootDeletePath 使用 Windows MoveFileEx API 注册指定路径在重启后删除
func scheduleRebootDeletePath(path string) {
	psCmd := fmt.Sprintf(
		`Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public class MoveFileEx {
    [DllImport("kernel32.dll", SetLastError=true, CharSet=CharSet.Unicode)]
    public static extern bool MoveFileEx(string lpExistingFileName, string lpNewFileName, int dwFlags);
}
'@; [MoveFileEx]::MoveFileEx('%s', $null, 4) | Out-Null`,
		strings.ReplaceAll(path, `'`, `''`),
	)
	_ = exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden",
		"-Command", psCmd).Start() // 忽略错误，异步删除
}

// extractPathFromUnlinkatError 从 "unlinkat <path>: <reason>" 格式的错误消息中提取路径
func extractPathFromUnlinkatError(msg string) string {
	const prefix = "unlinkat "
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return ""
	}
	rest := msg[idx+len(prefix):]
	colonIdx := strings.Index(rest, ": ")
	if colonIdx < 0 {
		return rest
	}
	return rest[:colonIdx]
}

// cleanGitBashRcFiles 清理 Windows 上 Git Bash 用户的 .bashrc/.bash_profile 中的 pvm 配置块
func cleanGitBashRcFiles() {
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return
	}

	rcFiles := []string{
		filepath.Join(userProfile, ".bashrc"),
		filepath.Join(userProfile, ".bash_profile"),
	}

	for _, rcPath := range rcFiles {
		data, err := os.ReadFile(rcPath)
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(content, "PVM_HOME") && !strings.Contains(content, "pvm") {
			continue
		}
		cleaned := removePvmBlock(content)
		// 也清理独立 PATH / PVM_HOME 行
		cleaned = removePvmPathEntriesGeneric(cleaned)
		cleaned = removePvmHomeExportsGeneric(cleaned)
		if cleaned != content {
			if err := os.WriteFile(rcPath, []byte(cleaned), 0644); err != nil {
				fmt.Printf("  ⚠ Cannot update %s: %v\n", rcPath, err)
				continue
			}
			fmt.Printf("  ✓ Cleaned %s\n", rcPath)
		}
	}
}

// cleanPowerShellProfiles 清理所有 PowerShell Profile 文件中的 pvm 配置
// PowerShell 有多个 profile 位置，都需要清理：
//   - $PROFILE.CurrentUserCurrentHost: Documents\PowerShell\Microsoft.PowerShell_profile.ps1
//   - $PROFILE.CurrentUserAllHosts:   Documents\PowerShell\profile.ps1
//   - $PROFILE.AllUsersCurrentHost:   安装目录\Microsoft.PowerShell_profile.ps1
//   - $PROFILE.AllUsersAllHosts:      安装目录\profile.ps1
//   - Windows PowerShell (5.x) 同理但路径是 Documents\WindowsPowerShell\
func cleanPowerShellProfiles() {
	documentsDir := os.Getenv("USERPROFILE")
	if documentsDir == "" {
		return
	}

	// PowerShell 7+ 和 Windows PowerShell 5.x 的 profile 路径
	profilePaths := []string{
		// PowerShell 7+ (pwsh)
		filepath.Join(documentsDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(documentsDir, "Documents", "PowerShell", "profile.ps1"),
		// Windows PowerShell 5.x
		filepath.Join(documentsDir, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(documentsDir, "Documents", "WindowsPowerShell", "profile.ps1"),
	}

	for _, profilePath := range profilePaths {
		data, err := os.ReadFile(profilePath)
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(content, "PVM_HOME") && !strings.Contains(content, "pvm") &&
			!strings.Contains(content, ".pvm") {
			continue
		}

		cleaned := cleanPvmFromPowershellProfile(content)
		if cleaned != content {
			if err := os.WriteFile(profilePath, []byte(cleaned), 0644); err != nil {
				fmt.Printf("  ⚠ Cannot update %s: %v\n", profilePath, err)
				continue
			}
			fmt.Printf("  ✓ Cleaned %s\n", profilePath)
		}
	}
}

// cleanPvmFromPowershellProfile 从 PowerShell Profile 内容中移除 pvm 相关行
// 匹配以下模式：
//   - $env:PVM_HOME = "..."
//   - $env:PATH += ";...\pvm\..."
//   - # pvm (Polyglot Version Manager) ... 配置块
//   - setx PVM_HOME ...
//   - [Environment]::SetEnvironmentVariable('PVM_HOME', ...)
func cleanPvmFromPowershellProfile(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 匹配 pvm 配置块起始注释
		if strings.HasPrefix(trimmed, "# pvm (Polyglot Version Manager)") {
			skip = true
		}

		if skip {
			if trimmed == "" {
				skip = false
				continue
			}
			continue
		}

		// 匹配独立的 pvm 环境变量设置行
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "$env:pvm_home") ||
			strings.Contains(lower, "[environment]::setenvironmentvariable('pvm_home'") ||
			strings.Contains(lower, "setx pvm_home") {
			continue
		}

		// 匹配 PATH 中追加 pvm 目录的行
		if (strings.Contains(lower, "$env:path") || strings.Contains(lower, "[environment]::setenvironmentvariable('path'")) &&
			(strings.Contains(lower, ".pvm") || strings.Contains(lower, "\\pvm") || strings.Contains(lower, "/pvm")) {
			continue
		}

		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// cleanWslPvmConfig 检测并清理 WSL (Windows Subsystem for Linux) 内的 pvm 残留
// 通过 wsl.exe 执行命令来清理，不依赖 WSL 是否安装
func cleanWslPvmConfig() {
	// 检测 wsl.exe 是否可用
	wslPath, err := exec.LookPath("wsl.exe")
	if err != nil || wslPath == "" {
		return
	}

	// 检测 WSL 是否有已安装的发行版
	out, err := exec.Command("wsl.exe", "-l", "-q").Output()
	if err != nil {
		return
	}
	// WSL 输出可能包含 UTF-16LE BOM，简单检查是否有内容
	if len(out) < 4 {
		return
	}

	// WSL 可用且有发行版，尝试清理 WSL 内的 pvm
	// 使用 wsl.exe 执行清理脚本
	psScript := `
		$wslOutput = wsl.exe -l -q 2>$null
		if ($LASTEXITCODE -ne 0 -or $wslOutput -eq $null) { return }

		# 清理默认 WSL 发行版中的 pvm shell rc 文件
		$rcFiles = @(".bashrc", ".bash_profile", ".zshrc", ".profile")
		foreach ($rc in $rcFiles) {
			$check = wsl.exe -e bash -c "test -f ~/$rc && grep -l 'PVM_HOME\|\.pvm' ~/$rc 2>/dev/null"
			if ($LASTEXITCODE -eq 0 -and $check -ne "") {
				wsl.exe -e bash -c "sed -i '/# pvm (Polyglot Version Manager)/,/^$/d; /PVM_HOME/d; /\.pvm/d' ~/$rc 2>/dev/null"
				if ($LASTEXITCODE -eq 0) {
					Write-Host "  ✓ Cleaned WSL ~/$rc"
				}
			}
		}

		# 清理 WSL 内的 .pvm 目录
		$pvmExists = wsl.exe -e bash -c "test -d ~/.pvm && echo yes"
		if ($LASTEXITCODE -eq 0 -and $pvmExists -match 'yes') {
			wsl.exe -e bash -c "rm -rf ~/.pvm 2>/dev/null"
			if ($LASTEXITCODE -eq 0) {
				Write-Host "  ✓ Removed WSL ~/.pvm"
			}
		}
	`
	encodedCmd := encodePowerShellCommand(psScript)
	cmd := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encodedCmd)
	_ = cmd.Run() // 尽力清理，忽略错误
}

// removePvmPathEntriesGeneric 从 shell rc 内容中移除独立的包含 pvm 路径的 PATH 条目行
// 通用版本，适用于 bash/zsh 等 POSIX shell 格式
func removePvmPathEntriesGeneric(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
			continue
		}
		// export PATH= 行包含 .pvm 或 /pvm/
		if strings.HasPrefix(trimmed, "export PATH=") {
			if strings.Contains(trimmed, ".pvm") || strings.Contains(trimmed, "/pvm/") {
				continue
			}
		}
		// PATH= 赋值行
		if strings.HasPrefix(trimmed, "PATH=") {
			if strings.Contains(trimmed, ".pvm") || strings.Contains(trimmed, "/pvm/") {
				continue
			}
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// removePvmHomeExportsGeneric 从 shell rc 内容中移除独立的 PVM_HOME export 行
// 通用版本
func removePvmHomeExportsGeneric(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result = append(result, line)
			continue
		}
		if strings.HasPrefix(trimmed, "export PVM_HOME=") {
			continue
		}
		if strings.HasPrefix(trimmed, "set -gx PVM_HOME") {
			continue
		}
		if strings.HasPrefix(trimmed, "setenv PVM_HOME") {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}
