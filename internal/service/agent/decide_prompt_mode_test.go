package agent

import (
	"testing"

	"github.com/google/uuid"
)

// TestDecidePromptMode_FreshInvocation
// 首次调用（无 sessionID / strategy=New）：mode=New，无 fallback，
// 且下轮读取到 lastInjectedRegistryVersion。
func TestDecidePromptMode_FreshInvocation(t *testing.T) {
	es := &ExecutionService{}
	threadID := uuid.NewString()
	agentID := uuid.NewString()
	identity := IdentityKey("", agentID, threadID)
	defer func() {
		ClearInjectedRegistryVersion(identity)
		ConsumeReinjectionFlag(identity)
	}()

	// 保证 registry 有个非零 revision，避免 currentRev=0 时的边界干扰
	DefaultAgentConfigRegistry.Reset()
	baseRev := DefaultAgentConfigRegistry.GetRevision()

	layers := &ContextLayers{Layer0: "SYS"}
	mode, fallback := es.decidePromptMode(threadID, agentID, "", SessionStrategyNew, layers)

	if mode != PromptModeNew {
		t.Fatalf("want mode=New, got %s", mode)
	}
	if fallback != "" {
		t.Fatalf("New mode should not carry fallback, got %q", fallback)
	}
	if rev, ok := GetLastInjectedRegistryVersion(identity); !ok || rev != baseRev {
		t.Fatalf("expected lastInjectedRev=%d recorded, got rev=%d ok=%v", baseRev, rev, ok)
	}
}

// TestDecidePromptMode_ResumeSameRevision
// 已有 sessionID + Resume 策略 + registry 没变过 → PromptModeResume + fallback=Layer0
func TestDecidePromptMode_ResumeSameRevision(t *testing.T) {
	es := &ExecutionService{}
	threadID := uuid.NewString()
	agentID := uuid.NewString()
	identity := IdentityKey("", agentID, threadID)
	defer func() {
		ClearInjectedRegistryVersion(identity)
		ConsumeReinjectionFlag(identity)
	}()

	// 模拟"上一轮 New 时已注入过"：记录当前 revision
	currentRev := DefaultAgentConfigRegistry.GetRevision()
	RecordInjectedRegistryVersion(identity, currentRev)

	layers := &ContextLayers{Layer0: "SYS_L0_CONTENT"}
	mode, fallback := es.decidePromptMode(threadID, agentID, "sess-123", SessionStrategyResume, layers)

	if mode != PromptModeResume {
		t.Fatalf("want mode=Resume, got %s", mode)
	}
	if fallback != "SYS_L0_CONTENT" {
		t.Fatalf("Resume should carry Layer0 as fallback, got %q", fallback)
	}
}

// TestDecidePromptMode_RegistryChanged
// Resume 但 registry 变过 → ForceRefresh（重注 systemPrompt），fallback 应为空
func TestDecidePromptMode_RegistryChanged(t *testing.T) {
	es := &ExecutionService{}
	threadID := uuid.NewString()
	agentID := uuid.NewString()
	identity := IdentityKey("", agentID, threadID)
	defer func() {
		ClearInjectedRegistryVersion(identity)
		ConsumeReinjectionFlag(identity)
	}()

	oldRev := DefaultAgentConfigRegistry.GetRevision()
	RecordInjectedRegistryVersion(identity, oldRev)

	// 触发 revision 变化
	DefaultAgentConfigRegistry.Reset()

	layers := &ContextLayers{Layer0: "SYS"}
	mode, fallback := es.decidePromptMode(threadID, agentID, "sess-999", SessionStrategyResume, layers)

	if mode != PromptModeForceRefresh {
		t.Fatalf("registry changed should trigger ForceRefresh, got %s", mode)
	}
	if fallback != "" {
		t.Fatalf("ForceRefresh should not carry fallback (Layer0 already in prompt), got %q", fallback)
	}
	newRev := DefaultAgentConfigRegistry.GetRevision()
	if rev, _ := GetLastInjectedRegistryVersion(identity); rev != newRev {
		t.Fatalf("expected recorded rev=%d after ForceRefresh, got %d", newRev, rev)
	}
}

// TestDecidePromptMode_ForceReinjectionFlag_ConsumedOnce
// 手工 flag 触发 ForceRefresh，且 flag 只消费一次
func TestDecidePromptMode_ForceReinjectionFlag_ConsumedOnce(t *testing.T) {
	es := &ExecutionService{}
	threadID := uuid.NewString()
	agentID := uuid.NewString()
	identity := IdentityKey("", agentID, threadID)
	defer func() {
		ClearInjectedRegistryVersion(identity)
		ConsumeReinjectionFlag(identity)
	}()

	rev := DefaultAgentConfigRegistry.GetRevision()
	RecordInjectedRegistryVersion(identity, rev)
	FlagNeedsReinjection(identity)

	// 第一次：flag 被消费 → ForceRefresh
	mode1, _ := es.decidePromptMode(threadID, agentID, "sess", SessionStrategyResume, &ContextLayers{})
	if mode1 != PromptModeForceRefresh {
		t.Fatalf("first call with flag should be ForceRefresh, got %s", mode1)
	}

	// 第二次：flag 已消费 → Resume（registry 也没变）
	mode2, _ := es.decidePromptMode(threadID, agentID, "sess", SessionStrategyResume, &ContextLayers{})
	if mode2 != PromptModeResume {
		t.Fatalf("second call after flag consumed should be Resume, got %s", mode2)
	}
}

// TestDecidePromptMode_FirstResumeRecordsRevision
// 首次 Resume（hasLast=false）也应当记录 revision，防止下轮误判"变过"
func TestDecidePromptMode_FirstResumeRecordsRevision(t *testing.T) {
	es := &ExecutionService{}
	threadID := uuid.NewString()
	agentID := uuid.NewString()
	identity := IdentityKey("", agentID, threadID)
	defer func() {
		ClearInjectedRegistryVersion(identity)
		ConsumeReinjectionFlag(identity)
	}()

	// 没有 lastInjectedRev 记录（模拟服务重启后首次 Resume）
	ClearInjectedRegistryVersion(identity)
	currentRev := DefaultAgentConfigRegistry.GetRevision()

	mode, _ := es.decidePromptMode(threadID, agentID, "sess", SessionStrategyResume, &ContextLayers{Layer0: "SYS"})
	if mode != PromptModeResume {
		t.Fatalf("first resume with no prior record should be Resume, got %s", mode)
	}
	if rev, ok := GetLastInjectedRegistryVersion(identity); !ok || rev != currentRev {
		t.Fatalf("first resume should backfill lastInjectedRev=%d, got rev=%d ok=%v", currentRev, rev, ok)
	}
}

// TestDecidePromptMode_ResumeStrategyButNoSessionID
// SessionStrategyResume 但 sessionID 为空（首次 auto-resume 失败）→ 视为 New
func TestDecidePromptMode_ResumeStrategyButNoSessionID(t *testing.T) {
	es := &ExecutionService{}
	threadID := uuid.NewString()
	agentID := uuid.NewString()
	identity := IdentityKey("", agentID, threadID)
	defer ClearInjectedRegistryVersion(identity)

	mode, fallback := es.decidePromptMode(threadID, agentID, "", SessionStrategyResume, &ContextLayers{Layer0: "SYS"})
	if mode != PromptModeNew {
		t.Fatalf("Resume strategy without sessionID should degrade to New, got %s", mode)
	}
	if fallback != "" {
		t.Fatalf("New mode should have no fallback, got %q", fallback)
	}
}
