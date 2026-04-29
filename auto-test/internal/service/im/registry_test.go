package im_test

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if registry.factories == nil {
		t.Fatal("factories map not initialized")
	}
}

func TestRegister(t *testing.T) {
	registry := NewRegistry()

	factory := func(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error) {
		return nil, nil
	}

	registry.Register("test", factory)

	registry.mu.RLock()
	_, exists := registry.factories["test"]
	registry.mu.RUnlock()

	if !exists {
		t.Fatal("factory not registered")
	}
}

func TestRegisterPanicOnNilFactory(t *testing.T) {
	registry := NewRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil factory")
		}
	}()

	registry.Register("test", nil)
}

func TestRegisterPanicOnDuplicate(t *testing.T) {
	registry := NewRegistry()
	factory := func(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error) {
		return nil, nil
	}

	registry.Register("test", factory)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate registration")
		}
	}()

	registry.Register("test", factory)
}

func TestCreateUnregisteredPlatform(t *testing.T) {
	registry := NewRegistry()
	logger := zap.NewNop()
	cfg := IMPlatformConfig{Platform: "unknown"}

	_, err := registry.Create(cfg, logger)
	if err == nil {
		t.Fatal("expected error for unregistered platform")
	}
}

func TestCreateRegisteredPlatform(t *testing.T) {
	registry := NewRegistry()
	logger := zap.NewNop()

	mockAdapter := &mockAdapter{platform: "test"}
	factory := func(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error) {
		return mockAdapter, nil
	}

	registry.Register("test", factory)

	adapter, err := registry.Create(IMPlatformConfig{Platform: "test"}, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter != mockAdapter {
		t.Fatal("returned adapter does not match")
	}
}

func TestMustCreateSuccess(t *testing.T) {
	registry := NewRegistry()
	logger := zap.NewNop()

	mockAdapter := &mockAdapter{platform: "test"}
	factory := func(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error) {
		return mockAdapter, nil
	}

	registry.Register("test", factory)

	adapter := registry.MustCreate(IMPlatformConfig{Platform: "test"}, logger)
	if adapter != mockAdapter {
		t.Fatal("returned adapter does not match")
	}
}

func TestMustCreatePanic(t *testing.T) {
	registry := NewRegistry()
	logger := zap.NewNop()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unregistered platform")
		}
	}()

	registry.MustCreate(IMPlatformConfig{Platform: "unknown"}, logger)
}

func TestNewFeishuAdapterFactory(t *testing.T) {
	factory := NewFeishuAdapterFactory()
	if factory == nil {
		t.Fatal("NewFeishuAdapterFactory returned nil")
	}
}

func TestNewFeishuAdapterFactoryMissingPath(t *testing.T) {
	factory := NewFeishuAdapterFactory()
	logger := zap.NewNop()
	cfg := IMPlatformConfig{Platform: "feishu", LarkCLIPath: ""}

	_, err := factory(cfg, logger)
	if err == nil {
		t.Fatal("expected error for missing lark CLI path")
	}
}

type mockAdapter struct {
	platform string
}

func (m *mockAdapter) Platform() string {
	return m.platform
}

func (m *mockAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
	return SendResult{}
}

func (m *mockAdapter) SendCard(ctx context.Context, chatID, cardJSON string) SendResult {
	return SendResult{}
}

func (m *mockAdapter) ReplyText(ctx context.Context, chatID, messageID, text string) SendResult {
	return SendResult{}
}

func (m *mockAdapter) CreateStreamingCard(ctx context.Context, chatID string, agentName string) (cardID string, err error) {
	return "", nil
}

func (m *mockAdapter) UpdateStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	return nil
}

func (m *mockAdapter) FinalizeStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	return nil
}

func (m *mockAdapter) CheckHealth(ctx context.Context) error {
	return nil
}

func (m *mockAdapter) MaxMessageLength() int {
	return 4096
}
