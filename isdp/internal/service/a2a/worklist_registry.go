package a2a

import (
	"sync"

	"github.com/google/uuid"
)

// PushReason push 结果原因
type PushReason string

const (
	PushReasonNotFound     PushReason = "not_found"
	PushReasonDepthLimit   PushReason = "depth_limit"
	PushReasonCallerMismatch PushReason = "caller_mismatch"
	PushReasonAllDuplicate PushReason = "all_duplicate"
)

// PushResult push 结果
type PushResult struct {
	Added  []string   // 成功添加的 Agent IDs
	Reason PushReason // 原因（当 Added 为空时）
}

// WorklistRegistryEntry worklist 条目
type WorklistRegistryEntry struct {
	List               []string        // 可扩展的 worklist
	OriginalCount      int             // 原始用户选择的目标数量
	A2ACount           int             // A2A 深度计数
	MaxDepth           int             // 最大 A2A 深度
	ExecutedIndex      int             // 当前执行的 Agent 索引
	A2AFrom            map[string]string // targetCatID -> callerCatID
	A2ATriggerMessageID map[string]string // targetCatID -> triggerMessageID
}

// WorklistRegistry 全局 worklist 注册表
// 用于 A2A 统一：routeSerial 运行时注册 worklist，callback A2A 触发时 push 目标
// 参考 Clowder AI 的 WorklistRegistry.ts
type WorklistRegistry struct {
	registry    map[string]*WorklistRegistryEntry // registryKey -> entry
	threadIndex map[string]map[string]bool        // threadID -> set of registryKeys
	mu          sync.RWMutex
}

// NewWorklistRegistry 创建 worklist 注册表
func NewWorklistRegistry() *WorklistRegistry {
	return &WorklistRegistry{
		registry:    make(map[string]*WorklistRegistryEntry),
		threadIndex: make(map[string]map[string]bool),
	}
}

// registryKey 计算注册表键
// 优先使用 parentInvocationID，否则使用 threadID
func registryKey(threadID string, parentInvocationID string) string {
	if parentInvocationID != "" {
		return parentInvocationID
	}
	return threadID
}

