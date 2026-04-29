package im_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionLockNoContention(t *testing.T) {
	lock := NewSessionLock()
	sessionID := "session1"

	release := lock.Acquire(sessionID)
	release()

	// Should acquire immediately without blocking
	done := make(chan struct{})
	go func() {
		release2 := lock.Acquire(sessionID)
		release2()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second acquire blocked unexpectedly")
	}
}

func TestSessionLockSerialization(t *testing.T) {
	lock := NewSessionLock()
	sessionID := "session1"

	var order []int
	var mu sync.Mutex

	// First goroutine acquires and holds
	release1 := lock.Acquire(sessionID)
	go func() {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
		release1()
	}()

	// Second goroutine should block until first releases
	time.Sleep(10 * time.Millisecond) // Ensure first is holding
	release2 := lock.Acquire(sessionID)
	mu.Lock()
	order = append(order, 2)
	mu.Unlock()
	release2()

	// Verify order: first must complete before second acquires
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("expected order [1, 2], got %v", order)
	}
}

func TestSessionLockDifferentSessions(t *testing.T) {
	lock := NewSessionLock()

	var wg sync.WaitGroup
	var counter atomic.Int32

	// Two sessions should run in parallel
	for session := 0; session < 2; session++ {
		wg.Add(1)
		go func(s int) {
			defer wg.Done()
			sessionID := "session" + string(rune(s))
			release := lock.Acquire(sessionID)
			counter.Add(1)
			time.Sleep(50 * time.Millisecond)
			counter.Add(-1)
			release()
		}(session)
	}

	// Give goroutines time to both acquire
	time.Sleep(10 * time.Millisecond)

	// Both should be running concurrently (counter should be 2)
	if counter.Load() != 2 {
		t.Fatalf("expected 2 concurrent sessions, got %d", counter.Load())
	}

	wg.Wait()
}

func TestSessionLockChainCleanup(t *testing.T) {
	lock := NewSessionLock()
	sessionID := "session1"

	// Acquire and release multiple times
	for i := 0; i < 5; i++ {
		release := lock.Acquire(sessionID)
		release()
	}

	// Map should be empty after all releases
	lock.mu.Lock()
	if len(lock.chains) != 0 {
		t.Fatalf("expected empty chains map, got %d entries", len(lock.chains))
	}
	lock.mu.Unlock()
}

func TestSessionLockHighContention(t *testing.T) {
	lock := NewSessionLock()
	sessionID := "session1"

	var wg sync.WaitGroup
	var counter atomic.Int32
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := lock.Acquire(sessionID)
			counter.Add(1)
			counter.Add(-1)
			release()
		}()
	}

	wg.Wait()

	// All goroutines should have completed
	if counter.Load() != 0 {
		t.Fatalf("expected counter to be 0, got %d", counter.Load())
	}

	// Map should be empty
	lock.mu.Lock()
	if len(lock.chains) != 0 {
		t.Fatalf("expected empty chains map, got %d entries", len(lock.chains))
	}
	lock.mu.Unlock()
}
