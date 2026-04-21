package agent

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestGetOrCreateHumanChainHistory_FirstMessage 测试第一轮用户消息初始化 A2AContext
func TestGetOrCreateHumanChainHistory_FirstMessage(t *testing.T) {
	es := &ExecutionService{
		a2aContexts: make(map[uuid.UUID]*A2AContext),
	}
	threadID := uuid.New()
	userInput := "帮我设计API"

	chainHistory := es.getOrCreateHumanChainHistory(context.Background(), threadID, userInput)

	// 验证 ChainHistory 不为 nil
	if chainHistory == nil {
		t.Fatal("ChainHistory should not be nil")
	}

	// 验证 PreviousResponses 包含用户消息
	if len(chainHistory.PreviousResponses) != 1 {
		t.Errorf("Expected 1 PreviousResponse, got %d", len(chainHistory.PreviousResponses))
	}

	// 验证第一条是用户消息
	firstResp := chainHistory.PreviousResponses[0]
	if firstResp.AgentName != "User" {
		t.Errorf("Expected AgentName 'User', got '%s'", firstResp.AgentName)
	}
	if firstResp.Role != "user" {
		t.Errorf("Expected Role 'user', got '%s'", firstResp.Role)
	}

	// 验证 OriginalMessage
	if chainHistory.OriginalMessage != userInput {
		t.Errorf("Expected OriginalMessage '%s', got '%s'", userInput, chainHistory.OriginalMessage)
	}

	// 验证 ChainIndex = 1（人类是链路起点）
	if chainHistory.ChainIndex != 1 {
		t.Errorf("Expected ChainIndex 1, got %d", chainHistory.ChainIndex)
	}

	t.Log("First message initializes A2AContext correctly")
}

// TestGetOrCreateHumanChainHistory_SubsequentMessage 测试第二轮用户消息 Append 行为
func TestGetOrCreateHumanChainHistory_SubsequentMessage(t *testing.T) {
	es := &ExecutionService{
		a2aContexts: make(map[uuid.UUID]*A2AContext),
	}
	threadID := uuid.New()

	// 第一轮：初始化 A2AContext
	es.getOrCreateHumanChainHistory(context.Background(), threadID, "帮我设计API")

	// 模拟 Agent 响应完成：添加 Agent 输出到 PreviousResponses
	es.a2aMu.Lock()
	a2aCtx := es.a2aContexts[threadID]
	a2aCtx.PreviousResponses = append(a2aCtx.PreviousResponses, ChainResponse{
		AgentID:   uuid.New(),
		AgentName: "需求分析师",
		Content:   "API设计完成...",
		Role:      "requirement_analyst",
		Timestamp: time.Now().Unix(),
	})
	a2aCtx.InvokedAgents[uuid.New()] = true // 模拟已调用的 Agent
	a2aCtx.Depth = 1                        // 模拟 A2A 深度
	es.a2aMu.Unlock()

	// 第二轮：用户后续消息
	secondInput := "继续完善文档"
	chainHistory := es.getOrCreateHumanChainHistory(context.Background(), threadID, secondInput)

	// 验证 PreviousResponses 包含完整历史（User + Agent + User）
	// 应该有 3 条：初始 User + Agent + 新 User
	if len(chainHistory.PreviousResponses) < 3 {
		t.Errorf("Expected at least 3 PreviousResponses, got %d", len(chainHistory.PreviousResponses))
	}

	// 验证最后一条是新用户消息
	lastResp := chainHistory.PreviousResponses[len(chainHistory.PreviousResponses)-1]
	if lastResp.AgentName != "User" {
		t.Errorf("Expected last response AgentName 'User', got '%s'", lastResp.AgentName)
	}

	// 验证 OriginalMessage 被覆盖（新意图）
	if chainHistory.OriginalMessage != secondInput {
		t.Errorf("Expected OriginalMessage '%s', got '%s'", secondInput, chainHistory.OriginalMessage)
	}

	// 验证 ChainIndex 增加（第二轮用户消息）
	// ChainIndex = 1 (初始) + 1 (第二轮用户消息) = 2
	if chainHistory.ChainIndex != 2 {
		t.Errorf("Expected ChainIndex 2, got %d", chainHistory.ChainIndex)
	}

	// 验证 Depth 被重置为 0
	if a2aCtx.Depth != 0 {
		t.Errorf("Expected Depth 0 after reset, got %d", a2aCtx.Depth)
	}

	// 验证 InvokedAgents 被重置
	if len(a2aCtx.InvokedAgents) != 0 {
		t.Errorf("Expected InvokedAgents to be reset, got %d entries", len(a2aCtx.InvokedAgents))
	}

	t.Log("Subsequent message correctly appends and resets state")
}

// TestGetOrCreateHumanChainHistory_SafeguardLimit 测试 PreviousResponses 长度限制
func TestGetOrCreateHumanChainHistory_SafeguardLimit(t *testing.T) {
	es := &ExecutionService{
		a2aContexts: make(map[uuid.UUID]*A2AContext),
	}
	threadID := uuid.New()

	// 初始化
	es.getOrCreateHumanChainHistory(context.Background(), threadID, "初始消息")

	// 添加超过 MaxPreviousResponses 的条目
	es.a2aMu.Lock()
	a2aCtx := es.a2aContexts[threadID]
	// 添加 25 条 Agent 响应（超过限制 20）
	for i := 0; i < 25; i++ {
		a2aCtx.PreviousResponses = append(a2aCtx.PreviousResponses, ChainResponse{
			AgentID:   uuid.New(),
			AgentName: "Agent",
			Content:   "响应内容",
			Role:      "agent",
			Timestamp: time.Now().Unix(),
		})
	}
	es.a2aMu.Unlock()

	// 触发新一轮用户消息，应该触发 safeguard
	es.getOrCreateHumanChainHistory(context.Background(), threadID, "新消息")

	es.a2aMu.Lock()
	finalLen := len(es.a2aContexts[threadID].PreviousResponses)
	es.a2aMu.Unlock()

	// 验证 PreviousResponses 长度不超过 MaxPreviousResponses
	if finalLen > MaxPreviousResponses {
		t.Errorf("Expected PreviousResponses <= %d, got %d", MaxPreviousResponses, finalLen)
	}

	t.Logf("Safeguard correctly limits PreviousResponses to %d", finalLen)
}

// TestGetOrCreateHumanChainHistory_TokenBudgetProtection 测试 Token 预算保护（截断）
func TestGetOrCreateHumanChainHistory_TokenBudgetProtection(t *testing.T) {
	es := &ExecutionService{
		a2aContexts: make(map[uuid.UUID]*A2AContext),
	}
	threadID := uuid.New()

	// 创建超长用户输入（超过 500 字符）
	longInput := ""
	for i := 0; i < 1000; i++ {
		longInput += "x"
	}

	chainHistory := es.getOrCreateHumanChainHistory(context.Background(), threadID, longInput)

	// 验证 PreviousResponses 中的内容被截断
	if len(chainHistory.PreviousResponses) < 1 {
		t.Fatal("Expected at least 1 PreviousResponse")
	}

	userContent := chainHistory.PreviousResponses[0].Content
	// 内容应该被截断（不超过 500 字符，但 TruncateHeadTail 可能增加一些）
	if len(userContent) > 600 {
		t.Errorf("Expected truncated content (<=600 chars), got %d", len(userContent))
	}

	// OriginalMessage 应保留完整输入
	if chainHistory.OriginalMessage != longInput {
		t.Errorf("OriginalMessage should preserve full input")
	}

	t.Logf("Token budget protection: content truncated to %d chars, OriginalMessage preserved", len(userContent))
}