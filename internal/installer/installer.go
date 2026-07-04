package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/archive"
	"github.com/pvm/pvm/internal/compat"
	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/download"
	"github.com/pvm/pvm/internal/filelock"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/registry"
)

// Install 安装单个 runtime 的指定版本
// useMirror: 使用中国镜像; force: 强制覆盖安装
// 返回实际安装的精确版本号（模糊版本会被解析为精确版本）
func Install(rt, version string, useMirror, force bool) (string, error) {
	// 将模糊版本（如 "8"、"20"、"1.22"、"latest"）解析为精确版本
	// java 的版本号天然带 build 元数据（如 "21.0.5+11"），IsExactVersion 会判它为非精确，
	// 但对 java 来说带 "+build" 已经是 Adoptium 期望的精确 tag，无需再解析。
	needsResolve := !registry.IsExactVersion(version)
	if rt == "java" && strings.Contains(version, "+") {
		needsResolve = false
	}
	if needsResolve {
		logger.Info("  → Resolving %s@%s...", rt, version)
		exact, err := registry.ResolveExactVersion(rt, version, useMirror)
		if err != nil {
			return "", fmt.Errorf("resolve version: %w", err)
		}
		logger.Info("  → Resolved %s@%s → %s", rt, version, exact)
		version = exact
	}

	// pnpm 安装前检查与当前 node 版本的兼容性
	if rt == "pnpm" {
		cwd, _ := os.Getwd()
		nodeVer, _ := config.ResolveVersion("node", cwd)
		if nodeVer != "" && nodeVer != config.SystemVersion {
			if err := compat.CheckPnpmNodeCompat(version, nodeVer, useMirror); err != nil {
				logger.Error("%v", err)
				return "", fmt.Errorf("pnpm@%s is incompatible with current node@%s", version, nodeVer)
			}
		}
	}

	// 文件锁：防止并发安装同一版本
	lockName := fmt.Sprintf("install-%s-%s", rt, version)
	lockPath := filepath.Join(config.TempDir(), lockName)
	if err := config.EnsureDir(config.TempDir()); err != nil {
		return "", err
	}
	lock := filelock.New(lockPath)
	if err := lock.Lock(30 * time.Second); err != nil {
		return "", fmt.Errorf("another install is in progress: %w", err)
	}
	defer lock.Unlock()

	installDir := config.InstallDir(rt, version)
	binDir := config.BinDir(rt, version)

	// 全局唯一 runtime（如 git）：
	// 安装新版本前，移除该 runtime 下所有非当前版本的兄弟目录，保证 installs/<rt>/ 下只有一份。
	// 注意：必须在"已安装幂等检查"之前执行，否则同版本重装会被跳过、旧版本永远清不掉。
	if config.IsGlobalOnly(rt) {
		if err := pruneSiblingVersions(rt, version); err != nil {
			logger.Verbose("  prune sibling versions for %s: %v (ignored)", rt, err)
		}
	}

	// 已安装（幂等：同一版本在 ~/.pvm/installs/ 下只存一份，
	// 项目层/用户层/系统层共享，不会重复安装）
	if _, err := os.Stat(binDir); err == nil {
		if !force {
			// 即使目录存在，也要验证关键文件是否完整
			// 防止之前安装不完整（如 npm 丢失）却被幂等跳过
			if verr := verifyInstall(rt, version, binDir); verr != nil {
				logger.Info("  ⚠ %s@%s incomplete: %v, reinstalling...", rt, version, verr)
				os.RemoveAll(installDir)
			} else {
				logger.Info("  ✓ %s@%s already installed", rt, version)
				return version, nil
			}
		} else {
			// 强制覆盖：删除旧安装目录
			logger.Info("  → %s@%s already installed, forcing reinstall...", rt, version)
			os.RemoveAll(installDir)
		}
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
		return "", err
	}

	sourceLabel := "官方源"
	if useMirror {
		sourceLabel = "国内镜像"
	}
	logger.Info("  → Installing %s@%s", rt, version)
	logger.Info("  → Downloading from %s  [%s]", info.URL, sourceLabel)

	// 下载到 cache 目录（跨项目共享）
	cacheDir := config.CacheDir()
	if err := config.EnsureDir(cacheDir); err != nil {
		return "", err
	}
	archiveName := fmt.Sprintf("%s-%s.%s", rt, version, info.ArchiveType)
	archivePath := filepath.Join(cacheDir, archiveName)

	// 如果缓存已存在，跳过下载
	if _, err := os.Stat(archivePath); err == nil {
		logger.Verbose("  → Using cached archive: %s", archivePath)
	} else {
		if err := download.DownloadFile(info.URL, archivePath); err != nil {
			// 主 URL 失败，尝试 FallbackURL
			if info.FallbackURL != "" {
				logger.Info("  → Primary URL failed, trying fallback: %s", info.FallbackURL)
				if err2 := download.DownloadFile(info.FallbackURL, archivePath); err2 != nil {
					os.Remove(archivePath)
					return "", fmt.Errorf("download (primary and fallback failed): %w; fallback error: %v", err, err2)
				}
			} else {
				os.Remove(archivePath)
				return "", fmt.Errorf("download: %w", err)
			}
		}
	}

	// 解压到临时目录，再原子重命名
	logger.Info("  → Extracting...")
	extractTmp := installDir + ".tmp"
	os.RemoveAll(extractTmp)
	if err := config.EnsureDir(filepath.Dir(installDir)); err != nil {
		return "", err
	}
	if err := config.EnsureDir(extractTmp); err != nil {
		return "", err
	}
	if err := archive.Extract(archivePath, extractTmp, info.ArchiveType); err != nil {
		os.RemoveAll(extractTmp)
		return "", fmt.Errorf("extract: %w", err)
	}

	// 展平目录（Node/Python/Bun/Deno/Rust/Java/Maven/Gradle 压缩包顶层会多一层）
	switch rt {
	case "node":
		if err := flattenSingleChild(extractTmp, "node"); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("flatten node: %w", err)
		}
	case "python":
		if err := flattenSingleChild(extractTmp, "python"); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("flatten python: %w", err)
		}
		// Generate pip wrappers (python-build-standalone does not include pip.exe)
		if err := generatePythonWrappers(extractTmp); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("generate python wrappers: %w", err)
		}
	case "pnpm":
		if err := flattenSingleChild(extractTmp, "package"); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("flatten pnpm: %w", err)
		}
		if err := generatePnpmWrappers(extractTmp, version); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("generate pnpm wrappers: %w", err)
		}
	case "git":
		if runtime.GOOS != "windows" {
			if err := flattenSingleChild(extractTmp, "git-"); err != nil {
				logger.Verbose("  flatten git: %v (ignored)", err)
			}
		}
	case "bun":
		// Bun 解压后是 bun-{target}/bun
		if err := flattenSingleChild(extractTmp, "bun-"); err != nil {
			logger.Verbose("  flatten bun: %v (ignored)", err)
		}
	case "deno":
		// Deno 解压后通常直接是 deno 二进制（无外层目录），不需要 flatten
		// 但有些版本可能包了一层，安全起见尝试一下
		if err := flattenSingleChild(extractTmp, "deno"); err != nil {
			logger.Verbose("  flatten deno: %v (ignored)", err)
		}
	case "rust":
		// Rust 解压后是 rust-{version}-{target}/，里面有 install.sh、components 等
		// 我们需要把所有 component 的 bin/lib 合并到 base 下
		if err := flattenRust(extractTmp); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("flatten rust: %w", err)
		}
	case "java":
		// Adoptium 解压后是 jdk-{ver}/...
		if err := flattenSingleChild(extractTmp, "jdk"); err != nil {
			logger.Verbose("  flatten java: %v (ignored)", err)
		}
	case "yarn":
		// yarn npm 包: package/bin/yarn(.js) - 生成 wrapper
		if err := flattenSingleChild(extractTmp, "package"); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("flatten yarn: %w", err)
		}
		if err := generateYarnWrappers(extractTmp); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("generate yarn wrappers: %w", err)
		}
	case "uv":
		// uv 解压后是 uv-{target}/uv
		if err := flattenSingleChild(extractTmp, "uv-"); err != nil {
			logger.Verbose("  flatten uv: %v (ignored)", err)
		}
	case "maven":
		if err := flattenSingleChild(extractTmp, "apache-maven"); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("flatten maven: %w", err)
		}
	case "gradle":
		if err := flattenSingleChild(extractTmp, "gradle-"); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("flatten gradle: %w", err)
		}
	}

	// 先清理目标目录（可能存在旧版本或上次失败的残留）
	useMerge := false
	if _, err := os.Stat(installDir); err == nil {
		if removeErr := os.RemoveAll(installDir); removeErr != nil {
			// 删除失败说明有文件被占用（如 node.exe 被其他进程使用）。
			// 回退到合并安装：把新文件复制到旧目录，跳过被占用的文件。
			// 被占用的 node.exe 版本相同无需替换，其他文件（npm/npx/node_modules）会被正确更新。
			logger.Verbose("  → target dir busy (file in use), merging files into existing dir")
			useMerge = true
		}
	}
	if useMerge {
		if err := mergeInstall(extractTmp, installDir); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("cannot update install dir (file in use): %s\n  → close programs using %s (e.g. node, VSCode terminals) and retry", installDir, rt)
		}
		os.RemoveAll(extractTmp)
	} else {
		if err := renameWithRetry(extractTmp, installDir); err != nil {
			os.RemoveAll(extractTmp)
			return "", fmt.Errorf("finalize: %w", err)
		}
	}

	// 校验
	if _, err := os.Stat(binDir); err != nil {
		logger.Verbose("  BinDir expected: %s", binDir)
		entries, _ := os.ReadDir(installDir)
		for _, e := range entries {
			logger.Verbose("    %s (dir=%v)", e.Name(), e.IsDir())
		}
		return "", fmt.Errorf("post-install check failed: bin dir missing at %s", binDir)
	}

	logger.Info("  ✓ %s@%s installed", rt, version)

	// 安装后验证：检查核心可执行文件是否存在
	// 不返回错误，因为某些 runtime 可能没有标准的 bin 目录结构（仅记录警告）
	if err := verifyInstall(rt, version, binDir); err != nil {
		logger.Verbose("  ⚠ post-install verification: %v", err)
	}

	return version, nil
}

