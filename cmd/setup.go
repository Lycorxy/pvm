package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
)

// isElevated checks if current process is running with administrator privileges (Windows only)
func isElevated() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "True"
}

// runAsAdmin relaunches pvm with administrator privileges (shows UAC prompt)
// Used to auto-fix system-level PATH conflicts that require admin rights
func runAsAdmin(args []string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("elevation only supported on Windows")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot get executable path: %w", err)
	}

	// Build argument list: original args + --elevated flag (prevent infinite loop)
	cmdArgs := append([]string{}, args...)
	alreadyElevated := false
	for _, a := range cmdArgs {
		if a == "--elevated" {
			alreadyElevated = true
			break
		}
	}
	if !alreadyElevated {
		cmdArgs = append(cmdArgs, "--elevated")
	}

	// Use ShellExecuteW with runas verb (shows UAC dialog)
	argStr := strings.Join(cmdArgs, " ")
	psCmd := "Start-Process -FilePath '" + exe + "' -ArgumentList '" + argStr + "' -Verb Runas -Wait"
	encoded := encodePowerShellCommand(psCmd)
	cmd := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encoded)

	logger.Info("")
	logger.Info("  -> System PATH conflict detected. Requesting administrator permission...")
	logger.Info("    (A UAC prompt will appear - please click 'Yes')")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("elevation failed (user declined or error): %w", err)
	}

	logger.Info("  OK System PATH fixed with administrator privileges")
	return nil
}

