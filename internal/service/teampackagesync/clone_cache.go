package teampackagesync

import (
	"fmt"
	"sync"
)

// CloneCache 请求级克隆缓存（线程安全）
// 用于批量操作时避免对同一仓库重复克隆
type CloneCache struct {
	entries map[string]string // key: "url#branch", value: cloneDir
	mutex   sync.RWMutex
}

// NewCloneCache 创建克隆缓存
func NewCloneCache() *CloneCache {
	return &CloneCache{
		entries: make(map[string]string),
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
}