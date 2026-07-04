package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pvm/pvm/internal/compat"
	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/installer"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/shim"
)

// runInstall 安装一个或多个 runtime 版本
//
// 用法：
//
//	pvm install                          # 从项目 .pvmrc 安装全部（无则从用户全局）
//	pvm install --local                  # 强制从项目 .pvmrc 安装
//	pvm install --user                   # 从全局 ~/.pvm/versions 安装全部
//	pvm install node@20.11.0             # 安装并设为用户级默认（默认 --user）
//	pvm install node@20.11.0 --local     # 安装并写入项目 .pvmrc
//	pvm install node@20.11.0 --official  # 强制使用官方源下载
//	pvm install --mirror ...             # 使用国内镜像（已是默认，可省略）
//	pvm install --force node@20.11.0     # 强制覆盖安装
func runInstall(args []string) error {
	args, useMirrorFlag := hasFlag(args, "--mirror", "-m")
	args, useOfficial := hasFlag(args, "--official", "-o")
	args, forceInstall := hasFlag(args, "--force", "-f")
	args, userFlag := hasFlag(args, "--user", "-u")
	args, localFlag := hasFlag(args, "--local", "-l", "--save")
	args, systemFlag := hasFlag(args, "--system")

	if userFlag && localFlag {
		return fmt.Errorf("--user and --local cannot be used together")
	}

	// --system 不能用于安装：系统版本无需下载
	if systemFlag {
		return fmt.Errorf(
			"--system cannot be used with install (system versions are not downloaded).\n" +
				"  Use `pvm use <runtime> --system` to switch to the system-installed version")
	}

	// 默认使用国内镜像，--official 强制官方源，--mirror 显式指定镜像（已是默认）
	useMirror := !useOfficial || useMirrorFlag

	// 没有版本参数 → 从声明文件安装
	if len(args) == 0 {
		if userFlag {
			return installFromGlobal(useMirror, forceInstall)
		}
		if localFlag {
			return installFromDeclaration(useMirror, forceInstall)
		}
		// 默认：先尝试项目 .pvmrc，没有再回退到用户全局
		return installFromDeclarationOrGlobal(useMirror, forceInstall)
	}

	// 有版本参数：默认 --user（保存到用户全局），--local 时写入项目
	saveUser := !localFlag
	saveLocal := localFlag

	if err := config.EnsureAllDirs(); err != nil {
		return err
	}

	hadError := false
	for _, arg := range args {
		rt, ver, err := parseRuntimeArg(arg)
		if err != nil {
			logger.Error("  ✗ %s", err)
			hadError = true
			continue
		}
		// npm/npx/corepack 是 Node.js 自带工具，安装 node 即可获得
		// parseRuntimeArg 已将它们映射为 "node"，这里给出友好提示
		origArg := strings.ToLower(strings.SplitN(arg, "@", 2)[0])
		if origArg == "npm" || origArg == "npx" || origArg == "corepack" {
			logger.Info("  ℹ  %s is bundled with Node.js — installing node instead", origArg)
		}
		// 拦截旧的 @system 语法
		if ver == config.SystemVersion {
			logger.Error(
				"  ✗ the @system syntax is no longer supported.\n"+
					"    Use: pvm use %s --system", rt)
			hadError = true
			continue
		}

		// Git/Go 走工具型安装（安装到 ~/.pvm/installs，通过 shim + current junction 管理）
		// 参考 Scoop 架构：不需要管理员权限，VSCode 通过 shim 找到命令
		if installer.IsToolRuntime(rt) {
			if saveLocal {
				logger.Error("  ✗ %s does not support project-level installation (--local)", rt)
				logger.Info("  → %s is installed globally and does not require version management per project", rt)
				hadError = true
				continue
			}

			info, err := installer.InstallTool(rt, ver, useMirror, forceInstall)
			if err != nil {
				logger.Error("  ✗ %s: %v", rt, err)
				hadError = true
				continue
			}

			// 安装成功后重建 shims
			if err := shim.Reshim(); err != nil {
				if shim.IsReshimWarning(err) {
					logger.Info("  ! reshim: %v (file in use, will update on next run)", err)
				} else {
					logger.Error("  ! Reshim failed: %v", err)
				}
			}

			// 同时在 ~/.pvm/bin/ 中创建 .exe（VSCode 等 IDE 需要）
			if rt == "git" || rt == "go" {
				if err := config.SetCurrent(rt, info.Version); err != nil {
					logger.Info("  ! could not create %s.exe in bin: %v", rt, err)
				}
				// git: 把 git current bin 加入用户级 PATH，让 VSCode 自动识别 Git Bash
				// （路径含 "git"，VSCode 检测条件；无需管理员权限）
				if rt == "git" {
					ensureGitBashInUserPath()
				}
			}

			logger.Info("")
			logger.Info("  📍 Installed to: %s", info.InstallPath)
			logger.Info("  🔗 Current junction: %s", info.CurrentPath)
			logger.Info("  📝 Shims directory (in PATH): %s", config.ShimsDir())
			logger.Info("")
			logger.Info("  💡 Tips:")
			logger.Info("     - Restart your terminal/IDE for PATH changes to take effect")
			logger.Info("     - VSCode will now recognize %s commands automatically", rt)
			continue
		}

		// 检查前置依赖
		if err := compat.CheckPrerequisites(rt, ver, func(depRt string) string {
			cwd, _ := os.Getwd()
			v, _ := config.ResolveVersion(depRt, cwd)
			return v
		}); err != nil && !forceInstall {
			logger.Error("  %v", err)
			hadError = true
			continue
		}

		// 安装前验证版本信息
		if err := config.ValidateVersionInstall(rt, ver); err != nil {
			logger.Info("  ⚠  %v", err)
			// 仅作警告，继续安装
		}

		installed, err := installer.Install(rt, ver, useMirror, forceInstall)

		// 如果没有指定 --mirror 且出现 403/GitHub API 错误，自动降级到镜像源重试
		if err != nil && !useMirror && (strings.Contains(err.Error(), "403") ||
			strings.Contains(err.Error(), "GitHub API returned 403")) {
			logger.Info("  ⚠  GitHub API rate limited, retrying with mirror...")
			installed, err = installer.Install(rt, ver, true, forceInstall)
		}

		if err != nil {
			logger.Error("  ✗ %s@%s: %v", rt, ver, err)
			hadError = true
			continue
		}
		ver = installed // 使用安装后的精确版本号

		// 写入配置：默认用户级，--local 时写入项目级
		if saveLocal {
			cwd, _ := os.Getwd()
			homeDir, _ := os.UserHomeDir()
			if cwd == homeDir {
				logger.Info("  ℹ  current directory is home dir, writing to user config instead")
				if err := config.WriteGlobalVersion(rt, ver); err != nil {
					logger.Error("  ! write user config: %v", err)
				} else {
					logger.Info("  ✓ set user %s = %s (%s)", rt, ver, config.GlobalVersionsFile())
				}
			} else {
				path, err := config.WriteProjectVersion(cwd, rt, ver)
				if err != nil {
					logger.Error("  ! write project config: %v", err)
				} else {
					logger.Info("  ✓ set %s = %s in %s", rt, ver, path)
				}
			}
		} else if saveUser {
			if err := config.WriteGlobalVersion(rt, ver); err != nil {
				logger.Error("  ! write user config: %v", err)
			} else {
				logger.Info("  ✓ set user %s = %s (%s)", rt, ver, config.GlobalVersionsFile())
			}
		}

		// 安装后验证：确保安装结果可用
		if err := verifyInstallation(rt, ver); err != nil {
			logger.Error("  ✗ %s@%s verification failed: %v", rt, ver, err)
			hadError = true
		} else {
			logger.Info("  ✓ %s@%s verified", rt, ver)
		}
	}

	// 重建 shim（新装的版本可能带来新命令）
	if err := shim.Reshim(); err != nil {
		if shim.IsReshimWarning(err) {
			logger.Info("  ! reshim: %v (file in use, will update on next run)", err)
		} else {
			logger.Error("  ! Reshim failed: %v", err)
		}
	}

	if hadError {
		return fmt.Errorf("one or more installs failed")
	}
	return nil
}