// verifyInstall 安装后验证：检查核心可执行文件是否存在于 bin 目录
// 用于尽早发现解压/flatten 异常导致的安装不完整问题
// 返回 error 表示安装不完整（调用方可据此决定是否重装）
func verifyInstall(rt, version, binDir string) error {
	var exeName string
	switch rt {
	case "node":
		exeName = "node"
	case "python":
		exeName = "python"
	case "go":
		exeName = "go"
	case "git":
		exeName = "git"
	case "pnpm":
		exeName = "pnpm"
	case "yarn":
		exeName = "yarn"
	default:
		// 未知 runtime 不验证
		return nil
	}
	if runtime.GOOS == "windows" {
		// Windows 下检查 .exe 或 .cmd
		if _, err := os.Stat(filepath.Join(binDir, exeName+".exe")); err != nil {
			if _, err := os.Stat(filepath.Join(binDir, exeName+".cmd")); err != nil {
				return fmt.Errorf("core executable %s not found in %s", exeName, binDir)
			}
		}
	} else {
		if _, err := os.Stat(filepath.Join(binDir, exeName)); err != nil {
			return fmt.Errorf("core executable %s not found in %s", exeName, binDir)
		}
	}

	// Node.js 额外检查：npm 是核心依赖，缺失说明安装不完整
	// （正常的 node zip 包包含 npm.cmd / npx.cmd / node_modules/npm）
	if rt == "node" {
		npmMissing := false
		if runtime.GOOS == "windows" {
			if _, err := os.Stat(filepath.Join(binDir, "npm.cmd")); err != nil {
				npmMissing = true
			}
		} else {
			if _, err := os.Stat(filepath.Join(binDir, "npm")); err != nil {
				npmMissing = true
			}
		}
		if npmMissing {
			return fmt.Errorf("npm not found in %s (incomplete node installation)", binDir)
		}
	}

	return nil
}

