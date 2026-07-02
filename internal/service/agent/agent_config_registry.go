// Package agent — AgentConfigRegistry
//
// 借鉴 clowder-ai CatRegistry.ts 的单调递增 revision 机制：
//   - Update/Delete 都会让 revision += 1
//   - 消费侧对比 revision int64 判断 config 是否变过（比 hash 便宜）
//   - 用于 Resume 场景决定是否需要强制重注 SystemPrompt
//
// 与 SessionRecord 里的 fingerprint 字段互补：
//   - AgentConfigRegistry.revision：进程内 in-memory，快速判定"AgentConfig 有没有变"
//   - 每个 (userId, agentId, threadId) 的 lastInjectedRegistryVersion（见 injection_state.go）
//     记录"上次注入时看到的 revision"，两者比对得出 registryChanged bool
package agent

import (
	"sync"
	"sync/atomic"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentConfigRegistry 追踪 AgentConfig 变化，用 revision int64 表达变更
//
// 线程安全：
//   - revision 用 atomic 读写
//   - configs map 用 RWMutex 保护
type AgentConfigRegistry struct {
	mu       sync.RWMutex
	revision int64
	configs  map[uuid.UUID]*model.AgentConfig
}

// NewAgentConfigRegistry 创建一个空 registry
func NewAgentConfigRegistry() *AgentConfigRegistry {
	return &AgentConfigRegistry{
		configs: make(map[uuid.UUID]*model.AgentConfig),
	}
}

// Update 更新 (或新增) 一个 AgentConfig，revision 递增
// 幂等语义：即使 config 内容和上次一样，revision 也会 +1（简化实现，consumer 侧比较开销可忽略）
//
// 关键：revision 的 atomic bump 必须在锁内完成，否则 reader 可能读到
// "新 config + 旧 revision" —— 判断 revision 未变而跳过重注，丢一轮触发窗口。
func (r *AgentConfigRegistry) Update(id uuid.UUID, cfg *model.AgentConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[id] = cfg
	atomic.AddInt64(&r.revision, 1)
}

// Delete 移除一个 AgentConfig，revision 递增
func (r *AgentConfigRegistry) Delete(id uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, existed := r.configs[id]; existed {
		delete(r.configs, id)
		atomic.AddInt64(&r.revision, 1)
	}
}

// Get 读取当前 config（可能为 nil）
func (r *AgentConfigRegistry) Get(id uuid.UUID) *model.AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.configs[id]
}

// GetRevision 返回当前全局 revision，用于外部对比
// 借鉴 clowder-ai CatRegistry.getRevision(): number
func (r *AgentConfigRegistry) GetRevision() int64 {
	return atomic.LoadInt64(&r.revision)
}

// Reset 清空 registry（revision 依旧递增，通知消费侧"全部变更"）
// 用于测试或应急清理
func (r *AgentConfigRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs = make(map[uuid.UUID]*model.AgentConfig)
	atomic.AddInt64(&r.revision, 1)
}

// Size 返回当前注册的 config 数量（用于监控/调试）
func (r *AgentConfigRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.configs)
}

// DefaultAgentConfigRegistry 进程级单例
// 与 clowder-ai 的 `export const catRegistry = new CatRegistry()` 对齐
var DefaultAgentConfigRegistry = NewAgentConfigRegistry()
