package teampackagesync

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCloneCacheBasicOperations(t *testing.T) {
	cache := NewCloneCache()

	// Test Get on empty cache
	dir, exists := cache.Get("https://example.com", "master")
	if exists {
		t.Error("Get on empty cache should return false")
	}
	if dir != "" {
		t.Error("Get on empty cache should return empty string")
	}

	// Test Set
	cache.Set("https://example.com", "master", "/tmp/clone-1")

	// Test Get after Set
	dir, exists = cache.Get("https://example.com", "master")
	if !exists {
		t.Error("Get after Set should return true")
	}
	if dir != "/tmp/clone-1" {
		t.Errorf("Get should return the set directory, got: %s", dir)
	}

	// Test different key
	dir, exists = cache.Get("https://example.com", "dev")
	if exists {
		t.Error("Different branch should not exist")
	}

	// Test GetAllDirs
	dirs := cache.GetAllDirs()
	if len(dirs) != 1 {
		t.Errorf("GetAllDirs should return 1 dir, got: %d", len(dirs))
	}
	if dirs[0] != "/tmp/clone-1" {
		t.Errorf("GetAllDirs should return the set directory, got: %s", dirs[0])
	}

	// Test multiple entries
	cache.Set("https://example.com", "dev", "/tmp/clone-2")
	cache.Set("https://other.com", "master", "/tmp/clone-3")
	dirs = cache.GetAllDirs()
	if len(dirs) != 3 {
		t.Errorf("GetAllDirs should return 3 dirs, got: %d", len(dirs))
	}

	// Test Clear
	cache.Clear()
	dirs = cache.GetAllDirs()
	if len(dirs) != 0 {
		t.Errorf("Clear should empty the cache, got: %d dirs", len(dirs))
	}
	_, exists = cache.Get("https://example.com", "master")
	if exists {
		t.Error("Get after Clear should return false")
	}
}

func TestCloneCacheConcurrentAccess(t *testing.T) {
	cache := NewCloneCache()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			url := "https://repo-" + string(rune(idx%10+'0')) + ".com"
			branch := "branch-" + string(rune(idx%5+'0'))
			cache.Set(url, branch, "/tmp/clone-"+string(rune(idx+'0')))
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			url := "https://repo-" + string(rune(idx%10+'0')) + ".com"
			branch := "branch-" + string(rune(idx%5+'0'))
			_, _ = cache.Get(url, branch)
		}(i)
	}

	wg.Wait()

	// Verify cache is not corrupted
	dirs := cache.GetAllDirs()
	if len(dirs) == 0 {
		t.Error("Concurrent writes should populate cache")
	}
	// After 100 writes to 10*5=50 unique keys, we should have at most 50 entries
	if len(dirs) > 50 {
		t.Errorf("Cache should have at most 50 unique keys, got: %d", len(dirs))
	}
}

func TestCloneCacheGetAllDirsReturnsCopy(t *testing.T) {
	cache := NewCloneCache()
	cache.Set("https://example.com", "master", "/tmp/clone-1")

	dirs1 := cache.GetAllDirs()
	dirs2 := cache.GetAllDirs()

	// Modify one slice should not affect the other
	dirs1[0] = "/modified"
	if dirs2[0] == "/modified" {
		t.Error("GetAllDirs should return a copy, not a reference")
	}
}

func TestCacheKeyGeneration(t *testing.T) {
	tests := []struct {
		url    string
		branch string
		want   string
	}{
		{"https://example.com", "master", "https://example.com#master"},
		{"https://example.com", "dev", "https://example.com#dev"},
		{"https://other.com/repo", "main", "https://other.com/repo#main"},
	}

	for _, tt := range tests {
		key := cacheKey(tt.url, tt.branch)
		if key != tt.want {
			t.Errorf("cacheKey(%s, %s) = %s, want %s", tt.url, tt.branch, key, tt.want)
		}
	}
}