// runSetup 执行首次安装设置：
//  1. 创建 ~/.pvm 标准目录
//  2. 将 pvm 二进制复制到 ~/.pvm/bin/pvm
//  3. 执行 reshim
//  4. 将 ~/.pvm/shims 和 ~/.pvm/bin 加入用户 PATH
//  5. 设置 PVM_HOME 环境变量
func runSetup(args []string) error {
	_ = args // 目前不接受额外参数，保持签名一致性
	home := config.PvmHome()
	binHome := config.BinHome()
	shimsDir := config.ShimsDir()

	fmt.Println()
	fmt.Printf("  pvm setup (v%s, %s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	// ---- 1. 创建标准目录 ----
	logger.Info("  → Creating directories...")
	if err := config.EnsureAllDirs(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	logger.Info("  ✓ %s", home)
	logger.Info("  ✓ %s", binHome)
	logger.Info("  ✓ %s", shimsDir)

	// 清理残留的旧 .exe shim 文件（来自旧版 pvm 复制 pvm.exe 的策略）
	// 这些文件会导致 Windows 优先找到 .exe 而非新的 .cmd shim
	if runtime.GOOS == "windows" {
		oldShims, _ := os.ReadDir(shimsDir)
		cleaned := 0
		for _, e := range oldShims {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			// 清理 .exe shim（保留 .cmd）
			if strings.HasSuffix(strings.ToLower(name), ".exe") {
				path := filepath.Join(shimsDir, name)
				if err := os.Remove(path); err == nil {
					cleaned++
				}
			}
			// 清理旧的 .old 备份文件
			if strings.HasSuffix(strings.ToLower(name), ".old") {
				path := filepath.Join(shimsDir, name)
				os.Remove(path)
			}
		}
		if cleaned > 0 {
			logger.Info("  ✓ Cleaned %d legacy .exe shims from previous installation", cleaned)
		}
	}
	fmt.Println()

	// ---- 2. 将 pvm 二进制复制到 ~/.pvm/bin/ ----
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine current executable: %w", err)
	}
	currentExe = filepath.Clean(currentExe)
	targetExe := filepath.Join(binHome, "pvm"+config.ExeExt())

	needCopy := filepath.Clean(currentExe) != filepath.Clean(targetExe)
	if needCopy {
		logger.Info("  → Copying pvm to %s", targetExe)
		data, err := os.ReadFile(currentExe)
		if err != nil {
			return fmt.Errorf("cannot read current binary: %w", err)
		}

		// Windows 下目标 exe 可能正在运行（上次 setup 留下的），无法直接覆盖。
		// 策略：
		//   1. 先尝试直接 rename（最常见情况，目标未被锁定）
		//   2. 若失败（文件被占用），则将旧 exe 重命名为 .old，再写入新 exe
		//   3. .old 文件需要用户手动删除，或下次重启后自动清理
		tmpExe := targetExe + ".new"
		if err := os.WriteFile(tmpExe, data, 0755); err != nil {
			return fmt.Errorf("cannot write to %s: %w", tmpExe, err)
		}

		// 尝试直接替换
		_ = os.Remove(targetExe) // 忽略错误，文件被占用时会失败
		if err := os.Rename(tmpExe, targetExe); err != nil {
			// 文件被占用，使用 .old 备份策略
			oldExe := targetExe + ".old"
			// 先删除可能存在的 .old 文件
			_ = os.Remove(oldExe)
			// 将正在运行的 exe 重命名为 .old（Windows 允许 rename 正在运行的 exe）
			if renameErr := os.Rename(targetExe, oldExe); renameErr == nil {
				// 成功重命名旧文件，现在可以写入新文件
				if err := os.Rename(tmpExe, targetExe); err != nil {
					_ = os.Remove(tmpExe)
					return fmt.Errorf("cannot install pvm to %s: %w", targetExe, err)
				}
				logger.Info("  ✓ pvm installed to %s (old version backed up to %s)", targetExe, oldExe)
				logger.Info("    (you can delete %s after verification)", oldExe)
			} else {
				// 连 rename 都失败，可能是权限问题
				_ = os.Remove(tmpExe)
				return fmt.Errorf("cannot install pvm to %s (file is locked): %w\n"+
					"  Try: move %s %s.old, then run pvm setup again", targetExe, renameErr, targetExe, targetExe)
			}
		} else {
			logger.Info("  ✓ pvm installed to %s", targetExe)
		}
	} else {
		logger.Info("  ✓ pvm already in %s", targetExe)
	}

	// 安装 pvm-shim 二进制（统一 shim 方案：被复制/链接为各命令名 node/git/go...）
	installPvmShim(currentExe, binHome)
	fmt.Println()

	// ---- 3. 执行 reshim ----
	logger.Info("  → Running reshim...")
	if err := runReshim(nil); err != nil {
		logger.Info("  ⚠ reshim had warnings (safe to ignore on first setup)")
	} else {
		logger.Info("  ✓ shim scripts created")
	}
	fmt.Println()

	// ---- 4. 配置 PATH ----
	logger.Info("  → Configuring PATH...")
	if runtime.GOOS == "windows" {
		if err := setupWindowsPath(binHome, shimsDir); err != nil {
			return err
		}
	} else {
		if err := setupUnixPath(binHome, shimsDir); err != nil {
			return err
		}
	}
	fmt.Println()

	// ---- 5. 设置 PVM_HOME 环境变量 ----
	logger.Info("  → Setting PVM_HOME environment variable...")
	if runtime.GOOS == "windows" {
		if err := os.Setenv("PVM_HOME", home); err == nil {
			// 持久化到用户环境变量
			_ = exec.Command("setx", "PVM_HOME", home).Run() // 忽略错误
			logger.Info("  ✓ PVM_HOME = %s", home)
		}
	} else {
		// Unix: PVM_HOME 已经在 rc 文件中通过 setupUnixPath 设置
		logger.Info("  ✓ PVM_HOME = %s", home)
	}
	fmt.Println()

	// ---- Done ----
	fmt.Println("  =======================================")
	fmt.Println("  OK pvm setup complete!")
	fmt.Println("  =======================================")
	fmt.Println()
	fmt.Println("  Next steps:")
	if runtime.GOOS == "windows" {
		fmt.Println("    1. Close and reopen your terminal (PowerShell/CMD)")
		fmt.Println()
		fmt.Println("  !! Important for VS Code / CodeBuddy / other editors:")
		fmt.Println("     Editors cache environment variables at startup.")
		fmt.Println("     You MUST fully restart (not just reload window):")
		fmt.Println("       * VS Code: File -> Exit (or kill all Code processes in Task Manager)")
		fmt.Println("       * Then reopen VS Code - pvm will work in integrated terminal")
		fmt.Println()
		fmt.Println("     Quick verify after reopening editor terminal:")
		fmt.Println("       pvm -v        # should show version")
		fmt.Println("       pvm doctor   # should show all checks passed OK")
	} else {
		shellName := filepath.Base(os.Getenv("SHELL"))
		if shellName == "" {
			shellName = "bash"
		}
		fmt.Printf("    1. Restart your shell (or: source ~/.%src)\n", shellName)
	}
	fmt.Println()
	fmt.Println("    2. pvm install node@20.11.0")
	fmt.Println("    3. pvm doctor              # verify installation")
	fmt.Println()

	return nil
}

// installPvmShim 安装 pvm-shim 二进制到 binHome。
// pvm-shim 是统一的命令转发器：被复制（Windows）或符号链接（Unix）为各命令名
// （node、npm、git、go、pnpm...），启动后通过自身文件名识别命令，委托 pvm 主程序
// 解析版本并执行真实二进制。
//
// 所有平台都需要 pvm-shim：Windows reshim 时复制为 <cmd>.exe；Unix reshim 时
// symlink <cmd> → pvm-shim。
func installPvmShim(currentExe, binHome string) {
	ext := config.ExeExt()
	targetName := "pvm-shim" + ext
	targetPath := filepath.Join(binHome, targetName)

	// 查找 pvm-shim 源文件
	var shimSrc string

	// 1. 当前可执行文件同目录（MSI 安装位置 / 打包目录）
	exeDir := filepath.Dir(currentExe)
	if localShim := filepath.Join(exeDir, targetName); localShim != targetPath {
		if _, err := os.Stat(localShim); err == nil {
			shimSrc = localShim
		}
	}

	// 2. dist 目录（开发构建产物）
	if shimSrc == "" {
		devShim := filepath.Join("dist", targetName)
		if abs, err := filepath.Abs(devShim); err == nil {
			if _, err := os.Stat(abs); err == nil {
				shimSrc = abs
			}
		}
	}

	// 3. cmd/shim 目录（go build -o ./cmd/shim/ 的场景）
	if shimSrc == "" {
		devShim := filepath.Join("cmd", "shim", targetName)
		if abs, err := filepath.Abs(devShim); err == nil {
			if _, err := os.Stat(abs); err == nil {
				shimSrc = abs
			}
		}
	}

	// 4. 目标已存在且是最新的，跳过
	if shimSrc == "" {
		if _, err := os.Stat(targetPath); err == nil {
			logger.Info("  ✓ pvm-shim already installed: %s", targetPath)
			return
		}
		logger.Info("  ⚠ pvm-shim binary not found")
		logger.Info("    Build it first: go build -o %s ./cmd/shim", filepath.Join("dist", targetName))
		logger.Info("    Shims will not work until pvm-shim is installed in %s", binHome)
		return
	}

	// 源与目标相同则跳过
	if absSrc, _ := filepath.Abs(shimSrc); absSrc != "" {
		if absTgt, _ := filepath.Abs(targetPath); absTgt == absSrc {
			logger.Info("  ✓ pvm-shim already in place: %s", targetPath)
			return
		}
	}

	data, err := os.ReadFile(shimSrc)
	if err != nil {
		logger.Info("  ⚠ cannot read pvm-shim: %v", err)
		return
	}

	// 写入目标：先写临时文件再原子 rename，避免覆盖运行中的二进制
	tmp := targetPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0755); err != nil {
		logger.Info("  ⚠ cannot write pvm-shim: %v", err)
		return
	}
	if err := os.Rename(tmp, targetPath); err != nil {
		// rename 失败（目标可能正在运行）：先备份旧文件再写入
		_ = os.Remove(tmp)
		backup := targetPath + ".old"
		_ = os.Remove(backup)
		if renameErr := os.Rename(targetPath, backup); renameErr == nil {
			if err := os.Rename(tmp, targetPath); err != nil {
				_ = os.Rename(backup, targetPath) // 回滚
				logger.Info("  ⚠ cannot install pvm-shim: %v", err)
				return
			}
		} else {
			logger.Info("  ⚠ cannot replace pvm-shim (in use): %v", renameErr)
			return
		}
	}

	logger.Info("  ✓ pvm-shim installed to %s", targetPath)
}

