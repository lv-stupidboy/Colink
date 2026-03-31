package mcp

// ToolDefinition MCP 工具定义
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MultiMentionToolInput isdp_multi_mention 工具输入
type MultiMentionToolInput struct {
	Targets        []string `json:"targets"`        // 目标 Agent ID 列表 (1-3 个)
	Question       string   `json:"question"`       // 问题内容
	CallbackTo     string   `json:"callbackTo"`     // 回调 Agent ID
	Context        string   `json:"context,omitempty"`        // 附加上下文
	TimeoutMinutes int      `json:"timeoutMinutes,omitempty"` // 超时时间（默认 8）
	SearchEvidence []string `json:"searchEvidence,omitempty"` // 搜索证据（必须提供或提供 OverrideReason）
	OverrideReason string   `json:"overrideReason,omitempty"` // 跳过搜索的理由
}

// GetTeammateRosterToolInput isdp_get_teammate_roster 工具输入
type GetTeammateRosterToolInput struct {
	// 无需参数，从执行上下文获取 threadId
}

// GetToolDefinitions 获取所有 MCP 工具定义
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name: "isdp_multi_mention",
			Description: `并行邀请 1-3 个 Agent 讨论同一问题。

使用场景：
- 需要多 Agent 视角的复杂决策
- 架构选型、技术方案评估
- 跨领域问题需要多方意见

重要规则：
1. 先搜后问：必须先搜索相关资料，提供 searchEvidenceRefs 参数
2. 防止级联：被召唤的 Agent 不能再次发起 multi_mention
3. 目标限制：最多 3 个目标 Agent

使用示例：
{
  "targets": ["架构师", "安全专家"],
  "question": "用户认证方案选型：JWT vs Session？",
  "callbackTo": "前端开发",
  "searchEvidenceRefs": ["已查看 auth/auth.go 现有实现"]
}`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"targets": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"minItems":    1.0,
						"maxItems":    3.0,
						"description": "目标 Agent ID 列表 (1-3 个)",
					},
					"question": map[string]interface{}{
						"type":        "string",
						"maxLength":   5000.0,
						"description": "问题内容",
					},
					"callbackTo": map[string]interface{}{
						"type":        "string",
						"description": "回调 Agent ID（通常是自己）",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"maxLength":   5000.0,
						"description": "附加上下文（可选）",
					},
					"timeoutMinutes": map[string]interface{}{
						"type":        "integer",
						"minimum":     3.0,
						"maximum":     20.0,
						"default":     8.0,
						"description": "超时时间（分钟，默认 8）",
					},
					"searchEvidence": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "搜索证据引用列表（必须提供或提供 overrideReason）",
					},
					"overrideReason": map[string]interface{}{
						"type":        "string",
						"maxLength":   500.0,
						"description": "跳过搜索的理由（可选，仅在无法搜索时使用）",
					},
				},
				"required": []string{"targets", "question", "callbackTo"},
			},
		},
		{
			Name:        "isdp_get_teammate_roster",
			Description: "获取当前团队中可 @ 的队友列表及其擅长领域。",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

// ToolNames 返回工具名称列表
func ToolNames() []string {
	return []string{
		"isdp_multi_mention",
		"isdp_get_teammate_roster",
	}
}