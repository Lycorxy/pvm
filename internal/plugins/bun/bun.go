// Package bun 提供 Bun runtime 插件
package bun

import (
	"fmt"

	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
)

const name = "bun"

// BunPlugin Bun 插件实现
type BunPlugin struct {
	plugin.BasePlugin
}

// New 创建 Bun 插件
func New() *BunPlugin {
	return &BunPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypeRuntime,
			false,
			"bun",
			[]string{"bun", "bunx"},
		),
	}
}

// GetDownloadInfo 返回 Bun 下载信息
func (p *BunPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	target, err := p.bunTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://github.com/oven-sh/bun/releases/download/bun-v%s/bun-%s.zip",
		version, target,
	)
	return &registry.RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// GetDownloadInfoMirror 使用国内镜像
func (p *BunPlugin) GetDownloadInfoMirror(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	target, err := p.bunTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://registry.npmmirror.com/-/binary/bun/bun-v%s/bun-%s.zip",
		version, target,
	)
	return &registry.RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// bunTarget 将 GOOS/GOARCH 映射为 Bun 的 target 字符串
func (p *BunPlugin) bunTarget(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		switch goarch {
		case "amd64":
			return "windows-x64", nil
		default:
			return "", fmt.Errorf("bun on windows only supports x64 (got %s)", goarch)
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "linux-x64", nil
		case "arm64":
			return "linux-aarch64", nil
		default:
			return "", fmt.Errorf("unsupported arch for bun on linux: %s", goarch)
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "darwin-x64", nil
		case "arm64":
			return "darwin-aarch64", nil
		default:
			return "", fmt.Errorf("unsupported arch for bun on darwin: %s", goarch)
		}
	default:
		return "", fmt.Errorf("unsupported OS for bun: %s", goos)
	}
}

// ListRemoteVersions 返回 Bun 可用版本列表
func (p *BunPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	return registry.ListRemoteVersions(name, !stableOnly)
}

// Install 安装 Bun
func (p *BunPlugin) Install(extractTmp, version, installDir string) error {
	// Bun 解压后目录结构需要处理
	return nil
}

// Verify 验证 Bun 安装
func (p *BunPlugin) Verify(installDir string) error {
	return nil
}