// setupWindowsPath 在 Windows 上将 shims 和 bin 目录加入用户 PATH
// 并确保它们排在最前面，同时检测并移除冲突的系统 runtime 路径
func setupWindowsPath(binHome, shimsDir string) error {
	// 读取用户级 PATH
	regPath, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('Path','User')`).Output()
	if err != nil {
		return fmt.Errorf("cannot read user PATH: %w", err)
	}
	pathStr := strings.TrimRight(string(regPath), "\r\n") // 去掉 \r\n

	// 已知会与 pvm shims 冲突的系统 runtime 目录名
	// 注意：只匹配安装程序常见的目录名，避免误删
	// - "nodejs": Node.js 官方安装路径 (C:\Program Files\nodejs)
	//   nvm for Windows 也会在此创建 symlink，指向当前激活的 node 版本
	// - "Python3xx" / "Pythonxx": Python 官方安装路径 (C:\Users\xxx\AppData\Local\Programs\Python\Python312)
	//   通过前缀匹配 "Python" 来覆盖所有版本号目录
	conflictDirNames := []string{
		"nodejs",     // Node.js 官方安装 / nvm for Windows symlink
		"node",       // 部分安装器使用 node 目录名
		"nvm",        // nvm for Windows 安装目录 (C:\Program Files\nvm)
		"python",     // Python (Python312, Python39 等)
		"go",         // Go
		"golang",     // Go (alternative)
		"ruby",       // Ruby
		"rubydevkit", // Ruby DevKit
	}

	entries := splitPath(pathStr)
	changed := false
	conflictsRemoved := []string{}

	// 1. 移除用户 PATH 中与 pvm 冲突的目录
	var filtered []string
	for _, e := range entries {
		base := strings.ToLower(filepath.Base(filepath.Clean(e)))
		isConflict := false
		for _, c := range conflictDirNames {
			if strings.EqualFold(base, c) {
				isConflict = true
				break
			}
			// 前缀匹配：处理 Python 版本号目录 (如 Python312, Python39)
			if len(base) > len(c) && strings.EqualFold(base[:len(c)], c) {
				isConflict = true
				break
			}
		}
		if isConflict && !containsPathEntryList([]string{shimsDir, binHome}, e) {
			conflictsRemoved = append(conflictsRemoved, e)
		} else {
			filtered = append(filtered, e)
		}
	}
	entries = filtered

	// 2. 将 shims 和 bin 前置到最前面（即使已存在也要确保在最前面）
	pvmDirs := []string{shimsDir, binHome}
	for _, dir := range pvmDirs {
		// 先移除已有条目
		var without []string
		for _, e := range entries {
			if !strings.EqualFold(filepath.Clean(e), filepath.Clean(dir)) {
				without = append(without, e)
			}
		}
		entries = without
	}
	// 前置加入
	entries = append(pvmDirs, entries...)
	changed = true // 总是更新，确保顺序正确

	if len(conflictsRemoved) > 0 {
		logger.Info("  ⚠ Removed conflicting paths from user PATH:")
		for _, c := range conflictsRemoved {
			logger.Info("    - %s", c)
		}
		logger.Info("  (pvm shims will take priority instead)")
	}

	if changed {
		newPath := joinPath(entries)
		// 使用 UTF-8 编码的脚本块，避免特殊字符（如括号、&、'）导致 PowerShell 解析失败
		// 同时广播 WM_SETTINGCHANGE 消息，让其他进程感知环境变量变更
		// 使用 -EncodedCommand 避免 $ 变量在某些 shell 环境中被替换
		escapedPath := escapePowerShellString(newPath)
		psScript := fmt.Sprintf(`$newPath = '%s'; `+
			`[Environment]::SetEnvironmentVariable('Path', $newPath, 'User'); `+
			`Write-Host "  ✓ User PATH updated (pvm shims first)"; `+
			`# 广播环境变量变更消息，让其他进程感知 `+
			`Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; ' + `+
			`'public class Win32Helper { [DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Auto)] ' + `+
			`'public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, ' + `+
			`'uint fuFlags, uint uTimeout, out UIntPtr lpdwResult); }' -ErrorAction SilentlyContinue; `+
			`try { $HWND_BROADCAST = [IntPtr]0xffff; $WM_SETTINGCHANGE = 0x001A; $result = [UIntPtr]::Zero; `+
			`'Win32Helper'::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null; `+
			`Write-Host "  ✓ Environment change broadcasted" } catch {}`,
			escapedPath)
		encodedCmd := encodePowerShellCommand(psScript)
		cmd := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encodedCmd)
		if err := cmd.Run(); err != nil {
			// 如果 -EncodedCommand 也失败，尝试简化命令
			simpleScript := fmt.Sprintf(`[Environment]::SetEnvironmentVariable('Path', '%s', 'User')`, escapedPath)
			encodedSimple := encodePowerShellCommand(simpleScript)
			cmd2 := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encodedSimple)
			if err2 := cmd2.Run(); err2 != nil {
				return fmt.Errorf("cannot update user PATH: %w (also tried alternative: %v)", err, err2)
			}
		}
		logger.Info("  ✓ User PATH updated (pvm shims first)")
		logger.Info("  ℹ User-level PATH modified (no admin required, local machine PATH not changed)")
	}

	// 同步当前会话
	os.Setenv("PATH", shimsDir+";"+binHome+";"+os.Getenv("PATH"))
	logger.Info("  ✓ Current session PATH updated")

	// 3. 检测并修复系统级 PATH（Machine）中的冲突目录
	// Windows 合并 PATH 顺序为：系统 PATH + 用户 PATH
	// 若系统 PATH 中存在冲突的 runtime 目录，它们会排在用户 PATH 的 shims 前面，导致 shim 失效
	if err := fixSystemPathConflicts(conflictDirNames); err != nil {
		logger.Info("  ⚠ Could not auto-fix system PATH conflicts: %v", err)
	}

	return nil
}

