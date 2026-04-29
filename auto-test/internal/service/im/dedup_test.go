package im_test

import (
	"sync"
	"testing"
)

func TestDedupCache_IsDuplicate(t *testing.T) {
	tests := []struct {
		name     string
		maxSize  int
		keys     []string
		expected []bool // expected return values for each key in order
	}{
		{
			name:     "single key not duplicate",
			maxSize:  10,
			keys:     []string{"key1"},
			expected: []bool{false},
		},
		{
			name:     "duplicate detection",
			maxSize:  10,
			keys:     []string{"key1", "key1", "key1"},
			expected: []bool{false, true, true},
		},
		{
			name:     "multiple unique keys",
			maxSize:  10,
			keys:     []string{"key1", "key2", "key3"},
			expected: []bool{false, false, false},
		},
		{
			name:     "mixed unique and duplicate",
			maxSize:  10,
			keys:     []string{"key1", "key2", "key1", "key3", "key2"},
			expected: []bool{false, false, true, false, true},
		},
		{
			name:     "LRU eviction at capacity",
			maxSize:  3,
			keys:     []string{"key1", "key2", "key3", "key4"},
			expected: []bool{false, false, false, false},
		},
		{
			name:     "evicted key becomes new after eviction",
			maxSize:  2,
			keys:     []string{"key1", "key2", "key3", "key1"},
			expected: []bool{false, false, false, false}, // key1 evicted, then re-added as new
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewDedupCache(tt.maxSize)

			for i, key := range tt.keys {
				result := cache.IsDuplicate(key)
				if result != tt.expected[i] {
					t.Errorf("IsDuplicate(%q) = %v, want %v", key, result, tt.expected[i])
				}
			}
		})
	}
}

func TestDedupCache_Size(t *testing.T) {
	cache := NewDedupCache(5)

	if cache.Size() != 0 {
		t.Errorf("initial size = %d, want 0", cache.Size())
	}

	cache.IsDuplicate("key1")
	if cache.Size() != 1 {
		t.Errorf("after 1 insert, size = %d, want 1", cache.Size())
	}

	cache.IsDuplicate("key2")
	cache.IsDuplicate("key3")
	if cache.Size() != 3 {
		t.Errorf("after 3 inserts, size = %d, want 3", cache.Size())
	}

	// Duplicate doesn't increase size
	cache.IsDuplicate("key1")
	if cache.Size() != 3 {
		t.Errorf("after duplicate, size = %d, want 3", cache.Size())
	}
}

func TestDedupCache_Clear(t *testing.T) {
	cache := NewDedupCache(10)

	cache.IsDuplicate("key1")
	cache.IsDuplicate("key2")
	cache.IsDuplicate("key3")

	if cache.Size() != 3 {
		t.Errorf("before clear, size = %d, want 3", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("after clear, size = %d, want 0", cache.Size())
	}

	// After clear, keys should be new again
	if cache.IsDuplicate("key1") {
		t.Error("after clear, key1 should not be duplicate")
	}
}

func TestDedupCache_DefaultMaxSize(t *testing.T) {
	cache := NewDedupCache(0)
	if cache.maxSize != 1000 {
		t.Errorf("default maxSize = %d, want 1000", cache.maxSize)
	}

	cache = NewDedupCache(-5)
	if cache.maxSize != 1000 {
		t.Errorf("negative maxSize = %d, want 1000", cache.maxSize)
	}
}

func TestDedupCache_ThreadSafety(t *testing.T) {
	cache := NewDedupCache(100)
	var wg sync.WaitGroup
	numGoroutines := 10
	keysPerGoroutine := 50

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < keysPerGoroutine; j++ {
				key := "key"
				cache.IsDuplicate(key)
			}
		}(i)
	}

	wg.Wait()

	// Size should be 1 (all goroutines used same key)
	if cache.Size() != 1 {
		t.Errorf("after concurrent writes with same key, size = %d, want 1", cache.Size())
	}
}

func TestDedupCache_LRUEvictionOrder(t *testing.T) {
	cache := NewDedupCache(3)

	// Insert 3 keys: cache={key1, key2, key3}
	cache.IsDuplicate("key1")
	cache.IsDuplicate("key2")
	cache.IsDuplicate("key3")

	if cache.Size() != 3 {
		t.Errorf("after 3 inserts, size = %d, want 3", cache.Size())
	}

	// Insert 4th key: evict key1 (oldest), cache={key2, key3, key4}
	cache.IsDuplicate("key4")

	if cache.Size() != 3 {
		t.Errorf("after eviction, size = %d, want 3", cache.Size())
	}

	// Verify key1 was evicted (returns false when checked)
	if cache.IsDuplicate("key1") {
		t.Error("key1 should not be duplicate after eviction")
	}

	// After re-inserting key1, cache={key3, key4, key1} (key2 was evicted)
	if cache.Size() != 3 {
		t.Errorf("after re-inserting key1, size = %d, want 3", cache.Size())
	}

	// Verify key2 was evicted (returns false when checked)
	if cache.IsDuplicate("key2") {
		t.Error("key2 should not be duplicate (was evicted)")
	}

	// After re-inserting key2, cache={key4, key1, key2} (key3 was evicted)
	if cache.Size() != 3 {
		t.Errorf("after re-inserting key2, size = %d, want 3", cache.Size())
	}

	// Verify key3 was evicted (returns false when checked)
	if cache.IsDuplicate("key3") {
		t.Error("key3 should not be duplicate (was evicted)")
	}

	// After re-inserting key3, cache={key1, key2, key3} (key4 was evicted)
	if cache.Size() != 3 {
		t.Errorf("after re-inserting key3, size = %d, want 3", cache.Size())
	}

	// Verify key4 was evicted (returns false when checked)
	if cache.IsDuplicate("key4") {
		t.Error("key4 should not be duplicate (was evicted)")
	}
}
