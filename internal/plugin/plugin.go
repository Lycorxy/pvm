// Package plugin 定义了 runtime 插件的接口
//
// 每个运行时（如 node、bun、deno、rust 等）通过实现 RuntimePlugin 接口，
// 可以独立定义自己的下载、安装、验证逻辑，便于扩展和维护。
package plugin

import (
	"github.com/pvm/pvm/internal/registry"
)

// RuntimeType runtime 类型
type RuntimeType string

const (
	TypeRuntime        RuntimeType = "runtime"         // 运行时：node, bun, deno, python
	TypePackageManager RuntimeType = "package_manager" // 包管理器：pnpm, yarn
	TypeTool           RuntimeType = "tool"            // 工具：go, git, rust
)

// RuntimePlugin runtime 插件接口
//
// 所有 runtime 必须实现此接口才能被 pvm 管理。
type RuntimePlugin interface {
	// ========== 元信息 ==========

	// Name 返回 runtime 名称（如 "node", "bun", "rust"）
	Name() string

	// Type 返回 runtime 类型
	Type() RuntimeType

	// IsGlobalOnly 返回是否只支持全局安装
	// 如 go、git、rust 等工具，不需要项目级版本管理
	IsGlobalOnly() bool

	// ========== 下载 ==========

	// GetDownloadInfo 返回下载信息
	// version: 版本号
	// goos: 操作系统（windows, linux, darwin）
	// goarch: 架构（amd64, arm64）
	GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error)

	// ListRemoteVersions 返回远程可用版本列表
	// stableOnly: 是否只返回稳定版本
	ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error)

	// ========== 安装 ==========

	// Install 执行安装逻辑
	// extractTmp: 解压后的临时目录
	// version: 版本号
	// installDir: 最终安装目录
	Install(extractTmp, version, installDir string) error

	// Verify 验证安装是否成功
	// installDir: 安装目录
	Verify(installDir string) error

	// ========== 命令 ==========

	// MainCommand 返回主命令名（如 "node", "bun"）
	MainCommand() string

	// Commands 返回所有需要 shim 拦截的命令列表
	// 如 node 返回 ["node", "npm", "npx"]
	Commands() []string
}

// BasePlugin 插件基础实现
//
// 提供一些通用方法，具体插件可以嵌入此结构体来减少重复代码。
type BasePlugin struct {
	name        string
	typ         RuntimeType
	globalOnly  bool
	mainCommand string
	commands    []string
}

// NewBasePlugin 创建 BasePlugin
func NewBasePlugin(name string, typ RuntimeType, globalOnly bool, mainCommand string, commands []string) BasePlugin {
	return BasePlugin{
		name:        name,
		typ:         typ,
		globalOnly:  globalOnly,
		mainCommand: mainCommand,
		commands:    commands,
	}
}

// Name 返回名称
func (p *BasePlugin) Name() string {
	return p.name
}

// Type 返回类型
func (p *BasePlugin) Type() RuntimeType {
	return p.typ
}

// IsGlobalOnly 返回是否全局
func (p *BasePlugin) IsGlobalOnly() bool {
	return p.globalOnly
}

// MainCommand 返回主命令
func (p *BasePlugin) MainCommand() string {
	return p.mainCommand
}

// Commands 返回命令列表
func (p *BasePlugin) Commands() []string {
	return p.commands
}