// installFromDeclaration 从当前目录向上查找 .pvmrc 并安装
func installFromDeclaration(useMirror, forceInstall bool) error {
	cwd, _ := os.Getwd()
	vf, err := config.FindVersionFile(cwd)
	if err != nil {
		return err
	}
	if vf == nil || len(vf.Versions) == 0 {
		logger.Info("No .pvmrc found from %s", cwd)
		logger.Info("")
		logger.Info("Run `pvm init` to create one, or specify a runtime explicitly:")
		logger.Info("  pvm install node@20.11.0")
		return nil
	}

	logger.Info("  → Installing from %s", vf.Path)
	if err := config.EnsureAllDirs(); err != nil {
		return err
	}

	for _, rt := range config.SupportedRuntimes {
		ver, ok := vf.Versions[rt]
		if !ok || ver == "" {
			continue
		}
		if ver == config.SystemVersion {
			logger.Info("  ℹ  %s = system (skipping install)", rt)
			continue
		}
		if _, err := installer.Install(rt, ver, useMirror, forceInstall); err != nil {
			logger.Error("  ✗ %s@%s: %v", rt, ver, err)
		}
	}

	if err := shim.Reshim(); err != nil {
		if shim.IsReshimWarning(err) {
			logger.Info("  ! reshim: %v (file in use, will update on next run)", err)
		} else {
			logger.Error("  ! Reshim failed: %v", err)
		}
	}
	return nil
}

