// Package git 提供 Git 工具插件
package git

import (
	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
)

const name = "git"

// GitPlugin Git 插件实现
type GitPlugin struct {
	plugin.BasePlugin
}

// New 创建 Git 插件
func New() *GitPlugin {
	return &GitPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypeTool,
			true, // Git 只支持全局安装
			"git",
			[]string{"git"},
		),
	}
}

// GetDownloadInfo 返回 Git 下载信息
func (p *GitPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	return registry.GetDownloadInfo(name, version)
}

// ListRemoteVersions 返回 Git 可用版本列表
func (p *GitPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	return registry.ListRemoteVersions(name, !stableOnly)
}

// Install 安装 Git
func (p *GitPlugin) Install(extractTmp, version, installDir string) error {
	return nil
}

// Verify 验证 Git 安装
func (p *GitPlugin) Verify(installDir string) error {
	return nil
}