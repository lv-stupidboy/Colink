package im_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRetryableSend_SucceedsFirstTry(t *testing.T) {
	originalSleep := retrySleepFn
	retrySleepFn = func(context.Context, time.Duration) error {
		t.Fatal("sleep should not be called on first success")
		return nil
	}
	defer func() { retrySleepFn = originalSleep }()

	var calls int32
	result := RetryableSend(context.Background(), RetryConfig{MaxAttempts: 3}, zap.NewNop(), func() SendResult {
		atomic.AddInt32(&calls, 1)
		return SendResult{OK: true}
	})

	if !result.OK {
		t.Fatalf("expected success, got %+v", result)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryableSend_RetriesServerErrorThenSucceeds(t *testing.T) {
	originalSleep := retrySleepFn
	retrySleepFn = func(context.Context, time.Duration) error { return nil }
	defer func() { retrySleepFn = originalSleep }()

	var calls int32
	result := RetryableSend(context.Background(), RetryConfig{MaxAttempts: 3}, zap.NewNop(), func() SendResult {
		attempt := atomic.AddInt32(&calls, 1)
		if attempt == 1 {
			return SendResult{OK: false, HTTPStatus: 500, Error: "server error"}
		}
		return SendResult{OK: true}
	})

	if !result.OK {
		t.Fatalf("expected success on retry, got %+v", result)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRetryableSend_GivesUpAfterMaxAttempts(t *testing.T) {
	originalSleep := retrySleepFn
	retrySleepFn = func(context.Context, time.Duration) error { return nil }
	defer func() { retrySleepFn = originalSleep }()

	const maxAttempts = 3
	var calls int32
	result := RetryableSend(context.Background(), RetryConfig{MaxAttempts: maxAttempts}, zap.NewNop(), func() SendResult {
		atomic.AddInt32(&calls, 1)
		return SendResult{OK: false, HTTPStatus: 503, Error: "service unavailable"}
	})

	if result.OK {
		t.Fatalf("expected failure after max attempts, got %+v", result)
	}
	if calls != maxAttempts {
		t.Fatalf("expected %d calls, got %d", maxAttempts, calls)
	}
}

func TestRetryableSend_DoesNotRetryClientError(t *testing.T) {
	originalSleep := retrySleepFn
	retrySleepFn = func(context.Context, time.Duration) error {
		t.Fatal("sleep should not be called for non-retryable error")
		return nil
	}
	defer func() { retrySleepFn = originalSleep }()

	var calls int32
	result := RetryableSend(context.Background(), RetryConfig{MaxAttempts: 3}, zap.NewNop(), func() SendResult {
		atomic.AddInt32(&calls, 1)
		return SendResult{OK: false, HTTPStatus: 400, Error: "bad request"}
	})

	if result.OK {
		t.Fatalf("expected failure, got %+v", result)
	}
	if calls != 1 {
		t.Fatalf("expected single call for client error, got %d", calls)
	}
}

func TestRetryableSend_DoesNotRetryParseError(t *testing.T) {
	originalSleep := retrySleepFn
	retrySleepFn = func(context.Context, time.Duration) error {
		t.Fatal("sleep should not be called for parse error")
		return nil
	}
	defer func() { retrySleepFn = originalSleep }()

	var calls int32
	result := RetryableSend(context.Background(), RetryConfig{MaxAttempts: 3}, zap.NewNop(), func() SendResult {
		atomic.AddInt32(&calls, 1)
		return SendResult{OK: false, Error: "failed to parse json"}
	})

	if result.OK {
		t.Fatalf("expected failure, got %+v", result)
	}
	if calls != 1 {
		t.Fatalf("expected single call for parse error, got %d", calls)
	}
}

func TestRetryableSend_BackoffIncreasesBetweenAttempts(t *testing.T) {
	originalSleep := retrySleepFn
	originalJitter := retryJitterFn
	defer func() {
		retrySleepFn = originalSleep
		retryJitterFn = originalJitter
	}()

	delays := make([]time.Duration, 0, 4)
	retrySleepFn = func(_ context.Context, d time.Duration) error {
		delays = append(delays, d)
		return nil
	}
	retryJitterFn = func(time.Duration) time.Duration { return 0 }

	_ = RetryableSend(context.Background(), RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		JitterMax:   1 * time.Millisecond,
	}, zap.NewNop(), func() SendResult {
		return SendResult{OK: false, HTTPStatus: 500, Error: "server error"}
	})

	if len(delays) != 2 {
		t.Fatalf("expected 2 delay intervals, got %d", len(delays))
	}
	if delays[1] <= delays[0] {
		t.Fatalf("expected backoff to increase, got delays %v", delays)
	}
}

func TestRetryableSend_ContextCancellationStopsRetryLoop(t *testing.T) {
	originalSleep := retrySleepFn
	originalJitter := retryJitterFn
	defer func() {
		retrySleepFn = originalSleep
		retryJitterFn = originalJitter
	}()

	ctx, cancel := context.WithCancel(context.Background())

	var calls int32
	retryJitterFn = func(time.Duration) time.Duration { return 0 }
	retrySleepFn = func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}

	result := RetryableSend(ctx, RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		JitterMax:   0,
	}, zap.NewNop(), func() SendResult {
		atomic.AddInt32(&calls, 1)
		return SendResult{OK: false, HTTPStatus: 500, Error: "server error"}
	})

	if result.Error != context.Canceled.Error() {
		t.Fatalf("expected context canceled error, got %+v", result)
	}
	if calls != 1 {
		t.Fatalf("expected retry loop to stop after cancellation, got %d calls", calls)
	}
}
