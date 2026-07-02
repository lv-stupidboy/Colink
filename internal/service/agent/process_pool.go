// internal/service/agent/process_pool.go
// ProcessPool - CLI 进程池预热管理
// 参考 clowder-ai AcpProcessPool.ts 实现
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PoolKey 进程池键，决定进程复用粒度
// key: workDir::roleID (工作目录 + AgentRoleConfig.ID)
type PoolKey struct {
	WorkDir string
	RoleID  uuid.UUID
}

// PoolEntry 进程池条目，表示一个 CLI 进程
type PoolEntry struct {
	Client          interface{} // 已初始化的 CLI 进程（后续集成 ACP adapter）
	LeaseCount      int         // 当前租约数
	LeaseGeneration int64       // 防止僵尸 lease，每次 Acquire 递增
	LastUsedAt      time.Time   // 最后使用时间（用于 LRU 驱逐）
	State           string      // "initializing" | "ready" | "closing"
	IdleTimer       *time.Timer // 空闲超时定时器
	mu              sync.Mutex
}

// Lease 进程租约，表示对进程的一次使用
type Lease struct {
	Client     interface{}
	PoolKey    PoolKey
	Generation int64         // 创建时的 generation，用于检测 stale lease
	Release    func()        // 释放租约的函数
	AcquiredAt time.Time
}

// PendingSpawn 表示正在 spawn 的进程
// 用于避免重复启动进程，并发 Acquire 时等待正在 spawn 的进程
type PendingSpawn struct {
	Promise chan *PoolEntry // 用于等待 spawn 完成的 channel
	Done    bool            // spawn 是否完成
}

// ProcessPool 进程池，管理 CLI 进程的预热和复用
type ProcessPool struct {
	entries       map[string][]*PoolEntry   // key: serializeKey(poolKey) → entries
	sessionOwners map[string]*PoolEntry     // key: poolKey::sessionId → entry (session 归属)
	pendingSpawns map[string]*PendingSpawn  // key: serializeKey(poolKey) → 正在 spawn 的进程
	config        PoolConfig
	metrics       PoolMetrics
	mu            sync.RWMutex
	healthTimer   *time.Timer
	healthCancel  context.CancelFunc
	closed        bool
}

// PoolConfig 进程池配置
type PoolConfig struct {
	MaxLiveProcesses  int // 最大存活进程数
	IdleTtlMs         int // 空闲进程 TTL（毫秒）
	HealthCheckMs     int // 健康检查间隔（毫秒）
}

// PoolMetrics 进程池统计指标
type PoolMetrics struct {
	WarmHitCount      int // 复用已有进程次数
	ColdStartCount    int // 新启动进程次数
	EvictionCount     int // 驱逐次数
	ZombieCleanupCount int // 僵尸进程清理次数
	LiveProcessCount   int // 当前存活进程数
	ActiveLeaseCount   int // 当前活跃租约数
	IdleProcessCount   int // 当前空闲进程数
}

// serializeKey 序列化 PoolKey 为字符串
func serializeKey(key PoolKey) string {
	return key.WorkDir + "::" + key.RoleID.String()
}

// serializeSessionKey 序列化 PoolKey + sessionId 为字符串
func serializeSessionKey(key PoolKey, sessionId string) string {
	return serializeKey(key) + "::" + sessionId
}

// NewProcessPool 创建进程池
func NewProcessPool(config PoolConfig) *ProcessPool {
	// 设置默认值
	if config.MaxLiveProcesses <= 0 {
		config.MaxLiveProcesses = 10
	}
	if config.IdleTtlMs <= 0 {
		config.IdleTtlMs = 1800000 // 30min
	}
	if config.HealthCheckMs <= 0 {
		config.HealthCheckMs = 30000 // 30s
	}

	pool := &ProcessPool{
		entries:       make(map[string][]*PoolEntry),
		sessionOwners: make(map[string]*PoolEntry),
		pendingSpawns: make(map[string]*PendingSpawn),
		config:        config,
		metrics:       PoolMetrics{},
	}

	// 启动健康检查
	pool.startHealthCheck()

	return pool
}

