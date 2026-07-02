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

// TestFillRatio 测试填充比率计算
func TestFillRatio(t *testing.T) {
	manager := NewTokenBudgetManager()
	model := "claude-sonnet-4-6" // 200K context window

	tests := []struct {
		name     string
		usage    *TokenUsage
		expected float64
	}{
		{
			name:     "empty window - no tokens used",
			usage:    &TokenUsage{InputTokens: 0, OutputTokens: 0},
			expected: 0.0,
		},
		{
			name:     "half full - 50% usage",
			usage:    &TokenUsage{InputTokens: 100_000, OutputTokens: 0},
			expected: 0.5,
		},
		{
			name:     "nearly full - 90% usage",
			usage:    &TokenUsage{InputTokens: 100_000, OutputTokens: 80_000},
			expected: 0.9,
		},
		{
			name:     "mixed input and output tokens",
			usage:    &TokenUsage{InputTokens: 75_000, OutputTokens: 25_000},
			expected: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := manager.FillRatio(model, tt.usage)
			// Allow small floating point error
			if ratio < tt.expected-0.0001 || ratio > tt.expected+0.0001 {
				t.Errorf("FillRatio() = %f, want %f", ratio, tt.expected)
			}
		})
	}
}

// TestFillRatioUnknownModel 测试未知模型的填充比率
func TestFillRatioUnknownModel(t *testing.T) {
	manager := NewTokenBudgetManager()

	// Unknown model should use default 200K window
	usage := &TokenUsage{InputTokens: 50_000, OutputTokens: 50_000}
	ratio := manager.FillRatio("unknown-model", usage)

	expected := 0.5 // 100K / 200K = 0.5
	if ratio < expected-0.0001 || ratio > expected+0.0001 {
		t.Errorf("FillRatio for unknown model = %f, want %f", ratio, expected)
	}
}

// TestShouldTakeAction 测试 ShouldTakeAction 决策逻辑
func TestShouldTakeAction(t *testing.T) {
	tests := []struct {
		name            string
		fillRatio       float64
		remainingTokens int
		expected        StrategyAction
	}{
		{
			name:            "no action needed - low fill ratio and sufficient tokens",
			fillRatio:       0.5,
			remainingTokens: 15000,
			expected:        ActionNone,
		},
		{
			name:            "warn threshold - fill ratio at 0.75",
			fillRatio:       0.75,
			remainingTokens: 15000, // Above TurnBudget to test fillRatio condition
			expected:        ActionWarn,
		},
		{
			name:            "action threshold - fill ratio at 0.85",
			fillRatio:       0.85,
			remainingTokens: 5000,
			expected:        ActionSeal,
		},
		{
			name:            "seal required - low remaining tokens below TurnBudget",
			fillRatio:       0.6,
			remainingTokens: 10000, // Below TurnBudget (12000)
			expected:        ActionSeal,
		},
		{
			name:            "seal required - fill ratio exceeds ActionThreshold",
			fillRatio:       0.90,
			remainingTokens: 20000,
			expected:        ActionSeal,
		},
		{
			name:            "warn - fill ratio between WarnThreshold and ActionThreshold",
			fillRatio:       0.80,
			remainingTokens: 15000,
			expected:        ActionWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldTakeAction(tt.fillRatio, tt.remainingTokens)
			if result != tt.expected {
				t.Errorf("ShouldTakeAction(%f, %d) = %v, want %v",
					tt.fillRatio, tt.remainingTokens, result, tt.expected)
			}
		})
	}
}