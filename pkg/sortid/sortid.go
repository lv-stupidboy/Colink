// Package sortid — Sortable ID generator
//
// Sortable IDs are lexicographically monotonic strings suitable for cursor-based
// pagination and delivery-cursor advancement.
//
// 借鉴 clowder-ai packages/api/src/domains/cats/services/stores/ports/MessageStore.ts:348-353：
//
//	let _seq = 0;
//	export function generateSortableId(timestamp: number): string {
//	  const ts = String(timestamp).padStart(16, '0');
//	  const seq = String(_seq++).padStart(6, '0');
//	  const suffix = randomUUID().slice(0, 8);
//	  return `${ts}-${seq}-${suffix}`;
//	}
//
// 生成规则：`{ts_16位}-{seq_6位}-{uuid_8字符}`
//   - ts: 毫秒时间戳左填零 16 位，确保 2286 年前字典序单调
//   - seq: 进程内 atomic 计数器 mod 10^6，防止同毫秒碰撞
//   - suffix: uuid[:8]，跨进程 / 跨机器 collision 兜底
//
// 字典序 == 时间序 保证：
//   - ts 相同时 seq 单调递增 → 字典序仍单调
//   - seq wrap 到 0 时同一毫秒内理论上可能倒退，但需要 100 万条/ms，实际不可达
//
// 长度：16+1+6+1+8 = 32 字符，固定长度，便于列宽度约束。
//
// 放在 pkg/sortid 而非 internal/service/agent 是因为：
//   - repo 层需要在 Create 时调用（保证与 CreatedAt 单调对齐）
//   - service/agent 也可能直接使用
//   - 避免 repo → service/agent 的反向依赖
package sortid

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// _seq 进程级单调计数器
// atomic 保证并发安全，不用锁
var _seq int64

// New 用当前时间生成 sortable ID
//
// 单调性：单进程内保证严格递增（time.Now 单调 + seq atomic）
// 唯一性：三段组合，实际碰撞概率约 1 / (2^32 · 10^6) ≈ 2e-16 per ms
func New() string {
	return NewAt(time.Now())
}

// NewAt 用指定时间生成 sortable ID
// 供测试注入固定时间使用，也用于 repo.Create 里保证与 CreatedAt 同源。
func NewAt(t time.Time) string {
	ms := t.UnixMilli()
	seq := atomic.AddInt64(&_seq, 1)
	suffix := uuid.New().String()[:8]
	// %016d 16位 padded；%06d 6位 padded（超过 999999 自然会占更多位，字典序仍单调）
	return fmt.Sprintf("%016d-%06d-%s", ms, seq%1000000, suffix)
}

// Length 固定长度常量，供 SQL 列宽度对齐使用
// 16 (ts) + 1 + 6 (seq) + 1 + 8 (uuid prefix) = 32
const Length = 32