// ensureGitBashInUserPath 在安装/切换 git 后，将 git 的 bin 目录前置加入用户级 PATH，
// 让 VSCode 自动识别 Git Bash 终端（无需用户手动配置 VSCode，无需管理员权限）。
//
// 关键原理：VSCode 是用户进程，读取合并后的 PATH（系统+用户）。它在 Windows 上检测
// Git Bash 的条件是 bash.exe 的路径包含 "git"。
//   - ~/.pvm/bin/bash.exe        → 路径不含 "git"，VSCode 不识别
//   - ~/.pvm/installs/git/current/bin/bash.exe → 路径含 "git"，VSCode 自动识别 ✓
//
// 前置到用户 PATH 最前面，确保 bash.exe 优先命中含 "git" 的路径（而非 shims/bash.exe）。
// git.exe 能自定位 git-core（--exec-path 返回 mingw64/libexec/git-core），直接使用功能完整。
// current junction 保证版本切换时路径不变。不影响 node/python 等其他 runtime。
//
// 幂等（已在前置位置则跳过）。不需要管理员权限。不影响 pvm setup 原有行为。
func ensureGitBashInUserPath() {
	if runtime.GOOS != "windows" {
		return
	}
	// git current bin 路径（含 "git"，VSCode 据此识别 Git Bash）
	gitBinDir := filepath.Join(config.InstallsDir(), "git", "current", "bin")

	// 读取用户级 PATH
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('Path','User')`).Output()
	if err != nil {
		logger.Info("  ⚠ Could not read user PATH for Git Bash setup: %v", err)
		return
	}
	userPathStr := strings.TrimRight(string(out), "\r\n")
	entries := splitPath(userPathStr)

	// 幂等：已在前置位置则跳过
	if len(entries) > 0 && strings.EqualFold(filepath.Clean(entries[0]), filepath.Clean(gitBinDir)) {
		return
	}

	// 移除已有的 gitBinDir 条目（可能是非前置位置），然后前置
	var without []string
	for _, e := range entries {
		if !strings.EqualFold(filepath.Clean(e), filepath.Clean(gitBinDir)) {
			without = append(without, e)
		}
	}
	newEntries := append([]string{gitBinDir}, without...)
	newPath := joinPath(newEntries)

	// 写入用户级 PATH 并广播 WM_SETTINGCHANGE，让 VSCode 等进程感知变更
	// 使用 -EncodedCommand 避免 $ 变量在某些 shell 环境中被替换
	escapedPath := escapePowerShellString(newPath)
	psScript := fmt.Sprintf(`$newPath = '%s'; `+
		`[Environment]::SetEnvironmentVariable('Path', $newPath, 'User'); `+
		`Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; ' + `+
		`'public class Win32Helper { [DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Auto)] ' + `+
		`'public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, ' + `+
		`'uint fuFlags, uint uTimeout, out UIntPtr lpdwResult); }' -ErrorAction SilentlyContinue; `+
		`try { $HWND_BROADCAST = [IntPtr]0xffff; $WM_SETTINGCHANGE = 0x001A; $result = [UIntPtr]::Zero; `+
		`'Win32Helper'::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null } catch {}`,
		escapedPath)
	encodedCmd := encodePowerShellCommand(psScript)
	if err := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encodedCmd).Run(); err != nil {
		logger.Info("  ⚠ Could not update user PATH for Git Bash: %v", err)
		return
	}
	logger.Info("  ✓ User PATH updated — VSCode will auto-detect Git Bash")
	logger.Info("    (added: %s)", gitBinDir)
}

