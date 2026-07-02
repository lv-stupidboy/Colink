package sortid

import (
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNew_MonotonicSameMillisecond
// 同毫秒内快速生成 1000 个 ID，字典序必须严格递增
func TestNew_MonotonicSameMillisecond(t *testing.T) {
	const n = 1000
	fixed := time.UnixMilli(1_700_000_000_000)
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = NewAt(fixed)
	}
	for i := 1; i < n; i++ {
		if ids[i] <= ids[i-1] {
			t.Fatalf("monotonic broken at i=%d: %q <= %q", i, ids[i], ids[i-1])
		}
	}
}

// TestNew_MonotonicAcrossMilliseconds
func TestNew_MonotonicAcrossMilliseconds(t *testing.T) {
	early := NewAt(time.UnixMilli(1_700_000_000_000))
	later := NewAt(time.UnixMilli(1_700_000_000_001))
	if later <= early {
		t.Fatalf("later id should be greater: early=%q later=%q", early, later)
	}
}

// TestNew_FixedLength
func TestNew_FixedLength(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := New()
		if len(id) != Length {
			t.Fatalf("id length = %d, want %d, id=%q", len(id), Length, id)
		}
	}
}

// TestNew_FormatStructure
func TestNew_FormatStructure(t *testing.T) {
	id := NewAt(time.UnixMilli(1_234_567_890_123))
	parts := strings.Split(id, "-")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %q", len(parts), id)
	}
	if len(parts[0]) != 16 {
		t.Fatalf("ts part = %d chars, want 16", len(parts[0]))
	}
	if len(parts[1]) != 6 {
		t.Fatalf("seq part = %d chars, want 6", len(parts[1]))
	}
	if len(parts[2]) != 8 {
		t.Fatalf("suffix part = %d chars, want 8", len(parts[2]))
	}
	if parts[0] != "0001234567890123" {
		t.Fatalf("ts padding wrong: %q", parts[0])
	}
}

// TestNew_ConcurrentUnique
func TestNew_ConcurrentUnique(t *testing.T) {
	const goroutines = 32
	const perG = 1000
	all := make(chan string, goroutines*perG)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				all <- New()
			}
		}()
	}
	wg.Wait()
	close(all)

	seen := make(map[string]struct{}, goroutines*perG)
	ids := make([]string, 0, goroutines*perG)
	for id := range all {
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id: %q", id)
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	sort.Strings(ids)
	if len(ids) != goroutines*perG {
		t.Fatalf("count mismatch: got %d", len(ids))
	}
}

// TestNew_LexicographicMatchesTimeOrder
func TestNew_LexicographicMatchesTimeOrder(t *testing.T) {
	t0 := time.UnixMilli(1_700_000_000_000)
	id0a := NewAt(t0)
	id0b := NewAt(t0)
	id1 := NewAt(t0.Add(1 * time.Millisecond))
	id2 := NewAt(t0.Add(10 * time.Millisecond))

	ids := []string{id2, id0b, id1, id0a}
	sort.Strings(ids)

	expected := []string{id0a, id0b, id1, id2}
	for i, want := range expected {
		if ids[i] != want {
			t.Fatalf("sort order wrong at %d: got %q, want %q", i, ids[i], want)
		}
	}
}
