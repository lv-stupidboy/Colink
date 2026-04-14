package im

import (
	"sync"
	"time"
)

// RateLimiter implements a thread-safe sliding window rate limiter.
// It tracks message timestamps per chat ID and enforces rate limits.
type RateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*bucket
	maxMessages int
	window      time.Duration
}

// bucket holds timestamps for a single chat ID.
type bucket struct {
	timestamps []int64 // Unix milliseconds
}

// NewRateLimiter creates a new rate limiter with the given constraints.
// If maxMessages <= 0, defaults to 20.
// If window <= 0, defaults to 60 seconds.
func NewRateLimiter(maxMessages int, window time.Duration) *RateLimiter {
	if maxMessages <= 0 {
		maxMessages = 20
	}
	if window <= 0 {
		window = 60 * time.Second
	}

	return &RateLimiter{
		buckets:     make(map[string]*bucket),
		maxMessages: maxMessages,
		window:      window,
	}
}

// Acquire blocks until a message is allowed for the given chatID.
// It prunes expired timestamps, checks the limit, and sleeps if necessary.
func (rl *RateLimiter) Acquire(chatID string) {
	for {
		rl.mu.Lock()
		b := rl.getBucket(chatID)
		now := timeNowMs()

		// Prune expired timestamps
		rl.pruneExpired(b, now)

		// If under limit, add timestamp and return
		if len(b.timestamps) < rl.maxMessages {
			b.timestamps = append(b.timestamps, now)
			rl.mu.Unlock()
			return
		}

		// At limit: calculate wait time until oldest expires
		oldestExpiry := b.timestamps[0] + rl.window.Milliseconds()
		waitMs := oldestExpiry - now
		rl.mu.Unlock()

		// Sleep and retry
		if waitMs > 0 {
			time.Sleep(time.Duration(waitMs) * time.Millisecond)
		}
	}
}

// TryAcquire attempts to acquire a message slot without blocking.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) TryAcquire(chatID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b := rl.getBucket(chatID)
	now := timeNowMs()

	// Prune expired timestamps
	rl.pruneExpired(b, now)

	// Check if under limit
	if len(b.timestamps) < rl.maxMessages {
		b.timestamps = append(b.timestamps, now)
		return true
	}

	return false
}

// WaitTime returns the duration until the next message is allowed.
// Returns 0 if a message can be sent immediately.
func (rl *RateLimiter) WaitTime(chatID string) time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b := rl.getBucket(chatID)
	now := timeNowMs()

	// Prune expired timestamps
	rl.pruneExpired(b, now)

	// If under limit, no wait needed
	if len(b.timestamps) < rl.maxMessages {
		return 0
	}

	// Calculate wait time until oldest expires
	oldestExpiry := b.timestamps[0] + rl.window.Milliseconds()
	waitMs := oldestExpiry - now
	if waitMs <= 0 {
		return 0
	}

	return time.Duration(waitMs) * time.Millisecond
}

// getBucket returns the bucket for a chatID, creating it if necessary.
// Must be called with mu locked.
func (rl *RateLimiter) getBucket(chatID string) *bucket {
	if b, exists := rl.buckets[chatID]; exists {
		return b
	}

	b := &bucket{timestamps: []int64{}}
	rl.buckets[chatID] = b
	return b
}

// pruneExpired removes timestamps older than the window.
// Must be called with mu locked.
func (rl *RateLimiter) pruneExpired(b *bucket, now int64) {
	cutoff := now - rl.window.Milliseconds()

	// Find first non-expired timestamp
	i := 0
	for i < len(b.timestamps) && b.timestamps[i] <= cutoff {
		i++
	}

	// Trim expired timestamps
	b.timestamps = b.timestamps[i:]
}

// timeNowMs returns the current time in Unix milliseconds.
func timeNowMs() int64 {
	return time.Now().UnixMilli()
}