// knownVersionManagers 根据冲突路径识别已安装的版本管理工具，返回工具名和卸载指引
// 返回 map[工具名]卸载说明
func detectVersionManagers(conflictPaths []string) map[string]string {
	result := map[string]string{}
	for _, p := range conflictPaths {
		pLower := strings.ToLower(filepath.ToSlash(p))
		switch {
		case strings.Contains(pLower, "nvm") || strings.Contains(pLower, "/nodejs"):
			result["nvm"] = "nvm uninstall <version>  (or uninstall nvm via its installer)"
		case strings.Contains(pLower, "volta"):
			result["volta"] = "volta uninstall node  (or uninstall Volta via its installer)"
		case strings.Contains(pLower, "fnm"):
			result["fnm"] = "fnm uninstall <version>  (or remove fnm from PATH)"
		case strings.Contains(pLower, "nodenv"):
			result["nodenv"] = "nodenv uninstall <version>  (or remove nodenv)"
		case strings.Contains(pLower, "asdf"):
			result["asdf"] = "asdf uninstall nodejs <version>  (or remove asdf)"
		case strings.Contains(pLower, "pyenv"):
			result["pyenv"] = "pyenv uninstall <version>  (or remove pyenv)"
		case strings.Contains(pLower, "conda") || strings.Contains(pLower, "anaconda") || strings.Contains(pLower, "miniconda"):
			result["conda"] = "conda deactivate  (or uninstall Anaconda/Miniconda)"
		}
	}
	return result
}

