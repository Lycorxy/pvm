package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/shim"
)

// runInit 创建版本配置文件
//
//	pvm init                     # 初始化用户全局配置 ~/.pvm/versions（默认 --user）
//	pvm init --user              # 同上，显式指定
//	pvm init --user node@20      # 初始化用户全局配置并设置版本
//	pvm init --local             # 在当前目录创建 .pvmrc
//	pvm init --local node@20     # 在当前目录创建 .pvmrc 并带初始版本
func runInit(args []string) error {
	args, userFlag := hasFlag(args, "--user", "-u")
	args, localFlag := hasFlag(args, "--local", "-l")

	if userFlag && localFlag {
		return fmt.Errorf("--user and --local cannot be used together")
	}

	// 默认 --user；只有显式 --local 才创建 .pvmrc
	if !localFlag {
		return runInitGlobal(args)
	}

	cwd, _ := os.Getwd()
	target := filepath.Join(cwd, ".pvmrc")

	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf(".pvmrc already exists at %s", target)
	}

	versions := make(map[string]string)

	if len(args) == 0 {
		// 默认模板
		logger.Info("  → Creating .pvmrc template at %s", target)
	} else {
		for _, arg := range args {
			rt, ver, err := parseRuntimeArg(arg)
			if err != nil {
				return err
			}
			versions[rt] = ver
		}
	}

	if err := config.SaveVersionFile(target, versions); err != nil {
		return err
	}

	// 如果内容为空，再追加占位注释让用户知道怎么用
	if len(versions) == 0 {
		content := `# pvm project runtime versions
# Syntax: <runtime> <version>
#
# Example:
#   node 20.11.0
#   python 3.12.0
#   go 1.22.0
#
# Then run: pvm install
`
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			return err
		}
	}

	logger.Info("  ✓ created %s", target)
	logger.Info("  → edit it and run: pvm install")
	return nil
}

// runInitGlobal 初始化用户全局配置 ~/.pvm/versions
func runInitGlobal(args []string) error {
	globalFile := config.GlobalVersionsFile()

	versions := make(map[string]string)
	for _, arg := range args {
		rt, ver, err := parseRuntimeArg(arg)
		if err != nil {
			return err
		}
		versions[rt] = ver
	}

	// 如果用户全局配置已存在，提示用户
	if _, err := os.Stat(globalFile); err == nil && len(versions) == 0 {
		logger.Info("  ℹ  User config already exists: %s", globalFile)
		logger.Info("  → Use `pvm use <runtime>@<version> --user` to update versions")
		logger.Info("  → Use `pvm list --user` to view current user config")
		return nil
	}

	if len(versions) > 0 {
		for rt, ver := range versions {
			if err := config.WriteGlobalVersion(rt, ver); err != nil {
				return fmt.Errorf("write user config: %w", err)
			}
			logger.Info("  ✓ set user %s = %s", rt, ver)
		}
		logger.Info("  → User config: %s", globalFile)
	} else {
		// 创建空的用户全局配置
		if err := config.EnsureDir(filepath.Dir(globalFile)); err != nil {
			return err
		}
		if err := config.SaveVersionFile(globalFile, versions); err != nil {
			return err
		}
		logger.Info("  ✓ initialized user config: %s", globalFile)
		logger.Info("  → Use `pvm use <runtime>@<version> --user` to set versions")
	}
	return nil
}

// runReshim 手动触发重建 shim
// 支持选项：--validate 先验证，--auto-fix 自动修复问题
func runReshim(args []string) error {
	validate := false
	autoFix := false
	for _, arg := range args {
		if arg == "--validate" || arg == "-v" {
			validate = true
		}
		if arg == "--auto-fix" || arg == "-f" {
			autoFix = true
		}
	}

	// 如果指定验证，先进行检查
	if validate {
		result := shim.ValidateAllShims()
		if !result.Valid {
			logger.Error("  ✗ Shim validation failed")
			if len(result.BrokenShims) > 0 {
				logger.Error("  Broken shims:")
				for _, b := range result.BrokenShims {
					logger.Error("    • %s", b)
				}
			}
			if !autoFix {
				logger.Info("  💡 Run: pvm reshim --auto-fix  (to auto-repair)")
				return fmt.Errorf("validation failed")
			}
		}
	}

	// 自动修复
	if autoFix {
		logger.Info("  🔧 Auto-repairing shims...")
		n, err := shim.AutoRepair()
		if err != nil {
			return fmt.Errorf("auto-repair failed: %w", err)
		}
		logger.Info("  ✓ Repaired %d issues", n)
	}

	// 标准 reshim
	if err := shim.Reshim(); err != nil {
		if shim.IsReshimWarning(err) {
			logger.Info("  ! reshim: %v (some files in use, will update on next run)", err)
		} else {
			return err
		}
	}
	logger.Info("  ✓ shims rebuilt at %s", config.ShimsDir())
	return nil
}

// runWhich 显示命令对应的真实二进制
func runWhich(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: pvm which <command>")
	}
	cmdName := args[0]
	rt := shim.FindRuntimeForCommand(cmdName)
	if rt == "" {
		return fmt.Errorf("pvm does not know which runtime provides %q", cmdName)
	}
	cwd, _ := os.Getwd()
	ver, _ := config.ResolveVersion(rt, cwd)
	if ver == "" {
		return fmt.Errorf("no version of %s is set for this directory", rt)
	}
	bin, err := shim.ResolveBinary(rt, ver, cmdName)
	if err != nil {
		return err
	}
	fmt.Println(bin)
	return nil
}

// runWhere 显示某个 runtime 的安装目录
func runWhere(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: pvm where <runtime>")
	}
	rt := strings.ToLower(args[0])
	if !config.IsSupportedRuntime(rt) {
		return fmt.Errorf("unsupported runtime: %s", rt)
	}
	cwd, _ := os.Getwd()
	ver, _ := config.ResolveVersion(rt, cwd)
	if ver == "" {
		return fmt.Errorf("no version of %s set", rt)
	}
	// system 版本：显示系统 PATH 中的真实路径
	if ver == config.SystemVersion {
		sysPath, err := config.SystemCommandPath(config.RuntimeMainCommand(rt))
		if err != nil {
			return fmt.Errorf("%s is set to system but not found in system PATH", rt)
		}
		fmt.Println(filepath.Dir(sysPath))
		return nil
	}
	fmt.Println(config.InstallDir(rt, ver))
	return nil
}
