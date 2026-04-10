package im

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

type mockLarkCLIClient struct {
	mu                   sync.Mutex
	sendTextErr          error
	sendCardErr          error
	replyErr             error
	healthErr            error
	createCardErr        error
	updateCardErr        error
	setStreamingErr      error
	lastChatID           string
	lastText             string
	lastCardJSON         string
	lastMsgID            string
	lastCardID           string
	lastContent          string
	lastSequence         int
	lastStreamingEnabled bool
	updateCardCalls      int
	setStreamingCalls    int
}

func (m *mockLarkCLIClient) SendTextMessage(ctx context.Context, chatID, text string) error {
	m.lastChatID = chatID
	m.lastText = text
	return m.sendTextErr
}

func (m *mockLarkCLIClient) SendCardMessage(ctx context.Context, chatID, cardJSON string) error {
	m.lastChatID = chatID
	m.lastCardJSON = cardJSON
	return m.sendCardErr
}

func (m *mockLarkCLIClient) ReplyMessage(ctx context.Context, chatID, messageID, text string) error {
	m.lastChatID = chatID
	m.lastMsgID = messageID
	m.lastText = text
	return m.replyErr
}

func (m *mockLarkCLIClient) CheckHealth(ctx context.Context) error {
	return m.healthErr
}

func (m *mockLarkCLIClient) CreateCard(ctx context.Context, chatID string) (cardID, messageID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createCardErr != nil {
		return "", "", m.createCardErr
	}
	return "test-card-id", "test-message-id", nil
}

func (m *mockLarkCLIClient) UpdateCardContent(ctx context.Context, cardID, content string, sequence int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCardCalls++
	m.lastCardID = cardID
	m.lastContent = content
	m.lastSequence = sequence
	return m.updateCardErr
}

func (m *mockLarkCLIClient) SetStreamingMode(ctx context.Context, cardID string, enabled bool, sequence int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setStreamingCalls++
	m.lastStreamingEnabled = enabled
	return m.setStreamingErr
}

func TestFeishuAdapterPlatform(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, logger)

	if got := adapter.Platform(); got != "feishu" {
		t.Errorf("Platform() = %q, want %q", got, "feishu")
	}
}

func TestFeishuAdapterSendText(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	result := adapter.SendText(ctx, "chat123", "hello")

	if !result.OK {
		t.Errorf("SendText() OK = false, want true")
	}
	if result.Error != "" {
		t.Errorf("SendText() Error = %q, want empty", result.Error)
	}
	if mock.lastChatID != "chat123" {
		t.Errorf("SendText() chatID = %q, want %q", mock.lastChatID, "chat123")
	}
	if mock.lastText != "hello" {
		t.Errorf("SendText() text = %q, want %q", mock.lastText, "hello")
	}
}

func TestFeishuAdapterSendTextError(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{
		sendTextErr: errors.New("network error"),
	}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	result := adapter.SendText(ctx, "chat123", "hello")

	if result.OK {
		t.Errorf("SendText() OK = true, want false")
	}
	if result.Error == "" {
		t.Errorf("SendText() Error = empty, want non-empty")
	}
}

func TestFeishuAdapterSendCard(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	cardJSON := `{"config": {"wide_screen_mode": true}}`
	result := adapter.SendCard(ctx, "chat123", cardJSON)

	if !result.OK {
		t.Errorf("SendCard() OK = false, want true")
	}
	if result.Error != "" {
		t.Errorf("SendCard() Error = %q, want empty", result.Error)
	}
	if mock.lastCardJSON != cardJSON {
		t.Errorf("SendCard() cardJSON mismatch")
	}
}

func TestFeishuAdapterSendCardError(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{
		sendCardErr: errors.New("invalid card"),
	}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	result := adapter.SendCard(ctx, "chat123", "{}")

	if result.OK {
		t.Errorf("SendCard() OK = true, want false")
	}
	if result.Error == "" {
		t.Errorf("SendCard() Error = empty, want non-empty")
	}
}

func TestFeishuAdapterReplyText(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	result := adapter.ReplyText(ctx, "chat123", "msg456", "reply text")

	if !result.OK {
		t.Errorf("ReplyText() OK = false, want true")
	}
	if result.Error != "" {
		t.Errorf("ReplyText() Error = %q, want empty", result.Error)
	}
	if mock.lastMsgID != "msg456" {
		t.Errorf("ReplyText() messageID = %q, want %q", mock.lastMsgID, "msg456")
	}
}