// generatePnpmWrappers 在 pnpm 安装目录的 bin/ 下生成可执行包装脚本
func generatePnpmWrappers(installDir, version string) error {
	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	cjsInBin := filepath.Join(installDir, "bin", "pnpm.cjs")
	cjsRelToBin := "pnpm.cjs"
	if _, err := os.Stat(cjsInBin); err != nil {
		cjsInDist := filepath.Join(installDir, "dist", "pnpm.cjs")
		if _, err := os.Stat(cjsInDist); err == nil {
			cjsRelToBin = filepath.Join("..", "dist", "pnpm.cjs")
		} else {
			cjsRelToBin = filepath.Join("..", "pnpm.cjs")
		}
	}

	cjsRelPosix := filepath.ToSlash(cjsRelToBin)

	shScript := fmt.Sprintf(`#!/bin/sh
exec node "$(dirname "$0")/%s" "$@"
`, cjsRelPosix)
	shPath := filepath.Join(binDir, "pnpm")
	if err := os.WriteFile(shPath, []byte(shScript), 0755); err != nil {
		return err
	}

	pnpxScript := fmt.Sprintf(`#!/bin/sh
exec node "$(dirname "$0")/%s" dlx "$@"
`, cjsRelPosix)
	pnpxPath := filepath.Join(binDir, "pnpx")
	if err := os.WriteFile(pnpxPath, []byte(pnpxScript), 0755); err != nil {
		return err
	}

	cmdScript := fmt.Sprintf(`@echo off
node "%%~dp0%s" %%*
`, cjsRelPosix)
	if err := os.WriteFile(filepath.Join(binDir, "pnpm.cmd"), []byte(cmdScript), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(binDir, "pnpx.cmd"),
		[]byte(fmt.Sprintf("@echo off\nnode \"%%~dp0%s\" dlx %%*\n", cjsRelPosix)), 0755); err != nil {
		return err
	}

	logger.Verbose("  → Generated pnpm wrappers in %s", binDir)
	return nil
}

