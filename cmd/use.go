package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/installer"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/shim"
)

// runUse 设置当前使用的版本
//
// 用法：
//
//	pvm use node@20.11.0              # 智能判断：项目目录有 .pvmrc 则写项目级，否则写用户级
//	pvm use node@20.11.0 --user       # 显式用户级（写入 ~/.pvm/versions）
//	pvm use node@20.11.0 --local      # 显式项目级（写入 .pvmrc）
//	pvm use node --system             # 使用系统 PATH 中已安装的版本（用户级）
//	pvm use node --system --local     # 使用系统版本（项目级）
//	pvm use                           # 显示当前生效的版本配置
//	pvm use --user                    # 显示用户级版本配置
//	pvm use --local                   # 显示项目级版本配置
func runUse(args []string) error {
	args, userFlag := hasFlag(args, "--user", "-u")
	args, localFlag := hasFlag(args, "--local", "-l")
	args, systemFlag := hasFlag(args, "--system", "-s")

	// --user 与 --local 不可同时指定
	if userFlag && localFlag {
		return fmt.Errorf("--user and --local cannot be used together")
	}

	// 作用域决策（智能判断）：
	//   - 显式 --local → 项目级
	//   - 显式 --user  → 用户级
	//   - 都没指定    → 如果当前目录（向上查找）存在 .pvmrc，默认项目级；否则用户级
	//     这样在项目中运行 `pvm use node@20` 会自动更新 .pvmrc，符合直觉
	scopeUser := !localFlag // 不指定或 --user 都视为 user
	if !userFlag && !localFlag {
		cwd, _ := os.Getwd()
		homeDir, _ := os.UserHomeDir()
		// 家目录不算项目目录
		if cwd != homeDir {
			if vf, _ := config.FindVersionFile(cwd); vf != nil {
				scopeUser = false
				localFlag = true
				logger.Info("  ℹ  detected %s, writing to project config (use --user to override)", vf.Path)
			}
		}
	}

	// 无参数：显示当前生效的版本配置
	if len(args) == 0 {
		if localFlag {
			return runListProject("")
		}
		// 默认 / --user：显示用户级配置
		if scopeUser && (userFlag || !systemFlag) {
			return runListGlobal("")
		}
		return runUseCurrent()
	}

	// 检查是否使用镜像标志
	args, useMirrorFlag := hasFlag(args, "--mirror", "-m")

	for _, arg := range args {
		rt, ver, err := parseRuntimeArg(arg)
		if err != nil {
			return err
		}

		// 拦截旧的 @system 语法，引导用户使用 --system
		if ver == config.SystemVersion {
			return fmt.Errorf(
				"the @system syntax is no longer supported.\n"+
					"  Use --system flag instead:\n"+
					"    pvm use %s --system           # user-level (default)\n"+
					"    pvm use %s --system --local   # project-level",
				rt, rt)
		}

		// --system 与 @<version> 不能同时指定
		if systemFlag && ver != "latest" {
			return fmt.Errorf(
				"--system cannot be combined with a version number (got %s@%s).\n"+
					"  Either pick a pvm-managed version: pvm use %s@%s\n"+
					"  Or use the system-installed one:    pvm use %s --system",
				rt, ver, rt, ver, rt)
		}

		// git 只支持用户全局配置，不写入 .pvmrc
		if config.IsGlobalOnly(rt) {
			if localFlag {
				logger.Info("  ℹ  %s is a user-only runtime, switching to --user automatically", rt)
				localFlag = false
				scopeUser = true
			}
		}

		// --system：使用系统 PATH 中已安装的版本
		if systemFlag {
			if !config.SystemHasRuntime(rt) {
				return fmt.Errorf("%s is not found in system PATH. Install it first", rt)
			}
			if err := writeScopedVersion(rt, config.SystemVersion, scopeUser); err != nil {
				return err
			}
			continue
		}

		// 确保版本已安装（installer.Install 内部会：
		//   1. 解析模糊版本 → 精确版本
		//   2. 检查 pnpm 与 node 兼容性
		//   3. 幂等检查：已安装则跳过，不会重复安装
		// 一个版本在 ~/.pvm/installs/ 下只存在一份，项目/用户/系统层共享）

		// 安装前版本验证
		if err := config.ValidateVersionInstall(rt, ver); err != nil {
			logger.Info("  ⚠  %v", err)
		}

		// 工具型运行时（git/go）走 InstallTool，确保 current junction 正确
		// InstallTool 会创建 ~/.pvm/installs/<rt>/current/ junction，shim.exe 需要它
		var installed string
		if installer.IsToolRuntime(rt) {
			info, installErr := installer.InstallTool(rt, ver, useMirrorFlag, false)
			if installErr != nil && !useMirrorFlag && (strings.Contains(installErr.Error(), "403") ||
				strings.Contains(installErr.Error(), "504") ||
				strings.Contains(installErr.Error(), "502") ||
				strings.Contains(installErr.Error(), "500") ||
				strings.Contains(installErr.Error(), "timeout") ||
				strings.Contains(installErr.Error(), "Timeout") ||
				strings.Contains(installErr.Error(), "GitHub API returned")) {
				logger.Info("  ⚠  GitHub API error, retrying with mirror... [国内镜像]")
				info, installErr = installer.InstallTool(rt, ver, true, false)
			}
			if installErr != nil {
				return fmt.Errorf("install %s@%s: %w", rt, ver, installErr)
			}
			installed = info.Version
		} else {
			var installErr error
			installed, installErr = installer.Install(rt, ver, useMirrorFlag, false)

			// 如果没有指定 --mirror 且出现 403/504/超时/GitHub API 错误，自动降级到镜像源重试
			if installErr != nil && !useMirrorFlag && (strings.Contains(installErr.Error(), "403") ||
				strings.Contains(installErr.Error(), "504") ||
				strings.Contains(installErr.Error(), "502") ||
				strings.Contains(installErr.Error(), "500") ||
				strings.Contains(installErr.Error(), "timeout") ||
				strings.Contains(installErr.Error(), "Timeout") ||
				strings.Contains(installErr.Error(), "GitHub API returned")) {
				logger.Info("  ⚠  GitHub API error, retrying with mirror... [国内镜像]")
				installed, installErr = installer.Install(rt, ver, true, false)
			}

			if installErr != nil {
				return fmt.Errorf("install %s@%s: %w", rt, ver, installErr)
			}
		}
		ver = installed // 使用安装后的精确版本号写入配置

		// 安装后刷新 shim（新版本可能带来新命令）
		if err := shim.Reshim(); err != nil {
			if shim.IsReshimWarning(err) {
				logger.Info("  ! reshim: %v (file in use, will update on next run)", err)
			} else {
				logger.Error("  ! reshim: %v", err)
			}
		}

		if err := writeScopedVersion(rt, ver, scopeUser); err != nil {
			return err
		}

		// 对于需要被 IDE 直接识别的运行时（git/python/go），
		// 将关键 exe 复制到 ~/.pvm/bin（该目录在 setup 时已加入 PATH）。
		// 这样 IDE（如 VS Code）可直接找到真实的 git.exe/python.exe/go.exe，
		// 无需动态修改 PATH，最稳定可靠。
		if rt == "git" || rt == "python" || rt == "go" {
			if err := config.SetCurrent(rt, ver); err != nil {
				logger.Info("  ⚠ could not copy %s to bin: %v", rt, err)
			} else {
				logger.Info("  ✓ %s.exe copied to ~/.pvm/bin (IDE-ready)", rt)
			}
			// git: 把 git current bin 加入用户级 PATH，让 VSCode 自动识别 Git Bash
			// （路径含 "git"，VSCode 检测条件；无需管理员权限）
			if rt == "git" {
				ensureGitBashInUserPath()
			}
		}
	}

	// 写完配置后自检：shims 是否在 PATH 中且优先级最高
	// 否则用户会以为切换成功，但实际命令仍走系统旧版本
	warnIfShimsNotInPath()
	return nil
}

