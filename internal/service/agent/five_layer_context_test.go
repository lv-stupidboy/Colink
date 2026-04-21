package agent

import (
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
)

// TestFiveLayerContextIntegration 验证五层上下文结构集成
func TestFiveLayerContextIntegration(t *testing.T) {
	// 模拟 AgentConfig
	config := &model.AgentConfig{
		Name:         "架构师",
		Description:  "负责系统架构设计",
		SystemPrompt: "你需要分析需求并设计技术方案",
	}

	// 1. 测试 L0 构建（含治理摘要）
	t.Run("Layer0_WithGovernanceDigest", func(t *testing.T) {
		layer0 := BuildStaticLayer0(config)

		// 验证包含角色定义
		if !strings.Contains(layer0, "架构师") {
			t.Error("L0 should contain agent name")
		}

		// 验证包含系统提示
		if !strings.Contains(layer0, "技术方案") {
			t.Error("L0 should contain system prompt")
		}

		// 验证包含治理摘要关键内容
		keyPhrases := []string{
			"协作守则",
			"出口检查",
			"@mention",
			"质量约束",
			"交接规范",
			"GOVERNANCE_DIGEST_VERSION",
		}

		for _, phrase := range keyPhrases {
			if !strings.Contains(layer0, phrase) {
				t.Errorf("L0 should contain governance phrase '%s'", phrase)
			}
		}

		// 验证 Token 数估算
		l0Tokens := EstimateTokens(layer0)
		t.Logf("Layer0 estimated tokens: %d", l0Tokens)
		if l0Tokens > 500 {
			t.Errorf("L0 tokens (%d) is too high, consider simplifying", l0Tokens)
		}
	})

	// 2. 测试 L0 最小版本（不含治理摘要）
	t.Run("Layer0_Minimal", func(t *testing.T) {
		layer0Minimal := BuildStaticLayer0Minimal(config)

		// 验证包含角色定义
		if !strings.Contains(layer0Minimal, "架构师") {
			t.Error("L0Minimal should contain agent name")
		}

		// 验证不包含治理摘要
		if strings.Contains(layer0Minimal, "协作守则") {
			t.Error("L0Minimal should NOT contain governance digest")
		}

		// 验证 Token 数低于完整版本
		minimalTokens := EstimateTokens(layer0Minimal)
		fullTokens := EstimateTokens(BuildStaticLayer0(config))

		if minimalTokens >= fullTokens {
			t.Errorf("L0Minimal tokens (%d) should be less than full L0 (%d)", minimalTokens, fullTokens)
		}

		t.Logf("L0Minimal tokens: %d, Full L0 tokens: %d, Saved: %d", minimalTokens, fullTokens, fullTokens-minimalTokens)
	})

	// 3. 测试 A2A Handoff 提取
	t.Run("A2AHandoffExtraction", func(t *testing.T) {
		outputWithHandoff := `<a2a-handoff>
### What
修改了 internal/service/agent/context_builder.go (Edit)

### Why
治理规则需要嵌入 L0 层，避免重复注入

### Tradeoff
放弃了在 L4 动态注入的方式

### Open Questions
无

### Next Action
请测试验证治理摘要编译
</a2a-handoff>

这是我的分析结论...`

		handoff, found := ExtractHandoffBlock(outputWithHandoff)
		if !found {
			t.Error("Should find handoff block")
		}

		if !strings.Contains(handoff, "What") || !strings.Contains(handoff, "Why") {
			t.Error("Handoff should contain What and Why sections")
		}

		// 验证 Token 预算约束
		constrained := ConstrainHandoffBudget(handoff, DefaultHandoffMaxTokens)
		constrainedTokens := EstimateTokens(constrained)
		if constrainedTokens > DefaultHandoffMaxTokens {
			t.Errorf("Constrained handoff tokens (%d) exceeds limit (%d)", constrainedTokens, DefaultHandoffMaxTokens)
		}

		t.Logf("Handoff original tokens: %d, Constrained tokens: %d", EstimateTokens(handoff), constrainedTokens)
	})

	// 4. 测试无 Handoff 时的降级处理
	t.Run("A2AHandoffNotFound", func(t *testing.T) {
		outputNoHandoff := "这是我的分析结论，没有交接块。"

		handoff, found := ExtractHandoffBlock(outputNoHandoff)
		if found {
			t.Error("Should NOT find handoff block when not present")
		}

		if handoff != "" {
			t.Error("Handoff should be empty when not found")
		}
	})
}

// TestTokenBudgetManagerIntegration 验证 Token 预算管理集成
func TestTokenBudgetManagerIntegration(t *testing.T) {
	tbm := NewTokenBudgetManager()

	// 测试上下文窗口获取
	t.Run("ContextWindowSize", func(t *testing.T) {
		models := []string{
			"claude-opus-4-6",
			"claude-sonnet-4-6",
			"claude-haiku-4-5",
			"unknown-model",
		}

		for _, model := range models {
			windowSize := tbm.GetContextWindowSize(model)
			if windowSize <= 0 {
				t.Errorf("Context window for model '%s' should be positive", model)
			}
			t.Logf("Model '%s' context window: %d", model, windowSize)
		}
	})

	// 测试 A2A 深度计算
	t.Run("MaxA2ADepthCalculation", func(t *testing.T) {
		testCases := []struct {
			budget    int64
			avgTokens int64
		}{
			{100_000, 10_000}, // 10 层
			{50_000, 10_000},  // 5 层
			{10_000, 10_000},  // 1 层（最少）
			{200_000, 5_000},  // 15 层（上限）
		}

		for _, tc := range testCases {
			depth := tbm.CalculateMaxA2ADepth(tc.budget, tc.avgTokens)
			if depth < 1 {
				t.Errorf("Depth should be at least 1 (budget=%d, avg=%d)", tc.budget, tc.avgTokens)
			}
			if depth > MaxA2ADepth {
				t.Errorf("Depth should not exceed MaxA2ADepth (got %d, max %d)", depth, MaxA2ADepth)
			}
			t.Logf("Budget=%d, AvgTurn=%d -> MaxDepth=%d", tc.budget, tc.avgTokens, depth)
		}
	})
}

// TestGovernanceDigestValidation 验证治理摘要有效性
func TestGovernanceDigestValidation(t *testing.T) {
	// 验证版本号
	if GovernanceDigestVersion == "" {
		t.Error("GovernanceDigestVersion should not be empty")
	}
	t.Logf("Governance digest version: %s", GovernanceDigestVersion)

	// 验证摘要内容
	digest := BuildGovernanceDigest()
	if digest == "" {
		t.Error("Governance digest should not be empty")
	}

	// 验证规则编号索引完整性（规则定义在 shared-rules.md）
	ruleContents := map[string]string{
		"R1": "出口检查",
		"R2": "@mention",
		"R3": "协作守则", // 角色边界在完整规则文件中，摘要简化
		"R4": "落盘记录",
		"R5": "阻塞流程",
		"R6": "工作成果",
	}

	for ruleNum, content := range ruleContents {
		if !strings.Contains(digest, content) {
			t.Errorf("Governance digest should contain content for rule %s (%s)", ruleNum, content)
		}
	}

	// 验证 Token 约束
	tokens := GovernanceDigestTokens()
	t.Logf("Governance digest tokens: %d", tokens)
	if tokens > 300 {
		t.Errorf("Governance digest tokens (%d) exceeds constraint (300)", tokens)
	}

	// 验证整体有效性
	if !ValidateGovernanceDigest() {
		t.Error("Governance digest validation should pass")
	}
}