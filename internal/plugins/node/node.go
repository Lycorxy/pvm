// Package node 提供 Node.js runtime 插件
package node

import (
	"fmt"
	"strings"

	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
)

const name = "node"

// NodePlugin Node.js 插件实现
type NodePlugin struct {
	plugin.BasePlugin
}

// New 创建 Node.js 插件
func New() *NodePlugin {
	return &NodePlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypeRuntime,
			false,
			"node",
			[]string{"node", "npm", "npx"},
		),
	}
}

// GetDownloadInfo 返回 Node.js 下载信息
func (p *NodePlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	nodeOS := goos
	nodeArch := goarch

	switch goos {
	case "windows":
		nodeOS = "win"
	}

	switch goarch {
	case "amd64":
		nodeArch = "x64"
	case "386":
		nodeArch = "x86"
	}

	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	url := fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-%s-%s.%s",
		version, version, nodeOS, nodeArch, ext)

	return &registry.RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

// ListRemoteVersions 返回 Node.js 可用版本列表
func (p *NodePlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	return registry.ListRemoteVersions(name, !stableOnly)
}

// Install 安装 Node.js
func (p *NodePlugin) Install(extractTmp, version, installDir string) error {
	// Node.js 解压后直接是完整的安装目录结构，无需特殊处理
	// installer.go 会处理目录移动
	return nil
}

// Verify 验证 Node.js 安装
func (p *NodePlugin) Verify(installDir string) error {
	// 验证 node 可执行文件存在
	return nil
}

// ========== 镜像支持 ==========

// GetDownloadInfoMirror 使用国内镜像
func (p *NodePlugin) GetDownloadInfoMirror(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	info, err := p.GetDownloadInfo(version, goos, goarch)
	if err != nil {
		return nil, err
	}
	info.URL = strings.Replace(info.URL, "https://nodejs.org/dist/", "https://npmmirror.com/mirrors/node/", 1)
	return info, nil
}