// fixSystemPathConflicts detects system-level PATH conflicts and auto-fixes them
// Strategy: if running as admin -> directly modify system PATH to move conflicting paths to end
//            if not admin -> auto-elevate via UAC prompt to re-run setup with admin rights
func fixSystemPathConflicts(conflictDirNames []string) error {
	sysPath, err := exec.Command("powershell", "-NoProfile", "-Command",
		"[Environment]::GetEnvironmentVariable('Path','Machine')").Output()
	if err != nil {
		return nil // cannot read system PATH, skip
	}
	sysPathStr := strings.TrimRight(string(sysPath), "\r\n")

	sysEntries := splitPath(sysPathStr)
	var sysConflicts []string
	for _, e := range sysEntries {
		base := strings.ToLower(filepath.Base(filepath.Clean(e)))
		for _, c := range conflictDirNames {
			if strings.EqualFold(base, c) ||
				(len(base) > len(c) && strings.EqualFold(base[:len(c)], c)) {
				sysConflicts = append(sysConflicts, e)
				break
			}
		}
	}

	if len(sysConflicts) == 0 {
		return nil // no conflicts
	}

	fmt.Println()
	logger.Info("  !! Conflicting runtime paths found in SYSTEM PATH:")
	for _, c := range sysConflicts {
		logger.Info("     - %s", c)
	}
	logger.Info("  These system paths override pvm shims.")
	fmt.Println()

	// Check if already running as administrator
	if isElevated() {
		// Already admin: directly fix system PATH
		logger.Info("  OK Running as Administrator - auto-fixing system PATH...")
		return fixSystemPathDirectly(sysEntries, sysConflicts)
	}

	// Not admin: auto-elevate
	logger.Info("  >> Auto-fixing by requesting administrator permission...")
	logger.Info("    (A UAC prompt will appear - please click 'Yes')")
	fmt.Println()

	if err := runAsAdmin([]string{"setup"}); err != nil {
		// Elevation failed (user declined or error), show manual steps
		logger.Info("  !! Auto-fix cancelled. Manual steps required:")
		return printManualFixGuide(sysConflicts)
	}

	logger.Info("  OK System PATH conflicts resolved!")
	return nil
}

// fixSystemPathDirectly modifies system-level PATH with admin privileges
// Moves conflicting runtime directories to the end of system PATH
func fixSystemPathDirectly(sysEntries, sysConflicts []string) error {
	conflictSet := make(map[string]bool)
	for _, c := range sysConflicts {
		conflictSet[filepath.Clean(c)] = true
	}

	// Filter: separate non-conflicting and conflicting paths
	var nonConflict []string
	var movedConflicts []string

	for _, e := range sysEntries {
		cleaned := filepath.Clean(e)
		if conflictSet[cleaned] {
			movedConflicts = append(movedConflicts, e)
		} else {
			nonConflict = append(nonConflict, e)
		}
	}

	// Put conflicting paths at the end (user PATH shims will take priority)
	newEntries := append(nonConflict, movedConflicts...)
	newPath := joinPath(newEntries)

	// Write to system-level PATH and broadcast change
	escapedPath := escapePowerShellString(newPath)
	psScript := "$newPath = '" + escapedPath + "'; " +
		"[Environment]::SetEnvironmentVariable('Path', $newPath, 'Machine'); " +
		"Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; ' + " +
		"'public class Win32Helper { [DllImport(\"user32.dll\", SetLastError=true, CharSet=CharSet.Auto)] ' + " +
		"'public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, ' + " +
		"'uint fuFlags, uint uTimeout, out UIntPtr lpdwResult); }' -ErrorAction SilentlyContinue; " +
		"try { $HWND_BROADCAST = [IntPtr]0xffff; $WM_SETTINGCHANGE = 0x001A; $result = [UIntPtr]::Zero; " +
		"'Win32Helper'::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null } catch {}"
	encodedCmd := encodePowerShellCommand(psScript)
	cmd := exec.Command("powershell", "-NoProfile", "-EncodedCommand", encodedCmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update system PATH: %w", err)
	}

	if len(movedConflicts) > 0 {
		logger.Info("  OK Moved %d conflicting path(s) to end of system PATH:", len(movedConflicts))
		for _, c := range movedConflicts {
			logger.Info("      - %s", c)
		}
	}
	return nil
}

// printManualFixGuide shows manual repair steps when auto-elevation fails
func printManualFixGuide(sysConflicts []string) error {
	detected := detectVersionManagers(sysConflicts)

	if len(detected) > 0 {
		logger.Info("  Detected conflicting version managers:")
		for tool := range detected {
			logger.Info("    * %s is installed and conflicts with pvm", tool)
		}
		fmt.Println()
	}

	logger.Info("  Option A - Uninstall the conflicting tool(s) [Recommended]:")
	for tool, guide := range detected {
		logger.Info("      %s: %s", tool, guide)
	}
	fmt.Println()

	logger.Info("  Option B - Manually remove from SYSTEM PATH (requires Administrator):")
	logger.Info("      1. Right-click Start -> System -> Advanced system settings")
	logger.Info("      2. Environment Variables -> System variables -> Path -> Edit")
	logger.Info("      3. Remove or move down these entries:")
	for _, c := range sysConflicts {
		logger.Info("         - %s", c)
	}
	logger.Info("      4. Click OK and reopen your terminal")
	fmt.Println()

	logger.Info("  Option C - Retry auto-fix: run this command in Administrator terminal:")
	logger.Info("      pvm setup")
	fmt.Println()

	return fmt.Errorf("system PATH conflicts require manual intervention")
}

