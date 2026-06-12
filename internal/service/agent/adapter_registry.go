package agent

import (
	"fmt"
	"sort"
	"sync"

	"github.com/anthropic/isdp/internal/model"
	"go.uber.org/zap"
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

// Claude Code ACP 模式配置
// 通过 config.yaml 的 claude_code.use_acp 控制
var claudeCodeUseACP bool
var claudeCodeUseACPMu sync.RWMutex

// SetClaudeCodeUseACP 设置 Claude Code 是否使用 ACP 协议
// 由 main.go 在启动时根据配置调用
func SetClaudeCodeUseACP(useACP bool) {
	claudeCodeUseACPMu.Lock()
	claudeCodeUseACP = useACP
	claudeCodeUseACPMu.Unlock()
}

// GetClaudeCodeUseACP 获取 Claude Code 是否使用 ACP 协议
func GetClaudeCodeUseACP() bool {
	claudeCodeUseACPMu.RLock()
	defer claudeCodeUseACPMu.RUnlock()
	return claudeCodeUseACP
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
	// 按类型名称排序，确保顺序固定（Go map 遍历顺序不固定）
	sort.Slice(types, func(i, j int) bool {
		return types[i].Type < types[j].Type
	})
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

// GetConfigGeneratorFactory 获取配置生成器工厂
func GetConfigGeneratorFactory(typ model.BaseAgentType) ConfigGeneratorFactory {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	meta, exists := globalRegistry.plugins[typ]
	if !exists || meta.ConfigGeneratorFactory == nil {
		return nil
	}
	return meta.ConfigGeneratorFactory
}

// CreateConfigGenerator 创建配置生成器实例
// 传入存储路径参数，调用工厂函数创建 AssetConfigGenerator
func CreateConfigGenerator(
	typ model.BaseAgentType,
	skillStoragePath string,
	subagentStoragePath string,
	commandStoragePath string,
	ruleStoragePath string,
	logger *zap.Logger,
) AssetConfigGenerator {
	factory := GetConfigGeneratorFactory(typ)
	if factory == nil {
		return nil
	}
	return factory(skillStoragePath, subagentStoragePath, commandStoragePath, ruleStoragePath, logger)
}