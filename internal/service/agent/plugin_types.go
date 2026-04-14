package agent

import (
	"github.com/anthropic/isdp/internal/model"
)

// PluginMeta 插件元数据
type PluginMeta struct {
	Type        model.BaseAgentType // "claude_code", "open_code"
	Name        string              // 显示名称："ClaudeCode", "OpenCode"
	Description string              // 描述
	Factory     AdapterFactory      // func(baseAgent) AgentAdapter
	ConfigDir   string              // 配置目录名：".claude", ".opencode"
	DefaultPath string              // 默认CLI路径："claude", "opencode"
}

// AdapterFactory 适配器工厂函数
type AdapterFactory func(baseAgent *model.BaseAgent) AgentAdapter

// PluginTypeInfo 插件类型信息（用于API返回）
type PluginTypeInfo struct {
	Type        model.BaseAgentType `json:"type"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
}