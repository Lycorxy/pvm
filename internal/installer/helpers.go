package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
)

// flattenRust 处理 Rust 官方预编译包的目录结构
//
// rust-{version}-{target}/
//
//	├── install.sh
//	├── components
//	├── rustc/{bin,lib,share,...}
//	├── cargo/{bin,...}
//	├── rust-std-{target}/lib/rustlib/...
//	├── rustfmt-preview/bin/rustfmt
//	├── clippy-preview/bin/cargo-clippy
//	├── rust-docs/...
//	└── ...
//
// 我们需要把每个 component 下的 bin/lib/share 合并到 base 下：
//
//	base/
//	├── bin/{rustc,cargo,rustfmt,cargo-clippy,clippy-driver,...}
//	├── lib/...
//	└── share/...
func flattenRust(extractTmp string) error {
	// 第一步：找到顶层 rust-... 目录
	entries, err := os.ReadDir(extractTmp)
	if err != nil {
		return err
	}

	var topDir string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "rust-") {
			topDir = filepath.Join(extractTmp, e.Name())
			break
		}
	}
	if topDir == "" {
		return fmt.Errorf("rust top-level directory not found")
	}

	// 第二步：遍历 topDir 下的每个 component，把它们的 bin/lib/share 合并到 extractTmp
	components, err := os.ReadDir(topDir)
	if err != nil {
		return err
	}

	mergeDirs := []string{"bin", "lib", "libexec", "share", "etc"}
	for _, c := range components {
		if !c.IsDir() {
			continue
		}
		// 跳过非 component 目录
		name := c.Name()
		if name == "components" || name == "manifest.in" {
			continue
		}
		compDir := filepath.Join(topDir, name)
		for _, sub := range mergeDirs {
			srcDir := filepath.Join(compDir, sub)
			if _, err := os.Stat(srcDir); err != nil {
				continue
			}
			dstDir := filepath.Join(extractTmp, sub)
			if err := mergeDir(srcDir, dstDir); err != nil {
				logger.Verbose("  merge %s/%s: %v", name, sub, err)
			}
		}
	}

	// 第三步：清理顶层 rust-... 目录
	_ = os.RemoveAll(topDir)

	// 校验 bin 目录存在
	binDir := filepath.Join(extractTmp, "bin")
	if _, err := os.Stat(binDir); err != nil {
		return fmt.Errorf("rust bin directory not found after flatten: %w", err)
	}
	return nil
}

// mergeDir 把 src 下的所有内容移动/合并到 dst
// 如果 dst 不存在则直接重命名；否则递归合并
func mergeDir(src, dst string) error {
	if _, err := os.Stat(dst); err != nil {
		// dst 不存在，直接 rename
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		return os.Rename(src, dst)
	}
	// dst 存在，逐个移动子项
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if _, err := os.Stat(d); err == nil {
				if err := mergeDir(s, d); err != nil {
					return err
				}
				continue
			}
		}
		_ = os.Rename(s, d)
	}
	return os.RemoveAll(src)
}

// generatePythonWrappers 在 Python 安装目录生成 pip 包装脚本
// python-build-standalone 的 install_only 包包含 pip 模块，但没有 pip.exe 启动器
func generatePythonWrappers(installDir string) error {
	// 检查 pip 模块是否存在
	pipModulePath := filepath.Join(installDir, "Lib", "site-packages", "pip", "__init__.py")
	if _, err := os.Stat(pipModulePath); err != nil {
		logger.Verbose("  pip module not found at %s, skipping wrapper generation", pipModulePath)
		return nil // pip 模块不存在不是致命错误
	}

	// Windows 上 python.exe 在根目录（BinDir == InstallDir）
	var binDir string
	if runtime.GOOS == "windows" {
		binDir = installDir
	} else {
		binDir = filepath.Join(installDir, "bin")
	}

	// 生成 pip 和 pip3 包装脚本
	for _, name := range []string{"pip", "pip3"} {
		if runtime.GOOS == "windows" {
			cmdContent := fmt.Sprintf(`@echo off
python -m pip %%*
`)
			if err := os.WriteFile(filepath.Join(binDir, name+".cmd"), []byte(cmdContent), 0755); err != nil {
				return err
			}
		}

		shScript := fmt.Sprintf(`#!/bin/sh
exec "$(dirname "$0")/python" -m pip "$@"
`)
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(shScript), 0755); err != nil {
			return err
		}
	}

	logger.Verbose("  → Generated pip wrappers in %s", binDir)
	return nil
}

