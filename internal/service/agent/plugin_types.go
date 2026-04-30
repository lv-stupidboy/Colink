package agent

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PluginMeta 插件元数据
type PluginMeta struct {
	Type                model.BaseAgentType // "claude_code", "open_code"
	Name                string              // 显示名称："ClaudeCode", "OpenCode"
	Description         string              // 描述
	Factory             AdapterFactory      // func(baseAgent) AgentAdapter
	ConfigDir           string              // 配置目录名：".claude", ".opencode"
	DefaultPath         string              // 默认CLI路径："claude", "opencode"
	ConfigGeneratorFactory ConfigGeneratorFactory // 配置生成器工厂（可选）
}

// AdapterFactory 适配器工厂函数
type AdapterFactory func(baseAgent *model.BaseAgent) AgentAdapter

// ConfigGeneratorFactory 配置生成器工厂函数
// 接收存储路径参数，返回 AssetConfigGenerator 实例
type ConfigGeneratorFactory func(
	skillStoragePath string,
	subagentStoragePath string,
	commandStoragePath string,
	ruleStoragePath string,
	logger *zap.Logger,
) AssetConfigGenerator

// PluginTypeInfo 插件类型信息（用于API返回）
type PluginTypeInfo struct {
	Type        model.BaseAgentType `json:"type"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
}

// AssetConfigGenerator 资产配置生成器接口
// 每种 Agent 类型可以实现自己的配置生成逻辑
// 注意：与 config_service.go 的 ConfigGenerator 接口不同（用于 BatchGenerateConfig）
type AssetConfigGenerator interface {
	// GenerateConfig 生成Agent配置
	GenerateConfig(ctx context.Context, req *ConfigGenerateRequest) (*ConfigGenerateResult, error)

	// PreviewConfig 预览配置内容（可选，用于调试）
	PreviewConfig(ctx context.Context, req *ConfigPreviewRequest) (*ConfigPreviewResult, error)
}

// ConfigGenerateRequest 配置生成请求
type ConfigGenerateRequest struct {
	AgentRoleID   uuid.UUID       // Agent角色ID
	BaseAgentType string          // Agent类型
	ConfigPath    string          // 目标配置路径
	Skills        []*model.Skill  // 已过滤的技能列表
	Commands      []*model.Command // 已过滤的命令列表
	Subagents     []*model.Subagent // 已过滤的子代理列表
	Rules         []*model.Rule   // 已过滤的规则列表
	Settings      []*model.Settings // 已过滤的设置列表
	CleanExisting bool            // 是否清理现有配置
}

// ConfigGenerateResult 配置生成结果
type ConfigGenerateResult struct {
	ConfigPath     string // 生成的配置路径
	SkillsCount    int    // 技能数量
	CommandsCount  int    // 命令数量
	SubagentsCount int    // 子代理数量
	RulesCount     int    // 规则数量
	SettingsCount  int    // 设置数量
}

// ConfigPreviewRequest 配置预览请求
type ConfigPreviewRequest struct {
	AgentRoleID   uuid.UUID
	BaseAgentType string
	ConfigPath    string
}

// ConfigPreviewResult 配置预览结果
type ConfigPreviewResult struct {
	Files []ConfigPreviewFile
}

// ConfigPreviewFile 预览文件信息
type ConfigPreviewFile struct {
	Path    string // 相对路径
	Content string // 文件内容
}