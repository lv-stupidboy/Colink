package agent

import (
	"context"
	"os/exec"
	"reflect"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestGetAdapter_NilBaseAgent(t *testing.T) {
	adapter := GetAdapter(nil)
	if adapter != nil {
		t.Error("Expected nil adapter for nil base agent")
	}
}

func TestClaudeCodeUseACPToggle(t *testing.T) {
	SetClaudeCodeUseACP(true)
	if !GetClaudeCodeUseACP() {
		t.Fatal("expected Claude Code ACP flag to be true")
	}
	SetClaudeCodeUseACP(false)
	if GetClaudeCodeUseACP() {
		t.Fatal("expected Claude Code ACP flag to be false")
	}
}

// Note: Tests for GetAdapter require plugins to be registered.
// Plugin registration happens via init() when importing plugins/all.
// Due to import cycle restrictions, these tests are in a separate test package.
// See: internal/service/agent/adapter_integration_test.go

func TestGetMeta_RegisteredPlugin(t *testing.T) {
	// This test only works if plugins are registered
	// For now, we test the registry structure without plugins
	meta := GetMeta("claude_code")
	// If no plugins registered, meta will be nil - that's expected in this test context
	if meta != nil {
		if meta.Name != "ClaudeCode" {
			t.Errorf("Expected name 'ClaudeCode', got '%s'", meta.Name)
		}
	}
}

func TestGetTypes_EmptyRegistry(t *testing.T) {
	types := GetTypes()
	// Without plugin imports, registry may be empty
	// This tests the registry structure, not plugin registration
	t.Logf("Registered types count: %d", len(types))
}

func TestRegisterPlugin(t *testing.T) {
	// Test that RegisterPlugin works correctly
	// Use a test-specific type to avoid conflicts
	testType := model.BaseAgentType("test_type_" + uuid.New().String())

	testMeta := PluginMeta{
		Type:        testType,
		Name:        "TestPlugin",
		Description: "Test plugin for unit testing",
		Factory: func(baseAgent *model.BaseAgent) AgentAdapter {
			return nil // Mock adapter
		},
	}

	RegisterPlugin(testMeta)

	// Verify registration
	meta := GetMeta(testType)
	if meta == nil {
		t.Error("Expected plugin to be registered")
	}
	if meta.Name != "TestPlugin" {
		t.Errorf("Expected name 'TestPlugin', got '%s'", meta.Name)
	}
}

func TestGetAdapterRegisteredAndMissing(t *testing.T) {
	testType := model.BaseAgentType("adapter_type_" + uuid.New().String())
	expected := &mockAgentAdapter{}
	RegisterPlugin(PluginMeta{
		Type: testType,
		Name: "AdapterPlugin",
		Factory: func(baseAgent *model.BaseAgent) AgentAdapter {
			if baseAgent.Type != testType {
				t.Fatalf("factory received wrong base agent type %s", baseAgent.Type)
			}
			return expected
		},
	})

	got := GetAdapter(&model.BaseAgent{Type: testType})
	if gotAdapter, ok := got.(*mockAgentAdapter); !ok || gotAdapter != expected {
		t.Fatalf("expected registered adapter, got %#v", got)
	}
	if got := GetAdapter(&model.BaseAgent{Type: model.BaseAgentType("missing_" + uuid.New().String())}); got != nil {
		t.Fatalf("expected nil for missing adapter, got %#v", got)
	}
}

func TestRegisterPluginPanicsOnDuplicate(t *testing.T) {
	testType := model.BaseAgentType("duplicate_type_" + uuid.New().String())
	meta := PluginMeta{
		Type: testType,
		Name: "First",
		Factory: func(baseAgent *model.BaseAgent) AgentAdapter {
			return nil
		},
	}

	RegisterPlugin(meta)

	defer func() {
		if recover() == nil {
			t.Fatal("expected duplicate plugin registration to panic")
		}
	}()
	RegisterPlugin(meta)
}

func TestGetTypesSortedAndConfigHelpers(t *testing.T) {
	prefix := "sorted_type_" + uuid.New().String()
	typeA := model.BaseAgentType(prefix + "_a")
	typeB := model.BaseAgentType(prefix + "_b")
	factoryCalled := false
	generatorFactory := func(string, string, string, string, *zap.Logger) AssetConfigGenerator {
		factoryCalled = true
		return nil
	}

	RegisterPlugin(PluginMeta{
		Type:        typeB,
		Name:        "B",
		Description: "second",
		ConfigDir:   ".b",
		Factory: func(baseAgent *model.BaseAgent) AgentAdapter {
			return nil
		},
	})
	RegisterPlugin(PluginMeta{
		Type:                   typeA,
		Name:                   "A",
		Description:            "first",
		ConfigDir:              ".a",
		ConfigGeneratorFactory: generatorFactory,
		Factory: func(baseAgent *model.BaseAgent) AgentAdapter {
			return nil
		},
	})

	types := GetTypes()
	positions := map[model.BaseAgentType]int{}
	for i, typ := range types {
		positions[typ.Type] = i
	}
	if positions[typeA] >= positions[typeB] {
		t.Fatalf("expected types to be sorted by type, got positions %v", positions)
	}
	if got := GetConfigDir(typeA); got != ".a" {
		t.Fatalf("expected config dir .a, got %q", got)
	}
	if got := GetConfigDir("missing_" + model.BaseAgentType(uuid.New().String())); got != ".claude" {
		t.Fatalf("expected fallback config dir .claude, got %q", got)
	}
	if CreateConfigGenerator(typeA, "", "", "", "", zap.NewNop()) != nil || !factoryCalled {
		t.Fatal("expected registered config generator factory to be called")
	}
	if CreateConfigGenerator(model.BaseAgentType("missing_"+uuid.New().String()), "", "", "", "", zap.NewNop()) != nil {
		t.Fatal("expected missing config generator to return nil")
	}

	meta := GetMeta(typeA)
	if meta == nil || !reflect.DeepEqual(meta.Type, typeA) {
		t.Fatalf("expected meta for %s, got %#v", typeA, meta)
	}
}

type mockAgentAdapter struct{}

func (m *mockAgentAdapter) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	return &ExecutionResult{}, nil
}

func (m *mockAgentAdapter) ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error) {
	return &ExecutionResult{}, nil
}

func (m *mockAgentAdapter) CheckHealth(ctx context.Context) error {
	return nil
}

func (m *mockAgentAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
	return nil
}

func (m *mockAgentAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error {
	return nil
}

func (m *mockAgentAdapter) StopSession(sessionID string) error {
	return nil
}

func (m *mockAgentAdapter) GetSessionStatus(sessionID string) SessionStatus {
	return SessionStatusIdle
}

func (m *mockAgentAdapter) GetCurrentProcess() *exec.Cmd {
	return nil
}
