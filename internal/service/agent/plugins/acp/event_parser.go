// internal/service/agent/plugins/acp/event_parser.go
// ACP event parser
package acp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

func parseACPSessionUpdate(raw json.RawMessage, session *acpSession) ([]agent.Chunk, error) {
	var header acpSessionUpdateHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		LogError("ACP: failed to parse session update header", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse session update header: %w", err)
	}

	switch header.SessionUpdate {
	case "agent_message_chunk":
		return parseACPAgentMessageChunk(raw)
	case "agent_thought_chunk":
		return parseACPAgentThoughtChunk(raw)
	case "tool_call":
		return parseACPToolCall(raw, session)
	case "tool_call_update":
		return parseACPToolCallUpdate(raw, session)
	case "usage_update":
		return parseACPUsageUpdate(raw)
	case "plan":
		return parseACPPlanUpdate(raw)
	default:
		LogDebug("ACP: skip unknown session update",
			zap.String("sessionUpdate", header.SessionUpdate),
		)
		return nil, nil
	}
}

func parseACPAgentMessageChunk(raw json.RawMessage) ([]agent.Chunk, error) {
	var msg acpAgentMessageChunk
	if err := json.Unmarshal(raw, &msg); err != nil {
		LogError("ACP: failed to parse agent_message_chunk", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse agent_message_chunk: %w", err)
	}

	return []agent.Chunk{{
		Type:    agent.ChunkTypeText,
		Content: msg.Content.Text,
	}}, nil
}

func parseACPAgentThoughtChunk(raw json.RawMessage) ([]agent.Chunk, error) {
	var thought acpAgentThoughtChunk
	if err := json.Unmarshal(raw, &thought); err != nil {
		LogError("ACP: failed to parse agent_thought_chunk", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse agent_thought_chunk: %w", err)
	}

	return []agent.Chunk{{
		Type:    agent.ChunkTypeThinking,
		Content: thought.Content.Text,
	}}, nil
}

func parseACPToolCall(raw json.RawMessage, session *acpSession) ([]agent.Chunk, error) {
	var tc acpToolCall
	if err := json.Unmarshal(raw, &tc); err != nil {
		LogError("ACP: failed to parse tool_call", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse tool_call: %w", err)
	}

	// 详细日志：输出完整的工具调用信息
	LogDebug("ACP: received tool_call",
		zap.String("toolCallId", tc.ToolCallID),
		zap.String("title", tc.Title),
		zap.String("kind", tc.Kind),
		zap.String("status", tc.Status),
		zap.Any("rawInput", tc.RawInput))

	var toolInput map[string]interface{}
	if m, ok := tc.RawInput.(map[string]interface{}); ok {
		toolInput = m
	}

	// 检测 AskUserQuestion 工具（更精确的匹配）
	// 工具名可能是 "AskUserQuestion" 或 "ask_user_question" 或类似的
	isAskUserQuestion := false

	// 方法1：检查 title 是否包含关键关键词（大小写不敏感）
	titleLower := strings.ToLower(tc.Title)
	if strings.Contains(titleLower, "askuserquestion") ||
		strings.Contains(titleLower, "ask user") ||
		strings.Contains(titleLower, "user question") {
		isAskUserQuestion = true
	}

	// 方法2：检查 kind 字段（如果存在）
	if tc.Kind == "ask_user" || tc.Kind == "user_input" || tc.Kind == "question" {
		isAskUserQuestion = true
	}

	// 方法3：检查 rawInput 是否包含 questions 数组（特征识别）
	if toolInput != nil {
		if _, hasQuestions := toolInput["questions"]; hasQuestions {
			// 如果输入包含 questions 数组，很可能是 AskUserQuestion 工具
			isAskUserQuestion = true
		}
	}

	if isAskUserQuestion {
		LogInfo("ACP: detected AskUserQuestion tool",
			zap.String("toolCallId", tc.ToolCallID),
			zap.String("title", tc.Title),
			zap.String("kind", tc.Kind),
			zap.Any("input", toolInput))

		// 解析问题列表
		questions := parseQuestionsFromInput(toolInput)

		chunk := agent.Chunk{
			Type:      agent.ChunkTypeQuestion,
			ToolName:  tc.Title,
			ToolID:    tc.ToolCallID,
			ToolInput: toolInput,
			Questions: questions,
		}

		// 存储到 session 以便等待用户响应
		if session != nil {
			session.pendingQuestion = &chunk
		}

		return []agent.Chunk{chunk}, nil
	}

	// 存储工具调用名称到 session（用于后续 tool_call_update 查找）
	if session != nil && tc.ToolCallID != "" && tc.Title != "" {
		session.mu.Lock()
		if session.toolCallNames == nil {
			session.toolCallNames = make(map[string]string)
		}
		session.toolCallNames[tc.ToolCallID] = tc.Title
		session.mu.Unlock()
	}

	return []agent.Chunk{{
		Type:      agent.ChunkTypeToolUse,
		ToolName:  tc.Title,
		ToolID:    tc.ToolCallID,
		ToolInput: toolInput,
	}}, nil
}

func parseACPToolCallUpdate(raw json.RawMessage, session *acpSession) ([]agent.Chunk, error) {
	// 首先打印完整的 raw JSON，便于调试
	LogInfo("ACP: parseACPToolCallUpdate raw JSON",
		zap.String("raw", string(raw)))

	var update acpToolCallUpdate
	if err := json.Unmarshal(raw, &update); err != nil {
		LogError("ACP: failed to parse tool_call_update", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse tool_call_update: %w", err)
	}

	// 打印解析后的字段
	LogInfo("ACP: parseACPToolCallUpdate parsed",
		zap.String("toolCallId", update.ToolCallID),
		zap.String("status", update.Status),
		zap.String("title", update.Title),
		zap.String("kind", update.Kind),
		zap.Any("rawInput", update.RawInput),
		zap.Int("contentBlocksCount", len(update.Content)))

	// 如果 Title 为空，从 session 的工具名称映射中查找
	toolName := update.Title
	if toolName == "" && session != nil && update.ToolCallID != "" {
		session.mu.Lock()
		if session.toolCallNames != nil {
			if name, ok := session.toolCallNames[update.ToolCallID]; ok {
				toolName = name
				LogDebug("ACP: tool_call_update found tool name from session",
					zap.String("toolCallId", update.ToolCallID),
					zap.String("toolName", toolName))
			}
		}
		session.mu.Unlock()
	}

	status := strings.ToLower(update.Status)

	// 空状态 + 空内容：当作工具调用开始处理（发送 tool_use chunk）
	// 只有 status 为 "completed" 或 "failed" 时才当作完成处理
	if status == "" || status == "in_progress" || status == "pending" {
		var toolInput map[string]interface{}
		if m, ok := update.RawInput.(map[string]interface{}); ok {
			toolInput = m
		}

		// 发送 tool_use chunk（即使 input 为空，也通知前端工具调用开始）
		LogInfo("ACP: tool_call_update as tool_use",
			zap.String("toolCallId", update.ToolCallID),
			zap.String("status", update.Status),
			zap.String("title", toolName),
			zap.Any("rawInput", toolInput))

		return []agent.Chunk{{
			Type:      agent.ChunkTypeToolUse,
			ToolName:  toolName,
			ToolID:    update.ToolCallID,
			ToolInput: toolInput,
		}}, nil
	}

	// completed/failed 状态：发送 tool_result
	isError := status == "failed"

	// 解析 content，处理嵌套结构
	// OpenCode 格式: [{"type":"content","content":{"type":"text","text":"..."}}]
	// 标准格式: [{"type":"text","text":"..."}]
	content := extractToolCallContent(update.Content)

	LogInfo("ACP: tool_call_update completed/failed",
		zap.String("toolCallId", update.ToolCallID),
		zap.String("status", update.Status),
		zap.Bool("isError", isError),
		zap.Int("contentLen", len(content)),
		zap.String("contentPreview", content[:min(500, len(content))]))

	return []agent.Chunk{{
		Type:    agent.ChunkTypeToolResult,
		Content: content,
		ToolID:  update.ToolCallID,
		IsError: isError,
	}}, nil
}

// extractToolCallContent 从 content 数组中提取文本内容
// 支持标准格式和 OpenCode 嵌套格式
func extractToolCallContent(blocks []acpContentBlock) string {
	for _, block := range blocks {
		// 标准格式：直接有 Text 字段
		if block.Text != "" {
			return block.Text
		}

		// OpenCode 嵌套格式: {"type":"content","content":{"type":"text","text":"..."}}
		if block.Type == "content" && len(block.Content) > 0 {
			var nested acpContentBlock
			if err := json.Unmarshal(block.Content, &nested); err == nil && nested.Text != "" {
				return nested.Text
			}
			// 尝试直接解析为 map
			var nestedMap map[string]interface{}
			if err := json.Unmarshal(block.Content, &nestedMap); err == nil {
				if text, ok := nestedMap["text"].(string); ok && text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func parseACPUsageUpdate(raw json.RawMessage) ([]agent.Chunk, error) {
	var usage acpUsageUpdate
	if err := json.Unmarshal(raw, &usage); err != nil {
		LogError("ACP: failed to parse usage_update", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse usage_update: %w", err)
	}

	LogInfo("ACP: usage_update parsed",
		zap.Int64("used", usage.Used),
		zap.Int64("size", usage.Size))

	return []agent.Chunk{{
		Type: agent.ChunkTypeUsage,
		Usage: &agent.TokenUsage{
			ContextUsed: usage.Used,
			ContextSize: usage.Size,
		},
	}}, nil
}

func parseACPPlanUpdate(raw json.RawMessage) ([]agent.Chunk, error) {
	var plan acpPlanUpdate
	if err := json.Unmarshal(raw, &plan); err != nil {
		LogError("ACP: failed to parse plan", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse plan: %w", err)
	}

	lines := make([]string, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		line := "[" + strconv.Itoa(entry.Priority) + "] [" + entry.Status + "] " + entry.Content
		lines = append(lines, line)
	}

	return []agent.Chunk{{
		Type:    agent.ChunkTypeStatus,
		Content: strings.Join(lines, "\n"),
	}}, nil
}

// parseQuestionsFromInput 从工具输入中解析问题列表
func parseQuestionsFromInput(input map[string]interface{}) []agent.QuestionItem {
	if input == nil {
		return nil
	}

	questions := make([]agent.QuestionItem, 0)

	// 尝试解析 questions 数组
	if questionsArray, ok := input["questions"].([]interface{}); ok {
		for _, q := range questionsArray {
			if qMap, ok := q.(map[string]interface{}); ok {
				question := agent.QuestionItem{
					Header:      getStringFromMap(qMap, "header"),
					Question:    getStringFromMap(qMap, "question"),
					MultiSelect: getBoolFromMap(qMap, "multiSelect"),
				}

				// 解析选项
				if optionsArray, ok := qMap["options"].([]interface{}); ok {
					for _, o := range optionsArray {
						if oMap, ok := o.(map[string]interface{}); ok {
							question.Options = append(question.Options, agent.QuestionOption{
								Label:       getStringFromMap(oMap, "label"),
								Description: getStringFromMap(oMap, "description"),
								Preview:     getStringFromMap(oMap, "preview"),
							})
						}
					}
				}

				questions = append(questions, question)
			}
		}
	}

	return questions
}

// parseACPUserInputRequest 解析用户输入请求通知
func parseACPUserInputRequest(req acpUserInputRequest) agent.Chunk {
	questions := parseQuestionsFromInput(req.Input)

	return agent.Chunk{
		Type:      agent.ChunkTypeQuestion,
		ToolName:  req.ToolName,
		ToolID:    req.ToolCallID,
		ToolInput: req.Input,
		Questions: questions,
	}
}

// getStringFromMap 安全地从 map 中获取字符串
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getBoolFromMap 安全地从 map 中获取布尔值
func getBoolFromMap(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
