package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 版本声明文件名（仅 .pvmrc）
var VersionFileNames = []string{".pvmrc"}

// VersionFile 表示一个版本声明文件
type VersionFile struct {
	Path     string            // 文件绝对路径
	Versions map[string]string // runtime -> version
}

// FindVersionFile 从 startDir 向上查找第一个版本声明文件
// 返回 nil 表示未找到
// 注意：家目录（~）下的 .pvmrc 不视为项目配置，全局配置统一使用 ~/.pvm/versions
func FindVersionFile(startDir string) (*VersionFile, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}

	// 获取家目录，家目录的 .pvmrc 不是项目配置（全局配置在 ~/.pvm/versions）
	homeDir, _ := os.UserHomeDir()
	homeDir, _ = filepath.Abs(homeDir)

	for {
		// 到达家目录时停止向上查找（家目录及以上不是项目目录）
		if homeDir != "" && strings.EqualFold(dir, homeDir) {
			return nil, nil
		}

		// 查找 .pvmrc
		for _, name := range VersionFileNames {
			p := filepath.Join(dir, name)
			if fileExists(p) {
				return LoadVersionFile(p)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil // 到达根目录
		}
		dir = parent
	}
}

// LoadVersionFile 解析版本声明文件（.pvmrc）
// 支持两种语法：
//
//	node 20.11.0          （空格分隔，推荐）
//	node=20.11.0          （等号分隔，向后兼容）
//
// 注释行以 # 开头
func LoadVersionFile(path string) (*VersionFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vf := &VersionFile{
		Path:     path,
		Versions: make(map[string]string),
	}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var rt, ver string
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			rt = strings.TrimSpace(parts[0])
			ver = strings.TrimSpace(parts[1])
		} else {
			// 空格/制表符分隔
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			rt = parts[0]
			ver = parts[1]
		}

		rt = strings.ToLower(rt)
		ver = strings.TrimPrefix(ver, "v")

		// 兼容 nodejs / golang 别名写法
		switch rt {
		case "nodejs":
			rt = "node"
		case "golang":
			rt = "go"
		}

		if IsSupportedRuntime(rt) {
			vf.Versions[rt] = ver
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}
	return vf, nil
}

