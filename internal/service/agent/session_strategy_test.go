package agent

import (
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
)

func TestGetSessionStrategyBuiltInsAndFallback(t *testing.T) {
	tests := []struct {
		name         string
		agentType    model.BaseAgentType
		wantResume   bool
		wantParent   string
		wantDuration time.Duration
	}{
		{name: "claude", agentType: "claude_code", wantResume: true, wantDuration: 7 * 24 * time.Hour},
		{name: "open code", agentType: "open_code", wantResume: true, wantDuration: 7 * 24 * time.Hour},
		{name: "code agent inherits opencode", agentType: "code_agent", wantResume: true, wantParent: "open_code", wantDuration: 7 * 24 * time.Hour},
		{name: "unknown", agentType: "unknown_agent", wantResume: false, wantDuration: 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSessionStrategy(tt.agentType)
			if got.UseNativeResume != tt.wantResume {
				t.Fatalf("UseNativeResume=%v, want %v", got.UseNativeResume, tt.wantResume)
			}
			if got.ParentType != tt.wantParent {
				t.Fatalf("ParentType=%q, want %q", got.ParentType, tt.wantParent)
			}
			if got.GetResumeExpiry() != tt.wantDuration {
				t.Fatalf("GetResumeExpiry=%v, want %v", got.GetResumeExpiry(), tt.wantDuration)
			}
		})
	}
}

func TestRegisterAgentTypeOverridesStrategy(t *testing.T) {
	agentType := model.BaseAgentType("custom_agent_for_test")

	RegisterAgentType(agentType, SessionStrategyConfig{
		UseNativeResume: true,
		ResumeExpiry:    12,
		ParentType:      "open_code",
	})

	got := GetSessionStrategy(agentType)
	if !got.UseNativeResume || got.ParentType != "open_code" {
		t.Fatalf("unexpected strategy: %#v", got)
	}
	if got.GetResumeExpiry() != 12*time.Hour {
		t.Fatalf("expected 12h expiry, got %v", got.GetResumeExpiry())
	}

	types := GetRegisteredTypes()
	for _, typ := range types {
		if typ == agentType {
			return
		}
	}
	t.Fatalf("registered type %s not found in %v", agentType, types)
}

func TestSessionStrategyDefaultExpiryForInvalidValues(t *testing.T) {
	if got := (SessionStrategyConfig{}).GetResumeExpiry(); got != 7*24*time.Hour {
		t.Fatalf("zero expiry should default to 7 days, got %v", got)
	}
	if got := (SessionStrategyConfig{ResumeExpiry: -1}).GetResumeExpiry(); got != 7*24*time.Hour {
		t.Fatalf("negative expiry should default to 7 days, got %v", got)
	}
}
