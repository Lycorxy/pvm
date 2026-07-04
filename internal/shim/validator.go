package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pvm/pvm/internal/config"
)

// ValidateResult 表示验证结果
type ValidateResult struct {
	Valid        bool
	Errors       []string
	Warnings     []string
	BrokenShims  []string
	ConflictPaths []string
}

// ValidateAllShims 全面验证 shims 目录的完整性和有效性
// 检查项：
//  1. 所有 shim 指向的二进制是否存在
//  2. 是否有残留的旧格式 shim（.ps1、.bat）
//  3. 是否有版本不匹配的硬编码路径
//  4. PATH 中是否有冲突的版本管理器
func ValidateAllShims() ValidateResult {
	result := ValidateResult{
		Valid:        true,
		Errors:       []string{},
		Warnings:     []string{},
		BrokenShims:  []string{},
		ConflictPaths: []string{},
	}

	shimsDir := config.ShimsDir()
	if _, err := os.Stat(shimsDir); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("shims directory missing: %s", shimsDir))
		return result
	}

	entries, err := os.ReadDir(shimsDir)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("cannot read shims directory: %v", err))
		return result
	}

	// 1. 检查所有 .exe shim 指向的二进制
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		lower := strings.ToLower(name)

		// 跳过非 shim 文件和旧格式
		if !strings.HasSuffix(lower, ".exe") {
			if strings.HasSuffix(lower, ".ps1") || strings.HasSuffix(lower, ".bat") {
				result.Warnings = append(result.Warnings, fmt.Sprintf("legacy shim format: %s", name))
			}
			continue
		}

		// 提取命令名
		cmdName := name[:len(name)-4]

		// 尝试解析命令对应的 runtime
		rt := FindRuntimeForCommand(cmdName)
		if rt == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("cannot determine runtime for command: %s", cmdName))
			continue
		}

		// 获取当前版本
		version := ""
		v, _ := config.ResolveVersion(rt, "")
		version = v
		if version == "" || version == config.SystemVersion {
			// 没有设置版本，跳过验证（使用系统版本）
			continue
		}

		// 验证二进制是否存在
		bin, binErr := ResolveBinary(rt, version, cmdName)
		if binErr != nil {
			result.BrokenShims = append(result.BrokenShims, fmt.Sprintf("%s (runtime=%s@%s): %v", cmdName, rt, version, binErr))
			result.Valid = false
		}

		// 验证二进制是否真的存在
		if bin != "" {
			if _, err := os.Stat(bin); err != nil {
				result.BrokenShims = append(result.BrokenShims, fmt.Sprintf("%s points to missing file: %s", cmdName, bin))
				result.Valid = false
			}
		}
	}

	// 2. 检查 PATH 中的冲突
	result.ConflictPaths = detectPathConflicts()

	return result
}

// detectPathConflicts 检测 PATH 中可能与 pvm 冲突的路径
// 返回有冲突的路径列表
func detectPathConflicts() []string {
	var conflicts []string
	
	shimsDir := filepath.Clean(config.ShimsDir())
	pathEnv := os.Getenv("PATH")
	sep := string(os.PathListSeparator)

	shimsFound := false
	knownConflictBases := []string{"nodejs", "node", "python", "go", "golang", "ruby", "nvm", "volta", "fnm"}

	for _, p := range strings.Split(pathEnv, sep) {
		if p == "" {
			continue
		}
		cleaned := filepath.Clean(p)
		base := strings.ToLower(filepath.Base(cleaned))

		// 检查是否是 pvm shims
		if cleaned == shimsDir {
			shimsFound = true
			continue
		}

		// 检查是否在 shims 之前有冲突路径
		if !shimsFound {
			for _, conflict := range knownConflictBases {
				if strings.Contains(base, conflict) || strings.Contains(cleaned, conflict) {
					conflicts = append(conflicts, p)
					break
				}
			}
		}
	}

	return conflicts
}

// CheckVersionConsistency 检查运行时版本的一致性
// 验证 pvm 内部版本设置和实际安装的版本是否匹配
func CheckVersionConsistency(rt string) (string, error) {
	// 获取配置中的版本
	configVersion, _ := config.ResolveVersion(rt, "")

	if configVersion == "" || configVersion == config.SystemVersion {
		return config.SystemVersion, nil
	}

	// 验证该版本是否真的已安装
	if !config.IsInstalled(rt, configVersion) {
		return "", fmt.Errorf("%s@%s not installed (configured but missing)", rt, configVersion)
	}

	// 验证 bin 目录是否存在且有可执行文件
	binDir := config.BinDir(rt, configVersion)
	entries, readErr := os.ReadDir(binDir)
	if readErr != nil {
		return "", fmt.Errorf("cannot read bin dir %s: %w", binDir, readErr)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("bin dir is empty: %s", binDir)
	}

	return configVersion, nil
}

// CleanupBrokenShims 清理损坏的 shim 文件
// 返回清理的文件数和任何错误
func CleanupBrokenShims(result ValidateResult) (int, error) {
	shimsDir := config.ShimsDir()
	cleaned := 0

	// 删除残留的旧格式 shim
	entries, readErr := os.ReadDir(shimsDir)
	if readErr != nil {
		return 0, readErr
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		lower := strings.ToLower(name)

		// 删除 .ps1 和无后缀的 bat 脚本
		if strings.HasSuffix(lower, ".ps1") {
			path := filepath.Join(shimsDir, name)
			if rmErr := os.Remove(path); rmErr == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}

// AutoRepair 尝试自动修复已知的问题
// 返回修复的项目数和任何错误
func AutoRepair() (int, error) {
	result := ValidateAllShims()
	repaired := 0

	// 1. 清理旧格式 shim
	if len(result.Warnings) > 0 {
		n, err := CleanupBrokenShims(result)
		if err == nil {
			repaired += n
		}
	}

	// 2. 重新生成所有 shim（确保格式正确）
	if err := Reshim(); err != nil {
		// Reshim 返回警告时继续
		if !IsReshimWarning(err) {
			return repaired, err
		}
	}
	repaired++

	return repaired, nil
}
