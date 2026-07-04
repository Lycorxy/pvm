package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pvm/pvm/internal/archive"
	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/download"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/registry"
)

// ToolRuntimes 工具型运行时（全局唯一版本，安装到 ~/.pvm/installs，但通过 current junction 管理）
// 参考 Scoop 的架构：apps/<name>/<version>/ + apps/<name>/current/ (junction)
var ToolRuntimes = []string{"git", "go"}

// IsToolRuntime 判断是否是工具型运行时
func IsToolRuntime(rt string) bool {
	for _, r := range ToolRuntimes {
		if r == rt {
			return true
		}
	}
	return false
}

// ToolInstallInfo 工具安装结果
type ToolInstallInfo struct {
	Runtime     string
	Version     string
	InstallPath string   // 版本目录：~/.pvm/installs/<rt>/<version>/
	CurrentPath string   // current junction：~/.pvm/installs/<rt>/current/
	BinPaths    []string // 添加到 PATH 的路径
}

// InstallTool 安装工具型运行时（Git/Go）到用户目录
//
// 架构设计（参考 Scoop）：
//
//	~/.pvm/
//	  installs/
//	    git/
//	      2.45.0/          <- 版本目录
//	      current/         <- Junction 指向 2.45.0/
//	    go/
//	      1.22.0/
//	      current/
//	  shims/               <- 唯一需要加入 PATH 的目录
//	    git.cmd            <- shim 脚本
//	    go.cmd
//
// 优势：
//  1. 不需要管理员权限
//  2. shims 目录加入用户 PATH 后永不变化
//  3. 切换版本只需更新 current junction
//  4. VSCode 等 IDE 通过 shim 自动找到命令
func InstallTool(rt, version string, useMirror, force bool) (*ToolInstallInfo, error) {
	if !IsToolRuntime(rt) {
		return nil, fmt.Errorf("%s is not a tool runtime (supported: git, go)", rt)
	}

	// 解析精确版本
	needsResolve := !registry.IsExactVersion(version)
	if needsResolve {
		logger.Info("  → Resolving %s@%s...", rt, version)
		exact, err := registry.ResolveExactVersion(rt, version, useMirror)
		if err != nil {
			return nil, fmt.Errorf("resolve version: %w", err)
		}
		logger.Info("  → Resolved %s@%s → %s", rt, version, exact)
		version = exact
	}

	// 目标目录
	versionDir := config.InstallDir(rt, version)
	currentDir := filepath.Join(config.InstallsDir(), rt, "current")

	// 检查是否已安装
	if !force && config.IsInstalled(rt, version) {
		logger.Info("  ✓ %s@%s is already installed", rt, version)
		// 确保 current junction 正确
		if err := linkCurrent(versionDir, currentDir); err != nil {
			logger.Verbose("  → Warning: update current junction: %v", err)
		}
		// 确保用户级版本配置存在，使 shim-exec 能解析到版本（统一 shim 方案必需）
		ensureToolVersionConfig(rt, version)
		// 注：shim.Reshim() 由调用者负责调用，避免循环依赖
		return &ToolInstallInfo{
			Runtime:     rt,
			Version:     version,
			InstallPath: versionDir,
			CurrentPath: currentDir,
			BinPaths:    []string{config.ShimsDir()},
		}, nil
	}

	// 下载信息
	var info *registry.RuntimeInfo
	var err error
	if useMirror {
		info, err = registry.GetMirrorURL(rt, version)
	} else {
		info, err = registry.GetDownloadInfo(rt, version)
	}
	if err != nil {
		return nil, err
	}

	sourceLabel := "官方源"
	if useMirror {
		sourceLabel = "国内镜像"
	}
	logger.Info("  → Installing %s@%s", rt, version)
	logger.Info("  → Downloading from %s  [%s]", info.URL, sourceLabel)

	// 下载到缓存目录
	cacheDir := config.CacheDir()
	if err := config.EnsureDir(cacheDir); err != nil {
		return nil, err
	}
	archiveName := fmt.Sprintf("%s-%s.%s", rt, version, info.ArchiveType)
	archivePath := filepath.Join(cacheDir, archiveName)

	// 下载（如果缓存不存在）
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		if err := download.DownloadFile(info.URL, archivePath); err != nil {
			if info.FallbackURL != "" {
				logger.Info("  → Primary URL failed, trying fallback...")
				if err2 := download.DownloadFile(info.FallbackURL, archivePath); err2 != nil {
					os.Remove(archivePath)
					return nil, fmt.Errorf("download failed: %w", err)
				}
			} else {
				os.Remove(archivePath)
				return nil, fmt.Errorf("download: %w", err)
			}
		}
	} else {
		logger.Info("  → Using cached archive")
	}

	// 全局唯一：先移除其他版本
	rtDir := filepath.Join(config.InstallsDir(), rt)
	if entries, err := os.ReadDir(rtDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != version && e.Name() != "current" {
				oldDir := filepath.Join(rtDir, e.Name())
				logger.Verbose("  → Removing old version: %s", e.Name())
				os.RemoveAll(oldDir)
			}
		}
	}

	// 解压
	logger.Info("  → Extracting...")
	tempDir := config.TempDir()
	if err := config.EnsureDir(tempDir); err != nil {
		return nil, err
	}
	extractTmp := filepath.Join(tempDir, fmt.Sprintf("%s-%s-extract", rt, version))
	os.RemoveAll(extractTmp)
	if err := os.MkdirAll(extractTmp, 0755); err != nil {
		return nil, err
	}
	if err := archive.Extract(archivePath, extractTmp, info.ArchiveType); err != nil {
		os.RemoveAll(extractTmp)
		return nil, fmt.Errorf("extract: %w", err)
	}

	// 展平目录结构
	if err := flattenExtracted(rt, extractTmp); err != nil {
		os.RemoveAll(extractTmp)
		return nil, fmt.Errorf("flatten: %w", err)
	}

	// 移动到版本目录
	if force {
		os.RemoveAll(versionDir)
	}
	if err := os.MkdirAll(filepath.Dir(versionDir), 0755); err != nil {
		os.RemoveAll(extractTmp)
		return nil, err
	}
	if err := os.Rename(extractTmp, versionDir); err != nil {
		// rename 失败，尝试复制
		logger.Verbose("  → Rename failed, copying...")
		if err := copyDirRecursive(extractTmp, versionDir); err != nil {
			os.RemoveAll(extractTmp)
			return nil, fmt.Errorf("install: %w", err)
		}
		os.RemoveAll(extractTmp)
	}

	// 创建/更新 current junction（参考 Scoop 的 link_current）
	logger.Info("  → Linking current → %s", version)
	if err := linkCurrent(versionDir, currentDir); err != nil {
		return nil, fmt.Errorf("link current: %w", err)
	}

	// Git for Windows 后处理：修复 tar.bz2 解压丢失的硬链接
	if rt == "git" {
		fixGitHardlinks(versionDir)
	}

	// 写入用户级版本配置，使 pvm-shim → shim-exec 能解析到版本
	ensureToolVersionConfig(rt, version)

	// 注：shim.Reshim() 由调用者负责调用，避免循环依赖
	// 调用者应在 InstallTool 返回后调用 shim.Reshim()

	// 确保 shims 目录在用户 PATH 中
	shimsDir := config.ShimsDir()
	if err := ensureShimsInPath(shimsDir); err != nil {
		logger.Info("  ⚠  Failed to add shims to PATH: %v", err)
		logger.Info("  → Please add manually: %s", shimsDir)
	}

	logger.Info("  ✓ %s@%s installed successfully", rt, version)
	logger.Info("")
	logger.Info("  💡 Tips:")
	logger.Info("     • Restart terminal/IDE for PATH changes to take effect")
	logger.Info("     • Install location: %s", versionDir)

	return &ToolInstallInfo{
		Runtime:     rt,
		Version:     version,
		InstallPath: versionDir,
		CurrentPath: currentDir,
		BinPaths:    []string{shimsDir},
	}, nil
}

