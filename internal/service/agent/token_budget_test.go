package agent

import (
	"testing"

	"github.com/google/uuid"
)

// TestCalculateMaxA2ADepthSufficient 测试预算充足时的深度计算
func TestCalculateMaxA2ADepthSufficient(t *testing.T) {
	manager := NewTokenBudgetManager()

	// 假设剩余预算 100K，平均每轮 10K，应允许 10 层
	remainingBudget := int64(100_000)
	avgTurnTokens := int64(10_000)

	depth := manager.CalculateMaxA2ADepth(remainingBudget, avgTurnTokens)

	if depth < 1 {
		t.Errorf("Depth should be at least 1, got %d", depth)
	}
	if depth > MaxA2ADepth {
		t.Errorf("Depth should not exceed max %d, got %d", MaxA2ADepth, depth)
	}
	// 100K / 10K = 10，应该返回 10
	if depth != 10 {
		t.Errorf("Expected depth 10, got %d", depth)
	}
}

// TestCalculateMaxA2ADepthInsufficient 测试预算不足时的限制
func TestCalculateMaxA2ADepthInsufficient(t *testing.T) {
	manager := NewTokenBudgetManager()

	// 假设剩余预算只有 5K，平均每轮 10K
	remainingBudget := int64(5_000)
	avgTurnTokens := int64(10_000)

	depth := manager.CalculateMaxA2ADepth(remainingBudget, avgTurnTokens)

	// 预算不足时，最少允许 1 层
	if depth != 1 {
		t.Errorf("Expected minimum depth 1, got %d", depth)
	}
}

// TestCalculateMaxA2ADepthMaxLimit 测试最大深度限制
func TestCalculateMaxA2ADepthMaxLimit(t *testing.T) {
	manager := NewTokenBudgetManager()

	// 假设预算充足，但不应超过最大限制
	remainingBudget := int64(500_000)
	avgTurnTokens := int64(10_000)

	depth := manager.CalculateMaxA2ADepth(remainingBudget, avgTurnTokens)

	// 500K / 10K = 50，但最大限制是 MaxA2ADepth
	if depth > MaxA2ADepth {
		t.Errorf("Depth should be capped at %d, got %d", MaxA2ADepth, depth)
	}
	if depth != MaxA2ADepth {
		t.Errorf("Expected max depth %d, got %d", MaxA2ADepth, depth)
	}
}

// TestCalculateMaxA2ADepthZeroAvg 测试平均 Token 为零的情况
func TestCalculateMaxA2ADepthZeroAvg(t *testing.T) {
	manager := NewTokenBudgetManager()

	remainingBudget := int64(100_000)
	avgTurnTokens := int64(0) // 零值

	depth := manager.CalculateMaxA2ADepth(remainingBudget, avgTurnTokens)

	// 当 avgTurnTokens 为零或负数时，应返回默认最大深度
	if depth != MaxA2ADepth {
		t.Errorf("Expected default max depth %d when avgTurnTokens is zero, got %d", MaxA2ADepth, depth)
	}
}

// TestUpdateUsageFromCLI 测试 Usage 信息更新
func TestUpdateUsageFromCLI(t *testing.T) {
	manager := NewTokenBudgetManager()
	invocationID := uuid.New()

	usage := &TokenUsage{
		InputTokens:     50_000,
		OutputTokens:    20_000,
		CacheReadTokens: 10_000,
	}

	manager.UpdateUsageFromCLI(invocationID, usage)

	// 验证缓存已更新
 cachedUsage := manager.GetUsage(invocationID)
	if cachedUsage == nil {
		t.Error("Usage should be cached")
	}
	if cachedUsage.InputTokens != 50_000 {
		t.Errorf("InputTokens mismatch: got %d, want 50000", cachedUsage.InputTokens)
	}
}

// TestGetUsageNotFound 测试获取不存在的 Usage
func TestGetUsageNotFound(t *testing.T) {
	manager := NewTokenBudgetManager()
	invocationID := uuid.New() // 新的 ID，没有缓存

	usage := manager.GetUsage(invocationID)

	if usage != nil {
		t.Error("Expected nil for non-existent usage")
	}
}

// TestGetRemainingBudget 测试剩余预算计算
func TestGetRemainingBudget(t *testing.T) {
	manager := NewTokenBudgetManager()

	usage := &TokenUsage{
		InputTokens:  50_000,
		OutputTokens: 20_000,
	}

	// 使用 claude-sonnet-4-6 模型（上下文窗口 200K）
	model := "claude-sonnet-4-6"
	remaining := manager.GetRemainingBudget(model, usage)

	// 200K - 50K - 20K = 130K
	expected := int64(130_000)
	if remaining != expected {
		t.Errorf("Remaining budget mismatch: got %d, want %d", remaining, expected)
	}
}

// TestGetRemainingBudgetUnknownModel 测试未知模型的预算计算
func TestGetRemainingBudgetUnknownModel(t *testing.T) {
	manager := NewTokenBudgetManager()

	usage := &TokenUsage{
		InputTokens:  50_000,
		OutputTokens: 20_000,
	}

	model := "unknown-model-xyz"
	remaining := manager.GetRemainingBudget(model, usage)

	// 未知模型使用默认值 200K
	// 200K - 50K - 20K = 130K
	expected := int64(130_000)
	if remaining != expected {
		t.Errorf("Remaining budget for unknown model: got %d, want %d", remaining, expected)
	}
}

// TestGetAvgTurnTokens 测试动态平均 Token 获取
func TestGetAvgTurnTokens(t *testing.T) {
	manager := NewTokenBudgetManager()
	threadID := uuid.New()

	// 没有缓存时，返回默认值
	avg := manager.GetAvgTurnTokens(threadID)
	if avg != 10_000 {
		t.Errorf("Default avgTurnTokens should be 10000, got %d", avg)
	}

	// 更新后返回更新值
	manager.UpdateAvgTurnTokens(threadID, 15_000)
	avg = manager.GetAvgTurnTokens(threadID)
	if avg != 15_000 {
		t.Errorf("Updated avgTurnTokens should be 15000, got %d", avg)
	}
}

// TestGetContextWindowSize 测试上下文窗口大小获取
func TestGetContextWindowSize(t *testing.T) {
	manager := NewTokenBudgetManager()

	tests := []struct {
		model    string
		expected int64
	}{
		{"claude-opus-4-6", 200_000},
		{"claude-sonnet-4-5", 200_000},
		{"claude-haiku-4-5", 200_000},
		{"gpt-5.3", 128_000},
		{"gpt-5.2", 128_000},
		{"o3", 200_000},
		{"o4-mini", 200_000},
		{"claude-sonnet-4-6-custom", 200_000}, // 前缀匹配
		{"unknown", 200_000},                  // 默认值
	}

	for _, tt := range tests {
		size := manager.GetContextWindowSize(tt.model)
		if size != tt.expected {
			t.Errorf("Context window size for %s: got %d, want %d", tt.model, size, tt.expected)
		}
	}
}