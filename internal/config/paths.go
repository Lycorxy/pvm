package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pvm/pvm/internal/plugin"
)

// 支持的运行时
// pnpm 作为独立 runtime 管理，可在 .pvmrc 中声明版本
// 包管理器（yarn）也作为独立 runtime 管理
//
// 注意：poetry/pdm 等 Python CLI 工具不由 pvm 管理，请使用 pipx：
//
//	python -m pip install --user pipx
//	python -m pipx ensurepath
//	pipx install poetry
var SupportedRuntimes = []string{
	// 运行时（支持用户级、系统级、项目级）
	"node", "bun", "deno", "python",
	// 包管理器（支持用户级、系统级、项目级）
	"pnpm", "yarn",
	// 工具（只支持用户级和系统级）
	"go", "git", "rust",
}

// GlobalOnlyRuntimes 只支持用户级和系统级安装的 runtime
//
// 这类工具（如 go/git/rust）在实际场景中没有"项目 A 用 1.x、项目 B 用 2.x"
// 的强需求，全局装一份足以满足所有使用场景。约束如下：
//  1. 不允许写入项目级 .pvmrc（只能 --user 全局）
//  2. 同一时刻在 ~/.pvm/installs/<rt>/ 下只保留一个版本目录
//     （安装新版本前会自动移除已存在的旧版本）
//
// 注：目录结构仍是 installs/<rt>/<version>/，这样不破坏 shim/Reshim 等
// 通用扫描逻辑，仅靠"安装新版前先清空兄弟目录"来保证全局唯一性。
var GlobalOnlyRuntimes = []string{"go", "git", "rust"}

// RuntimeDeps 描述包管理器对运行时的依赖关系
// key 是包管理器，value 是它需要的运行时
// 用于安装前提示用户，但不强制阻断（运行时可由系统提供）
var RuntimeDeps = map[string]string{
	"pnpm": "node",
	"yarn": "node",
}

// IsGlobalOnly 判断某 runtime 是否只支持全局安装
//
// 同时检查静态列表和插件系统，确保向后兼容
func IsGlobalOnly(rt string) bool {
	// 检查静态列表（向后兼容）
	for _, r := range GlobalOnlyRuntimes {
		if r == rt {
			return true
		}
	}
	// 检查插件系统
	if p := plugin.GetPlugin(rt); p != nil {
		return p.IsGlobalOnly()
	}
	return false
}

// IsSupportedRuntime 判断是否是支持的运行时
//
// 同时检查静态列表和插件系统，确保向后兼容
func IsSupportedRuntime(rt string) bool {
	// 检查静态列表（向后兼容）
	for _, r := range SupportedRuntimes {
		if r == rt {
			return true
		}
	}
	// 检查插件系统
	return plugin.IsSupportedRuntime(rt)
}

// PvmHome 返回 pvm 根目录：
//   - 优先读取环境变量 PVM_HOME
//   - 否则使用 ~/.pvm
func PvmHome() string {
	if env := os.Getenv("PVM_HOME"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".pvm")
}

// BinHome 返回 pvm 主程序目录 ~/.pvm/bin
func BinHome() string {
	return filepath.Join(PvmHome(), "bin")
}

// ShimsDir 返回 shim 目录 ~/.pvm/shims
// 这是唯一需要加入用户 PATH 的目录
func ShimsDir() string {
	return filepath.Join(PvmHome(), "shims")
}

// InstallsDir 返回所有运行时的安装根目录 ~/.pvm/installs
func InstallsDir() string {
	return filepath.Join(PvmHome(), "installs")
}

// InstallDir 返回某个版本的安装目录 ~/.pvm/installs/<runtime>/<version>
func InstallDir(rt string, version string) string {
	return filepath.Join(InstallsDir(), rt, version)
}

// CacheDir 返回下载缓存目录 ~/.pvm/cache
func CacheDir() string {
	return filepath.Join(PvmHome(), "cache")
}