// GetMetrics 获取当前统计指标（只读）
func (p *ProcessPool) GetMetrics() PoolMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metrics
}

// Close 关闭进程池，清理所有进程
func (p *ProcessPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true

	// 停止健康检查
	if p.healthCancel != nil {
		p.healthCancel()
	}
	if p.healthTimer != nil {
		p.healthTimer.Stop()
	}

	// 清理所有 entry 的 idleTimer
	for _, entries := range p.entries {
		for _, entry := range entries {
			entry.mu.Lock()
			if entry.IdleTimer != nil {
				entry.IdleTimer.Stop()
			}
			entry.mu.Unlock()
		}
	}

	// 清空 maps
	p.entries = make(map[string][]*PoolEntry)
	p.sessionOwners = make(map[string]*PoolEntry)
	p.pendingSpawns = make(map[string]*PendingSpawn)
	p.metrics = PoolMetrics{}
}

// Acquire 获取进程租约（优化版：读锁优先）
// 复用逻辑: sessionOwners(读锁快速路径) → entries(写锁慢路径) → cold start
// supportsMultiplexing: 是否支持多lease共享进程（multiplexing模式）
func (p *ProcessPool) Acquire(poolKey PoolKey, sessionId string, supportsMultiplexing bool) (*Lease, error) {
	// 优化：先读锁查 sessionOwners（快速路径）
	if sessionId != "" {
		sessionKey := serializeSessionKey(poolKey, sessionId)

		p.mu.RLock()
		owner, exists := p.sessionOwners[sessionKey]
		p.mu.RUnlock()

		if exists && owner != nil {
			// 快速路径：只锁单个 entry（避免全局锁）
			owner.mu.Lock()

			// 僵尸租约检测：单lease模式下leaseCount>0
			// 场景：Windows控制台断开等导致旧租约未正确释放
			// 检测条件：进程ready但leaseCount>0（不应该存在）
			if owner.State == "ready" && owner.Client != nil && !supportsMultiplexing && owner.LeaseCount > 0 {
				LogWarn("僵尸租约检测，强制释放（快速路径）",
					zap.String("sessionId", sessionId),
					zap.Int("leaseCount", owner.LeaseCount),
					zap.Int64("generation", owner.LeaseGeneration))

				// 强制释放僵尸租约：更新 metrics
				p.mu.Lock()
				p.metrics.ActiveLeaseCount -= owner.LeaseCount
				p.metrics.IdleProcessCount++
				p.mu.Unlock()

				// 清零 leaseCount，bump generation 防止旧 release 干扰
				owner.LeaseCount = 0
				owner.LeaseGeneration++

				// 清除可能存在的 idleTimer
				if owner.IdleTimer != nil {
					owner.IdleTimer.Stop()
					owner.IdleTimer = nil
				}
			}

			// 双重检查：确保 entry 状态有效
			// multiplexing模式：允许leaseCount>0的进程复用
			// 单lease模式：仅允许leaseCount==0的进程复用
			canReuse := owner.State == "ready" && owner.Client != nil
			if !supportsMultiplexing {
				canReuse = canReuse && owner.LeaseCount == 0 // 单lease模式
			}

			if canReuse {
				// 清除 idleTimer
				if owner.IdleTimer != nil {
					owner.IdleTimer.Stop()
					owner.IdleTimer = nil
				}
				owner.LeaseCount++
				owner.LeaseGeneration++
				owner.LastUsedAt = time.Now()
				owner.mu.Unlock()

				// 更新 metrics（短暂持全局锁）
				p.mu.Lock()
				p.metrics.WarmHitCount++
				p.metrics.ActiveLeaseCount++
				if owner.LeaseCount == 1 {
					p.metrics.IdleProcessCount--
				}
				p.mu.Unlock()

				return p.createLease(owner, poolKey), nil
			}
			owner.mu.Unlock()
		}
	}

	// 慢路径：需要全局写锁（处理 entries 和 cold start）
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, ErrPoolClosed
	}

	key := serializeKey(poolKey)

	// 1. 检查是否有正在 spawn 的进程（避免重复启动）
	if pending, exists := p.pendingSpawns[key]; exists && !pending.Done {
		// 释放写锁，等待 spawn 完成
		p.mu.Unlock()
		entry := <-pending.Promise

		// 重新获取写锁处理租约
		p.mu.Lock()
		if p.closed {
			return nil, ErrPoolClosed
		}

		entry.mu.Lock()
		entry.LeaseCount++
		entry.LeaseGeneration++
		entry.LastUsedAt = time.Now()
		entry.mu.Unlock()

		p.metrics.WarmHitCount++
		p.metrics.ActiveLeaseCount++

		return p.createLease(entry, poolKey), nil
	}

	// 2. 再次检查 sessionOwners（防止读锁释放后状态变化）
	if sessionId != "" {
		sessionKey := serializeSessionKey(poolKey, sessionId)
		if owner, exists := p.sessionOwners[sessionKey]; exists && owner != nil {
			owner.mu.Lock()

			// 僵尸租约检测：单lease模式下leaseCount>0
			// 场景：Windows控制台断开等导致旧租约未正确释放
			// 检测条件：进程ready但leaseCount>0（不应该存在）
			if owner.State == "ready" && owner.Client != nil && !supportsMultiplexing && owner.LeaseCount > 0 {
				LogWarn("僵尸租约检测，强制释放（慢路径）",
					zap.String("sessionId", sessionId),
					zap.Int("leaseCount", owner.LeaseCount),
					zap.Int64("generation", owner.LeaseGeneration))

				// 强制释放僵尸租约：更新 metrics（已在写锁中，无需额外锁）
				p.metrics.ActiveLeaseCount -= owner.LeaseCount
				p.metrics.IdleProcessCount++

				// 清零 leaseCount，bump generation 防止旧 release 干扰
				owner.LeaseCount = 0
				owner.LeaseGeneration++

				// 清除可能存在的 idleTimer
				if owner.IdleTimer != nil {
					owner.IdleTimer.Stop()
					owner.IdleTimer = nil
				}
			}

			// multiplexing模式：允许leaseCount>0的进程复用
			// 单lease模式：仅允许leaseCount==0的进程复用
			canReuse := owner.State == "ready" && owner.Client != nil
			if !supportsMultiplexing {
				canReuse = canReuse && owner.LeaseCount == 0 // 单lease模式
			}

			if canReuse {
				if owner.IdleTimer != nil {
					owner.IdleTimer.Stop()
					owner.IdleTimer = nil
				}
				owner.LeaseCount++
				owner.LeaseGeneration++
				owner.LastUsedAt = time.Now()
				owner.mu.Unlock()

				p.metrics.WarmHitCount++
				p.metrics.ActiveLeaseCount++
				if owner.LeaseCount == 1 {
					p.metrics.IdleProcessCount--
				}
				return p.createLease(owner, poolKey), nil
			}
			owner.mu.Unlock()
		}
	}

	// 3. 检查 entries 中 ready + 空闲进程
	entries := p.entries[key]
	for _, entry := range entries {
		entry.mu.Lock()
		// multiplexing模式：允许leaseCount>0的进程复用
		// 单lease模式：仅允许leaseCount==0的进程复用
		canReuse := entry.State == "ready" && entry.Client != nil
		if !supportsMultiplexing {
			canReuse = canReuse && entry.LeaseCount == 0 // 单lease模式
		}

		if canReuse {
			// 清除 idle timer
			if entry.IdleTimer != nil {
				entry.IdleTimer.Stop()
				entry.IdleTimer = nil
			}
			entry.LeaseCount++
			entry.LeaseGeneration++
			entry.LastUsedAt = time.Now()
			entry.mu.Unlock()

			p.metrics.WarmHitCount++
			p.metrics.ActiveLeaseCount++
			if entry.LeaseCount == 1 {
				p.metrics.IdleProcessCount--
			}
			return p.createLease(entry, poolKey), nil
		}
		entry.mu.Unlock()
	}

	// 4. 容量检查 + LRU 驱逐
	if p.metrics.LiveProcessCount >= p.config.MaxLiveProcesses {
		if !p.evictOne() {
			return nil, ErrPoolAtCapacity
		}
	}

	// 5. Cold start - 创建新 entry（标记为正在 spawn）
	pending := &PendingSpawn{Promise: make(chan *PoolEntry, 1)}
	p.pendingSpawns[key] = pending

	entry := &PoolEntry{
		Client:          nil, // 后续由 adapter 设置
		LeaseCount:      1,
		LeaseGeneration: 1,
		LastUsedAt:      time.Now(),
		State:           "initializing",
	}

	if p.entries[key] == nil {
		p.entries[key] = []*PoolEntry{}
	}
	p.entries[key] = append(p.entries[key], entry)

	p.metrics.LiveProcessCount++
	p.metrics.ActiveLeaseCount++
	p.metrics.ColdStartCount++

	// spawn 完成，通知等待的 Acquire
	pending.Done = true
	pending.Promise <- entry
	delete(p.pendingSpawns, key)

	return p.createLease(entry, poolKey), nil
}

