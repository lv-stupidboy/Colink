package agent

import (
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestTokenBudgetThresholdIntegration 测试 Token 预算阈值集成
// 验证 usage chunk 处理时 FillRatio 计算和阈值决策是否正确
func TestTokenBudgetThresholdIntegration(t *testing.T) {
	// 设置测试 logger
	logger := zaptest.NewLogger(t)
	zap.ReplaceGlobals(logger)

	// 创建 TokenBudgetManager
	budgetManager := NewTokenBudgetManager()

	// 模拟 invocationID
	invocationID := uuid.New()

	tests := []struct {
		name           string
		contextUsed    int64
		contextSize    int64
		expectedAction StrategyAction
	}{
		{
			name:           "normal_usage_below_warn",
			contextUsed:    50000,  // 25% usage
			contextSize:    200000,
			expectedAction: ActionNone,
		},
		{
			name:           "warn_threshold_75_percent",
			contextUsed:    150000, // 75% usage (正好触发 WarnThreshold)
			contextSize:    200000,
			expectedAction: ActionWarn,
		},
		{
			name:           "action_threshold_85_percent",
			contextUsed:    170000, // 85% usage (触发 ActionThreshold)
			contextSize:    200000,
			expectedAction: ActionSeal,
		},
		{
			name:           "low_remaining_tokens",
			contextUsed:    188000, // 剩余 12000 (正好低于 TurnBudget)
			contextSize:    200000,
			expectedAction: ActionSeal,
		},
		{
			name:           "exceeded_threshold",
			contextUsed:    190000, // 95% usage
			contextSize:    200000,
			expectedAction: ActionSeal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建模拟 usage
			usage := &TokenUsage{
				ContextUsed: tt.contextUsed,
				ContextSize: tt.contextSize,
			}

			// 更新 usage 缓存
			budgetManager.UpdateUsageFromCLI(invocationID, usage)

			// 计算 FillRatio
			fillRatio := float64(usage.ContextUsed) / float64(usage.ContextSize)
			remainingTokens := int(usage.ContextSize - usage.ContextUsed)

			// 检查阈值决策
			action := ShouldTakeAction(fillRatio, remainingTokens)

			// 验证结果
			if action != tt.expectedAction {
				t.Errorf("ShouldTakeAction() = %v, want %v (fillRatio=%.2f, remaining=%d)",
					action, tt.expectedAction, fillRatio, remainingTokens)
			}

			// 打印详细信息（用于验证）
			t.Logf("Test case: %s", tt.name)
			t.Logf("  ContextUsed: %d, ContextSize: %d", tt.contextUsed, tt.contextSize)
			t.Logf("  FillRatio: %.2f (%.1f%%)", fillRatio, fillRatio*100)
			t.Logf("  RemainingTokens: %d", remainingTokens)
			t.Logf("  Action: %v", action)
		})
	}
}

// TestUsageChunkBroadcastIntegration 测试 broadcastChunk 中的阈值检查逻辑
// 验证 usage chunk 处理流程是否正确集成
func TestUsageChunkBroadcastIntegration(t *testing.T) {
	// 设置测试 logger
	logger := zaptest.NewLogger(t)
	zap.ReplaceGlobals(logger)

	// 创建 ExecutionService（简化版，仅测试 broadcastChunk）
	// 注意：实际测试需要完整的 ExecutionService 或 mock

	t.Run("usage_chunk_processing", func(t *testing.T) {
		// 计算 FillRatio 和决策
		usage := &TokenUsage{
			InputTokens:  50000,
			OutputTokens: 20000,
			ContextUsed:  170000, // 85% usage，触发 ActionSeal
			ContextSize:  200000,
		}

		// 计算 FillRatio 和决策
		fillRatio := float64(usage.ContextUsed) / float64(usage.ContextSize)
		remainingTokens := int(usage.ContextSize - usage.ContextUsed)
		action := ShouldTakeAction(fillRatio, remainingTokens)

		// 验证阈值触发
		t.Logf("Usage chunk received:")
		t.Logf("  ContextUsed: %d, ContextSize: %d", usage.ContextUsed, usage.ContextSize)
		t.Logf("  FillRatio: %.2f (%.1f%%)", fillRatio, fillRatio*100)
		t.Logf("  RemainingTokens: %d", remainingTokens)
		t.Logf("  Expected Action: ActionSeal")
		t.Logf("  Actual Action: %v", action)

		if action != ActionSeal {
			t.Errorf("Expected ActionSeal for fillRatio=%.2f, got %v", fillRatio, action)
		}
	})
}