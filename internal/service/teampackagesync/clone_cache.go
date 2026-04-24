package teampackagesync

import (
	"fmt"
	"sync"
)

// CloneCache 请求级克隆缓存（线程安全）
// 用于批量操作时避免对同一仓库重复克隆
// 使用 sync.Cond 确保对同一 key 只执行一次克隆
type CloneCache struct {
	entries map[string]string // key: "url#branch", value: cloneDir（成功的结果）
	results map[string]string // key: "url#branch", value: error message（失败的结果）
	mutex   sync.RWMutex      // 保护 entries 和 results
	pending sync.Map          // key: "url#branch", value: *pendingEntry
}

// pendingEntry 用于同步正在执行或已完成的克隆
type pendingEntry struct {
	mu   sync.Mutex
	cond *sync.Cond
	done bool   // true 表示克隆已完成
	dir  string // 克隆结果（成功时的目录）
	err  error  // 克隆错误（失败时）
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
// 使用 sync.Map + sync.Cond 确保对同一 key 只执行一次克隆
// 返回值：
//   - dir: 克隆目录路径
//   - err: 克隆错误（如果失败）
//   - isFirst: true 表示调用者需要执行克隆并调用 Complete
//              false 表示等待其他请求完成，返回其缓存结果
func (c *CloneCache) GetOrMarkPending(url, branch string) (dir string, err error, isFirst bool) {
	key := cacheKey(url, branch)

	// 1. 先检查缓存（快速路径）
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

	// 2. 使用 LoadOrStore 原子性地注册或获取 pendingEntry
	// 创建一个新的 pendingEntry（可能不会被使用）
	pe := &pendingEntry{}
	pe.cond = sync.NewCond(&pe.mu)

	actual, loaded := c.pending.LoadOrStore(key, pe)

	// 3. 根据 LoadOrStore 结果处理
	pendingActual := actual.(*pendingEntry)
	pendingActual.mu.Lock()

	if pendingActual.done {
		// pendingEntry 已完成，直接返回结果
		dir = pendingActual.dir
		err = pendingActual.err
		pendingActual.mu.Unlock()
		return dir, err, false
	}

	if loaded {
		// 已有其他请求正在克隆，等待其完成
		for !pendingActual.done {
			pendingActual.cond.Wait()
		}
		// 等待完成后，返回结果
		dir = pendingActual.dir
		err = pendingActual.err
		pendingActual.mu.Unlock()
		return dir, err, false
	}

	// 首次请求，需要执行克隆
	// 注意：pendingActual.mu 已经 Lock，需要在 Complete 时 Unlock
	pendingActual.mu.Unlock()
	return "", nil, true
}

// Complete 完成 pending 克隆操作（存储结果并通知等待者）
func (c *CloneCache) Complete(url, branch string, dir string, err error) {
	key := cacheKey(url, branch)

	// 存储结果到缓存
	c.mutex.Lock()
	if err == nil && dir != "" {
		c.entries[key] = dir
	} else if err != nil {
		// 存储失败信息（避免重复尝试）
		c.results[key] = err.Error()
	}
	c.mutex.Unlock()

	// 从 pending 中获取并标记完成（不删除，保留作为"已完成"标记）
	val, loaded := c.pending.Load(key)
	if loaded {
		pe := val.(*pendingEntry)
		pe.mu.Lock()
		pe.done = true
		pe.dir = dir
		pe.err = err
		pe.cond.Broadcast() // 通知所有等待者
		pe.mu.Unlock()
	}
}