package im

import (
	"sync"
	"time"
)

// DedupCache is a thread-safe LRU cache for deduplicating messages.
// It tracks seen message keys and evicts oldest entries when max capacity is reached.
type DedupCache struct {
	mu       sync.RWMutex
	maxSize  int
	entries  map[string]time.Time // key -> timestamp
	keyOrder []string             // tracks insertion order for LRU eviction
}

// NewDedupCache creates a new dedup cache with the given max size.
func NewDedupCache(maxSize int) *DedupCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &DedupCache{
		maxSize:  maxSize,
		entries:  make(map[string]time.Time),
		keyOrder: make([]string, 0, maxSize),
	}
}

// IsDuplicate checks if a key has been seen before.
// Returns true if the key was already in the cache (duplicate).
// Returns false if the key is new and has been inserted.
// Thread-safe.
func (dc *DedupCache) IsDuplicate(key string) bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Check if key already exists
	if _, exists := dc.entries[key]; exists {
		return true
	}

	// Key is new, insert it
	dc.entries[key] = time.Now()
	dc.keyOrder = append(dc.keyOrder, key)

	// Evict oldest if over capacity
	if len(dc.entries) > dc.maxSize {
		dc.evictOldest()
	}

	return false
}

// evictOldest removes the oldest entry (first in keyOrder).
// Must be called with lock held.
func (dc *DedupCache) evictOldest() {
	if len(dc.keyOrder) == 0 {
		return
	}

	oldestKey := dc.keyOrder[0]
	dc.keyOrder = dc.keyOrder[1:]
	delete(dc.entries, oldestKey)
}

// Size returns the current number of entries in the cache.
// Thread-safe.
func (dc *DedupCache) Size() int {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return len(dc.entries)
}

// Clear removes all entries from the cache.
// Thread-safe.
func (dc *DedupCache) Clear() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.entries = make(map[string]time.Time)
	dc.keyOrder = make([]string, 0, dc.maxSize)
}
