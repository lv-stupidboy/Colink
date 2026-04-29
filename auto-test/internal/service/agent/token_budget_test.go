package agent

import (
	"testing"

	"github.com/google/uuid"
)

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