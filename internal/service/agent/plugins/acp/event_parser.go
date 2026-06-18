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
	// claude-agent-acp 在触发 elicitation/create 前会先发一条 sessionUpdate=tool_call
	// 初始通知，title 是 "Asking for your input"、rawInput 还是空（questions 还没填进去）。
	// 紧接着才会发 tool_call_update + elicitation/create 反向请求。
	// 如果按普通工具走 ChunkTypeToolUse 路径，前端会先创建 "Asking for your input" 这个
	// tool block，等 elicitation 进来再加一个 question block——双重展示，且第一个 tool
	// block 永远不会变成 completed（后续 tool_call_update 已被我们 skip），就一直停在
	// streaming 状态显示"进行中"。
	// 检测 _meta.claudeCode.toolName == "AskUserQuestion" 直接 skip，让 elicitation 独占 UI。
	var metaCheck struct {
		Meta struct {
			ClaudeCode struct {
				ToolName string `json:"toolName"`
			} `json:"claudeCode"`
		} `json:"_meta"`
	}
	_ = json.Unmarshal(raw, &metaCheck)
	if metaCheck.Meta.ClaudeCode.ToolName == "AskUserQuestion" {
		LogInfo("ACP: skip AskUserQuestion tool_call (UI handled by elicitation/create)")
		return nil, nil
	}

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

	// 检测 question / AskUserQuestion 一类工具，统一走 ChunkTypeQuestion 路径渲染选项卡片，
	// 而不是普通 ChunkTypeToolUse 的 INPUT/OUTPUT 块。
	if detectQuestionTool(tc.Title, tc.Kind, toolInput) {
		// 早期 tool_call 初始通知里 rawInput 经常还是空（questions 数组还没填）。
		// 这种情况下抑制本通知——后续 tool_call_update 会带完整 rawInput.questions，
		// 由 parseACPToolCallUpdate 那边的 detectQuestionTool 路径产生 ChunkTypeQuestion。
		// 如果这里就发一个空 questions 的卡片，会同时出现"执行: question"普通块 +
		// "AskUserQuestion"选项卡片两个块（截图里圈出来的红框就是这种）。
		if toolInput == nil {
			LogInfo("ACP: suppress question tool's empty initial tool_call (waiting for tool_call_update)",
				zap.String("toolCallId", tc.ToolCallID),
				zap.String("title", tc.Title))
			return nil, nil
		}
		if _, hasQuestions := toolInput["questions"]; !hasQuestions {
			LogInfo("ACP: suppress question tool's tool_call without questions (waiting for tool_call_update)",
				zap.String("toolCallId", tc.ToolCallID),
				zap.String("title", tc.Title))
			return nil, nil
		}

		LogInfo("ACP: detected AskUserQuestion tool",
			zap.String("toolCallId", tc.ToolCallID),
			zap.String("title", tc.Title),
			zap.String("kind", tc.Kind),
			zap.Any("input", toolInput))

		// 解析问题列表
		questions := parseQuestionsFromInput(toolInput)

		chunk := agent.Chunk{
			Type:      agent.ChunkTypeQuestion,
			ToolName:  "AskUserQuestion",
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

// detectQuestionTool 判断一次工具调用是否是"询问用户"类工具（AskUserQuestion / question 等）。
// 用于 parseACPToolCall 与 parseACPToolCallUpdate 共用：识别后转成 ChunkTypeQuestion 推给前端
// 渲染选项卡片，而不是当成普通工具走 INPUT/OUTPUT 块。
//
// 命中条件（任一）：
//   - title 含 askuserquestion / ask user / user question
//   - title 直接等于 "question"（OpenCode question 工具）
//   - kind 是 ask_user / user_input / question
//   - rawInput 含 questions 数组（最强信号——AskUserQuestion / OpenCode question 都用这个 schema）
func detectQuestionTool(title, kind string, rawInput map[string]interface{}) bool {
	titleLower := strings.ToLower(title)
	if strings.Contains(titleLower, "askuserquestion") ||
		strings.Contains(titleLower, "ask user") ||
		strings.Contains(titleLower, "user question") ||
		titleLower == "question" {
		return true
	}
	if kind == "ask_user" || kind == "user_input" || kind == "question" {
		return true
	}
	if rawInput != nil {
		if _, hasQuestions := rawInput["questions"]; hasQuestions {
			return true
		}
	}
	return false
}

func parseACPToolCallUpdate(raw json.RawMessage, session *acpSession) ([]agent.Chunk, error) {
	// 首先打印完整的 raw JSON，便于调试
	LogInfo("ACP: parseACPToolCallUpdate raw JSON",
		zap.String("raw", string(raw)))

	// claude-agent-acp 在触发 elicitation/create 反向请求的同时也会通过 tool_call_update
	// 通知 AskUserQuestion 工具的状态（in_progress / completed / 含 toolResponse 的中间态）。
	// 我们已经在 handleServerRequest 的 elicitation/create 分支把 question chunk 推给前端了，
	// 这里再把这个工具的 tool_call_update 转成 ToolUse / ToolResult 会形成"同一调用同时
	// 显示 question 卡片 + INPUT/OUTPUT 块"的双重展示。提交答案后 OUTPUT 还会覆盖掉
	// question 卡片的视觉重点。直接把 AskUserQuestion 的 tool_call_update 静默吞掉，
	// 让 elicitation 路径独占 UI。
	var metaCheck struct {
		Meta struct {
			ClaudeCode struct {
				ToolName string `json:"toolName"`
			} `json:"claudeCode"`
		} `json:"_meta"`
	}
	_ = json.Unmarshal(raw, &metaCheck)
	LogInfo("ACP: tool_call_update meta probe",
		zap.String("metaToolName", metaCheck.Meta.ClaudeCode.ToolName))
	if metaCheck.Meta.ClaudeCode.ToolName == "AskUserQuestion" {
		LogInfo("ACP: skip AskUserQuestion tool_call_update (UI handled by elicitation/create)")
		return nil, nil
	}

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

		// OpenCode 的 question 工具走 tool_call_update 通知（不发 tool_call 初始通知，
		// 也不走 elicitation/create 反向请求），rawInput.questions 是数组形态。这里识别后
		// 转 ChunkTypeQuestion，让前端渲染选项卡片而不是普通 INPUT/OUTPUT 块。
		// 仅在非 completed/failed 状态识别——completed 状态下 rawInput 还是同一份带
		// questions 数组的 input，但这时是工具结束通知，不应再创建 question 卡片。
		if detectQuestionTool(toolName, update.Kind, toolInput) {
			LogInfo("ACP: detected question tool via tool_call_update",
				zap.String("toolCallId", update.ToolCallID),
				zap.String("status", update.Status),
				zap.String("title", toolName),
				zap.String("kind", update.Kind))

			questions := parseQuestionsFromInput(toolInput)

			chunk := agent.Chunk{
				Type:      agent.ChunkTypeQuestion,
				ToolName:  "AskUserQuestion", // 统一标题，让前端 question 卡片识别一致
				ToolID:    update.ToolCallID,
				ToolInput: toolInput,
				Questions: questions,
			}
			if session != nil {
				session.mu.Lock()
				session.pendingQuestion = &chunk
				session.mu.Unlock()
			}
			return []agent.Chunk{chunk}, nil
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
				// 多选字段：Claude AskUserQuestion 用 "multiSelect"，OpenCode question
				// 工具用 "multiple"（packages/opencode/src/question/index.ts:Prompt.multiple）。
				// 两者择一命中即可。
				multi := getBoolFromMap(qMap, "multiSelect") || getBoolFromMap(qMap, "multiple")
				question := agent.QuestionItem{
					Header:      getStringFromMap(qMap, "header"),
					Question:    getStringFromMap(qMap, "question"),
					MultiSelect: multi,
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

				// 追加"自定义答案"占位选项：前端识别 label 含"其他"后会自动渲染输入框。
				question.Options = append(question.Options, agent.QuestionOption{
					Label:       elicitationCustomOptionLabel,
					Description: "上面选项都不合适？请填写你自己的答案。",
				})

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