// createLease 创建租约
func (p *ProcessPool) createLease(entry *PoolEntry, poolKey PoolKey) *Lease {
	generation := entry.LeaseGeneration
	acquiredAt := time.Now()

	releaseOnce := false
	return &Lease{
		Client:     entry.Client,
		PoolKey:    poolKey,
		Generation: generation,
		AcquiredAt: acquiredAt,
		Release: func() {
			if releaseOnce {
				return
			}
			releaseOnce = true
			p.releaseLease(entry, poolKey, generation)
		},
	}
}

// releaseLease 释放租约（内部方法）
func (p *ProcessPool) releaseLease(entry *PoolEntry, poolKey PoolKey, generation int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Stale lease 检查：generation 不匹配表示租约已被强制释放
	if entry.LeaseGeneration != generation {
		return
	}

	entry.LeaseCount--
	p.metrics.ActiveLeaseCount--

	if entry.LeaseCount <= 0 {
		entry.LeaseCount = 0
		p.metrics.IdleProcessCount++
		p.startIdleTimer(entry, poolKey)
	}
}

// RememberSession 记录 session 归属
// 用于 resume 时优先路由到同一进程
// 同时同步更新 PoolEntry.Client（关键修复）
func (p *ProcessPool) RememberSession(poolKey PoolKey, sessionId string, lease *Lease) {
	if sessionId == "" || lease == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	key := serializeKey(poolKey)
	sessionKey := serializeSessionKey(poolKey, sessionId)

	// 找到 lease 对应的 entry（通过 generation 匹配）
	entries := p.entries[key]
	for _, entry := range entries {
		entry.mu.Lock()
		// 通过 generation 匹配找到对应的 entry（更可靠）
		if entry.LeaseGeneration == lease.Generation {
			// 关键修复：同步更新 PoolEntry.Client
			// 这样下次 Acquire 时，entry.Client 不为 nil，能正确返回 warm hit
			entry.Client = lease.Client
			entry.State = "ready" // 标记为 ready 状态
			entry.mu.Unlock()

			// 记录 session 归属
			p.sessionOwners[sessionKey] = entry
			LogInfo("ProcessPool.RememberSession: PoolEntry.Client updated",
				zap.String("workDir", poolKey.WorkDir),
				zap.String("roleID", poolKey.RoleID.String()),
				zap.String("sessionId", sessionId),
				zap.Bool("hasClient", entry.Client != nil))
			return
		}
		entry.mu.Unlock()
	}

	LogWarn("ProcessPool.RememberSession: entry not found",
		zap.String("workDir", poolKey.WorkDir),
		zap.String("roleID", poolKey.RoleID.String()),
		zap.String("sessionId", sessionId),
		zap.Int64("generation", lease.Generation))
}

