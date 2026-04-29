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
	createEntityErr      error
	sendEntityMsgErr     error
	updateElementErr     error
	setStreamingErr      error
	lastChatID           string
	lastText             string
	lastCardJSON         string
	lastMsgID            string
	lastCardID           string
	lastContent          string
	lastElementID        string
	lastSequence         int
	lastStreamingEnabled bool
	updateElementCalls   int
	setStreamingCalls    int
	createEntityCalls    int
	sendEntityMsgCalls   int
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

func (m *mockLarkCLIClient) CreateStreamingCardEntity(ctx context.Context, title, elementID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createEntityCalls++
	if m.createEntityErr != nil {
		return "", m.createEntityErr
	}
	return "test-card-id", nil
}

func (m *mockLarkCLIClient) SendCardEntityMessage(ctx context.Context, chatID, cardID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendEntityMsgCalls++
	m.lastChatID = chatID
	m.lastCardID = cardID
	if m.sendEntityMsgErr != nil {
		return "", m.sendEntityMsgErr
	}
	return "test-message-id", nil
}

func (m *mockLarkCLIClient) UpdateStreamingElement(ctx context.Context, cardID, elementID, content string, sequence int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateElementCalls++
	m.lastCardID = cardID
	m.lastElementID = elementID
	m.lastContent = content
	m.lastSequence = sequence
	return m.updateElementErr
}

func (m *mockLarkCLIClient) SetCardStreamingMode(ctx context.Context, cardID string, enabled bool, sequence int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setStreamingCalls++
	m.lastCardID = cardID
	m.lastStreamingEnabled = enabled
	m.lastSequence = sequence
	return m.setStreamingErr
}

func TestFeishuAdapterPlatform(t *testing.T) {
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, zap.NewNop())
	if got := adapter.Platform(); got != "feishu" {
		t.Errorf("Platform() = %q, want %q", got, "feishu")
	}
}

func TestFeishuAdapterSendText(t *testing.T) {
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	result := adapter.SendText(context.Background(), "chat123", "hello")

	if !result.OK {
		t.Errorf("SendText() OK = false, want true")
	}
	if mock.lastChatID != "chat123" {
		t.Errorf("SendText() chatID = %q, want %q", mock.lastChatID, "chat123")
	}
}

func TestFeishuAdapterSendTextError(t *testing.T) {
	mock := &mockLarkCLIClient{sendTextErr: errors.New("network error")}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	result := adapter.SendText(context.Background(), "chat123", "hello")

	if result.OK {
		t.Errorf("SendText() OK = true, want false")
	}
}

func TestFeishuAdapterSendCard(t *testing.T) {
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	cardJSON := `{"config": {"wide_screen_mode": true}}`
	result := adapter.SendCard(context.Background(), "chat123", cardJSON)

	if !result.OK {
		t.Errorf("SendCard() OK = false, want true")
	}
	if mock.lastCardJSON != cardJSON {
		t.Errorf("SendCard() cardJSON mismatch")
	}
}

func TestFeishuAdapterSendCardError(t *testing.T) {
	mock := &mockLarkCLIClient{sendCardErr: errors.New("invalid card")}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	result := adapter.SendCard(context.Background(), "chat123", "{}")
	if result.OK {
		t.Errorf("SendCard() OK = true, want false")
	}
}

func TestFeishuAdapterReplyText(t *testing.T) {
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	result := adapter.ReplyText(context.Background(), "chat123", "msg456", "reply text")

	if !result.OK {
		t.Errorf("ReplyText() OK = false, want true")
	}
	if mock.lastMsgID != "msg456" {
		t.Errorf("ReplyText() messageID = %q, want %q", mock.lastMsgID, "msg456")
	}
}

func TestFeishuAdapterReplyTextError(t *testing.T) {
	mock := &mockLarkCLIClient{replyErr: errors.New("not found")}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	result := adapter.ReplyText(context.Background(), "chat123", "msg456", "reply")
	if result.OK {
		t.Errorf("ReplyText() OK = true, want false")
	}
}

