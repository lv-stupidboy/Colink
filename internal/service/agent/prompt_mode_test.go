package agent

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// -----------------------------------------------------------------------------
// AgentConfigRegistry
// -----------------------------------------------------------------------------

func TestAgentConfigRegistry_RevisionMonotonic(t *testing.T) {
	r := NewAgentConfigRegistry()
	if r.GetRevision() != 0 {
		t.Fatalf("initial revision should be 0, got %d", r.GetRevision())
	}
	id := uuid.New()
	r.Update(id, &model.AgentConfig{})
	if r.GetRevision() != 1 {
		t.Fatalf("expect 1 after 1 update, got %d", r.GetRevision())
	}
	r.Update(id, &model.AgentConfig{})
	if r.GetRevision() != 2 {
		t.Fatalf("expect 2 after 2 updates (idempotent still bumps), got %d", r.GetRevision())
	}
	r.Delete(id)
	if r.GetRevision() != 3 {
		t.Fatalf("expect 3 after delete, got %d", r.GetRevision())
	}
	r.Delete(id) // no-op — 已删除，revision 不再增加
	if r.GetRevision() != 3 {
		t.Fatalf("delete non-existent should not bump revision, got %d", r.GetRevision())
	}
	r.Reset()
	if r.GetRevision() != 4 {
		t.Fatalf("reset should bump revision, got %d", r.GetRevision())
	}
	if r.Size() != 0 {
		t.Fatalf("reset should clear configs, got size=%d", r.Size())
	}
}

func TestAgentConfigRegistry_ConcurrentUpdates(t *testing.T) {
	r := NewAgentConfigRegistry()
	const goroutines = 32
	const perG = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				r.Update(uuid.New(), &model.AgentConfig{})
			}
		}()
	}
	wg.Wait()
	got := r.GetRevision()
	want := int64(goroutines * perG)
	if got != want {
		t.Fatalf("expected revision=%d after concurrent updates, got %d", want, got)
	}
}

// -----------------------------------------------------------------------------
// InjectionState — consumed-once flag + revision map + context fill
// -----------------------------------------------------------------------------

func TestInjectionState_NeedsReinjection_ConsumedOnce(t *testing.T) {
	key := "test:" + uuid.NewString()
	defer ConsumeReinjectionFlag(key) // 清理

	if ConsumeReinjectionFlag(key) {
		t.Fatal("empty state should not have flag")
	}
	FlagNeedsReinjection(key)
	if !PeekReinjectionFlag(key) {
		t.Fatal("peek should see flag after set")
	}
	if !ConsumeReinjectionFlag(key) {
		t.Fatal("first consume should return true")
	}
	// 已消费，后续 consume 返回 false
	if ConsumeReinjectionFlag(key) {
		t.Fatal("second consume should return false (consumed-once)")
	}
	if PeekReinjectionFlag(key) {
		t.Fatal("peek should return false after consume")
	}
}

func TestInjectionState_ConcurrentConsume_ExactlyOneWins(t *testing.T) {
	// 多个 goroutine 同时 consume 同一 key，只有 1 个应该拿到 true
	const goroutines = 64
	key := "test-concurrent:" + uuid.NewString()
	defer ConsumeReinjectionFlag(key)

	FlagNeedsReinjection(key)

	var winners int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if ConsumeReinjectionFlag(key) {
				atomic.AddInt64(&winners, 1)
			}
		}()
	}
	wg.Wait()

	if winners != 1 {
		t.Fatalf("expected exactly 1 winner in concurrent consume, got %d", winners)
	}
}

func TestInjectionState_RegistryVersion_RecordAndCompare(t *testing.T) {
	key := "test-rev:" + uuid.NewString()
	defer ClearInjectedRegistryVersion(key)

	if _, ok := GetLastInjectedRegistryVersion(key); ok {
		t.Fatal("empty state should return ok=false")
	}

	RecordInjectedRegistryVersion(key, 42)
	rev, ok := GetLastInjectedRegistryVersion(key)
	if !ok {
		t.Fatal("should return ok=true after record")
	}
	if rev != 42 {
		t.Fatalf("want rev=42, got %d", rev)
	}

	RecordInjectedRegistryVersion(key, 100)
	rev, _ = GetLastInjectedRegistryVersion(key)
	if rev != 100 {
		t.Fatalf("update should overwrite, want 100, got %d", rev)
	}

	ClearInjectedRegistryVersion(key)
	if _, ok := GetLastInjectedRegistryVersion(key); ok {
		t.Fatal("clear should remove record")
	}
}