// generateYarnWrappers 在 yarn 安装目录的 bin/ 下生成可执行包装脚本
// yarn npm 包结构:
//
//	package/
//	├── bin/
//	│   ├── yarn         (shell 脚本)
//	│   ├── yarn.js      (Node.js 入口)
//	│   ├── yarnpkg      (shell 脚本)
//	│   └── yarn.cmd     (windows 入口，部分版本)
//	└── lib/cli.js
//
// 我们生成跨平台 wrapper：bin/yarn(.cmd), bin/yarnpkg(.cmd)
// 注意：yarn 依赖 node 在 PATH 中（通过 pvm shim 解析）
func generateYarnWrappers(installDir string) error {
	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	// 找 yarn 入口 js 文件
	jsCandidates := []string{
		filepath.Join("bin", "yarn.js"),
		filepath.Join("lib", "cli.js"),
	}
	var entryRel string
	for _, c := range jsCandidates {
		if _, err := os.Stat(filepath.Join(installDir, c)); err == nil {
			entryRel = filepath.ToSlash(c)
			break
		}
	}
	if entryRel == "" {
		return fmt.Errorf("yarn entry js not found")
	}

	// shell 脚本（unix）
	for _, name := range []string{"yarn", "yarnpkg"} {
		// 不直接覆盖原始的 bin/yarn 脚本（可能更完整），先看是否存在
		shPath := filepath.Join(binDir, name)
		if _, err := os.Stat(shPath); err == nil {
			// 已经存在原始脚本，确保有执行权限
			_ = os.Chmod(shPath, 0755)
		} else {
			// 我们生成一个简单的：先尝试本地 entry，再 fallback
			script := fmt.Sprintf(`#!/bin/sh
exec node "$(dirname "$0")/../%s" "$@"
`, entryRel)
			if err := os.WriteFile(shPath, []byte(script), 0755); err != nil {
				return err
			}
		}

		// Windows .cmd
		cmdPath := filepath.Join(binDir, name+".cmd")
		cmdScript := fmt.Sprintf(`@echo off
node "%%~dp0..\%s" %%*
`, strings.ReplaceAll(entryRel, "/", "\\"))
		if err := os.WriteFile(cmdPath, []byte(cmdScript), 0755); err != nil {
			return err
		}
	}

	logger.Verbose("  → Generated yarn wrappers in %s", binDir)
	return nil
}

// renameWithRetry 在 Windows 下，目录刚被 RemoveAll 后立即 Rename 偶尔会因为
// 杀软扫描 / 索引服务 / 句柄延迟释放而返回 "Access is denied"。
// 这里加上指数退避重试，最多 ~3 秒。
// 如果目标目录因文件被占用无法整体删除，则先逐文件清理，再 rename。
func renameWithRetry(src, dst string) error {
	delays := []time.Duration{0, 100 * time.Millisecond, 250 * time.Millisecond,
		500 * time.Millisecond, 1 * time.Second, 1500 * time.Millisecond}
	var lastErr error
	for i, d := range delays {
		if d > 0 {
			time.Sleep(d)
		}
		// 每次重试前都尝试再删一次目标目录，避免上一次部分残留。
		if _, err := os.Stat(dst); err == nil {
			if removeErr := os.RemoveAll(dst); removeErr != nil {
				// RemoveAll 失败（文件被占用），改用逐文件清理
				// 只在第一次遇到此情况时执行，避免重复
				if i == 0 {
					logger.Verbose("  → target dir busy, trying file-by-file cleanup: %s", dst)
					removeDirBestEffort(dst)
				}
			}
		}
		if err := os.Rename(src, dst); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// removeDirBestEffort 递归删除目录，跳过被占用的文件，尽量清理其他内容。
// 用于在 rename 前清理被部分占用的旧安装目录。
func removeDirBestEffort(path string) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}
	if !info.IsDir() {
		_ = os.Remove(path)
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		removeDirBestEffort(filepath.Join(path, entry.Name()))
	}
	// 尝试删除目录本身（只有子项全部清空才能成功）
	_ = os.Remove(path)
}

// pruneSiblingVersions 移除 ~/.pvm/installs/<rt>/ 下所有非 keepVersion 的兄弟目录。
//
// 用于全局唯一 runtime（如 git）的安装流程：保证同一时刻
// 该 runtime 在 installs 下只存在一个版本目录。
//
// keepVersion 为空字符串表示清除所有版本。
//
// 注意：
//   - 与 keepVersion 同名的 ".tmp" 目录也会被保留，因为正在解压中
//   - 找不到 runtime 目录时返回 nil（不视为错误）
//   - 单个子目录删除失败仅记录日志，继续处理其他兄弟，避免一个残留阻断整个安装
func pruneSiblingVersions(rt, keepVersion string) error {
	rtDir := filepath.Join(config.InstallsDir(), rt)
	entries, err := os.ReadDir(rtDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	keepName := keepVersion
	keepTmp := keepVersion + ".tmp"
	for _, e := range entries {
		name := e.Name()
		if name == keepName || name == keepTmp {
			continue
		}
		full := filepath.Join(rtDir, name)
		if err := os.RemoveAll(full); err != nil {
			// RemoveAll 失败（可能有文件被占用），改用逐文件清理
			logger.Verbose("  prune: RemoveAll failed for %s: %v, trying best-effort cleanup", full, err)
			removeDirBestEffort(full)
			// 检查是否清理成功
			if _, statErr := os.Stat(full); statErr == nil {
				logger.Info("  ⚠ Could not fully remove previous %s installation: %s (files in use, will retry on next install)", rt, name)
			} else {
				logger.Info("  → Removed previous %s installation: %s", rt, name)
			}
		} else {
			logger.Info("  → Removed previous %s installation: %s", rt, name)
		}
	}
	return nil
}