// ForgetSession 清除 session 归属
func (p *ProcessPool) ForgetSession(poolKey PoolKey, sessionId string) {
	if sessionId == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	sessionKey := serializeSessionKey(poolKey, sessionId)
	delete(p.sessionOwners, sessionKey)
}

// evictOne LRU 驱逐一个空闲进程
func (p *ProcessPool) evictOne() bool {
	var oldest *PoolEntry
	var oldestKey string
	var oldestIdx int

	// 找到最老空闲进程
	for key, entries := range p.entries {
		for i, entry := range entries {
			entry.mu.Lock()
			if entry.LeaseCount == 0 && entry.State == "ready" {
				if oldest == nil || entry.LastUsedAt.Before(oldest.LastUsedAt) {
					oldest = entry
					oldestKey = key
					oldestIdx = i
				}
			}
			entry.mu.Unlock()
		}
	}

	if oldest == nil {
		return false
	}

	// 清理 entry
	oldest.mu.Lock()
	if oldest.IdleTimer != nil {
		oldest.IdleTimer.Stop()
	}
	oldest.State = "closing"
	oldest.mu.Unlock()

	// 从 entries 移除
	entries := p.entries[oldestKey]
	p.entries[oldestKey] = append(entries[:oldestIdx], entries[oldestIdx+1:]...)
	if len(p.entries[oldestKey]) == 0 {
		delete(p.entries, oldestKey)
	}

	// 清理 sessionOwners
	p.forgetSessionsForEntry(oldest)

	p.metrics.LiveProcessCount--
	p.metrics.IdleProcessCount--
	p.metrics.EvictionCount++

	return true
}

