package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VersionConflict 表示版本冲突信息
type VersionConflict struct {
	Runtime    string
	ConfigVer  string
	SystemVer  string
	InstalledVers []string
	Message    string
}

// DetectVersionConflicts 检测多个版本管理来源的冲突
// 检查项：
//  1. .pvmrc（项目级）vs ~/.pvm/versions（用户级）
//  2. 配置版本 vs 实际安装的版本
//  3. 环境变量 vs 配置文件
func DetectVersionConflicts(rt string) []VersionConflict {
	var conflicts []VersionConflict

	// 1. 检查环境变量是否与配置冲突
	envVar := fmt.Sprintf("PVM_%s_VERSION", strings.ToUpper(rt))
	envVersion := os.Getenv(envVar)

	// 2. 获取最终生效的版本（已考虑 env > project > user > system 优先级）
	configVer, _ := ResolveVersion(rt, "")

	// 3. 列出所有已安装版本
	installedVers := ListInstalledVersions(rt)

	// 4. 检查配置的版本是否已安装
	if configVer != "" && configVer != SystemVersion {
		if !contains(installedVers, configVer) {
			conflicts = append(conflicts, VersionConflict{
				Runtime:    rt,
				ConfigVer:  configVer,
				SystemVer:  SystemVersion,
				InstalledVers: installedVers,
				Message:    fmt.Sprintf("configured version %s not installed", configVer),
			})
		}
	}

	// 5. 检查环境变量冲突
	if envVersion != "" && configVer != "" && envVersion != configVer {
		conflicts = append(conflicts, VersionConflict{
			Runtime:    rt,
			ConfigVer:  configVer,
			SystemVer:  envVersion,
			InstalledVers: installedVers,
			Message:    fmt.Sprintf("env var %s=%s conflicts with config %s", envVar, envVersion, configVer),
		})
	}

	// 注：项目级 (.pvmrc) 与用户级 (~/.pvm/versions) 是独立的配置层，
	// 版本不同是正常现象（不同项目可以用不同版本），不视为冲突。
	// 全局安装的版本在项目未配置 .pvmrc 时会作为 fallback 使用。

	return conflicts
}

// ResolveVersionWithDiagnostics 解析版本，并返回诊断信息
// 这是 ResolveVersion 的增强版本，提供更多调试信息
type ResolutionDiagnostics struct {
	FinalVersion  string
	Source        string // "env", "local", "user", "system"
	Checked       []string // 检查过的路径
	Warnings      []string
	Conflicts     []VersionConflict
}

func ResolveVersionWithDiagnostics(rt, cwd string) (ResolutionDiagnostics, error) {
	diag := ResolutionDiagnostics{
		Checked: []string{},
	}

	// 1. 检查环境变量
	envVar := fmt.Sprintf("PVM_%s_VERSION", strings.ToUpper(rt))
	if env := os.Getenv(envVar); env != "" {
		diag.FinalVersion = env
		diag.Source = "env"
		diag.Checked = append(diag.Checked, fmt.Sprintf("env:%s=%s", envVar, env))
		diag.Conflicts = DetectVersionConflicts(rt)
		return diag, nil
	}

	// 2. 检查项目级配置
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	pvmrcPath := searchPvmrc(cwd)
	if pvmrcPath != "" {
		if ver := getVersionFromFile(pvmrcPath, rt); ver != "" {
			diag.FinalVersion = ver
			diag.Source = "local"
			diag.Checked = append(diag.Checked, fmt.Sprintf("local:%s=%s", pvmrcPath, ver))
			diag.Conflicts = DetectVersionConflicts(rt)
			return diag, nil
		}
		diag.Checked = append(diag.Checked, fmt.Sprintf("local:%s (no entry)", pvmrcPath))
	}

	// 3. 检查用户级配置
	globalFile := GlobalVersionsFile()
	if ver := getVersionFromFile(globalFile, rt); ver != "" {
		diag.FinalVersion = ver
		diag.Source = "user"
		diag.Checked = append(diag.Checked, fmt.Sprintf("user:%s=%s", globalFile, ver))
		diag.Conflicts = DetectVersionConflicts(rt)
		return diag, nil
	}
	diag.Checked = append(diag.Checked, fmt.Sprintf("user:%s (no entry)", globalFile))

	// 4. 使用系统版本
	diag.FinalVersion = SystemVersion
	diag.Source = "system"
	diag.Checked = append(diag.Checked, "fallback:system")
	diag.Conflicts = DetectVersionConflicts(rt)

	return diag, nil
}

// ListInstalledVersions 列出某个 runtime 的所有已安装版本
func ListInstalledVersions(rt string) []string {
	var versions []string
	rtDir := filepath.Join(InstallsDir(), rt)
	entries, err := os.ReadDir(rtDir)
	if err != nil {
		return versions
	}

	for _, e := range entries {
		if e.IsDir() {
			if IsInstalled(rt, e.Name()) {
				versions = append(versions, e.Name())
			}
		}
	}

	return versions
}

// getVersionFromFile 从配置文件中提取某个 runtime 的版本
// 支持格式：runtime=version 或 runtime version
func getVersionFromFile(path, rt string) string {
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	rtLower := strings.ToLower(rt)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 格式 1: runtime=version
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.ToLower(strings.TrimSpace(parts[0])) == rtLower {
				return strings.TrimSpace(parts[1])
			}
		}

		// 格式 2: runtime version（空格分隔）
		parts := strings.Fields(line)
		if len(parts) >= 2 && strings.ToLower(parts[0]) == rtLower {
			return parts[1]
		}
	}

	return ""
}

// searchPvmrc 向上搜索 .pvmrc 文件（从 cwd 开始）
func searchPvmrc(cwd string) string {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	current := cwd
	for {
		pvmrcPath := filepath.Join(current, ".pvmrc")
		if _, err := os.Stat(pvmrcPath); err == nil {
			return pvmrcPath
		}

		parent := filepath.Dir(current)
		if parent == current {
			// 到达根目录
			break
		}
		current = parent
	}

	return ""
}

// contains 判断切片是否包含字符串
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ValidateVersionInstall 在安装前验证版本信息是否有冲突
// 返回是否可以安全安装
func ValidateVersionInstall(rt, version string) error {
	// 1. 检查版本冲突
	conflicts := DetectVersionConflicts(rt)
	if len(conflicts) > 0 {
		var warnings []string
		for _, c := range conflicts {
			warnings = append(warnings, c.Message)
		}
		// 仅作为警告，不阻断安装
		if len(warnings) > 0 {
			return fmt.Errorf("version conflicts detected: %s (continuing anyway)", strings.Join(warnings, "; "))
		}
	}

	// 2. 检查是否已安装
	if IsInstalled(rt, version) {
		return nil // 幂等：已安装就不用再装
	}

	return nil
}