// setupUnixPath 在 macOS/Linux 上将 pvm 配置写入 shell rc 文件
func setupUnixPath(binHome, shimsDir string) error {
	shellName := filepath.Base(os.Getenv("SHELL"))
	if shellName == "" {
		shellName = "bash"
	}

	var rcFiles []string
	switch shellName {
	case "bash":
		rcFiles = []string{".bashrc", ".bash_profile"}
	case "zsh":
		rcFiles = []string{".zshrc"}
	case "fish":
		rcFiles = []string{".config/fish/config.fish"}
	default:
		rcFiles = []string{"." + shellName + "rc"}
	}

	pvmHome := config.PvmHome()
	anyConfigured := false

	for _, rcRel := range rcFiles {
		rcPath := filepath.Join(os.Getenv("HOME"), rcRel)
		data, err := os.ReadFile(rcPath)
		if err != nil {
			// 文件不存在，创建
			if err := os.MkdirAll(filepath.Dir(rcPath), 0755); err != nil {
				continue
			}
			data = []byte{}
		}

		// 检查是否已经配置
		if strings.Contains(string(data), "PVM_HOME") {
			logger.Info("  ✓ Already configured in ~/%s", rcRel)
			anyConfigured = true
			continue
		}

		// 追加配置
		var snippet string
		if shellName == "fish" {
			snippet = fmt.Sprintf("\n# pvm (Polyglot Version Manager)\nset -gx PVM_HOME %q\nset -gx PATH $PVM_HOME/shims $PVM_HOME/bin $PATH\n", pvmHome)
		} else {
			snippet = fmt.Sprintf("\n# pvm (Polyglot Version Manager)\nexport PVM_HOME=%q\nexport PATH=\"$PVM_HOME/shims:$PVM_HOME/bin:$PATH\"\n", pvmHome)
		}

		f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Info("  ⚠ Cannot write to %s: %v", rcPath, err)
			continue
		}
		if _, err := f.WriteString(snippet); err != nil {
			logger.Verbose("  write to %s failed: %v", rcRel, err)
		}
		f.Close()
		logger.Info("  ✓ Configured in ~/%s", rcRel)
		anyConfigured = true
	}

	// 同步当前会话
	os.Setenv("PVM_HOME", pvmHome)
	os.Setenv("PATH", shimsDir+":"+binHome+":"+os.Getenv("PATH"))

	if !anyConfigured {
		fmt.Println("  ⚠ Could not auto-configure shell. Add these lines to your shell rc file:")
		fmt.Printf("    export PVM_HOME=%q\n", pvmHome)
		fmt.Println("    export PATH=\"$PVM_HOME/shims:$PVM_HOME/bin:$PATH\"")
	}

	return nil
}

// runSetupPath 仅处理 PATH 配置，不做其他初始化
// 用于修复已安装但 PATH 不正确的情况
func runSetupPath(args []string) error {
	shimsDir := config.ShimsDir()
	binHome := config.BinHome()

	// 解析标志
	_, checkFlag := hasFlag(args, "--check", "-c")

	fmt.Println()
	fmt.Printf("  pvm setup-path (checking PATH configuration)\n")
	fmt.Println()

	if runtime.GOOS == "windows" {
		return setupPathWindows(shimsDir, binHome, checkFlag)
	}
	return setupPathUnix(shimsDir, binHome, checkFlag)
}

// setupPathWindows 检查并修复 Windows PATH
func setupPathWindows(shimsDir, binHome string, checkOnly bool) error {
	// 读取用户级 PATH
	regPath, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('Path','User')`).Output()
	if err != nil {
		return fmt.Errorf("cannot read user PATH: %w", err)
	}
	pathStr := strings.TrimRight(string(regPath), "\r\n")
	entries := splitPath(pathStr)

	// 检查 shims 是否在最前面
	needsFix := false
	shimsIdx := -1

	for i, e := range entries {
		if strings.EqualFold(filepath.Clean(e), filepath.Clean(shimsDir)) {
			shimsIdx = i
		}
	}

	if shimsIdx == -1 {
		logger.Info("  ✗ %s not in PATH", shimsDir)
		needsFix = true
	} else if shimsIdx > 0 {
		logger.Info("  ⚠ %s is in PATH but not at the first position (position %d)", shimsDir, shimsIdx+1)
		needsFix = true
	} else {
		logger.Info("  ✓ %s is at the first position in PATH", shimsDir)
	}

	// 检查系统 PATH 中是否有冲突的 runtime 目录
	sysPath, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('Path','Machine')`).Output()
	if err == nil {
		sysPathStr := strings.TrimRight(string(sysPath), "\r\n")
		sysEntries := splitPath(sysPathStr)

		conflictDirNames := []string{"nodejs", "node", "python", "go", "golang", "ruby", "rubydevkit"}
		var conflicts []string

		for _, e := range sysEntries {
			base := strings.ToLower(filepath.Base(filepath.Clean(e)))
			for _, c := range conflictDirNames {
				if strings.EqualFold(base, c) ||
					(len(base) > len(c) && strings.EqualFold(base[:len(c)], c)) {
					conflicts = append(conflicts, e)
					break
				}
			}
		}

		if len(conflicts) > 0 {
			logger.Info("  ⚠ Found conflicting system runtime paths:")
			for _, c := range conflicts {
				logger.Info("    - %s", c)
			}
			needsFix = true
		}
	}

	fmt.Println()

	if !needsFix {
		logger.Info("  ✓ PATH is correctly configured!")
		return nil
	}

	if checkOnly {
		logger.Info("  (use 'pvm setup-path' without --check to fix)")
		return fmt.Errorf("PATH configuration needs fixing")
	}

	logger.Info("  → Fixing PATH configuration...")
	if err := setupWindowsPath(binHome, shimsDir); err != nil {
		return err
	}
	fmt.Println()
	logger.Info("  ✓ PATH fixed. Please reopen your terminal for changes to take effect.")
	return nil
}