// ensureToolVersionConfig 确保工具型运行时（git/go）在用户级版本配置中存在条目。
// 统一 shim 方案下，pvm-shim → pvm shim-exec → ResolveVersion 需要版本配置才能解析，
// 否则会报 "no version configured"。工具型运行时安装后应立即可用，故自动写入。
func ensureToolVersionConfig(rt, version string) {
	cwd, _ := os.Getwd()
	current, _ := config.ResolveVersion(rt, cwd)
	if current == version {
		return // 已配置为该版本
	}
	if err := config.WriteGlobalVersion(rt, version); err != nil {
		logger.Verbose("  → Warning: write user version config for %s: %v", rt, err)
	} else {
		logger.Verbose("  → Set user %s = %s", rt, version)
	}
}

// linkCurrent 创建或更新 current junction（参考 Scoop 的 link_current）
func linkCurrent(versionDir, currentDir string) error {
	// 移除旧的 current（不管是 junction 还是普通目录）
	if _, err := os.Lstat(currentDir); err == nil {
		if runtime.GOOS == "windows" {
			// Windows: 使用 cmd /c rmdir 删除 junction
			cmd := exec.Command("cmd", "/c", "rmdir", currentDir)
			if err := cmd.Run(); err != nil {
				// rmdir 失败，尝试 RemoveAll
				os.RemoveAll(currentDir)
			}
		} else {
			os.RemoveAll(currentDir)
		}
	}

	// 创建新的 junction/symlink
	if runtime.GOOS == "windows" {
		// Windows: 使用 cmd /c mklink /J 创建 directory junction
		cmd := exec.Command("cmd", "/c", "mklink", "/J", currentDir, versionDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("mklink /J failed: %w", err)
		}
	} else {
		// Unix: 符号链接
		if err := os.Symlink(versionDir, currentDir); err != nil {
			return fmt.Errorf("symlink failed: %w", err)
		}
	}

	return nil
}