func TestFeishuAdapterReplyTextError(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{
		replyErr: errors.New("message not found"),
	}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	result := adapter.ReplyText(ctx, "chat123", "msg456", "reply")

	if result.OK {
		t.Errorf("ReplyText() OK = true, want false")
	}
	if result.Error == "" {
		t.Errorf("ReplyText() Error = empty, want non-empty")
	}
}

func TestFeishuAdapterCreateStreamingCard(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	cardID, err := adapter.CreateStreamingCard(ctx, "chat123")

	if err != nil {
		t.Fatalf("CreateStreamingCard() err = %v, want nil", err)
	}
	if cardID != "test-card-id" {
		t.Errorf("CreateStreamingCard() cardID = %q, want %q", cardID, "test-card-id")
	}

	val, ok := adapter.cardStates.Load(cardID)
	if !ok {
		t.Fatal("card state not stored")
	}

	state := val.(*cardState)
	if state.cardID != cardID {
		t.Errorf("state.cardID = %q, want %q", state.cardID, cardID)
	}
	if state.messageID != "test-message-id" {
		t.Errorf("state.messageID = %q, want %q", state.messageID, "test-message-id")
	}
}

func TestFeishuAdapterCreateStreamingCardError(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{
		createCardErr: errors.New("creation failed"),
	}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	_, err := adapter.CreateStreamingCard(ctx, "chat123")

	if err == nil {
		t.Fatal("CreateStreamingCard() err = nil, want error")
	}
}

func TestFeishuAdapterUpdateStreamingCardImmediateFlush(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	cardID, _ := adapter.CreateStreamingCard(ctx, "chat123")

	time.Sleep(250 * time.Millisecond)

	err := adapter.UpdateStreamingCard(ctx, cardID, "content 1", 1)
	if err != nil {
		t.Fatalf("UpdateStreamingCard() err = %v, want nil", err)
	}

	if mock.updateCardCalls != 1 {
		t.Errorf("updateCardCalls = %d, want 1 (immediate flush)", mock.updateCardCalls)
	}
	if mock.lastContent != "content 1" {
		t.Errorf("lastContent = %q, want %q", mock.lastContent, "content 1")
	}
}

func TestFeishuAdapterUpdateStreamingCardThrottled(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	cardID, _ := adapter.CreateStreamingCard(ctx, "chat123")

	err := adapter.UpdateStreamingCard(ctx, cardID, "pending", 1)
	if err != nil {
		t.Fatalf("UpdateStreamingCard() err = %v, want nil", err)
	}

	if mock.updateCardCalls != 0 {
		t.Errorf("updateCardCalls = %d, want 0 (throttled)", mock.updateCardCalls)
	}

	time.Sleep(250 * time.Millisecond)

	if mock.updateCardCalls != 1 {
		t.Errorf("updateCardCalls = %d, want 1 (after throttle)", mock.updateCardCalls)
	}
	if mock.lastContent != "pending" {
		t.Errorf("lastContent = %q, want %q", mock.lastContent, "pending")
	}
}

func TestFeishuAdapterUpdateStreamingCardMultipleThrottled(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	cardID, _ := adapter.CreateStreamingCard(ctx, "chat123")

	adapter.UpdateStreamingCard(ctx, cardID, "content 1", 1)
	time.Sleep(50 * time.Millisecond)
	adapter.UpdateStreamingCard(ctx, cardID, "content 2", 2)
	time.Sleep(50 * time.Millisecond)
	adapter.UpdateStreamingCard(ctx, cardID, "content 3", 3)

	if mock.updateCardCalls != 0 {
		t.Errorf("updateCardCalls = %d, want 0 (all throttled)", mock.updateCardCalls)
	}

	time.Sleep(250 * time.Millisecond)

	if mock.updateCardCalls != 1 {
		t.Errorf("updateCardCalls = %d, want 1 (batched)", mock.updateCardCalls)
	}
	if mock.lastContent != "content 3" {
		t.Errorf("lastContent = %q, want %q", mock.lastContent, "content 3")
	}
	if mock.lastSequence != 3 {
		t.Errorf("lastSequence = %d, want 3", mock.lastSequence)
	}
}

