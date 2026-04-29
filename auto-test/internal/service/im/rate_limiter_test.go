package im_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRateLimiter_Defaults(t *testing.T) {
	rl := NewRateLimiter(0, 0)
	if rl.maxMessages != 20 {
		t.Errorf("expected maxMessages=20, got %d", rl.maxMessages)
	}
	if rl.window != 60*time.Second {
		t.Errorf("expected window=60s, got %v", rl.window)
	}
}

func TestNewRateLimiter_CustomValues(t *testing.T) {
	rl := NewRateLimiter(10, 30*time.Second)
	if rl.maxMessages != 10 {
		t.Errorf("expected maxMessages=10, got %d", rl.maxMessages)
	}
	if rl.window != 30*time.Second {
		t.Errorf("expected window=30s, got %v", rl.window)
	}
}

func TestAcquire_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Second)

	for i := 0; i < 3; i++ {
		done := make(chan bool)
		go func() {
			rl.Acquire("chat1")
			done <- true
		}()

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Acquire blocked when under limit (iteration %d)", i)
		}
	}
}

func TestAcquire_AtLimit_Blocks(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)

	rl.Acquire("chat1")
	rl.Acquire("chat1")

	start := time.Now()
	done := make(chan bool)
	go func() {
		rl.Acquire("chat1")
		done <- true
	}()

	select {
	case <-done:
		elapsed := time.Since(start)
		if elapsed < 80*time.Millisecond {
			t.Errorf("Acquire returned too quickly: %v", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Acquire blocked indefinitely")
	}
}

func TestTryAcquire_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Second)

	for i := 0; i < 3; i++ {
		if !rl.TryAcquire("chat1") {
			t.Errorf("TryAcquire failed when under limit (iteration %d)", i)
		}
	}
}

func TestTryAcquire_AtLimit(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Second)

	rl.TryAcquire("chat1")
	rl.TryAcquire("chat1")

	if rl.TryAcquire("chat1") {
		t.Error("TryAcquire succeeded when at limit")
	}
}

func TestTryAcquire_NonBlocking(t *testing.T) {
	rl := NewRateLimiter(1, 10*time.Second)
	rl.TryAcquire("chat1")

	start := time.Now()
	rl.TryAcquire("chat1")
	elapsed := time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Errorf("TryAcquire took too long: %v", elapsed)
	}
}

func TestIndependentBuckets(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Second)

	rl.Acquire("chat1")
	rl.Acquire("chat1")

	if !rl.TryAcquire("chat2") {
		t.Error("chat2 should have independent bucket")
	}
	if !rl.TryAcquire("chat2") {
		t.Error("chat2 should have independent bucket")
	}
	if rl.TryAcquire("chat2") {
		t.Error("chat2 should be at limit now")
	}
}

func TestWindowExpiry(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)

	rl.Acquire("chat1")
	rl.Acquire("chat1")

	if rl.TryAcquire("chat1") {
		t.Error("should be at limit")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.TryAcquire("chat1") {
		t.Error("should allow after window expiry")
	}
}

func TestWaitTime_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Second)

	rl.Acquire("chat1")
	wait := rl.WaitTime("chat1")

	if wait != 0 {
		t.Errorf("expected wait=0 when under limit, got %v", wait)
	}
}

func TestWaitTime_AtLimit(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)

	rl.Acquire("chat1")
	rl.Acquire("chat1")

	wait := rl.WaitTime("chat1")
	if wait <= 0 || wait > 100*time.Millisecond {
		t.Errorf("expected wait between 0 and 100ms, got %v", wait)
	}
}

func TestWaitTime_AfterExpiry(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)

	rl.Acquire("chat1")
	rl.Acquire("chat1")

	time.Sleep(60 * time.Millisecond)

	wait := rl.WaitTime("chat1")
	if wait != 0 {
		t.Errorf("expected wait=0 after expiry, got %v", wait)
	}
}

func TestThreadSafety_ConcurrentAcquire(t *testing.T) {
	rl := NewRateLimiter(100, 1*time.Second)
	var wg sync.WaitGroup
	var successCount int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.Acquire("chat1")
			atomic.AddInt32(&successCount, 1)
		}()
	}

	wg.Wait()

	if successCount != 50 {
		t.Errorf("expected 50 successful acquires, got %d", successCount)
	}
}

func TestThreadSafety_ConcurrentTryAcquire(t *testing.T) {
	rl := NewRateLimiter(25, 1*time.Second)
	var wg sync.WaitGroup
	var successCount int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.TryAcquire("chat1") {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != 25 {
		t.Errorf("expected 25 successful tries, got %d", successCount)
	}
}

func TestThreadSafety_MixedOperations(t *testing.T) {
	rl := NewRateLimiter(50, 1*time.Second)
	var wg sync.WaitGroup
	var acquireCount int32
	var tryCount int32

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.Acquire("chat1")
			atomic.AddInt32(&acquireCount, 1)
		}()
	}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.TryAcquire("chat1") {
				atomic.AddInt32(&tryCount, 1)
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rl.WaitTime("chat1")
		}()
	}

	wg.Wait()

	if acquireCount != 30 {
		t.Errorf("expected 30 acquires, got %d", acquireCount)
	}
	if tryCount > 50 {
		t.Errorf("expected at most 50 tries, got %d", tryCount)
	}
}

func TestMultipleChatIDs_ThreadSafe(t *testing.T) {
	rl := NewRateLimiter(10, 1*time.Second)
	var wg sync.WaitGroup
	results := make(map[string]int32)
	var mu sync.Mutex

	for chatID := 0; chatID < 5; chatID++ {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				chatIDStr := "chat" + string(rune(id))
				rl.Acquire(chatIDStr)
				mu.Lock()
				results[chatIDStr]++
				mu.Unlock()
			}(chatID)
		}
	}

	wg.Wait()

	for chatID := 0; chatID < 5; chatID++ {
		chatIDStr := "chat" + string(rune(chatID))
		if results[chatIDStr] != 10 {
			t.Errorf("expected 10 acquires for %s, got %d", chatIDStr, results[chatIDStr])
		}
	}
}

func TestPruneExpired_Correctness(t *testing.T) {
	rl := NewRateLimiter(5, 100*time.Millisecond)

	rl.Acquire("chat1")
	rl.Acquire("chat1")

	time.Sleep(60 * time.Millisecond)

	rl.Acquire("chat1")

	time.Sleep(60 * time.Millisecond)

	if !rl.TryAcquire("chat1") {
		t.Error("should allow after old timestamps expire")
	}
}

func TestEdgeCase_ZeroWindow(t *testing.T) {
	rl := NewRateLimiter(1, 0)
	if rl.window != 60*time.Second {
		t.Errorf("expected default window, got %v", rl.window)
	}
}

func TestEdgeCase_NegativeValues(t *testing.T) {
	rl := NewRateLimiter(-5, -10*time.Second)
	if rl.maxMessages != 20 || rl.window != 60*time.Second {
		t.Error("expected defaults for negative values")
	}
}
