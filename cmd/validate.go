package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/shim"
)

// runValidate 完整的 pvm 系统验证和修复
// 检查项：
//  1. shim 完整性
//  2. 版本一致性
//  3. PATH 冲突
//  4. 配置冲突
// 选项 --auto-fix 自动修复已知问题
func runValidate(args []string) error {
	autoFix := false
	for _, arg := range args {
		if arg == "--auto-fix" || arg == "-f" {
			autoFix = true
		}
	}

	fmt.Println()
	logger.Info("pvm system validation (version %s)", Version)
	fmt.Println()

	// 1. 验证 shims
	fmt.Println("  [1/4] Validating shims...")
	shimResult := shim.ValidateAllShims()
	printShimValidation(shimResult)

	if !shimResult.Valid {
		if autoFix {
			logger.Info("  🔧 Auto-fixing shims...")
			n, err := shim.AutoRepair()
			if err != nil {
				logger.Error("  ✗ Auto-repair failed: %v", err)
			} else {
				logger.Info("  ✓ Fixed %d issues", n)
			}
		}
	}

	// 2. 验证版本一致性
	fmt.Println()
	fmt.Println("  [2/4] Checking version consistency...")
	runtimes := []string{"node", "python", "go", "pnpm", "yarn", "git"}
	allOk := true
	for _, rt := range runtimes {
		version, err := shim.CheckVersionConsistency(rt)
		if err != nil {
			allOk = false
			logger.Error("  ✗ %s: %v", rt, err)
		} else if version != config.SystemVersion {
			logger.Info("  ✓ %s@%s", rt, version)
		}
	}

	// 3. 检查版本冲突
	fmt.Println()
	fmt.Println("  [3/4] Detecting version conflicts...")
	for _, rt := range runtimes {
		conflicts := config.DetectVersionConflicts(rt)
		if len(conflicts) > 0 {
			allOk = false
			for _, c := range conflicts {
				logger.Error("  ✗ %s: %s", rt, c.Message)
				if len(c.InstalledVers) > 0 {
					logger.Info("    Installed: %s", strings.Join(c.InstalledVers, ", "))
				}
			}
		}
	}

	// 4. 检查 PATH 冲突
	fmt.Println()
	fmt.Println("  [4/4] Checking PATH conflicts...")
	conflictPaths := shimResult.ConflictPaths
	if len(conflictPaths) > 0 {
		allOk = false
		logger.Error("  ✗ Found %d conflicting PATH entries:", len(conflictPaths))
		for _, p := range conflictPaths {
			logger.Info("    • %s", p)
		}
		logger.Info("  💡 Run: pvm setup  (as Administrator on Windows)")
	} else {
		logger.Info("  ✓ PATH is clean")
	}

	// 总结
	fmt.Println()
	if !allOk {
		logger.Error("⚠ Some issues detected")
		if !autoFix {
			logger.Info("💡 Run: pvm validate --auto-fix  (to auto-repair)")
		}
		return fmt.Errorf("validation failed")
	}

	logger.Info("✓ All validations passed!")
	return nil
}

// printShimValidation 打印 shim 验证结果
func printShimValidation(result shim.ValidateResult) {
	if result.Valid {
		logger.Info("  ✓ Shims are healthy")
		if len(result.Warnings) > 0 {
			for _, w := range result.Warnings {
				logger.Info("  ⚠ %s", w)
			}
		}
		return
	}

	logger.Error("  ✗ Shims have issues:")
	if len(result.Errors) > 0 {
		fmt.Println("    Errors:")
		for _, e := range result.Errors {
			logger.Error("      • %s", e)
		}
	}

	if len(result.BrokenShims) > 0 {
		fmt.Println("    Broken shims:")
		for _, b := range result.BrokenShims {
			logger.Error("      • %s", b)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println("    Warnings:")
		for _, w := range result.Warnings {
			logger.Info("      • %s", w)
		}
	}
}

// runDiagnostics 详细诊断（显示版本解析过程）
// 这是 doctor 的升级版，提供更详细的调试信息
func runDiagnostics(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: pvm diagnostics <runtime>")
	}

	rt := args[0]
	if !config.IsSupportedRuntime(rt) {
		return fmt.Errorf("unsupported runtime: %s", rt)
	}

	fmt.Println()
	logger.Info("Diagnostics for %s:", rt)
	fmt.Println()

	// 1. 版本解析过程
	logger.Info("  Version resolution chain:")
	diag, _ := config.ResolveVersionWithDiagnostics(rt, "")
	for _, check := range diag.Checked {
		logger.Info("    → %s", check)
	}
	logger.Info("  Final version: %s (source: %s)", diag.FinalVersion, diag.Source)
	fmt.Println()

	// 2. 已安装版本
	logger.Info("  Installed versions:")
	installed := config.ListInstalledVersions(rt)
	if len(installed) == 0 {
		logger.Info("    (none)")
	} else {
		for _, v := range installed {
			logger.Info("    • %s", v)
		}
	}
	fmt.Println()

	// 3. 版本冲突
	logger.Info("  Version conflicts:")
	conflicts := config.DetectVersionConflicts(rt)
	if len(conflicts) == 0 {
		logger.Info("    (none)")
	} else {
		for _, c := range conflicts {
			logger.Error("    ✗ %s", c.Message)
		}
	}
	fmt.Println()

	// 4. 二进制路径
	if diag.FinalVersion != config.SystemVersion {
		logger.Info("  Binary location:")
		bin, err := shim.ResolveBinary(rt, diag.FinalVersion, rt)
		if err != nil {
			logger.Error("    ✗ %v", err)
		} else {
			if _, err := os.Stat(bin); err != nil {
				logger.Error("    ✗ File not found: %s", bin)
			} else {
				logger.Info("    ✓ %s", bin)
			}
		}
	}

	return nil
}
