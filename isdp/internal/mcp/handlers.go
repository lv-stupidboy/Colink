package mcp

import (
	"context"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/google/uuid"
)

// ToolResult MCP 工具执行结果
type ToolResult struct {
	IsError bool   `json:"isError"`
	Content string `json:"content"`
}

// ToolHandlerContext 工具处理上下文
type ToolHandlerContext struct {
	ThreadID       uuid.UUID
	InvocationID   uuid.UUID
	CallbackToken  string
	CallerAgentID  string
	AvailableAgents []string
}

// MultiMentionHandler 处理 isdp_multi_mention 工具调用
func MultiMentionHandler(ctx context.Context, input MultiMentionToolInput, handlerCtx ToolHandlerContext, orchestrator *a2a.MultiMentionOrchestrator) *ToolResult {
	// 1. 参数校验：先搜后问原则
	if len(input.SearchEvidence) == 0 && input.OverrideReason == "" {
		return &ToolResult{
			IsError: true,
			Content: "multi_mention requires searchEvidence (what did you search first?) or overrideReason (why are you skipping search?). This enforces the '先搜后问' principle — search before asking.",
		}
	}

	// 2. 参数校验：targets 数量
	if len(input.Targets) == 0 || len(input.Targets) > 3 {
		return &ToolResult{
			IsError: true,
			Content: "targets must contain 1-3 agent IDs",
		}
	}

	// 3. 级联防护检查
	if orchestrator.IsActiveTarget(handlerCtx.ThreadID, handlerCtx.CallerAgentID) {
		return &ToolResult{
			IsError: true,
			Content: "Anti-cascade: caller is an active multi-mention target. Cannot create multi-mention while responding to one.",
		}
	}

	// 4. 创建请求
	params := a2a.CreateParams{
		ThreadID:       handlerCtx.ThreadID,
		Initiator:      handlerCtx.CallerAgentID,
		CallbackTo:     input.CallbackTo,
		Targets:        input.Targets,
		Question:       input.Question,
		Context:        input.Context,
		TimeoutMinutes: input.TimeoutMinutes,
		SearchEvidence: input.SearchEvidence,
		OverrideReason: input.OverrideReason,
	}

	result, err := orchestrator.Create(ctx, params, handlerCtx.AvailableAgents)
	if err != nil {
		return &ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Failed to create multi-mention request: %v", err),
		}
	}

	// 5. 启动请求
	if err := orchestrator.Start(ctx, result.RequestID); err != nil {
		return &ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Failed to start multi-mention request: %v", err),
		}
	}

	return &ToolResult{
		IsError: false,
		Content: fmt.Sprintf(`Multi-mention request created successfully.

Request ID: %s
Status: %s
Callback Token: %s

The target agents will be notified and their responses will be aggregated.
You will receive a callback when all responses are collected or timeout occurs.`,
			result.RequestID.String(),
			result.Status,
			result.CallbackToken,
		),
	}
}

// GetTeammateRosterHandler 处理 isdp_get_teammate_roster 工具调用
func GetTeammateRosterHandler(ctx context.Context, handlerCtx ToolHandlerContext, rosterBuilder interface {
	BuildByAvailableAgents(ctx context.Context, agentIDs []string, excludeAgentID string) ([]TeammateInfo, error)
}) *ToolResult {
	if len(handlerCtx.AvailableAgents) == 0 {
		return &ToolResult{
			IsError: false,
			Content: "No teammates available in the current thread.",
		}
	}

	teammates, err := rosterBuilder.BuildByAvailableAgents(ctx, handlerCtx.AvailableAgents, handlerCtx.CallerAgentID)
	if err != nil {
		return &ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Failed to get teammate roster: %v", err),
		}
	}

	// 构建输出
	output := "## Teammate Roster\n\n"
	output += "| Name | Role | Skills |\n"
	output += "|------|------|--------|\n"

	for _, t := range teammates {
		skills := "-"
		if len(t.Skills) > 0 {
			skills = ""
			for i, s := range t.Skills {
				if i > 0 {
					skills += ", "
				}
				skills += s
			}
		}
		output += fmt.Sprintf("| %s | %s | %s |\n", t.Name, t.Role, skills)
	}

	return &ToolResult{
		IsError: false,
		Content: output,
	}
}

// TeammateInfo 队友信息（复制自 agent 包以避免循环依赖）
type TeammateInfo struct {
	ID     string
	Name   string
	Role   string
	Skills []string
}

// ValidateMultiMentionInput 校验 multi_mention 输入
func ValidateMultiMentionInput(input MultiMentionToolInput) error {
	if len(input.Targets) == 0 {
		return fmt.Errorf("targets is required")
	}
	if len(input.Targets) > 3 {
		return fmt.Errorf("targets cannot exceed 3")
	}
	if input.Question == "" {
		return fmt.Errorf("question is required")
	}
	if input.CallbackTo == "" {
		return fmt.Errorf("callbackTo is required")
	}
	return nil
}

// ConvertMultiMentionResponse 转换多讨论结果为可读格式
func ConvertMultiMentionResponse(result *model.AggregatedMultiMentionResult) string {
	output := "## Multi-Mention Results\n\n"
	output += fmt.Sprintf("**Status**: %s\n", result.Status)
	output += fmt.Sprintf("**Timeout**: %v\n\n", result.Timeout)

	if len(result.Responses) == 0 {
		output += "No responses received.\n"
		return output
	}

	for _, resp := range result.Responses {
		output += fmt.Sprintf("### %s\n", resp.AgentID)
		output += resp.Content + "\n\n"
	}

	return output
}