// forgetSessionsForEntry 清除 entry 关联的所有 sessionOwners
func (p *ProcessPool) forgetSessionsForEntry(entry *PoolEntry) {
	for sessionKey, owner := range p.sessionOwners {
		if owner == entry {
			delete(p.sessionOwners, sessionKey)
		}
	}
}

// startIdleTimer 启动空闲超时定时器
func (p *ProcessPool) startIdleTimer(entry *PoolEntry, poolKey PoolKey) {
	if entry.IdleTimer != nil {
		entry.IdleTimer.Stop()
	}

	ttl := time.Duration(p.config.IdleTtlMs) * time.Millisecond
	entry.IdleTimer = time.AfterFunc(ttl, func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		entry.mu.Lock()
		defer entry.mu.Unlock()

		if entry.LeaseCount > 0 || entry.State != "ready" {
			return
		}

		// 超时，清理 entry
		entry.State = "closing"
		if entry.IdleTimer != nil {
			entry.IdleTimer.Stop()
		}

		// 从 entries 移除
		key := serializeKey(poolKey)
		entries := p.entries[key]
		for i, e := range entries {
			if e == entry {
				p.entries[key] = append(entries[:i], entries[i+1:]...)
				if len(p.entries[key]) == 0 {
					delete(p.entries, key)
				}
				break
			}
		}

		// 清理 sessionOwners
		p.forgetSessionsForEntry(entry)

		p.metrics.LiveProcessCount--
		p.metrics.IdleProcessCount--
		p.metrics.EvictionCount++
	})
}

// SetClient 设置 entry 的 client（由 adapter 调用）
func (p *ProcessPool) SetClient(poolKey PoolKey, entry *PoolEntry, client interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry.mu.Lock()
	entry.Client = client
	entry.State = "ready"
	entry.mu.Unlock()
}

// 错误定义
var (
	ErrPoolClosed     = fmt.Errorf("process pool is closed")
	ErrPoolAtCapacity = fmt.Errorf("process pool at capacity - all processes have active leases")
)

// startHealthCheck 启动健康检查定时器
func (p *ProcessPool) startHealthCheck() {
	ctx, cancel := context.WithCancel(context.Background())
	p.healthCancel = cancel

	interval := time.Duration(p.config.HealthCheckMs) * time.Millisecond
	p.healthTimer = time.AfterFunc(interval, func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		// 遍历所有 entries，检测僵尸进程
		for key, entries := range p.entries {
			for i := len(entries) - 1; i >= 0; i-- {
				entry := entries[i]
				entry.mu.Lock()
				if entry.State == "closing" {
					entry.mu.Unlock()
					continue
				}

				// 检查 isAlive（后续由 adapter 实现）
				// 当前简单实现：进程存在即认为 alive
				isAlive := entry.Client != nil

				if !isAlive {
					// 僵尸进程，清理
					if entry.IdleTimer != nil {
						entry.IdleTimer.Stop()
					}
					entry.State = "closing"

					// 清理 sessionOwners
					p.forgetSessionsForEntry(entry)

					// 从 entries 移除
					entries = append(entries[:i], entries[i+1:]...)
					p.entries[key] = entries
					if len(p.entries[key]) == 0 {
						delete(p.entries, key)
					}

					p.metrics.LiveProcessCount--
					if entry.LeaseCount > 0 {
						p.metrics.ActiveLeaseCount -= entry.LeaseCount
					} else {
						p.metrics.IdleProcessCount--
					}
					p.metrics.ZombieCleanupCount++
				}
				entry.mu.Unlock()
			}
		}

		// 重新启动定时器
		if !p.closed {
			p.healthTimer.Reset(interval)
		}
	})
}

