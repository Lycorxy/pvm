// Package rust 提供 Rust runtime 插件
package rust

import (
	"fmt"
	"regexp"

	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
)

const name = "rust"

// RustPlugin Rust 插件实现
type RustPlugin struct {
	plugin.BasePlugin
}

// New 创建 Rust 插件
func New() *RustPlugin {
	return &RustPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypeTool,
			true, // Rust 只支持全局安装
			"rustc",
			[]string{"rustc", "rustup", "cargo"},
		),
	}
}

// GetDownloadInfo 返回 Rust 下载信息
func (p *RustPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	target, err := p.rustTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://static.rust-lang.org/dist/rust-%s-%s.tar.gz",
		version, target,
	)
	return &registry.RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
}

// GetDownloadInfoMirror 使用国内镜像（rsproxy.cn 字节跳动）
func (p *RustPlugin) GetDownloadInfoMirror(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	target, err := p.rustTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://rsproxy.cn/dist/rust-%s-%s.tar.gz",
		version, target,
	)
	return &registry.RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
}

// rustTarget 返回 Rust 的 target triple
func (p *RustPlugin) rustTarget(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		switch goarch {
		case "amd64":
			return "x86_64-pc-windows-msvc", nil
		case "arm64":
			return "aarch64-pc-windows-msvc", nil
		default:
			return "", fmt.Errorf("unsupported arch for rust on windows: %s", goarch)
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-gnu", nil
		case "arm64":
			return "aarch64-unknown-linux-gnu", nil
		default:
			return "", fmt.Errorf("unsupported arch for rust on linux: %s", goarch)
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-darwin", nil
		case "arm64":
			return "aarch64-apple-darwin", nil
		default:
			return "", fmt.Errorf("unsupported arch for rust on darwin: %s", goarch)
		}
	default:
		return "", fmt.Errorf("unsupported OS for rust: %s", goos)
	}
}

// rustReleaseRe 匹配 Rust GitHub release tag
var rustReleaseRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// ListRemoteVersions 返回 Rust 可用版本列表
func (p *RustPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	return registry.ListRemoteVersions(name, !stableOnly)
}

// Install 安装 Rust
func (p *RustPlugin) Install(extractTmp, version, installDir string) error {
	return nil
}

// Verify 验证 Rust 安装
func (p *RustPlugin) Verify(installDir string) error {
	return nil
}