func TestInjectionState_ContextFill(t *testing.T) {
	key := "test-fill:" + uuid.NewString()
	defer ClearContextFill(key)

	if _, ok := GetPrevContextFill(key); ok {
		t.Fatal("empty state should return ok=false")
	}

	RecordContextFill(key, 8000)
	got, _ := GetPrevContextFill(key)
	if got != 8000 {
		t.Fatalf("want 8000, got %d", got)
	}
}

func TestInjectionState_CrossKeyIsolation(t *testing.T) {
	// 验证不同 key 之间完全隔离 —— 保护 IdentityKey 语义（"userId:agentId:threadId"）
	// 万一将来有人把 map 键改成合并 threadId 之类的 bug，本用例会失败
	keyA := "user1:agent1:thread1"
	keyB := "user1:agent1:thread2" // 只有 threadId 不同
	keyC := "user1:agent2:thread1" // 只有 agentId 不同
	keyD := "user2:agent1:thread1" // 只有 userId 不同

	defer func() {
		ConsumeReinjectionFlag(keyA)
		ConsumeReinjectionFlag(keyB)
		ConsumeReinjectionFlag(keyC)
		ConsumeReinjectionFlag(keyD)
		ClearInjectedRegistryVersion(keyA)
		ClearInjectedRegistryVersion(keyB)
		ClearInjectedRegistryVersion(keyC)
		ClearInjectedRegistryVersion(keyD)
		ClearContextFill(keyA)
		ClearContextFill(keyB)
		ClearContextFill(keyC)
		ClearContextFill(keyD)
	}()

	// needsReinjection 隔离
	FlagNeedsReinjection(keyA)
	if PeekReinjectionFlag(keyB) || PeekReinjectionFlag(keyC) || PeekReinjectionFlag(keyD) {
		t.Fatal("flag on keyA leaked to keyB/C/D")
	}
	if !ConsumeReinjectionFlag(keyA) {
		t.Fatal("keyA should still have flag")
	}

	// lastInjectedRegistryVersion 隔离
	RecordInjectedRegistryVersion(keyA, 10)
	RecordInjectedRegistryVersion(keyB, 20)
	revA, _ := GetLastInjectedRegistryVersion(keyA)
	revB, _ := GetLastInjectedRegistryVersion(keyB)
	if revA != 10 || revB != 20 {
		t.Fatalf("revision cross-contamination: A=%d B=%d", revA, revB)
	}
	if _, ok := GetLastInjectedRegistryVersion(keyC); ok {
		t.Fatal("keyC should not have any record")
	}

	// prevContextFill 隔离
	RecordContextFill(keyA, 1000)
	RecordContextFill(keyD, 9000)
	fillA, _ := GetPrevContextFill(keyA)
	fillD, _ := GetPrevContextFill(keyD)
	if fillA != 1000 || fillD != 9000 {
		t.Fatalf("context fill cross-contamination: A=%d D=%d", fillA, fillD)
	}
}

func TestIdentityKey_Format(t *testing.T) {
	// 保证 IdentityKey 格式稳定，避免有人无意改成会碰撞的格式
	got := IdentityKey("u1", "a1", "t1")
	if got != "u1:a1:t1" {
		t.Fatalf("IdentityKey format changed: %q", got)
	}

	// 不同顺序应产生不同 key
	if IdentityKey("a", "b", "c") == IdentityKey("c", "b", "a") {
		t.Fatal("IdentityKey should be order-sensitive")
	}
}

// -----------------------------------------------------------------------------
// BuildPromptV2 — mode 分支
// -----------------------------------------------------------------------------

func testLayers() *ContextLayers {
	return &ContextLayers{
		Layer0:        "SYS_L0",
		Layer2:        "ART_L2",
		Layer3:        "ENV_L3",
		MemoryContext: "MEM_CTX",
	}
}