// ensureShimsInPath 确保 shims 目录在用户 PATH 中（无需管理员权限）
func ensureShimsInPath(shimsDir string) error {
	if runtime.GOOS == "windows" {
		return ensureShimsInPathWindows(shimsDir)
	}
	return ensureShimsInPathUnix(shimsDir)
}

// ensureShimsInPathWindows 确保 shims 在 Windows 用户 PATH 中
func ensureShimsInPathWindows(shimsDir string) error {
	// 读取用户级 PATH（不是系统级，不需要管理员权限）
	cmd := exec.Command("powershell", "-Command",
		"[Environment]::GetEnvironmentVariable('Path', 'User')")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("read user PATH: %w", err)
	}

	currentPath := strings.TrimSpace(string(output))

	// 检查是否已存在
	pathLower := strings.ToLower(currentPath)
	shimsLower := strings.ToLower(shimsDir)
	if strings.Contains(pathLower, shimsLower) {
		logger.Verbose("  → Shims already in PATH")
		return nil
	}

	// 添加到用户 PATH（前置，优先级最高）
	var newPath string
	if currentPath == "" {
		newPath = shimsDir
	} else {
		newPath = shimsDir + ";" + currentPath
	}

	psCmd := fmt.Sprintf("[Environment]::SetEnvironmentVariable('Path', '%s', 'User')",
		strings.ReplaceAll(newPath, "'", "''"))
	cmd = exec.Command("powershell", "-Command", psCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set user PATH: %w", err)
	}

	// 广播环境变量变更消息
	broadcastEnvChange()

	logger.Info("  ✓ Added to user PATH: %s", shimsDir)
	return nil
}

// ensureShimsInPathUnix 确保 shims 在 Unix PATH 中
func ensureShimsInPathUnix(shimsDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// 检测 shell 配置文件
	shellFiles := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".profile"),
	}

	line := fmt.Sprintf("export PATH=\"%s:$PATH\"", shimsDir)
	marker := "# pvm shims"

	for _, shellFile := range shellFiles {
		if _, err := os.Stat(shellFile); err != nil {
			continue
		}

		content, err := os.ReadFile(shellFile)
		if err != nil {
			continue
		}

		// 检查是否已添加
		if strings.Contains(string(content), shimsDir) {
			logger.Verbose("  → Shims already in %s", filepath.Base(shellFile))
			continue
		}

		// 追加
		f, err := os.OpenFile(shellFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		f.WriteString("\n" + marker + "\n" + line + "\n")
		f.Close()
		logger.Info("  ✓ Added to %s", filepath.Base(shellFile))
	}

	return nil
}

// UninstallTool 卸载工具型运行时
func UninstallTool(rt string) error {
	if !IsToolRuntime(rt) {
		return fmt.Errorf("%s is not a tool runtime", rt)
	}

	rtDir := filepath.Join(config.InstallsDir(), rt)
	currentDir := filepath.Join(rtDir, "current")

	// 检查是否已安装
	if _, err := os.Stat(rtDir); os.IsNotExist(err) {
		logger.Info("  ℹ  %s is not installed", rt)
		return nil
	}

	logger.Info("  → Uninstalling %s...", rt)

	// 移除 current junction
	if _, err := os.Lstat(currentDir); err == nil {
		if runtime.GOOS == "windows" {
			cmd := exec.Command("cmd", "/c", "rmdir", currentDir)
			cmd.Run()
		} else {
			os.Remove(currentDir)
		}
	}

	// 移除整个 runtime 目录
	if err := os.RemoveAll(rtDir); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	// 注：shim.Reshim() 由调用者负责调用，避免循环依赖

	logger.Info("  ✓ %s uninstalled successfully", rt)
	return nil
}

// GetInstalledToolVersion 获取已安装的工具版本
func GetInstalledToolVersion(rt string) string {
	rtDir := filepath.Join(config.InstallsDir(), rt)
	entries, err := os.ReadDir(rtDir)
	if err != nil {
		return ""
	}

	for _, e := range entries {
		if e.IsDir() && e.Name() != "current" {
			return e.Name()
		}
	}
	return ""
}

