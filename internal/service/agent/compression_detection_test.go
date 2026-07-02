package agent

import (
	"testing"

	"github.com/google/uuid"
)

// TestCompressionDetection_DropOver60Percent
// 模拟 clowder-ai invoke-single-cat.ts:2022 的 usage 骤降检测：
//   - 首次 usage：记录 prevFill，不触发 flag
//   - 后续 usage 骤降 >60%：触发 FlagNeedsReinjection
//   - 后续 usage 缓慢增长：不触发
func TestCompressionDetection_DropOver60Percent(t *testing.T) {
	threadID := uuid.NewString()
	agentID := uuid.NewString()
	identity := IdentityKey("", agentID, threadID)
	defer func() {
		ClearContextFill(identity)
		ConsumeReinjectionFlag(identity)
	}()

	// 复用 execution_service.go 里的判定逻辑（简化提取）
	check := func(usedTokens int) {
		if usedTokens > 0 {
			if prevFill, ok := GetPrevContextFill(identity); ok {
				if prevFill > 0 && usedTokens < prevFill*4/10 {
					FlagNeedsReinjection(identity)
				}
			}
			RecordContextFill(identity, usedTokens)
		}
	}

	// Round 1: 首次填充 10000 tokens
	check(10000)
	if PeekReinjectionFlag(identity) {
		t.Fatal("first usage should not trigger flag")
	}
	if fill, _ := GetPrevContextFill(identity); fill != 10000 {
		t.Fatalf("want fill=10000, got %d", fill)
	}

	// Round 2: 缓慢增长到 12000 —— 不触发
	check(12000)
	if PeekReinjectionFlag(identity) {
		t.Fatal("gradual growth should not trigger flag")
	}

	// Round 3: 骤降到 3000（从 12000 掉 75%）—— 应触发
	check(3000)
	if !PeekReinjectionFlag(identity) {
		t.Fatal("drop >60% should trigger reinjection flag")
	}

	// consumed-once：消费一次后清空
	if !ConsumeReinjectionFlag(identity) {
		t.Fatal("flag should be consumable")
	}
	if PeekReinjectionFlag(identity) {
		t.Fatal("flag should be gone after consume")
	}
}

// TestCompressionDetection_BorderCase_ExactlyAtThreshold
// 边界：正好在阈值上 (drop 60%) 时不触发（严格小于才算）
// 每个子用例独立起 baseline，因为 check() 会把上次值写进 prevFill 破坏后续比较。
func TestCompressionDetection_BorderCase_ExactlyAtThreshold(t *testing.T) {
	check := func(identity string, usedTokens int) {
		if usedTokens > 0 {
			if prevFill, ok := GetPrevContextFill(identity); ok {
				if prevFill > 0 && usedTokens < prevFill*4/10 {
					FlagNeedsReinjection(identity)
				}
			}
			RecordContextFill(identity, usedTokens)
		}
	}

	// Case A：正好 40% (4000/10000) —— 严格 < 不满足，不触发
	idA := "test-border-a:" + uuid.NewString()
	defer func() { ClearContextFill(idA); ConsumeReinjectionFlag(idA) }()
	check(idA, 10000)
	check(idA, 4000)
	if PeekReinjectionFlag(idA) {
		t.Fatal("exactly 40% should NOT trigger (strict <)")
	}

	// Case B：39.99% (3999/10000) —— 严格 < 满足，触发
	idB := "test-border-b:" + uuid.NewString()
	defer func() { ClearContextFill(idB); ConsumeReinjectionFlag(idB) }()
	check(idB, 10000)
	check(idB, 3999)
	if !PeekReinjectionFlag(idB) {
		t.Fatal("39.99% (just below 40%) SHOULD trigger")
	}
}

// TestCompressionDetection_ZeroTokens_Ignored
// contextUsed=0 (通常是 usage chunk 没上报) 直接跳过，不影响 prev 记录
func TestCompressionDetection_ZeroTokens_Ignored(t *testing.T) {
	identity := "test-zero:" + uuid.NewString()
	defer ClearContextFill(identity)

	check := func(usedTokens int) {
		if usedTokens > 0 {
			if prevFill, ok := GetPrevContextFill(identity); ok {
				if prevFill > 0 && usedTokens < prevFill*4/10 {
					FlagNeedsReinjection(identity)
				}
			}
			RecordContextFill(identity, usedTokens)
		}
	}

	check(5000)
	check(0) // ignored
	fill, _ := GetPrevContextFill(identity)
	if fill != 5000 {
		t.Fatalf("zero usedTokens should not overwrite prevFill, got %d", fill)
	}
}
