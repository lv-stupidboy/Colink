package agent

import (
	"fmt"
	"sync"

	"github.com/anthropic/isdp/internal/model"
)

// AdapterRegistry 全局适配器注册中心
type AdapterRegistry struct {
	plugins map[model.BaseAgentType]PluginMeta
	mu      sync.RWMutex
}

// 全局注册中心实例
var globalRegistry = &AdapterRegistry{
	plugins: make(map[model.BaseAgentType]PluginMeta),
}

// RegisterPlugin 注册插件（插件 init() 调用）
func RegisterPlugin(meta PluginMeta) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if _, exists := globalRegistry.plugins[meta.Type]; exists {
		panic(fmt.Sprintf("plugin %s already registered", meta.Type))
	}

	globalRegistry.plugins[meta.Type] = meta
}

// GetAdapter 获取适配器（编排层调用）
func GetAdapter(baseAgent *model.BaseAgent) AgentAdapter {
	if baseAgent == nil {
		return nil
	}

	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	meta, exists := globalRegistry.plugins[baseAgent.Type]
	if !exists {
		return nil
	}

	return meta.Factory(baseAgent)
}

// GetTypes 获取所有已注册类型（API调用）
func GetTypes() []PluginTypeInfo {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	types := make([]PluginTypeInfo, 0, len(globalRegistry.plugins))
	for _, meta := range globalRegistry.plugins {
		types = append(types, PluginTypeInfo{
			Type:        meta.Type,
			Name:        meta.Name,
			Description: meta.Description,
		})
	}
	return types
}

// GetMeta 获取插件元数据
func GetMeta(typ model.BaseAgentType) *PluginMeta {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	meta, exists := globalRegistry.plugins[typ]
	if !exists {
		return nil
	}
	return &meta
}

// GetConfigDir 获取配置目录名
func GetConfigDir(typ model.BaseAgentType) string {
	meta := GetMeta(typ)
	if meta == nil {
		return ".claude" // 默认
	}
	return meta.ConfigDir
}