func TestFeishuAdapterFinalizeStreamingCard(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	cardID, _ := adapter.CreateStreamingCard(ctx, "chat123")

	adapter.UpdateStreamingCard(ctx, cardID, "pending", 1)

	err := adapter.FinalizeStreamingCard(ctx, cardID, "final", 2)
	if err != nil {
		t.Fatalf("FinalizeStreamingCard() err = %v, want nil", err)
	}

	if mock.updateCardCalls != 1 {
		t.Errorf("updateCardCalls = %d, want 1 (finalize)", mock.updateCardCalls)
	}

	if mock.setStreamingCalls != 2 {
		t.Errorf("setStreamingCalls = %d, want 2 (enable + disable)", mock.setStreamingCalls)
	}

	if mock.lastStreamingEnabled {
		t.Error("lastStreamingEnabled = true, want false")
	}

	_, ok := adapter.cardStates.Load(cardID)
	if ok {
		t.Error("card state should be deleted after finalize")
	}
}

func TestFeishuAdapterFinalizeStreamingCardCancelsThrottle(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	cardID, _ := adapter.CreateStreamingCard(ctx, "chat123")

	adapter.UpdateStreamingCard(ctx, cardID, "pending", 1)
	adapter.FinalizeStreamingCard(ctx, cardID, "final", 2)

	time.Sleep(250 * time.Millisecond)

	if mock.updateCardCalls != 1 {
		t.Errorf("updateCardCalls = %d, want 1 (finalize only, throttle cancelled)", mock.updateCardCalls)
	}
}

func TestFeishuAdapterUpdateStreamingCardNotFound(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	err := adapter.UpdateStreamingCard(ctx, "nonexistent", "content", 1)

	if err == nil {
		t.Fatal("UpdateStreamingCard() err = nil, want error for nonexistent card")
	}
}

func TestFeishuAdapterFinalizeStreamingCardNotFound(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	err := adapter.FinalizeStreamingCard(ctx, "nonexistent", "content", 1)

	if err == nil {
		t.Fatal("FinalizeStreamingCard() err = nil, want error for nonexistent card")
	}
}

func TestFeishuAdapterCheckHealth(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	err := adapter.CheckHealth(ctx)

	if err != nil {
		t.Errorf("CheckHealth() err = %v, want nil", err)
	}
}

func TestFeishuAdapterCheckHealthError(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockLarkCLIClient{
		healthErr: errors.New("lark-cli not available"),
	}
	adapter := NewFeishuAdapter(mock, logger)

	ctx := context.Background()
	err := adapter.CheckHealth(ctx)

	if err == nil {
		t.Errorf("CheckHealth() err = nil, want error")
	}
}

func TestFeishuAdapterMaxMessageLength(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, logger)

	if got := adapter.MaxMessageLength(); got != 4000 {
		t.Errorf("MaxMessageLength() = %d, want 4000", got)
	}
}

func TestFeishuAdapterBuildSimpleCard(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, logger)

	card := adapter.buildSimpleCard("Test message", "blue")

	if card == "" {
		t.Errorf("buildSimpleCard() returned empty string")
	}
	if !contains(card, "Test message") {
		t.Errorf("buildSimpleCard() missing content")
	}
	if !contains(card, "blue") {
		t.Errorf("buildSimpleCard() missing color")
	}
}

func TestFeishuAdapterBuildCompletionCard(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, logger)

	tests := []struct {
		status    string
		wantIcon  string
		wantColor string
	}{
		{"completed", "✅", "green"},
		{"failed", "❌", "red"},
		{"stopped", "⏹️", "red"},
	}

	for _, tt := range tests {
		card := adapter.buildCompletionCard(tt.status)
		if card == "" {
			t.Errorf("buildCompletionCard(%q) returned empty string", tt.status)
		}
		if !contains(card, tt.wantIcon) {
			t.Errorf("buildCompletionCard(%q) missing icon %q", tt.status, tt.wantIcon)
		}
		if !contains(card, tt.wantColor) {
			t.Errorf("buildCompletionCard(%q) missing color %q", tt.status, tt.wantColor)
		}
	}
}

func TestFeishuAdapterImplementsInterface(t *testing.T) {
	logger := zap.NewNop()
	var _ IMAdapter = NewFeishuAdapter(&mockLarkCLIClient{}, logger)
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
