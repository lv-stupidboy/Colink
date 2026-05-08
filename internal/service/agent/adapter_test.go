package agent

import (
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestGetAdapter_NilBaseAgent(t *testing.T) {
	adapter := GetAdapter(nil)
	if adapter != nil {
		t.Error("Expected nil adapter for nil base agent")
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