// Register 注册 worklist
// 由 routeSerial 在开始时调用
func (r *WorklistRegistry) Register(threadID string, worklist []string, maxDepth int, parentInvocationID string) *WorklistRegistryEntry {
	key := registryKey(threadID, parentInvocationID)

	entry := &WorklistRegistryEntry{
		List:               worklist,
		OriginalCount:      len(worklist),
		A2ACount:           0,
		MaxDepth:           maxDepth,
		ExecutedIndex:      0,
		A2AFrom:            make(map[string]string),
		A2ATriggerMessageID: make(map[string]string),
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.registry[key] = entry

	// 维护反向索引
	if r.threadIndex[threadID] == nil {
		r.threadIndex[threadID] = make(map[string]bool)
	}
	r.threadIndex[threadID][key] = true

	return entry
}

// Unregister 注销 worklist
// 由 routeSerial 在退出时调用
// owner check: 只有当存储的 entry 匹配调用者的 entry 时才移除
func (r *WorklistRegistry) Unregister(threadID string, owner *WorklistRegistryEntry, parentInvocationID string) {
	key := registryKey(threadID, parentInvocationID)

	r.mu.Lock()
	defer r.mu.Unlock()

	if owner != nil {
		current := r.registry[key]
		if current != owner {
			return // 过期调用者 — 新的 invocation 拥有该 slot
		}
	}

	delete(r.registry, key)

	// 维护反向索引
	if keys, ok := r.threadIndex[threadID]; ok {
		delete(keys, key)
		if len(keys) == 0 {
			delete(r.threadIndex, threadID)
		}
	}
}

// Push 将 Agent 推送到 worklist（callback A2A 路径）
// 仅对 pending（未执行）部分去重 — 已执行的 Agent 可以重新入队
func (r *WorklistRegistry) Push(threadID string, cats []string, callerCatID string, parentInvocationID string, triggerMessageID string) PushResult {
	key := registryKey(threadID, parentInvocationID)

	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.registry[key]
	if entry == nil {
		return PushResult{Added: []string{}, Reason: PushReasonNotFound}
	}

	// 调用者授权：只有当前执行的 Agent 才能 push
	if callerCatID != "" {
		currentCat := ""
		if entry.ExecutedIndex < len(entry.List) {
			currentCat = entry.List[entry.ExecutedIndex]
		}
		if currentCat != callerCatID {
			return PushResult{Added: []string{}, Reason: PushReasonCallerMismatch}
		}
	}

	// 仅对 pending tail 去重
	pending := make(map[string]bool)
	for i := entry.ExecutedIndex; i < len(entry.List); i++ {
		pending[entry.List[i]] = true
	}

	added := make([]string, 0)
	hitDepthLimit := false

	for _, cat := range cats {
		if entry.A2ACount >= entry.MaxDepth {
			hitDepthLimit = true
			break
		}
		if !pending[cat] {
			entry.List = append(entry.List, cat)
			entry.A2ACount++
			added = append(added, cat)
			pending[cat] = true

			if callerCatID != "" {
				entry.A2AFrom[cat] = callerCatID
			}
			if triggerMessageID != "" {
				entry.A2ATriggerMessageID[cat] = triggerMessageID
			}
		} else if callerCatID != "" {
			// 目标已在 pending：
			// - 原始目标必须继续回复用户（不覆盖 A2A sender）
			// - A2A 入队的目标可以在执行前更新为最新 sender
			existingIndex := -1
			for i := entry.ExecutedIndex; i < len(entry.List); i++ {
				if entry.List[i] == cat {
					existingIndex = i
					break
				}
			}
			isOriginalPendingTarget := existingIndex != -1 && existingIndex < entry.OriginalCount
			if !isOriginalPendingTarget {
				entry.A2AFrom[cat] = callerCatID
				if triggerMessageID != "" {
					entry.A2ATriggerMessageID[cat] = triggerMessageID
				}
			}
		}
	}

	if len(added) == 0 {
		reason := PushReasonAllDuplicate
		if hitDepthLimit {
			reason = PushReasonDepthLimit
		}
		return PushResult{Added: []string{}, Reason: reason}
	}

	return PushResult{Added: added}
}

// Has 检查线程是否有活跃的 worklist
func (r *WorklistRegistry) Has(threadID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys, ok := r.threadIndex[threadID]
	return ok && len(keys) > 0
}

// Get 获取指定 invocation 或线程的 worklist
func (r *WorklistRegistry) Get(threadID string, parentInvocationID string) *WorklistRegistryEntry {
	key := registryKey(threadID, parentInvocationID)

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.registry[key]
}

// UpdateExecutedIndex 更新已执行索引
func (r *WorklistRegistry) UpdateExecutedIndex(threadID string, parentInvocationID string, index int) {
	key := registryKey(threadID, parentInvocationID)

	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.registry[key]; ok {
		entry.ExecutedIndex = index
	}
}

// GetA2AFrom 获取 A2A 触发者
func (r *WorklistRegistry) GetA2AFrom(threadID string, parentInvocationID string, targetCatID string) string {
	key := registryKey(threadID, parentInvocationID)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if entry, ok := r.registry[key]; ok {
		return entry.A2AFrom[targetCatID]
	}
	return ""
}

// GetA2ATriggerMessageID 获取 A2A 触发消息 ID
func (r *WorklistRegistry) GetA2ATriggerMessageID(threadID string, parentInvocationID string, targetCatID string) string {
	key := registryKey(threadID, parentInvocationID)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if entry, ok := r.registry[key]; ok {
		return entry.A2ATriggerMessageID[targetCatID]
	}
	return ""
}

// 全局 WorklistRegistry 实例
var globalWorklistRegistry = NewWorklistRegistry()

// GetWorklistRegistry 获取全局 WorklistRegistry
func GetWorklistRegistry() *WorklistRegistry {
	return globalWorklistRegistry
}

// 辅助方法：从 UUID 转换为字符串
func uuidToString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}