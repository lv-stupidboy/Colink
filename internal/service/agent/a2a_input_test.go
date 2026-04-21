package agent

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestBuildA2AInputWithOptionsFormat 测试原始消息 + 前序摘要格式
func TestBuildA2AInputWithOptionsFormat(t *testing.T) {
	// 创建 mock ExecutionService
	es := &ExecutionService{}

	fromAgent := &AgentInfo{
		ID:   uuid.New(),
		Name: "architect",
		Role: "planner",
	}

	a2aCtx := &A2AContext{
		Depth:         2,
		FromAgent:     fromAgent,
		SessionStrategy: SessionStrategyResume,
	}

	output := "这是上游 Agent 的输出，包含分析结果..."

	opts := &A2AInputOptions{
		IncludeTokenBudget: false,
		MaxSummaryLength:   500,
	}

	input := es.BuildA2AInputWithOptions(a2aCtx, output, opts)

	// 验证包含协作规则
	if !strings.Contains(input, "协作规则") {
		t.Error("A2A input should contain collaboration rules")
	}

	// 验证包含会话策略
	if !strings.Contains(input, "会话策略") {
		t.Error("A2A input should contain session strategy")
	}

	// 验证包含 Resume 说明
	if !strings.Contains(input, "Resume") {
		t.Error("A2A input should indicate Resume strategy")
	}

	// 验证包含前序摘要
	if !strings.Contains(input, "前序分析") {
		t.Error("A2A input should contain predecessor summary")
	}

	// 验证包含触发者信息
	if !strings.Contains(input, fromAgent.Name) {
		t.Error("A2A input should mention the from agent name")
	}
}

// TestBuildA2AInputWithOptionsNewSession 测试新会话策略
func TestBuildA2AInputWithOptionsNewSession(t *testing.T) {
	es := &ExecutionService{}

	a2aCtx := &A2AContext{
		Depth:           1,
		SessionStrategy: SessionStrategyNew,
	}

	output := "分析结果"

	opts := &A2AInputOptions{
		IncludeTokenBudget: false,
	}

	input := es.BuildA2AInputWithOptions(a2aCtx, output, opts)

	// 验证包含 New 说明
	if !strings.Contains(input, "New") {
		t.Error("A2A input should indicate New strategy")
	}

	if !strings.Contains(input, "全新会话") {
		t.Error("A2A input should explain New session")
	}
}

// TestBuildA2AInputWithOptionsTokenBudget 测试 Token 预算信息注入
func TestBuildA2AInputWithOptionsTokenBudget(t *testing.T) {
	es := &ExecutionService{
		tokenBudgetManager: NewTokenBudgetManager(),
	}

	a2aCtx := &A2AContext{
		Depth: 2,
	}

	// 设置 Usage 缓存
	invocationID := uuid.New()
	usage := &TokenUsage{
		InputTokens:  50_000,
		OutputTokens: 20_000,
	}
	es.tokenBudgetManager.UpdateUsageFromCLI(invocationID, usage)

	output := "结果"

	opts := &A2AInputOptions{
		IncludeTokenBudget: true,
	}

	input := es.BuildA2AInputWithOptions(a2aCtx, output, opts)

	// 当有 token budget 信息时应该包含
	// 注意：如果没有设置 model，可能无法计算剩余预算
	// 这里主要验证格式正确性
	if !strings.Contains(input, "Token") {
		t.Error("A2A input should mention Token when IncludeTokenBudget is true")
	}
}

// TestBuildA2AInputWithOptionsSummaryTruncate 测试摘要截断
func TestBuildA2AInputWithOptionsSummaryTruncate(t *testing.T) {
	es := &ExecutionService{}

	a2aCtx := &A2AContext{
		Depth: 1,
	}

	// 超长输出
	longOutput := strings.Repeat("x", 1000)

	opts := &A2AInputOptions{
		MaxSummaryLength: 100,
	}

	input := es.BuildA2AInputWithOptions(a2aCtx, longOutput, opts)

	// 验证截断 - 在前序摘要部分，输出应该被截断到 MaxSummaryLength
	if strings.Contains(input, strings.Repeat("x", 500)) {
		t.Error("Summary should be truncated to MaxSummaryLength")
	}
}

// TestBuildA2AInputOptionsNilContext 测试空上下文情况
func TestBuildA2AInputOptionsNilContext(t *testing.T) {
	es := &ExecutionService{}

	input := es.BuildA2AInputWithOptions(nil, "output", nil)

	// 空 A2A 上下文时应该返回基本格式
	if input == "" {
		t.Error("A2A input should not be empty even with nil context")
	}
}