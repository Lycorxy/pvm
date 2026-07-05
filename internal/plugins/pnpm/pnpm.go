// Package pnpm 提供 pnpm 包管理器插件
package pnpm

import (
	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
)

const name = "pnpm"

// PnpmPlugin pnpm 插件实现
type PnpmPlugin struct {
	plugin.BasePlugin
}

// New 创建 pnpm 插件
func New() *PnpmPlugin {
	return &PnpmPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypePackageManager,
			false,
			"pnpm",
			[]string{"pnpm"},
		),
	}
}

// GetDownloadInfo 返回 pnpm 下载信息
func (p *PnpmPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	return registry.GetDownloadInfo(name, version)
}

// ListRemoteVersions 返回 pnpm 可用版本列表
func (p *PnpmPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	return registry.ListRemoteVersions(name, !stableOnly)
}

// Install 安装 pnpm
func (p *PnpmPlugin) Install(extractTmp, version, installDir string) error {
	return nil
}

// Verify 验证 pnpm 安装
func (p *PnpmPlugin) Verify(installDir string) error {
	return nil
}