// flattenSingleChild 如果 dir 里只有一个子目录（名字以 prefix 开头），
// 把这个子目录的内容上移一层
func flattenSingleChild(dir, prefix string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var target string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), prefix) {
			target = filepath.Join(dir, e.Name())
			break
		}
	}
	if target == "" {
		return nil
	}

	subEntries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	for _, se := range subEntries {
		src := filepath.Join(target, se.Name())
		dst := filepath.Join(dir, se.Name())

		// 移动文件/目录，带重试机制（Windows 杀软/句柄延迟问题）
		if err := robustMove(src, dst); err != nil {
			return fmt.Errorf("flatten %s: %w", se.Name(), err)
		}
	}

	// 删除空的父目录，带重试
	for i := 0; i < 5; i++ {
		if err := os.Remove(target); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// robustMove 健壮的移动文件/目录函数
// Windows 上经常因杀软扫描、句柄延迟等原因导致 rename 失败
// 此函数会先尝试 rename，失败后等待重试，最后回退到 copy+delete
func robustMove(src, dst string) error {
	// 第一次尝试直接 rename
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	
	// rename 失败，等待后重试（延迟逐渐增加）
	delays := []time.Duration{
		100 * time.Millisecond,
		300 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		3 * time.Second,
	}
	
	var lastErr error
	for _, delay := range delays {
		time.Sleep(delay)
		if err := os.Rename(src, dst); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	
	// rename 重试全部失败，尝试 copy + delete 作为最后手段
	if copyErr := copyMove(src, dst); copyErr != nil {
		return fmt.Errorf("rename failed: %v; copy fallback also failed: %v", lastErr, copyErr)
	}
	return nil
}

// copyMove 通过复制+删除的方式移动文件/目录（当 rename 失败时的回退方案）
// 适用于 Windows 上 rename 因权限/杀软扫描/句柄延迟而失败的场景
func copyMove(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return copyDir(src, dst)
	}

	// 普通文件：读取+写入+删除
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, info.Mode()); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			info, err := e.Info()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
				return err
			}
		}
	}
	return os.RemoveAll(src)
}

// mergeInstall 将 src 的所有文件合并到 dst 目录。
// 与 copyDir 不同：如果目标文件已存在且被占用（无法覆盖），则跳过该文件继续处理其他文件。
// 用于安装目录中有文件被占用（如 node.exe 被进程使用）时的回退方案：
// 被占用的文件保持不变（版本相同无需替换），其余文件正常更新。
func mergeInstall(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			// 目录：递归合并
			if _, err := os.Stat(dstPath); os.IsNotExist(err) {
				// dst 不存在，直接 rename
				if rerr := os.Rename(srcPath, dstPath); rerr != nil {
					// rename 失败，回退到递归合并
					_ = os.MkdirAll(dstPath, 0755)
					if merr := mergeInstall(srcPath, dstPath); merr != nil {
						logger.Verbose("  → merge dir %s: %v (skipped)", e.Name(), merr)
					}
				}
			} else {
				// dst 已存在，递归合并
				if merr := mergeInstall(srcPath, dstPath); merr != nil {
					logger.Verbose("  → merge dir %s: %v (skipped)", e.Name(), merr)
				}
			}
		} else {
			// 文件：先尝试删除旧文件，再 rename
			_ = os.Remove(dstPath)
			if rerr := os.Rename(srcPath, dstPath); rerr != nil {
				// rename 失败（目标被占用），跳过该文件
				logger.Verbose("  → skip %s (in use)", e.Name())
			}
		}
	}
	return nil
}
