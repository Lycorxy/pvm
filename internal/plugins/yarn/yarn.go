// Package yarn 提供 Yarn 包管理器插件
package yarn

import (
	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
)

const name = "yarn"

// YarnPlugin Yarn 插件实现
type YarnPlugin struct {
	plugin.BasePlugin
}

// New 创建 Yarn 插件
func New() *YarnPlugin {
	return &YarnPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypePackageManager,
			false,
			"yarn",
			[]string{"yarn", "yarnpkg"},
		),
	}
}

// GetDownloadInfo 返回 Yarn 下载信息
func (p *YarnPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	return registry.GetDownloadInfo(name, version)
}

// ListRemoteVersions 返回 Yarn 可用版本列表
func (p *YarnPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	return registry.ListRemoteVersions(name, !stableOnly)
}

// Install 安装 Yarn
func (p *YarnPlugin) Install(extractTmp, version, installDir string) error {
	return nil
}

// Verify 验证 Yarn 安装
func (p *YarnPlugin) Verify(installDir string) error {
	return nil
}