// flattenExtracted 展平解压后的目录结构
func flattenExtracted(rt, extractDir string) error {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return err
	}

	// 如果只有一个子目录，将其内容上移
	if len(entries) == 1 && entries[0].IsDir() {
		subDir := filepath.Join(extractDir, entries[0].Name())

		// 特殊处理：Go 的压缩包顶层有个 go/ 目录，但我们需要保留它
		// 因为 BinDir(go, version) 返回 base/go/bin
		if rt == "go" && strings.HasPrefix(strings.ToLower(entries[0].Name()), "go") {
			// Go 保持原样，不 flatten（go/ 目录结构需要保留）
			return nil
		}

		// Git 的 PortableGit 需要 flatten
		if rt == "git" {
			name := strings.ToLower(entries[0].Name())
			if strings.Contains(name, "portablegit") || strings.Contains(name, "git-") {
				return flattenSingleChild(extractDir, entries[0].Name())
			}
		}

		// 其他通用处理
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			return err
		}
		for _, se := range subEntries {
			src := filepath.Join(subDir, se.Name())
			dst := filepath.Join(extractDir, se.Name())
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("flatten %s: %w", se.Name(), err)
			}
		}
		os.Remove(subDir)
	}

	return nil
}

// fixGitHardlinks 修复 Git for Windows tar.bz2 解压后丢失的硬链接
//
// Git for Windows 的 tar.bz2 包中，git-remote-https.exe 是 git-remote-ftp.exe 的硬链接，
// 但 tar 解压时硬链接关系丢失，导致 git-remote-https.exe 不存在，
// 从而 git clone/push/pull 等 HTTPS 操作失败（"remote-https is not a git command"）。
//
// 修复方式：将 git-remote-ftp.exe 复制为 git-remote-https.exe
func fixGitHardlinks(installDir string) {
	gcDir := filepath.Join(installDir, "mingw64", "libexec", "git-core")

	// 需要从 git-remote-ftp.exe 创建的硬链接别名
	type hardlinkAlias struct {
		src  string // 已存在的文件（不含路径前缀）
		dest string // 需要创建的别名（不含路径前缀）
	}
	aliases := []hardlinkAlias{
		{"git-remote-ftp.exe", "git-remote-https.exe"},
	}

	for _, a := range aliases {
		srcPath := filepath.Join(gcDir, a.src)
		dstPath := filepath.Join(gcDir, a.dest)

		// 如果目标已存在，跳过
		if _, err := os.Stat(dstPath); err == nil {
			continue
		}

		// 检查源文件是否存在
		if _, err := os.Stat(srcPath); err != nil {
			logger.Verbose("  git hardlink: source %s not found, skipping", a.src)
			continue
		}

		// 尝试创建硬链接（节省磁盘空间）
		if err := os.Link(srcPath, dstPath); err != nil {
			// 硬链接失败（跨分区等），改用复制
			data, err := os.ReadFile(srcPath)
			if err != nil {
				logger.Verbose("  git hardlink: read %s failed: %v", a.src, err)
				continue
			}
			if err := os.WriteFile(dstPath, data, 0755); err != nil {
				logger.Verbose("  git hardlink: write %s failed: %v", a.dest, err)
				continue
			}
		}
		logger.Verbose("  git hardlink: created %s → %s", a.dest, a.src)
	}
}

// copyDirRecursive 递归复制目录
func copyDirRecursive(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			info, _ := entry.Info()
			if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
				return err
			}
		}
	}

	return nil
}

// broadcastEnvChange 广播环境变量变更（Windows）
// 参考 Scoop 的 Publish-EnvVar 实现
func broadcastEnvChange() {
	if runtime.GOOS != "windows" {
		return
	}
	// 使用 PowerShell 发送 WM_SETTINGCHANGE 消息
	cmd := exec.Command("powershell", "-Command",
		"$HWND_BROADCAST = [IntPtr]0xffff; "+
			"$WM_SETTINGCHANGE = 0x1a; "+
			"Add-Type -TypeDefinition @'\n"+
			"using System; using System.Runtime.InteropServices;\n"+
			"public class Win32 { [DllImport(\"user32.dll\", SetLastError=true, CharSet=CharSet.Auto)] "+
			"public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult); }\n"+
			"'@\n"+
			"[Win32]::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref][UIntPtr]::Zero)")
	cmd.Run() // 忽略错误
}

// ============== 兼容性别名（保留旧的函数名，内部调用新实现）==============

// SystemRuntimes 兼容旧代码（别名）
var SystemRuntimes = ToolRuntimes

// IsSystemRuntime 兼容旧代码（别名）
func IsSystemRuntime(rt string) bool {
	return IsToolRuntime(rt)
}

// SystemInstallInfo 兼容旧代码（别名）
type SystemInstallInfo = ToolInstallInfo

// InstallToSystem 兼容旧代码，内部调用 InstallTool
func InstallToSystem(rt, version string, useMirror, force bool) (*SystemInstallInfo, error) {
	return InstallTool(rt, version, useMirror, force)
}

// UninstallFromSystem 兼容旧代码，内部调用 UninstallTool
func UninstallFromSystem(rt string) error {
	return UninstallTool(rt)
}
