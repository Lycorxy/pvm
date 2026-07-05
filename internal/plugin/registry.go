// Package plugin 提供插件注册和管理功能
package plugin

import (
	"fmt"
	"sort"
	"sync"
)

// Registry 插件注册中心
var Registry = &PluginRegistry{
	plugins: make(map[string]RuntimePlugin),
}

// PluginRegistry 插件注册中心
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]RuntimePlugin
}

// Register 注册插件
func (r *PluginRegistry) Register(p RuntimePlugin) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.plugins[name]; exists {
		panic(fmt.Sprintf("plugin %s already registered", name))
	}
	r.plugins[name] = p
}

// Get 获取插件
func (r *PluginRegistry) Get(name string) RuntimePlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.plugins[name]
}

// Exists 检查插件是否存在
func (r *PluginRegistry) Exists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.plugins[name] != nil
}

// ListAll 返回所有插件名称列表
func (r *PluginRegistry) ListAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListByType 按类型返回插件列表
func (r *PluginRegistry) ListByType(typ RuntimeType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0)
	for name, p := range r.plugins {
		if p.Type() == typ {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ListGlobalOnly 返回只支持全局安装的插件列表
func (r *PluginRegistry) ListGlobalOnly() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0)
	for name, p := range r.plugins {
		if p.IsGlobalOnly() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ========== 全局辅助函数 ==========

// RegisterPlugin 注册插件（简写）
func RegisterPlugin(p RuntimePlugin) {
	Registry.Register(p)
}

// GetPlugin 获取插件（简写）
func GetPlugin(name string) RuntimePlugin {
	return Registry.Get(name)
}

// IsSupportedRuntime 检查是否为支持的 runtime
func IsSupportedRuntime(name string) bool {
	return Registry.Exists(name)
}

// SupportedRuntimes 返回所有支持的 runtime 名称
func SupportedRuntimes() []string {
	return Registry.ListAll()
}

// GlobalOnlyRuntimes 返回只支持全局安装的 runtime
func GlobalOnlyRuntimes() []string {
	return Registry.ListGlobalOnly()
}