// warnIfShimsNotInPath 检查 shims 目录是否生效，未生效则给出醒目提示
// 复用 doctor 中的 checkShimsInPath，避免重复实现
func warnIfShimsNotInPath() {
	ok, msg := checkShimsInPath()
	if ok {
		return
	}
	logger.Info("")
	logger.Info("  ⚠  WARNING: pvm shims are not effective on your PATH")
	logger.Info("     reason: %s", msg)
	logger.Info("     The version you just set will NOT take effect until this is fixed.")
	if runtime.GOOS == "windows" {
		logger.Info("     → Right-click terminal → Run as administrator → run: pvm setup")
	} else {
		logger.Info("     → Run: pvm setup     (then reopen the terminal)")
	}
	logger.Info("     → Verify with: pvm doctor")
}

// writeScopedVersion 根据作用域写入版本配置
//   - scopeUser=true：写入 ~/.pvm/versions（用户级）
//   - scopeUser=false：写入当前目录 .pvmrc（项目级）
//
// 注意：当 cwd 为家目录时，强制走用户级，避免在家目录创建 .pvmrc
func writeScopedVersion(rt, ver string, scopeUser bool) error {
	displayVer := ver
	if ver == config.SystemVersion {
		displayVer = "system"
	}

	if scopeUser {
		if err := config.WriteGlobalVersion(rt, ver); err != nil {
			return fmt.Errorf("write user config: %w", err)
		}
		logger.Info("  ✓ set user %s = %s (%s)", rt, displayVer, config.GlobalVersionsFile())
		return nil
	}

	// 项目级：处理家目录特殊情况
	cwd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()
	if cwd == homeDir {
		logger.Info("  ℹ  current directory is home dir, writing to user config instead")
		if err := config.WriteGlobalVersion(rt, ver); err != nil {
			return fmt.Errorf("write user config: %w", err)
		}
		logger.Info("  ✓ set user %s = %s (%s)", rt, displayVer, config.GlobalVersionsFile())
		return nil
	}

	path, err := config.WriteProjectVersion(cwd, rt, ver)
	if err != nil {
		return fmt.Errorf("write project version: %w", err)
	}
	logger.Info("  ✓ set %s = %s in %s", rt, displayVer, path)
	return nil
}



// runUseCurrent 无参数时显示当前目录生效的版本（类似 nvm use 无参数）
func runUseCurrent() error {
	cwd, _ := os.Getwd()
	fmt.Printf("Current versions (cwd: %s):\n", cwd)
	for _, rt := range config.SupportedRuntimes {
		ver, src := config.ResolveVersion(rt, cwd)
		if ver == "" {
			fmt.Printf("  %-7s (not set)\n", rt)
			continue
		}
		if ver == config.SystemVersion {
			sysPath, err := config.SystemCommandPath(config.RuntimeMainCommand(rt))
			if err != nil {
				fmt.Printf("  %-7s system   [%s]\n", rt, srcLabel(src))
			} else {
				fmt.Printf("  %-7s system   [%s]  (%s)\n", rt, srcLabel(src), sysPath)
			}
			continue
		}
		installed := ""
		if !config.IsInstalled(rt, ver) {
			installed = "  ⚠ not installed"
		}
		fmt.Printf("  %-7s %-12s [%s]%s\n", rt, ver, srcLabel(src), installed)
	}
	return nil
}
