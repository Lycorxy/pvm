// Package plugins 提供所有 runtime 插件的自动注册
package plugins

import (
	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/plugins/bun"
	"github.com/pvm/pvm/internal/plugins/deno"
	"github.com/pvm/pvm/internal/plugins/git"
	"github.com/pvm/pvm/internal/plugins/golang"
	"github.com/pvm/pvm/internal/plugins/node"
	"github.com/pvm/pvm/internal/plugins/pnpm"
	"github.com/pvm/pvm/internal/plugins/python"
	"github.com/pvm/pvm/internal/plugins/rust"
	"github.com/pvm/pvm/internal/plugins/yarn"
)

// RegisterAll 注册所有内置插件
//
// 在程序启动时调用此函数，自动注册所有支持的 runtime。
func RegisterAll() {
	// 运行时（支持项目级版本管理）
	plugin.RegisterPlugin(node.New())
	plugin.RegisterPlugin(bun.New())
	plugin.RegisterPlugin(deno.New())
	plugin.RegisterPlugin(python.New())

	// 包管理器（支持项目级版本管理）
	plugin.RegisterPlugin(pnpm.New())
	plugin.RegisterPlugin(yarn.New())

	// 工具（只支持全局安装）
	plugin.RegisterPlugin(golang.New())
	plugin.RegisterPlugin(git.New())
	plugin.RegisterPlugin(rust.New())
}
