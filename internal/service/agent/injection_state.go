// Package agent — Injection State
//
// 借鉴 clowder-ai invoke-single-cat.ts:241-243 的三个进程内决策容器：
//
//	const _prevContextFill = new Map<string, number>();
//	const _needsReinjection = new Set<string>();
//	const _staticIdentityRegistryRevision = new Map<string, number>();
//
// Go 版用 sync.Map 实现，key 统一为 "userId:agentId:threadId"。
//
// 语义要点：
//   - needsReinjection: **consumed-once** —— LoadAndDelete 消费，避免"错发一次修一辈子"
//   - lastInjectedRegistryVersion: 上次注入 SystemPrompt 时看到的 AgentConfigRegistry.revision
//   - prevContextFill: 上一轮 usedTokens，用于检测 CLI 自动压缩（drop >60%）
package agent

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// IdentityKey 计算注入状态的 key
// 与 clowder-ai sessionIdentityKey(userId, catId, threadId) 对齐
func IdentityKey(userID, agentID, threadID string) string {
	return userID + ":" + agentID + ":" + threadID
}

// IdentityKeyFromUUIDs 便捷版本
func IdentityKeyFromUUIDs(userID, agentID, threadID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", userID, agentID, threadID)
}

// ==============================================================================
// InjectionState 进程级状态容器（三合一，避免暴露多个全局变量）
// 生产环境用 Default* 单例；单测可 new 独立实例避免污染。
// ==============================================================================

type injectionState struct {
	needsReinjection            syncMap[string, struct{}]
	lastInjectedRegistryVersion syncMap[string, int64]
	prevContextFill             syncMap[string, int]
}

// DefaultInjectionState 进程级单例
var DefaultInjectionState = newInjectionState()

func newInjectionState() *injectionState {
	return &injectionState{}
}

// ------------------------------ needsReinjection Set ------------------------------

// FlagNeedsReinjection 标记：下次 Build Prompt 时强制重注 SystemPrompt
// 调用点：压缩检测 (drop >60%)、Resume 失败降级、手工强制刷新
func FlagNeedsReinjection(key string) {
	DefaultInjectionState.needsReinjection.Store(key, struct{}{})
}

// ConsumeReinjectionFlag 消费 flag —— **一次性**，取到就删
// 调用点：Build Prompt 决策阶段
// 借鉴 clowder-ai invoke-single-cat.ts:1687:
//
//	const forceReinjection = _needsReinjection.delete(compressionKey);
func ConsumeReinjectionFlag(key string) bool {
	_, loaded := DefaultInjectionState.needsReinjection.LoadAndDelete(key)
	return loaded
}

// PeekReinjectionFlag 只查询不消费（用于日志/监控）
func PeekReinjectionFlag(key string) bool {
	_, ok := DefaultInjectionState.needsReinjection.Load(key)
	return ok
}

// ------------------------- lastInjectedRegistryVersion Map -------------------------

// RecordInjectedRegistryVersion 记录本次注入时看到的 registry revision
// 调用点：Build Prompt 决定注入 SystemPrompt 后立即调用
func RecordInjectedRegistryVersion(key string, revision int64) {
	DefaultInjectionState.lastInjectedRegistryVersion.Store(key, revision)
}

// GetLastInjectedRegistryVersion 返回上次注入的 revision，ok=false 表示没记录过
// 与 clowder-ai lastStaticIdentityRevision !== undefined 判断对齐
func GetLastInjectedRegistryVersion(key string) (revision int64, ok bool) {
	return DefaultInjectionState.lastInjectedRegistryVersion.Load(key)
}

// ClearInjectedRegistryVersion 移除记录（Session 结束或强制刷新时）
func ClearInjectedRegistryVersion(key string) {
	DefaultInjectionState.lastInjectedRegistryVersion.Delete(key)
}

// ------------------------------ prevContextFill Map ------------------------------

// RecordContextFill 记录本轮 usedTokens
// 调用点：每次 CLI usage chunk 到达时
func RecordContextFill(key string, usedTokens int) {
	DefaultInjectionState.prevContextFill.Store(key, usedTokens)
}

// GetPrevContextFill 读取上一轮 usedTokens
func GetPrevContextFill(key string) (used int, ok bool) {
	return DefaultInjectionState.prevContextFill.Load(key)
}

// ClearContextFill 清理（session 结束时）
func ClearContextFill(key string) {
	DefaultInjectionState.prevContextFill.Delete(key)
}

// ==============================================================================
// syncMap: 泛型化的 sync.Map（避免调用点手动 type-assert）
// ==============================================================================

// syncMap 是 sync.Map 的轻量泛型包装，保证类型安全。
// sync.Map 本身不是泛型（截至 Go 1.22），此处手工封装。
type syncMap[K comparable, V any] struct {
	inner sync.Map
}

func (m *syncMap[K, V]) Load(k K) (v V, ok bool) {
	raw, ok := m.inner.Load(k)
	if !ok {
		return v, false
	}
	v, ok = raw.(V)
	return v, ok
}

func (m *syncMap[K, V]) Store(k K, v V) { m.inner.Store(k, v) }
func (m *syncMap[K, V]) Delete(k K)     { m.inner.Delete(k) }
func (m *syncMap[K, V]) LoadAndDelete(k K) (v V, loaded bool) {
	raw, loaded := m.inner.LoadAndDelete(k)
	if !loaded {
		return v, false
	}
	v, ok := raw.(V)
	if !ok {
		return v, false
	}
	return v, true
}
