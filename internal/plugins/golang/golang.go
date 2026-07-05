// Package golang 提供 Go runtime 插件
package golang

import (
	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
)

const name = "go"

// GoPlugin Go 插件实现
type GoPlugin struct {
	plugin.BasePlugin
}

// New 创建 Go 插件
func New() *GoPlugin {
	return &GoPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypeTool,
			true, // Go 只支持全局安装
			"go",
			[]string{"go", "gofmt"},
		),
	}
}

// GetDownloadInfo 返回 Go 下载信息
func (p *GoPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	return registry.GetDownloadInfo(name, version)
}

// ListRemoteVersions 返回 Go 可用版本列表
func (p *GoPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	return registry.ListRemoteVersions(name, !stableOnly)
}

// Install 安装 Go
func (p *GoPlugin) Install(extractTmp, version, installDir string) error {
	return nil
}

// Verify 验证 Go 安装
func (p *GoPlugin) Verify(installDir string) error {
	return nil
}