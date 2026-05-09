package claude_code

import (
	"encoding/json"
	"strings"

	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

// parseToolResultContent 解析 tool_result 的 content 字段
// content 可能是两种格式：
// 1. 字符串格式: "content": "result text"
// 2. 数组格式: "content": [{"type": "text", "text": "result text"}, ...]
func parseToolResultContent(contentRaw json.RawMessage) string {
	if len(contentRaw) == 0 {
		return ""
	}

	// 尝试解析为字符串
	var strContent string
	if err := json.Unmarshal(contentRaw, &strContent); err == nil {
		return strContent
	}

	// 尝试解析为 content_block 数组
	var blockContents []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(contentRaw, &blockContents); err == nil {
		var result strings.Builder
		for i, block := range blockContents {
			if block.Type == "text" && block.Text != "" {
				if i > 0 {
					result.WriteString("\n")
				}
				result.WriteString(block.Text)
			}
		}
		return result.String()
	}

	// 如果两种格式都无法解析，返回原始 JSON 字符串（作为 fallback）
	return string(contentRaw)
}

// parseStreamJSONLine 解析 stream-json 格式的单行输出，返回 Chunk 数组
// isStreaming: 是否为增量模式，增量模式下忽略完整消息避免重复
func parseStreamJSONLine(line string, isStreaming bool) []agent.Chunk {
	var chunks []agent.Chunk

	var msg struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Event   struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			ContentBlock struct {
				Type  string                 `json:"type"`
				Name  string                 `json:"name"`
				ID    string                 `json:"id"`
				Input map[string]interface{} `json:"input"`
			} `json:"content_block"`
		} `json:"event"`
		Message struct {
			Content []struct {
				Type       string                 `json:"type"`
				Text       string                 `json:"text"`
				Name       string                 `json:"name"`
				ID         string                 `json:"id"`
				Input      map[string]interface{} `json:"input"`
				ToolUseID  string                 `json:"tool_use_id"` // tool_result 的关联ID
				ContentRaw json.RawMessage        `json:"content"`     // tool_result 的内容（可能是 string 或 []content_block）
				IsError    bool                   `json:"is_error"`    // tool_result 是否出错
			} `json:"content"`
			Usage *struct {
				InputTokens              int64 `json:"input_tokens"`
				OutputTokens             int64 `json:"output_tokens"`
				CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			} `json:"usage"`
		} `json:"message"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Delta struct {
			Usage struct {
				OutputTokens int64 `json:"output_tokens"`
			} `json:"usage"`
		} `json:"delta"`
		Result        string  `json:"result"`
		CostUsd       float64 `json:"cost_usd"`
		DurationMs    int64   `json:"duration_ms"`
		DurationApiMs int64   `json:"duration_api_ms"`
		NumTurns      int     `json:"num_turns"`
	}

	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		logInfo("parseStreamJSONLine: JSON parse error", zap.Error(err), zap.String("line", line[:minLen(100, len(line))]))
		return nil
	}

	switch msg.Type {
	case "stream_event":
		switch msg.Event.Type {
		case "content_block_start":
			// 内容块开始
			switch msg.Event.ContentBlock.Type {
			case "thinking":
				// 思考块开始，发送空内容让前端初始化
				chunks = append(chunks, agent.Chunk{
					Type:    agent.ChunkTypeThinking,
					Content: "",
				})
			case "tool_use":
				// 特殊处理 AskUserQuestion 工具
				if msg.Event.ContentBlock.Name == "AskUserQuestion" {
					// 提取 questions 字段
					var questions []agent.QuestionItem
					if questionsRaw, ok := msg.Event.ContentBlock.Input["questions"]; ok {
						// 将 questionsRaw 转换为 JSON 再解析为 QuestionItem 数组
						questionsJSON, err := json.Marshal(questionsRaw)
						if err == nil {
							json.Unmarshal(questionsJSON, &questions)
						}
						logInfo("parseStreamJSONLine: AskUserQuestion parsed from stream_event", zap.Int("questionsCount", len(questions)), zap.String("toolId", msg.Event.ContentBlock.ID))
					} else {
						logInfo("parseStreamJSONLine: AskUserQuestion has no questions field in Input", zap.String("toolId", msg.Event.ContentBlock.ID))
					}
					chunks = append(chunks, agent.Chunk{
						Type:      agent.ChunkTypeQuestion,
						ToolName:  msg.Event.ContentBlock.Name,
						ToolID:    msg.Event.ContentBlock.ID,
						ToolIndex: msg.Event.Index,
						ToolInput: msg.Event.ContentBlock.Input,
						Questions: questions,
					})
				} else {
					chunks = append(chunks, agent.Chunk{
						Type:      agent.ChunkTypeToolUse,
						ToolName:  msg.Event.ContentBlock.Name,
						ToolID:    msg.Event.ContentBlock.ID,
						ToolIndex: msg.Event.Index,
						ToolInput: msg.Event.ContentBlock.Input,
					})
				}
			}
		case "content_block_delta":
			switch msg.Event.Delta.Type {
			case "text_delta":
				if msg.Event.Delta.Text != "" {
					chunks = append(chunks, agent.Chunk{
						Type:    agent.ChunkTypeText,
						Content: msg.Event.Delta.Text,
					})
				}
			case "thinking_delta":
				// 思考过程增量 - 发送实际的思考内容
				if msg.Event.Delta.Thinking != "" {
					chunks = append(chunks, agent.Chunk{
						Type:    agent.ChunkTypeThinking,
						Content: msg.Event.Delta.Thinking,
					})
				}
			case "input_json_delta":
				// 工具参数增量更新 - 发送累积的 JSON 片段
				if msg.Event.Delta.PartialJSON != "" {
					chunks = append(chunks, agent.Chunk{
						Type:        agent.ChunkTypeInputJSONDelta,
						ToolIndex:   msg.Event.Index,
						PartialJSON: msg.Event.Delta.PartialJSON,
					})
				}
			}
		case "content_block_stop":
			// 内容块结束 - 用于标记 thinking 完成
			// 发送一个带 Done 标记的空 thinking 块
			chunks = append(chunks, agent.Chunk{
				Type:    agent.ChunkTypeThinking,
				Content: "",
				Done:    true,
			})
		}
	case "message_start":
		// 解析 message.usage 字段（input tokens）
		if msg.Message.Usage != nil {
			chunks = append(chunks, agent.Chunk{
				Type: agent.ChunkTypeUsage,
				Usage: &agent.TokenUsage{
					InputTokens:         msg.Message.Usage.InputTokens,
					CacheReadTokens:     msg.Message.Usage.CacheReadInputTokens,
					CacheCreationTokens: msg.Message.Usage.CacheCreationInputTokens,
				},
			})
		}
	case "message_delta":
		// 解析 usage 字段（output tokens 通常在这里）
		if msg.Delta.Usage.OutputTokens > 0 {
			chunks = append(chunks, agent.Chunk{
				Type: agent.ChunkTypeUsage,
				Usage: &agent.TokenUsage{
					OutputTokens: msg.Delta.Usage.OutputTokens,
				},
			})
		}
	case "assistant":
		// 完整消息（非增量模式下的输出）
		// AskUserQuestion 特殊处理：即使在增量模式下也需要解析，因为 questions 数据只在 assistant 消息中出现
		// stream_event.content_block_start 时 input 为空，只有 assistant 消息才有完整的 questions
		// 其他 tool_use 的 input 已通过 input_json_delta 累积，无需在此处理
		for _, content := range msg.Message.Content {
			if content.Type == "tool_use" {
				if content.Name == "AskUserQuestion" {
					// 特殊处理 AskUserQuestion 工具 - 增量模式下也需要
					var questions []agent.QuestionItem
					if questionsRaw, ok := content.Input["questions"]; ok {
						questionsJSON, err := json.Marshal(questionsRaw)
						if err == nil {
							json.Unmarshal(questionsJSON, &questions)
						}
						logInfo("parseStreamJSONLine: AskUserQuestion parsed from assistant message", zap.Int("questionsCount", len(questions)), zap.String("toolId", content.ID))
					}
					chunks = append(chunks, agent.Chunk{
						Type:      agent.ChunkTypeQuestion,
						ToolName:  content.Name,
						ToolID:    content.ID,
						ToolInput: content.Input,
						Questions: questions,
					})
				}
				// 增量模式下，其他 tool_use 不发送 ChunkTypeToolUse
				// 因为 input 已通过 input_json_delta 累积，前端已收到完整参数
			} else if !isStreaming {
				// 非增量模式下处理其他内容（无 streaming event）
				if content.Type == "text" && content.Text != "" {
					chunks = append(chunks, agent.Chunk{
						Type:    agent.ChunkTypeText,
						Content: content.Text,
					})
				} else if content.Type == "tool_use" {
					chunks = append(chunks, agent.Chunk{
						Type:      agent.ChunkTypeToolUse,
						ToolName:  content.Name,
						ToolID:    content.ID,
						ToolInput: content.Input,
					})
				}
			}
		}
	case "user":
		// 用户消息（包含工具执行结果）
		// 解析 tool_result 内容块
		for _, content := range msg.Message.Content {
			if content.Type == "tool_result" {
				// 提取结果内容（适配字符串和数组两种格式）
				resultContent := parseToolResultContent(content.ContentRaw)
				if resultContent == "" && content.Text != "" {
					resultContent = content.Text
				}

				chunks = append(chunks, agent.Chunk{
					Type:    agent.ChunkTypeToolResult,
					ToolID:  content.ToolUseID,
					Content: resultContent,
					IsError: content.IsError,
				})
			}
		}
	case "result":
		// 最终结果（非增量模式下使用）
		// 在增量模式下忽略，避免重复
		if !isStreaming && msg.Result != "" {
			chunks = append(chunks, agent.Chunk{
				Type:    agent.ChunkTypeText,
				Content: msg.Result,
			})
		}
		// 解析完整 usage: input_tokens, output_tokens, cache_read_input_tokens
		// 解析 total_cost_usd, duration_ms, duration_api_ms, num_turns
		if msg.Usage.InputTokens > 0 || msg.Usage.OutputTokens > 0 || msg.CostUsd > 0 {
			chunks = append(chunks, agent.Chunk{
				Type: agent.ChunkTypeUsage,
				Usage: &agent.TokenUsage{
					InputTokens:         msg.Usage.InputTokens,
					OutputTokens:        msg.Usage.OutputTokens,
					CacheReadTokens:     msg.Usage.CacheReadInputTokens,
					CacheCreationTokens: msg.Usage.CacheCreationInputTokens,
					CostUsd:            msg.CostUsd,
					DurationMs:         msg.DurationMs,
					DurationApiMs:      msg.DurationApiMs,
					NumTurns:           msg.NumTurns,
				},
			})
		}
	}

	return chunks
}