// SaveVersionFile 写出版本声明文件（空格分隔）
// isUser=true（用户全局配置路径）时允许写入全局 only runtime（如 git）；项目级跳过
func SaveVersionFile(path string, versions map[string]string) error {
	isUser := path == GlobalVersionsFile()

	var header string
	if isUser {
		header = "# pvm user default versions"
	} else {
		header = "# pvm project runtime versions"
	}

	var lines []string
	lines = append(lines, header)

	// 固定顺序输出
	for _, rt := range SupportedRuntimes {
		// 项目文件跳过全局 only runtime（如 git）
		if !isUser && IsGlobalOnly(rt) {
			continue
		}
		if v, ok := versions[rt]; ok && v != "" {
			lines = append(lines, fmt.Sprintf("%s %s", rt, v))
		}
	}
	lines = append(lines, "")

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// SystemVersion 是一个特殊版本标记，表示使用系统 PATH 中全局安装的版本
const SystemVersion = "system"

// ResolveVersion 解析当前目录应该使用的版本
// 查找顺序：
//  1. 环境变量 PVM_<RT>_VERSION（例如 PVM_NODE_VERSION）
//  2. 向上查找 .pvmrc（项目级 local 配置）
//  3. 用户全局默认版本 ~/.pvm/versions（user 配置）
//  4. 系统 PATH 中的全局安装（system fallback，返回 "system"）
//
// 返回 (version, sourcePath)；version="system" 表示使用系统全局安装
func ResolveVersion(rt, startDir string) (string, string) {
	// 1) 环境变量
	envKey := fmt.Sprintf("PVM_%s_VERSION", strings.ToUpper(rt))
	if v := os.Getenv(envKey); v != "" {
		return strings.TrimPrefix(v, "v"), "env:" + envKey
	}

	// 2) 项目声明 local（.pvmrc）
	// 全局唯一 runtime（如 git）跳过项目层，避免历史 .pvmrc 中遗留的条目误生效
	if !IsGlobalOnly(rt) {
		if vf, _ := FindVersionFile(startDir); vf != nil {
			if v, ok := vf.Versions[rt]; ok && v != "" {
				return v, vf.Path
			}
		}
	}

	// 3) 用户全局默认 user（~/.pvm/versions）
	globalFile := GlobalVersionsFile()
	if vf, err := LoadVersionFile(globalFile); err == nil && vf != nil {
		if v, ok := vf.Versions[rt]; ok && v != "" {
			return v, vf.Path
		}
	}

	// 4) 系统 system PATH fallback：如果系统中有全局安装，则使用它
	if SystemHasRuntime(rt) {
		return SystemVersion, "system"
	}

	return "", ""
}

// SystemHasRuntime 检查系统 PATH 中是否存在某 runtime 的主命令
// 注意：会跳过 pvm shims 目录，避免循环
func SystemHasRuntime(rt string) bool {
	cmd := runtimeMainCommand(rt)
	_, err := lookPathSkipShims(cmd)
	return err == nil
}

// SystemCommandPath 返回系统 PATH 中某命令的真实路径（跳过 pvm shims）
func SystemCommandPath(cmdName string) (string, error) {
	return lookPathSkipShims(cmdName)
}

// lookPathSkipShims 在 PATH 中查找命令，跳过 pvm shims 目录
func lookPathSkipShims(name string) (string, error) {
	shimsDir := filepath.Clean(ShimsDir())
	binHome := filepath.Clean(BinHome())

	pathEnv := os.Getenv("PATH")
	dirs := filepath.SplitList(pathEnv)

	exts := []string{""}
	if IsWindows() {
		exts = []string{".exe", ".cmd", ".bat", ""}
	}

	for _, dir := range dirs {
		cleanDir := filepath.Clean(dir)
		// 跳过 pvm 自身管理的目录，避免 shim 循环调用
		if strings.EqualFold(cleanDir, shimsDir) || strings.EqualFold(cleanDir, binHome) {
			continue
		}
		// 跳过 Windows Store 的 App Execution Alias stub（WindowsApps\python3.exe 等）
		// 这些是 0 字节的 Store 重定向 stub，运行会跳转 Microsoft Store，不是真实可执行文件
		if IsWindows() && strings.Contains(strings.ToLower(cleanDir), `\windowsapps`) {
			continue
		}
		for _, ext := range exts {
			p := filepath.Join(dir, name+ext)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("%s: not found in system PATH", name)
}

// RuntimeMainCommand 返回 runtime 的主命令名（导出版）
func RuntimeMainCommand(rt string) string {
	return runtimeMainCommand(rt)
}

// runtimeMainCommand 返回 runtime 的主命令名
func runtimeMainCommand(rt string) string {
	switch rt {
	case "node":
		return "node"
	case "python":
		// Windows 上主命令是 "python"（python3 是 Unix 习惯）
		// WindowsApps\python3.exe 是 Store stub，不能作为可用判断依据
		if IsWindows() {
			return "python"
		}
		return "python3"
	case "go":
		return "go"
	case "pnpm":
		return "pnpm"
	case "git":
		return "git"
	default:
		return rt
	}
}

// WriteProjectVersion 在 dir 目录的 .pvmrc 中写入/更新一个 runtime 的版本
func WriteProjectVersion(dir, rt, version string) (string, error) {
	// 优先使用已有文件
	var target string
	for _, name := range VersionFileNames {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			target = p
			break
		}
	}
	if target == "" {
		target = filepath.Join(dir, ".pvmrc")
	}

	existing := make(map[string]string)
	if fileExists(target) {
		if vf, err := LoadVersionFile(target); err == nil && vf != nil {
			existing = vf.Versions
		}
	}
	existing[rt] = version
	return target, SaveVersionFile(target, existing)
}

// WriteGlobalVersion 将某 runtime 的版本写入用户全局 ~/.pvm/versions
func WriteGlobalVersion(rt, version string) error {
	target := GlobalVersionsFile()
	existing := make(map[string]string)
	if vf, err := LoadVersionFile(target); err == nil && vf != nil {
		existing = vf.Versions
	}
	existing[rt] = version
	if err := EnsureDir(PvmHome()); err != nil {
		return err
	}
	return SaveVersionFile(target, existing)
}

// RemoveGlobalVersion 从用户全局 ~/.pvm/versions 中移除某 runtime 的版本配置
func RemoveGlobalVersion(rt string) error {
	target := GlobalVersionsFile()
	existing := make(map[string]string)
	if vf, err := LoadVersionFile(target); err == nil && vf != nil {
		existing = vf.Versions
	}
	if _, ok := existing[rt]; !ok {
		return fmt.Errorf("%s is not set in user config", rt)
	}
	delete(existing, rt)
	if err := EnsureDir(PvmHome()); err != nil {
		return err
	}
	return SaveVersionFile(target, existing)
}

// RemoveProjectVersion 从当前目录的 .pvmrc 中移除某 runtime 的版本配置
// 返回修改的文件路径；如果没有找到配置文件则返回 ("", nil)
func RemoveProjectVersion(dir, rt string) (string, error) {
	var target string
	for _, name := range VersionFileNames {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			target = p
			break
		}
	}
	if target == "" {
		return "", nil
	}

	existing := make(map[string]string)
	if vf, err := LoadVersionFile(target); err == nil && vf != nil {
		existing = vf.Versions
	}
	if _, ok := existing[rt]; !ok {
		return target, fmt.Errorf("%s is not set in %s", rt, target)
	}
	delete(existing, rt)
	return target, SaveVersionFile(target, existing)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
