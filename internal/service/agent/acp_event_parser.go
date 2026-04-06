package agent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

func parseACPSessionUpdate(raw json.RawMessage) ([]Chunk, error) {
	var header acpSessionUpdateHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		logError("ACP: failed to parse session update header", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse session update header: %w", err)
	}

	switch header.SessionUpdate {
	case "agent_message_chunk":
		return parseACPAgentMessageChunk(raw)
	case "agent_thought_chunk":
		return parseACPAgentThoughtChunk(raw)
	case "tool_call":
		return parseACPToolCall(raw)
	case "tool_call_update":
		return parseACPToolCallUpdate(raw)
	case "usage_update":
		return parseACPUsageUpdate(raw)
	case "plan":
		return parseACPPlanUpdate(raw)
	default:
		logDebug("ACP: skip unknown session update",
			zap.String("sessionUpdate", header.SessionUpdate),
		)
		return nil, nil
	}
}

func parseACPAgentMessageChunk(raw json.RawMessage) ([]Chunk, error) {
	var msg acpAgentMessageChunk
	if err := json.Unmarshal(raw, &msg); err != nil {
		logError("ACP: failed to parse agent_message_chunk", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse agent_message_chunk: %w", err)
	}

	return []Chunk{{
		Type:    ChunkTypeText,
		Content: msg.Content.Text,
	}}, nil
}

func parseACPAgentThoughtChunk(raw json.RawMessage) ([]Chunk, error) {
	var thought acpAgentThoughtChunk
	if err := json.Unmarshal(raw, &thought); err != nil {
		logError("ACP: failed to parse agent_thought_chunk", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse agent_thought_chunk: %w", err)
	}

	return []Chunk{{
		Type:    ChunkTypeThinking,
		Content: thought.Content.Text,
	}}, nil
}

func parseACPToolCall(raw json.RawMessage) ([]Chunk, error) {
	var tc acpToolCall
	if err := json.Unmarshal(raw, &tc); err != nil {
		logError("ACP: failed to parse tool_call", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse tool_call: %w", err)
	}

	var toolInput map[string]interface{}
	if m, ok := tc.RawInput.(map[string]interface{}); ok {
		toolInput = m
	}

	return []Chunk{{
		Type:      ChunkTypeToolUse,
		ToolName:  tc.Title,
		ToolID:    tc.ToolCallID,
		ToolInput: toolInput,
	}}, nil
}

func parseACPToolCallUpdate(raw json.RawMessage) ([]Chunk, error) {
	var update acpToolCallUpdate
	if err := json.Unmarshal(raw, &update); err != nil {
		logError("ACP: failed to parse tool_call_update", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse tool_call_update: %w", err)
	}

	status := strings.ToLower(update.Status)
	isError := false
	switch status {
	case "completed":
		isError = false
	case "failed":
		isError = true
	default:
		logDebug("ACP: skip tool_call_update with unsupported status",
			zap.String("status", update.Status),
			zap.String("toolCallId", update.ToolCallID),
		)
		return nil, nil
	}

	content := ""
	if len(update.Content) > 0 {
		content = update.Content[0].Text
	}

	return []Chunk{{
		Type:    ChunkTypeToolResult,
		Content: content,
		ToolID:  update.ToolCallID,
		IsError: isError,
	}}, nil
}

func parseACPUsageUpdate(raw json.RawMessage) ([]Chunk, error) {
	var usage acpUsageUpdate
	if err := json.Unmarshal(raw, &usage); err != nil {
		logError("ACP: failed to parse usage_update", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse usage_update: %w", err)
	}

	return []Chunk{{
		Type: ChunkTypeUsage,
		Usage: &TokenUsage{
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			CostUsd:      usage.Cost,
		},
	}}, nil
}

func parseACPPlanUpdate(raw json.RawMessage) ([]Chunk, error) {
	var plan acpPlanUpdate
	if err := json.Unmarshal(raw, &plan); err != nil {
		logError("ACP: failed to parse plan", zap.Error(err))
		return nil, fmt.Errorf("ACP: parse plan: %w", err)
	}

	lines := make([]string, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		line := "[" + strconv.Itoa(entry.Priority) + "] [" + entry.Status + "] " + entry.Content
		lines = append(lines, line)
	}

	return []Chunk{{
		Type:    ChunkTypeStatus,
		Content: strings.Join(lines, "\n"),
	}}, nil
}