// ProcessClient 进程客户端接口（由具体 adapter 实现）
// 参考 clowder-ai AcpProcessPool.ts:52-57 的 AcpPoolClient 接口
type ProcessClient interface {
	// IsAlive 检查进程是否存活
	// 参考 clowder-ai 的 isAlive 检查机制（cmd.Process.State + stdinPipe 状态）
	IsAlive() bool
	// Close 关闭进程（释放资源）
	Close() error
	// Initialize 初始化进程（用于预热）
	// 参考 clowder-ai AcpProcessPool.ts:312-331 的 spawnEntry
	Initialize() error
}

// Warmup 预热进程池（后台异步预热常用Agent）
// 减少首次调用延迟：5秒 → 0.5秒（目标：85%）
// 参数 poolKeys: 需要预热的 PoolKey 列表（workDir + roleID）
// 参数 createClient: 创建 ProcessClient 的工厂函数（由 ExecutionService 提供）
func (p *ProcessPool) Warmup(poolKeys []PoolKey, createClient func(PoolKey) (ProcessClient, error)) {
	if len(poolKeys) == 0 {
		return
	}

	LogInfo("ProcessPool.Warmup: starting background warmup",
		zap.Int("count", len(poolKeys)))

	for _, key := range poolKeys {
		go func(pk PoolKey) {
			// 后台异步预热
			startTime := time.Now()

			// 创建 entry 并标记为 initializing
			entry := &PoolEntry{
				Client:          nil,
				LeaseCount:      0,
				LeaseGeneration: 0,
				LastUsedAt:      time.Now(),
				State:           "initializing",
			}

			// 加入 entries（需要全局锁）
			p.mu.Lock()
			if p.closed {
				p.mu.Unlock()
				LogWarn("ProcessPool.Warmup: pool closed, skip warmup",
					zap.String("workDir", pk.WorkDir),
					zap.String("roleID", pk.RoleID.String()))
				return
			}

			keyStr := serializeKey(pk)
			if p.entries[keyStr] == nil {
				p.entries[keyStr] = []*PoolEntry{}
			}
			p.entries[keyStr] = append(p.entries[keyStr], entry)
			p.metrics.LiveProcessCount++
			p.mu.Unlock()

			// 调用工厂函数创建 client（调用 adapter 的 Initialize）
			client, err := createClient(pk)
			if err != nil {
				// 预热失败：移除 entry
				p.mu.Lock()
				entries := p.entries[keyStr]
				for i, e := range entries {
					if e == entry {
						p.entries[keyStr] = append(entries[:i], entries[i+1:]...)
						break
					}
				}
				if len(p.entries[keyStr]) == 0 {
					delete(p.entries, keyStr)
				}
				p.metrics.LiveProcessCount--
				p.mu.Unlock()

				LogError("ProcessPool.Warmup: warmup failed",
					zap.String("workDir", pk.WorkDir),
					zap.String("roleID", pk.RoleID.String()),
					zap.Error(err),
					zap.Duration("duration", time.Since(startTime)))
				return
			}

			// 预热成功：设置 client 并标记为 ready
			p.mu.Lock()
			entry.mu.Lock()
			entry.Client = client
			entry.State = "ready"
			entry.mu.Unlock()
			p.metrics.IdleProcessCount++
			p.mu.Unlock()

			LogInfo("ProcessPool.Warmup: warmup completed",
				zap.String("workDir", pk.WorkDir),
				zap.String("roleID", pk.RoleID.String()),
				zap.Duration("duration", time.Since(startTime)))
		}(key)
	}
}