// TempDir 返回临时目录 ~/.pvm/tmp
func TempDir() string {
	return filepath.Join(PvmHome(), "tmp")
}

// GlobalConfigFile 返回全局配置文件路径 ~/.pvm/config.toml
// 存储全局默认版本、镜像源等
func GlobalConfigFile() string {
	return filepath.Join(PvmHome(), "config.toml")
}

// GlobalVersionsFile 返回全局默认版本文件 ~/.pvm/versions
// 格式与 .pvmrc 一致，作为最兜底的版本来源
func GlobalVersionsFile() string {
	return filepath.Join(PvmHome(), "versions")
}

// BinDir 返回某个运行时安装目录下真正的可执行目录
// 不同 OS 和 runtime 的二进制位置不同
func BinDir(rt string, version string) string {
	base := InstallDir(rt, version)
	switch rt {
	case "go":
		// Go 官方压缩包顶层有个 go/ 目录
		return filepath.Join(base, "go", "bin")
	case "node":
		if runtime.GOOS == "windows" {
			// Windows 上 node.exe 直接位于顶层（解压后会被 flatten）
			return base
		}
		return filepath.Join(base, "bin")
	case "python":
		if runtime.GOOS == "windows" {
			// python-build-standalone Windows 版 python.exe 位于顶层
			return base
		}
		return filepath.Join(base, "bin")
	case "pnpm":
		// pnpm 从 npm registry 下载，解压后结构：package/bin/pnpm.cjs
		// 我们在安装时会生成包装脚本，放在 bin/ 目录
		return filepath.Join(base, "bin")
	case "git":
		if runtime.GOOS == "windows" {
			// PortableGit 解压后的目录结构：
			//   cmd/git.exe           - Git 核心命令
			//   usr/bin/bash.exe      - Git Bash 及 Unix 工具 (ssh, scp, curl, vim 等)
			//   mingw64/bin/gcc.exe   - GCC 编译工具链
			// 使用 cmd 作为 bin 目录，只管理 git 核心命令，不管理 Unix 工具
			// 这样可以避免为 bash、ssh、curl 等 Unix 工具创建 shim
			return filepath.Join(base, "cmd")
		}
		// Linux/macOS 编译安装后：bin/git
		return filepath.Join(base, "bin")
	case "bun":
		// Bun 解压后：bun-{target}/bun (会被 flatten 到根目录)
		// 安装后 bun 二进制直接位于 base 根目录
		return base
	case "deno":
		// Deno 解压后只有一个 deno 可执行文件，直接位于 base
		return base
	case "rust":
		// Rust 官方包: rust-{version}-{target}/rustc/bin
		// 我们 flatten 后：bin
		return filepath.Join(base, "bin")
	case "java":
		// Adoptium JDK 解压后:
		//   Linux:   jdk-{ver}/bin
		//   macOS:   jdk-{ver}.jdk/Contents/Home/bin  ← 特殊
		//   Windows: jdk-{ver}/bin
		// 我们 flatten 后统一到 base/bin（macOS 特殊处理）
		if runtime.GOOS == "darwin" {
			return filepath.Join(base, "Contents", "Home", "bin")
		}
		return filepath.Join(base, "bin")
	case "yarn":
		// yarn npm 包: package/bin/yarn (.js)
		// 我们生成 wrapper 脚本到 bin/
		return filepath.Join(base, "bin")
	case "uv":
		// uv 解压后: uv-{target}/uv (会被 flatten)
		// 安装后 uv 二进制直接位于 base 根目录
		return base
	case "maven":
		// apache-maven-{ver}/bin
		return filepath.Join(base, "bin")
	case "gradle":
		// gradle-{ver}/bin
		return filepath.Join(base, "bin")
	default:
		return filepath.Join(base, "bin")
	}
}