func TestBuildPromptV2_NewMode_InjectsAll(t *testing.T) {
	out := BuildPromptV2(&PromptBuildRequest{
		Mode:   PromptModeNew,
		Layers: testLayers(),
		Input:  "hello",
	})

	for _, want := range []string{"<system>", "SYS_L0", "<artifacts>", "ART_L2", "<environment>", "ENV_L3", "<memory>", "MEM_CTX", "<user>", "hello"} {
		if !containsStr(out, want) {
			t.Errorf("New mode output should contain %q\nGOT: %s", want, out)
		}
	}
}

func TestBuildPromptV2_ResumeMode_SkipsStatic(t *testing.T) {
	out := BuildPromptV2(&PromptBuildRequest{
		Mode:         PromptModeResume,
		Layers:       testLayers(),
		Input:        "hello",
		EnvShortLine: "env: phase=dev, agent=xxx",
	})

	// Resume 场景不应含 static 段
	for _, forbid := range []string{"<system>", "SYS_L0", "<artifacts>", "ART_L2", "<environment>", "ENV_L3", "<memory>", "MEM_CTX"} {
		if containsStr(out, forbid) {
			t.Errorf("Resume mode should NOT contain %q\nGOT: %s", forbid, out)
		}
	}
	// 但短 env line + user input 必须在
	for _, want := range []string{"env: phase=dev, agent=xxx", "<user>", "hello"} {
		if !containsStr(out, want) {
			t.Errorf("Resume mode should contain %q\nGOT: %s", want, out)
		}
	}
}

func TestBuildPromptV2_ForceRefresh_LikeNew(t *testing.T) {
	out := BuildPromptV2(&PromptBuildRequest{
		Mode:   PromptModeForceRefresh,
		Layers: testLayers(),
		Input:  "hello",
	})
	if !containsStr(out, "SYS_L0") || !containsStr(out, "MEM_CTX") {
		t.Fatalf("force refresh should behave like new (full inject)\nGOT: %s", out)
	}
}

func TestBuildPromptV2_StagingAlwaysEmitted(t *testing.T) {
	staging := ">> STAGING <<"

	// Resume 也要发 staging（ADR-038 契约：每轮注入生效）
	out := BuildPromptV2(&PromptBuildRequest{
		Mode:          PromptModeResume,
		Layers:        testLayers(),
		Input:         "hi",
		StagingPrefix: staging,
	})
	if !containsStr(out, staging) {
		t.Fatalf("Staging must be emitted even in Resume mode\nGOT: %s", out)
	}
}

func TestBuildPromptV2_UpstreamHandoff_Independent(t *testing.T) {
	// 只有 Handoff，没有 layers
	out := BuildPromptV2(&PromptBuildRequest{
		Mode:            PromptModeResume,
		Input:           "downstream input",
		UpstreamHandoff: "HANDOFF_BODY",
		UpstreamName:    "架构师",
	})
	for _, want := range []string{"<a2a-handoff-from-upstream>", "HANDOFF_BODY", "架构师"} {
		if !containsStr(out, want) {
			t.Errorf("Upstream handoff missing %q\nGOT: %s", want, out)
		}
	}
}

func TestBuildPromptV2_A2AInputTag(t *testing.T) {
	// 输入含 "Direct message from" → user tag 应变成 a2a_input
	out := BuildPromptV2(&PromptBuildRequest{
		Mode:  PromptModeNew,
		Input: "Direct message from A: please implement",
	})
	if !containsStr(out, "<a2a_input>") {
		t.Fatalf("expected a2a_input tag when input contains 'Direct message from'\nGOT: %s", out)
	}
	if containsStr(out, "<user>") {
		t.Fatalf("should not emit <user> tag when a2a input detected")
	}
}

func TestPromptMode_String(t *testing.T) {
	cases := map[PromptMode]string{
		PromptModeNew:          "new",
		PromptModeResume:       "resume",
		PromptModeForceRefresh: "force_refresh",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("PromptMode(%d).String() = %q, want %q", m, got, want)
		}
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func containsStr(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
