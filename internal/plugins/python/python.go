// Package python 提供 Python runtime 插件
package python

import (
	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
)

const name = "python"

// PythonPlugin Python 插件实现
type PythonPlugin struct {
	plugin.BasePlugin
}

// New 创建 Python 插件
func New() *PythonPlugin {
	return &PythonPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypeRuntime,
			false,
			"python",
			[]string{"python", "pip", "pip3"},
		),
	}
}

// GetDownloadInfo 返回 Python 下载信息
func (p *PythonPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	return registry.GetDownloadInfo(name, version)
}

// ListRemoteVersions 返回 Python 可用版本列表
func (p *PythonPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	return registry.ListRemoteVersions(name, !stableOnly)
}

// Install 安装 Python
func (p *PythonPlugin) Install(extractTmp, version, installDir string) error {
	return nil
}

// Verify 验证 Python 安装
func (p *PythonPlugin) Verify(installDir string) error {
	return nil
}