// EnsureDir 创建目录（如果不存在）
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// EnsureAllDirs 确保所有 pvm 标准目录存在
func EnsureAllDirs() error {
	dirs := []string{
		PvmHome(),
		BinHome(),
		ShimsDir(),
		InstallsDir(),
		CacheDir(),
		TempDir(),
	}
	for _, d := range dirs {
		if err := EnsureDir(d); err != nil {
			return err
		}
	}
	return nil
}

// IsWindows 判断是否 Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// ExeExt 返回可执行文件后缀
func ExeExt() string {
	if IsWindows() {
		return ".exe"
	}
	return ""
}

// IsInstalled 判断某版本是否已安装
//
// 判定规则：
//  1. bin 目录必须存在
//  2. 对于 bin == InstallDir 的 runtime（bun/deno/uv），需要检查目录里是否有可执行文件
//     防止刚创建空目录就被判为已安装
func IsInstalled(rt, version string) bool {
	bin := BinDir(rt, version)
	info, err := os.Stat(bin)
	if err != nil || !info.IsDir() {
		return false
	}

	// bin == InstallDir 的特殊情况：检查标志可执行文件
	if bin == InstallDir(rt, version) {
		// 用具体的可执行文件名判断
		marker := ""
		switch rt {
		case "bun":
			marker = "bun"
		case "deno":
			marker = "deno"
		case "uv":
			marker = "uv"
		}
		if marker != "" {
			candidates := []string{marker, marker + ExeExt()}
			for _, c := range candidates {
				if _, err := os.Stat(filepath.Join(bin, c)); err == nil {
					return true
				}
			}
			return false
		}
	}
	return true
}

// FindInstalledMatch 在已安装版本中查找满足 wantVersion 要求的最新版本。
//
// 用于 shim 自动安装前的"先检查已安装"逻辑：
//   - 如果 wantVersion 是精确版本（如 "20.11.0"），直接检查是否已安装
//   - 如果 wantVersion 是模糊版本（如 "20"、"3.12"），扫描已安装版本，
//     找到前缀匹配的最新版本直接使用，避免触发网络请求
//
// 返回匹配到的精确版本号；未找到则返回 ""
func FindInstalledMatch(rt, wantVersion string) string {
	// 精确版本：直接检查
	if IsInstalled(rt, wantVersion) {
		return wantVersion
	}

	// 模糊版本：扫描已安装目录，找前缀匹配的最新版本
	rtDir := filepath.Join(InstallsDir(), rt)
	entries, err := os.ReadDir(rtDir)
	if err != nil {
		return ""
	}

	// 构建前缀：模糊版本 "20" 匹配 "20."，"3.12" 匹配 "3.12."
	prefix := wantVersion + "."

	var candidates []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		v := e.Name()
		if strings.HasPrefix(v, prefix) && IsInstalled(rt, v) {
			candidates = append(candidates, v)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// 返回最新版本（降序排列取第一个）
	sortVersionsDesc(candidates)
	return candidates[0]
}

// sortVersionsDesc 按语义版本降序排列（简单实现，避免循环依赖 semver 包）
func sortVersionsDesc(versions []string) {
	for i := 1; i < len(versions); i++ {
		for j := i; j > 0 && versionLess(versions[j-1], versions[j]); j-- {
			versions[j-1], versions[j] = versions[j], versions[j-1]
		}
	}
}

// versionLess 比较两个版本字符串，a < b 时返回 true
func versionLess(a, b string) bool {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		var ai, bi int
		if i < len(aParts) {
			for _, c := range aParts[i] {
				if c >= '0' && c <= '9' {
					ai = ai*10 + int(c-'0')
				} else {
					break
				}
			}
		}
		if i < len(bParts) {
			for _, c := range bParts[i] {
				if c >= '0' && c <= '9' {
					bi = bi*10 + int(c-'0')
				} else {
					break
				}
			}
		}
		if ai != bi {
			return ai < bi
		}
	}
	return false
}