func TestCloneCacheGetOrMarkPendingSingleflight(t *testing.T) {
	cache := NewCloneCache()
	var wg sync.WaitGroup
	var cloneCount int32

	// 模拟 10 个并发请求同一个 URL+branch
	// 应该只有一个请求执行"克隆"，其他请求等待并使用缓存结果
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			dir, err, isFirst := cache.GetOrMarkPending("https://example.com", "main")

			if !isFirst {
				// 不是第一个请求，检查结果
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if dir != "/tmp/test-clone" {
					t.Errorf("cached dir should be /tmp/test-clone, got: %s", dir)
				}
				return
			}

			// 是第一个请求，执行克隆
			atomic.AddInt32(&cloneCount, 1)

			// 模拟克隆操作
			cloneDir := "/tmp/test-clone"
			cache.Complete("https://example.com", "main", cloneDir, nil)
		}(i)
	}

	wg.Wait()

	// 验证只执行了一次"克隆"
	if cloneCount != 1 {
		t.Errorf("should only clone once, but cloned %d times", cloneCount)
	}

	// 验证缓存已存储
	dir, exists := cache.Get("https://example.com", "main")
	if !exists {
		t.Error("cache should have entry after complete")
	}
	if dir != "/tmp/test-clone" {
		t.Errorf("cached dir should be /tmp/test-clone, got: %s", dir)
	}
}

func TestCloneCacheGetOrMarkPendingFailure(t *testing.T) {
	cache := NewCloneCache()
	var wg sync.WaitGroup
	start := make(chan struct{})
	var cloneCount int32

	// 模拟克隆失败场景：5 个并发请求
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start

			dir, err, isFirst := cache.GetOrMarkPending("https://fail.com", "main")

			if !isFirst {
				// 不是第一个请求，检查失败结果
				if err == nil {
					t.Error("expected error for failed clone")
				}
				if dir != "" {
					t.Errorf("dir should be empty for failed clone, got: %s", dir)
				}
				return
			}

			// 是第一个请求，执行克隆（模拟失败）
			atomic.AddInt32(&cloneCount, 1)
			cache.Complete("https://fail.com", "main", "", fmt.Errorf("clone failed"))
		}(i)
	}

	close(start)
	wg.Wait()

	// 验证只执行了一次"克隆"
	if cloneCount != 1 {
		t.Errorf("should only clone once, but cloned %d times", cloneCount)
	}

	// 克隆失败，缓存不应该存储成功结果
	_, exists := cache.Get("https://fail.com", "main")
	if exists {
		t.Error("cache should not store failed clone result")
	}
}

func TestCloneCacheGetOrMarkPendingMultipleKeys(t *testing.T) {
	cache := NewCloneCache()
	var wg sync.WaitGroup
	var cloneCount int32

	// 模拟并发请求 3 个不同的 URL+branch
	urls := []string{"https://a.com", "https://b.com", "https://c.com"}

	for _, url := range urls {
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(u string, idx int) {
				defer wg.Done()

				dir, err, isFirst := cache.GetOrMarkPending(u, "main")

				if !isFirst {
					// 不是第一个请求，检查结果
					if err != nil {
						t.Errorf("unexpected error for %s: %v", u, err)
					}
					if dir != "/tmp/"+u {
						t.Errorf("cached dir mismatch for %s: got %s", u, dir)
					}
					return
				}

				// 是第一个请求，执行克隆
				atomic.AddInt32(&cloneCount, 1)
				cache.Complete(u, "main", "/tmp/"+u, nil)
			}(url, i)
		}
	}

	wg.Wait()

	// 3 个 URL，每个应该只克隆一次
	if cloneCount != 3 {
		t.Errorf("should clone 3 times (once per URL), but cloned %d times", cloneCount)
	}

	// 验证所有缓存已存储
	for _, url := range urls {
		dir, exists := cache.Get(url, "main")
		if !exists {
			t.Errorf("cache should have entry for %s", url)
		}
		if dir != "/tmp/"+url {
			t.Errorf("cached dir mismatch for %s: got %s", url, dir)
		}
	}
}