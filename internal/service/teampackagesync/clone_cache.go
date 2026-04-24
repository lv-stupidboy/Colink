package teampackagesync

import (
	"fmt"
	"sync"
)

// CloneCache 请求级克隆缓存（线程安全）
// 用于批量操作时避免对同一仓库重复克隆
// 使用 singleflight 模式确保对同一 URL+branch 只执行一次克隆
type CloneCache struct {
	entries map[string]string // key: "url#branch", value: cloneDir（成功的结果）
	results map[string]string // key: "url#branch", value: error message（失败的结果）
	mutex   sync.RWMutex
	pending sync.Map // key: "url#branch", value: *pendingEntry（正在进行的克隆）
}

// pendingEntry 表示一个正在进行中的克隆操作
type pendingEntry struct {
	wg  sync.WaitGroup
	dir string
	err error
}

// NewCloneCache 创建克隆缓存
func NewCloneCache() *CloneCache {
	return &CloneCache{
		entries: make(map[string]string),
		results: make(map[string]string),
	}
}

// cacheKey 生成缓存键
func cacheKey(url, branch string) string {
	return fmt.Sprintf("%s#%s", url, branch)
}

// Get 获取缓存的克隆目录（返回目录路径和是否存在）
func (c *CloneCache) Get(url, branch string) (string, bool) {
	key := cacheKey(url, branch)
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	dir, exists := c.entries[key]
	return dir, exists
}

// Set 设置缓存（存储克隆目录路径）
func (c *CloneCache) Set(url, branch string, cloneDir string) {
	key := cacheKey(url, branch)
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.entries[key] = cloneDir
}

// GetAllDirs 获取所有缓存的目录路径（用于清理）
func (c *CloneCache) GetAllDirs() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	dirs := make([]string, 0, len(c.entries))
	for _, dir := range c.entries {
		dirs = append(dirs, dir)
	}
	return dirs
}

// Clear 清空缓存记录（不删除目录，仅清空映射）
func (c *CloneCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.entries = make(map[string]string)
	c.results = make(map[string]string)
}

// GetOrMarkPending 原子性地获取缓存或标记克隆进行中
// 使用 singleflight 模式确保对同一 key 只执行一次克隆
// 返回值：
//   - dir: 克隆目录路径
//   - err: 克隆错误（如果失败）
//   - isFirst: true 表示调用者是第一个请求，需要执行克隆并调用 Complete
//              false 表示等待其他请求完成，返回其结果
func (c *CloneCache) GetOrMarkPending(url, branch string) (dir string, err error, isFirst bool) {
	key := cacheKey(url, branch)

	// 1. 先检查成功缓存
	c.mutex.RLock()
	if cachedDir, exists := c.entries[key]; exists {
		c.mutex.RUnlock()
		return cachedDir, nil, false
	}
	// 检查失败缓存（避免重复尝试失败的克隆）
	if errMsg, exists := c.results[key]; exists {
		c.mutex.RUnlock()
		return "", fmt.Errorf("previous clone failed: %s", errMsg), false
	}
	c.mutex.RUnlock()

	// 2. 缓存不存在，尝试注册为 pending
	pe := &pendingEntry{}
	pe.wg.Add(1) // 等待克隆完成

	actual, loaded := c.pending.LoadOrStore(key, pe)

	if loaded {
		// 已有其他请求正在克隆，等待其完成
		pendingActual := actual.(*pendingEntry)
		pendingActual.wg.Wait()

		// 克隆完成后，检查结果
		c.mutex.RLock()
		if cachedDir, exists := c.entries[key]; exists {
			c.mutex.RUnlock()
			return cachedDir, nil, false
		}
		if errMsg, exists := c.results[key]; exists {
			c.mutex.RUnlock()
			return "", fmt.Errorf("clone failed: %s", errMsg), false
		}
		c.mutex.RUnlock()

		// 如果既没有成功也没有失败记录，返回等待时的结果
		return pendingActual.dir, pendingActual.err, false
	}

	// 首次请求，需要执行克隆
	return "", nil, true
}

// Complete 完成 pending 克隆操作（存储结果并通知等待者）
func (c *CloneCache) Complete(url, branch string, dir string, err error) {
	key := cacheKey(url, branch)

	// 存储结果
	c.mutex.Lock()
	if err == nil && dir != "" {
		c.entries[key] = dir
	} else if err != nil {
		// 存储失败信息（避免重复尝试）
		c.results[key] = err.Error()
	}
	c.mutex.Unlock()

	// 从 pending 中移除并通知等待者
	val, loaded := c.pending.LoadAndDelete(key)
	if loaded {
		pe := val.(*pendingEntry)
		pe.dir = dir
		pe.err = err
		pe.wg.Done() // 通知所有等待者
	}
}