func TestFeishuAdapterCreateStreamingCard(t *testing.T) {
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	cardID, err := adapter.CreateStreamingCard(context.Background(), "chat123", "Agent")

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

	state := val.(*streamingCardState)
	if state.cardID != cardID {
		t.Errorf("state.cardID = %q, want %q", state.cardID, cardID)
	}
	if state.messageID != "test-message-id" {
		t.Errorf("state.messageID = %q, want %q", state.messageID, "test-message-id")
	}
	if state.sequence < 2 {
		t.Errorf("state.sequence = %d, want >= 2 (after enable streaming)", state.sequence)
	}
}

func TestFeishuAdapterCreateStreamingCardError(t *testing.T) {
	mock := &mockLarkCLIClient{createEntityErr: errors.New("creation failed")}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	_, err := adapter.CreateStreamingCard(context.Background(), "chat123", "Agent")
	if err == nil {
		t.Fatal("CreateStreamingCard() err = nil, want error")
	}
}

func TestFeishuAdapterUpdateStreamingCardImmediateFlush(t *testing.T) {
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	cardID, _ := adapter.CreateStreamingCard(context.Background(), "chat123", "Agent")
	baseSeq := mock.lastSequence

	// Wait for throttle to elapse since card creation
	time.Sleep(250 * time.Millisecond)

	err := adapter.UpdateStreamingCard(context.Background(), cardID, "content 1", 0)
	if err != nil {
		t.Fatalf("UpdateStreamingCard() err = %v, want nil", err)
	}

	// Queue is async; wait for it to execute
	time.Sleep(250 * time.Millisecond)

	if mock.updateElementCalls != 1 {
		t.Errorf("updateElementCalls = %d, want 1 (flushed via queue)", mock.updateElementCalls)
	}
	if mock.lastContent != "content 1" {
		t.Errorf("lastContent = %q, want %q", mock.lastContent, "content 1")
	}
	if mock.lastElementID != streamingElementID {
		t.Errorf("lastElementID = %q, want %q", mock.lastElementID, streamingElementID)
	}
	if mock.lastSequence <= baseSeq {
		t.Errorf("lastSequence = %d, want > %d", mock.lastSequence, baseSeq)
	}
}

func TestFeishuAdapterUpdateStreamingCardThrottled(t *testing.T) {
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	cardID, _ := adapter.CreateStreamingCard(context.Background(), "chat123", "Agent")

	err := adapter.UpdateStreamingCard(context.Background(), cardID, "pending", 0)
	if err != nil {
		t.Fatalf("UpdateStreamingCard() err = %v, want nil", err)
	}

	// Queue is async; first call hasn't executed yet (throttled)
	time.Sleep(50 * time.Millisecond)
	if mock.updateElementCalls != 0 {
		t.Errorf("updateElementCalls = %d, want 0 (not yet flushed)", mock.updateElementCalls)
	}

	// Wait for throttle to pass and queue to execute
	time.Sleep(250 * time.Millisecond)

	if mock.updateElementCalls != 1 {
		t.Errorf("updateElementCalls = %d, want 1 (after throttle)", mock.updateElementCalls)
	}
	if mock.lastContent != "pending" {
		t.Errorf("lastContent = %q, want %q", mock.lastContent, "pending")
	}
}

func TestFeishuAdapterUpdateStreamingCardMultipleThrottled(t *testing.T) {
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	cardID, _ := adapter.CreateStreamingCard(context.Background(), "chat123", "Agent")

	adapter.UpdateStreamingCard(context.Background(), cardID, "content 1", 0)
	time.Sleep(50 * time.Millisecond)
	adapter.UpdateStreamingCard(context.Background(), cardID, "content 2", 0)
	time.Sleep(50 * time.Millisecond)
	adapter.UpdateStreamingCard(context.Background(), cardID, "content 3", 0)

	// All three enqueue updates; they execute serially via queue
	time.Sleep(800 * time.Millisecond)

	// Each update is sent individually through the queue (not batched)
	// because each queued goroutine reads accumulated text at execution time
	if mock.updateElementCalls < 1 {
		t.Errorf("updateElementCalls = %d, want >= 1", mock.updateElementCalls)
	}
	// Final content should have all three accumulated
	if !contains(mock.lastContent, "content 1") {
		t.Errorf("lastContent should contain 'content 1', got %q", mock.lastContent)
	}
	if !contains(mock.lastContent, "content 3") {
		t.Errorf("lastContent should contain 'content 3', got %q", mock.lastContent)
	}
}

