package im

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeIMAdapter struct {
	maxMessageLength int
	textResults      []SendResult
	textCalls        []string
	cardCalls        int
}

func (f *fakeIMAdapter) Platform() string { return "fake" }

func (f *fakeIMAdapter) SendText(_ context.Context, _ string, text string) SendResult {
	f.textCalls = append(f.textCalls, text)
	if len(f.textResults) == 0 {
		return SendResult{OK: true}
	}
	result := f.textResults[0]
	f.textResults = f.textResults[1:]
	return result
}

func (f *fakeIMAdapter) SendCard(context.Context, string, string) SendResult {
	f.cardCalls++
	return SendResult{OK: true}
}

func (f *fakeIMAdapter) ReplyText(context.Context, string, string, string) SendResult {
	return SendResult{OK: true}
}

func (f *fakeIMAdapter) CreateStreamingCard(context.Context, string, string) (string, error) {
	return "", nil
}

func (f *fakeIMAdapter) UpdateStreamingCard(context.Context, string, string, int) error { return nil }

func (f *fakeIMAdapter) FinalizeStreamingCard(context.Context, string, string, int) error { return nil }

func (f *fakeIMAdapter) CheckHealth(context.Context) error { return nil }

func (f *fakeIMAdapter) MaxMessageLength() int {
	if f.maxMessageLength <= 0 {
		return 1000
	}
	return f.maxMessageLength
}

func TestChunkText_ShortMessage(t *testing.T) {
	chunks := chunkText("hello", 20)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "hello" {
		t.Fatalf("expected unchanged text, got %q", chunks[0])
	}
}

func TestChunkText_LongMessage_SplitsAtNewline(t *testing.T) {
	text := "12345\n67890ABCDEF"
	chunks := chunkText(text, 10)

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %#v", len(chunks), chunks)
	}
	if chunks[0] != "12345" {
		t.Fatalf("expected first chunk split at newline, got %q", chunks[0])
	}
}

func TestChunkText_NoNewlines_SplitsAtMaxLen(t *testing.T) {
	chunks := chunkText("abcdefghijk", 4)
	expected := []string{"abcd", "efgh", "ijk"}

	if len(chunks) != len(expected) {
		t.Fatalf("expected %d chunks, got %d", len(expected), len(chunks))
	}
	for i := range expected {
		if chunks[i] != expected[i] {
			t.Fatalf("chunk[%d]=%q, want %q", i, chunks[i], expected[i])
		}
	}
}

func TestDeliveryService_DedupSkip(t *testing.T) {
	adapter := &fakeIMAdapter{maxMessageLength: 50}
	cache := NewDedupCache(10)
	cache.IsDuplicate("dup-key")

	svc := NewDeliveryService(adapter, DefaultRetryConfig(), NewRateLimiter(10, time.Minute), cache, zap.NewNop())
	result := svc.DeliverText(context.Background(), "chat-1", "hello", "dup-key")

	if !result.OK {
		t.Fatalf("expected dedup skip success, got %+v", result)
	}
	if len(adapter.textCalls) != 0 {
		t.Fatalf("expected no send call for duplicate, got %d", len(adapter.textCalls))
	}
}

func TestDeliveryService_RateLimit(t *testing.T) {
	adapter := &fakeIMAdapter{maxMessageLength: 50}
	rl := NewRateLimiter(1, time.Minute)
	rl.TryAcquire("chat-1")

	svc := NewDeliveryService(adapter, DefaultRetryConfig(), rl, NewDedupCache(10), zap.NewNop())
	result := svc.DeliverText(context.Background(), "chat-1", "hello", "k1")

	if result.OK {
		t.Fatalf("expected rate-limited failure, got %+v", result)
	}
	if result.Category != ErrCategoryRateLimit {
		t.Fatalf("expected rate-limit category, got %s", result.Category.String())
	}
	if len(adapter.textCalls) != 0 {
		t.Fatalf("expected no send when rate limited, got %d", len(adapter.textCalls))
	}
}

func TestDeliveryService_ChunkedDelivery(t *testing.T) {
	originalSleep := deliverySleepFn
	deliverySleepFn = func(context.Context, time.Duration) error { return nil }
	defer func() { deliverySleepFn = originalSleep }()

	adapter := &fakeIMAdapter{maxMessageLength: 4}
	svc := NewDeliveryService(adapter, DefaultRetryConfig(), NewRateLimiter(10, time.Minute), NewDedupCache(10), zap.NewNop())

	result := svc.DeliverText(context.Background(), "chat-1", "abcdefghijk", "k1")
	if !result.OK {
		t.Fatalf("expected chunked delivery success, got %+v", result)
	}

	expected := []string{"abcd", "efgh", "ijk"}
	if len(adapter.textCalls) != len(expected) {
		t.Fatalf("expected %d send calls, got %d", len(expected), len(adapter.textCalls))
	}
	for i := range expected {
		if adapter.textCalls[i] != expected[i] {
			t.Fatalf("send chunk[%d]=%q, want %q", i, adapter.textCalls[i], expected[i])
		}
	}
}

func TestDeliveryService_RetryOnError(t *testing.T) {
	originalSleep := retrySleepFn
	retrySleepFn = func(context.Context, time.Duration) error { return nil }
	defer func() { retrySleepFn = originalSleep }()

	adapter := &fakeIMAdapter{
		maxMessageLength: 100,
		textResults: []SendResult{
			{OK: false, HTTPStatus: 500, Error: "server error"},
			{OK: true},
		},
	}
	svc := NewDeliveryService(adapter, RetryConfig{MaxAttempts: 2}, NewRateLimiter(10, time.Minute), NewDedupCache(10), zap.NewNop())

	result := svc.DeliverText(context.Background(), "chat-1", "hello", "k1")
	if !result.OK {
		t.Fatalf("expected retry success, got %+v", result)
	}
	if result.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", result.Attempts)
	}
	if len(adapter.textCalls) != 2 {
		t.Fatalf("expected 2 send calls, got %d", len(adapter.textCalls))
	}
}