// installFromGlobal 从用户配置 ~/.pvm/versions 安装所有声明的版本
func installFromGlobal(useMirror, forceInstall bool) error {
	globalFile := config.GlobalVersionsFile()
	vf, err := config.LoadVersionFile(globalFile)
	if err != nil || vf == nil || len(vf.Versions) == 0 {
		logger.Info("No user versions configured in %s", globalFile)
		logger.Info("")
		logger.Info("Set a user default first:")
		logger.Info("  pvm use node@20.11.0 --user")
		return nil
	}

	logger.Info("  → Installing from user config (%s)", globalFile)
	if err := config.EnsureAllDirs(); err != nil {
		return err
	}

	for _, rt := range config.SupportedRuntimes {
		ver, ok := vf.Versions[rt]
		if !ok || ver == "" {
			continue
		}
		if ver == config.SystemVersion {
			logger.Info("  ℹ  %s = system (skipping install)", rt)
			continue
		}
		if _, err := installer.Install(rt, ver, useMirror, forceInstall); err != nil {
			logger.Error("  ✗ %s@%s: %v", rt, ver, err)
		}
	}

	if err := shim.Reshim(); err != nil {
		if shim.IsReshimWarning(err) {
			logger.Info("  ! reshim: %v (file in use, will update on next run)", err)
		} else {
			logger.Error("  ! Reshim failed: %v", err)
		}
	}
	return nil
}

// installFromDeclarationOrGlobal 默认安装策略：
//  1. 优先从项目 .pvmrc 安装
//  2. 项目无配置时，回退到用户全局 ~/.pvm/versions
func installFromDeclarationOrGlobal(useMirror, forceInstall bool) error {
	cwd, _ := os.Getwd()
	vf, err := config.FindVersionFile(cwd)
	if err == nil && vf != nil && len(vf.Versions) > 0 {
		return installFromDeclaration(useMirror, forceInstall)
	}

	// 没有项目 .pvmrc，尝试用户全局
	globalFile := config.GlobalVersionsFile()
	gvf, gerr := config.LoadVersionFile(globalFile)
	if gerr == nil && gvf != nil && len(gvf.Versions) > 0 {
		logger.Info("  ℹ  no .pvmrc found, falling back to user config")
		return installFromGlobal(useMirror, forceInstall)
	}

	logger.Info("No version configuration found.")
	logger.Info("")
	logger.Info("Get started with one of:")
	logger.Info("  pvm install node@20.11.0          # install + set user default")
	logger.Info("  pvm init                          # create .pvmrc for this project")
	return nil
}

// verifyInstallation 验证安装是否成功可用
// 检查 bin 目录存在且核心可执行文件就位，避免"安装成功但实际无法使用"的隐患
func verifyInstallation(rt, version string) error {
	binDir := config.BinDir(rt, version)
	// 检查 bin 目录是否存在
	if _, err := os.Stat(binDir); err != nil {
		return fmt.Errorf("bin directory missing: %s", binDir)
	}
	// 检查核心可执行文件是否存在
	var exeName string
	switch rt {
	case "node":
		exeName = "node.exe"
	case "python":
		exeName = "python.exe"
	case "go":
		exeName = "go.exe"
	case "git":
		exeName = "git.exe"
	case "pnpm":
		exeName = "pnpm.cmd"
	case "yarn":
		exeName = "yarn.cmd"
	}
	if exeName != "" {
		if runtime.GOOS != "windows" {
			exeName = strings.TrimSuffix(exeName, ".exe")
			exeName = strings.TrimSuffix(exeName, ".cmd")
		}
		exePath := filepath.Join(binDir, exeName)
		if _, err := os.Stat(exePath); err != nil {
			return fmt.Errorf("core executable missing: %s", exePath)
		}
	}
	return nil
}