func TestFeishuAdapterFinalizeStreamingCard(t *testing.T) {
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	cardID, _ := adapter.CreateStreamingCard(context.Background(), "chat123", "Agent")
	baseStreamingCalls := mock.setStreamingCalls

	adapter.UpdateStreamingCard(context.Background(), cardID, "pending", 0)

	err := adapter.FinalizeStreamingCard(context.Background(), cardID, "final", 0)
	if err != nil {
		t.Fatalf("FinalizeStreamingCard() err = %v, want nil", err)
	}

	// Finalize waits for queued updates to drain, then sends its own update
	// So we get: 1 from the queued update + 1 from finalize = 2
	if mock.updateElementCalls < 1 {
		t.Errorf("updateElementCalls = %d, want >= 1", mock.updateElementCalls)
	}
	if !contains(mock.lastContent, "pending") {
		t.Errorf("lastContent should contain 'pending', got %q", mock.lastContent)
	}
	if !contains(mock.lastContent, "final") {
		t.Errorf("lastContent should contain 'final', got %q", mock.lastContent)
	}

	if mock.setStreamingCalls != baseStreamingCalls+1 {
		t.Errorf("setStreamingCalls = %d, want %d (disable after finalize)", mock.setStreamingCalls, baseStreamingCalls+1)
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
	mock := &mockLarkCLIClient{}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	cardID, _ := adapter.CreateStreamingCard(context.Background(), "chat123", "Agent")

	adapter.UpdateStreamingCard(context.Background(), cardID, "pending", 0)
	adapter.FinalizeStreamingCard(context.Background(), cardID, "final", 0)

	time.Sleep(250 * time.Millisecond)

	// Finalize drains the queue then sends its own update
	if mock.updateElementCalls < 1 {
		t.Errorf("updateElementCalls = %d, want >= 1", mock.updateElementCalls)
	}
}

func TestFeishuAdapterUpdateStreamingCardNotFound(t *testing.T) {
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, zap.NewNop())

	err := adapter.UpdateStreamingCard(context.Background(), "nonexistent", "content", 0)
	if err == nil {
		t.Fatal("UpdateStreamingCard() err = nil, want error for nonexistent card")
	}
}

func TestFeishuAdapterFinalizeStreamingCardNotFound(t *testing.T) {
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, zap.NewNop())

	err := adapter.FinalizeStreamingCard(context.Background(), "nonexistent", "content", 0)
	if err == nil {
		t.Fatal("FinalizeStreamingCard() err = nil, want error for nonexistent card")
	}
}

func TestFeishuAdapterCheckHealth(t *testing.T) {
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, zap.NewNop())

	if err := adapter.CheckHealth(context.Background()); err != nil {
		t.Errorf("CheckHealth() err = %v, want nil", err)
	}
}

func TestFeishuAdapterCheckHealthError(t *testing.T) {
	mock := &mockLarkCLIClient{healthErr: errors.New("lark-cli not available")}
	adapter := NewFeishuAdapter(mock, zap.NewNop())

	if err := adapter.CheckHealth(context.Background()); err == nil {
		t.Errorf("CheckHealth() err = nil, want error")
	}
}

func TestFeishuAdapterMaxMessageLength(t *testing.T) {
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, zap.NewNop())

	if got := adapter.MaxMessageLength(); got != 4000 {
		t.Errorf("MaxMessageLength() = %d, want 4000", got)
	}
}

func TestFeishuAdapterBuildSimpleCard(t *testing.T) {
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, zap.NewNop())

	card := adapter.buildSimpleCard("Test message", "blue")
	if card == "" {
		t.Errorf("buildSimpleCard() returned empty string")
	}
	if !contains(card, "Test message") {
		t.Errorf("buildSimpleCard() missing content")
	}
}

func TestFeishuAdapterBuildCompletionCard(t *testing.T) {
	adapter := NewFeishuAdapter(&mockLarkCLIClient{}, zap.NewNop())

	for _, status := range []string{"completed", "failed", "stopped"} {
		card := adapter.buildCompletionCard(status)
		if card == "" {
			t.Errorf("buildCompletionCard(%q) returned empty string", status)
		}
	}
}

func TestFeishuAdapterImplementsInterface(t *testing.T) {
	var _ IMAdapter = NewFeishuAdapter(&mockLarkCLIClient{}, zap.NewNop())
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