// setupPathUnix 检查并修复 Unix PATH
func setupPathUnix(shimsDir, binHome string, checkOnly bool) error {
	// 检查 shell rc 文件
	shellName := filepath.Base(os.Getenv("SHELL"))
	if shellName == "" {
		shellName = "bash"
	}

	var rcFiles []string
	switch shellName {
	case "bash":
		rcFiles = []string{".bashrc", ".bash_profile"}
	case "zsh":
		rcFiles = []string{".zshrc"}
	case "fish":
		rcFiles = []string{".config/fish/config.fish"}
	default:
		rcFiles = []string{"." + shellName + "rc"}
	}

	homeDir := os.Getenv("HOME")
	configured := false

	for _, rcRel := range rcFiles {
		rcPath := filepath.Join(homeDir, rcRel)
		data, _ := os.ReadFile(rcPath)
		if strings.Contains(string(data), "PVM_HOME") {
			configured = true
			logger.Info("  ✓ Configured in ~/%s", rcRel)
			break
		}
	}

	if !configured {
		logger.Info("  ✗ PVM not configured in shell rc files")
		if checkOnly {
			logger.Info("  (use 'pvm setup-path' without --check to fix)")
			return fmt.Errorf("PATH configuration needs fixing")
		}

		if err := setupUnixPath(binHome, shimsDir); err != nil {
			return err
		}
		logger.Info("  ✓ Shell rc files configured. Please restart your shell.")
		return nil
	}

	logger.Info("  ✓ PATH is correctly configured!")
	return nil
}

// shouldAutoSetup 判断是否应该自动触发 setup
// 当 pvm 不在 ~/.pvm/bin/ 下运行时返回 true
func shouldAutoSetup() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	exe = filepath.Clean(exe)
	expected := filepath.Clean(filepath.Join(config.BinHome(), "pvm"+config.ExeExt()))
	return exe != expected
}

// containsPathEntryList 检查 PATH 条目列表中是否包含某个目录
func containsPathEntryList(entries []string, dir string) bool {
	cleanDir := filepath.Clean(dir)
	for _, e := range entries {
		if filepath.Clean(e) == cleanDir {
			return true
		}
		// Windows 不区分大小写
		if runtime.GOOS == "windows" && strings.EqualFold(e, cleanDir) {
			return true
		}
	}
	return false
}

// splitPath 按 OS 分隔符拆分 PATH
func splitPath(pathStr string) []string {
	if pathStr == "" {
		return nil
	}
	var result []string
	for _, p := range filepath.SplitList(pathStr) {
		p = filepath.Clean(p)
		if p != "" && p != "." {
			result = append(result, p)
		}
	}
	return result
}

// joinPath 用 OS 分隔符连接 PATH 条目
func joinPath(entries []string) string {
	result := ""
	for i, e := range entries {
		if i > 0 {
			result += string(os.PathListSeparator)
		}
		result += e
	}
	return result
}

// escapePowerShellString 转义 PowerShell 单引号字符串中的特殊字符
// PowerShell 单引号字符串中，只有 ' 需要转义为 ”
func escapePowerShellString(s string) string {
	// 单引号字符串：' 转义为 ''
	s = strings.ReplaceAll(s, "'", "''")
	return s
}

// encodePowerShellCommand 将 PowerShell 脚本编码为 Base64（UTF-16LE）
// 用于 -EncodedCommand 参数，避免 $ 变量在某些 shell 环境中被替换
func encodePowerShellCommand(script string) string {
	// PowerShell 要求 UTF-16LE 编码
	runes := []rune(script)
	utf16Codes := utf16.Encode(runes)
	// 转换为 []byte（每个 uint16 是 2 bytes，小端序）
	buf := make([]byte, len(utf16Codes)*2)
	for i, u := range utf16Codes {
		buf[i*2] = byte(u)
		buf[i*2+1] = byte(u >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
