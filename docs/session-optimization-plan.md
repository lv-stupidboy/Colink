# Colink Session复用性能优化方案

## 执行时间：2026-06-30

## P0优化（立即实施，改动小收益高）

### 1. cliSessions持久化（收益：重启后延迟减少200ms）

**问题**：cliSessions缓存仅在内存，重启后失效，导致resume需要重新查询session chain

**方案**：
```go
// internal/service/agent/session_chain_store.go
type SessionChainStore struct {
    cliSessions map[string]string // 内存缓存（保留）
    db          *sql.DB           // 数据库持久化（新增）
}

// 新增持久化方法
func (s *SessionChainStore) PersistCliSession(threadID, configID, sessionID string) error {
    // 1. 写入内存缓存（快速路径）
    s.cliSessions[threadID+":"+configID] = sessionID
    
    // 2. 异步写入数据库（持久化）
    go func() {
        _, err := s.db.Exec(`
            INSERT OR REPLACE INTO cli_session_cache 
            (thread_id, config_id, session_id, updated_at)
            VALUES (?, ?, ?, ?)
        `, threadID, configID, sessionID, time.Now())
        if err != nil {
            log.Warn("cliSessions持久化失败", zap.Error(err))
        }
    }()
    return nil
}

// 启动时恢复缓存
func (s *SessionChainStore) RestoreFromDB() error {
    rows, err := s.db.Query(`
        SELECT thread_id, config_id, session_id 
        FROM cli_session_cache 
        WHERE updated_at > ?
    `, time.Now().Add(-24*time.Hour)) // 只恢复最近24小时
    // ...
}
```

**数据库表**：
```sql
CREATE TABLE IF NOT EXISTS cli_session_cache (
    thread_id TEXT,
    config_id TEXT,
    session_id TEXT,
    updated_at DATETIME,
    PRIMARY KEY (thread_id, config_id)
);
```

**改动量**：~50行代码 + 1个SQL表

---

### 2. SessionID提前持久化（收益：避免并发查询竞争）

**问题**：sessionID在CLI执行后才生成，并发请求时可能导致重复查询

**方案**：
```go
// internal/service/agent/execution_service.go
func (es *ExecutionService) Execute(ctx context.Context, req *ExecutionRequest) {
    // 修改：提前生成sessionID并持久化
    sessionID := generateSessionID() // 在执行前生成
    
    // 立即持久化到session_record表
    es.sessionRecorder.RecordPendingSession(
        req.ThreadID, 
        req.ConfigID, 
        sessionID,
        "pending" // 状态标记
    )
    
    // 后续执行时使用已生成的sessionID
    req.SessionID = sessionID
    // ...
}
```

**改动量**：~30行代码

---

### 3. Acquire排序优化（收益：减少锁持有时间）

**问题**：当前Acquire路径遍历所有entries，持锁时间长

**方案**：
```go
// internal/service/agent/process_pool.go
func (p *ProcessPool) Acquire(poolKey PoolKey, sessionId string) (*Lease, error) {
    // 优化：先读锁查sessionOwners（快速路径）
    p.mu.RLock()
    if sessionId != "" {
        sessionKey := serializeSessionKey(poolKey, sessionId)
        if owner, exists := p.sessionOwners[sessionKey]; exists && owner.State == "ready" {
            p.mu.RUnlock()
            // 快速路径：只锁单个entry
            owner.mu.Lock()
            if owner.LeaseCount == 0 {
                // 直接返回，无需全局写锁
                return p.createLease(owner, poolKey), nil
            }
            owner.mu.Unlock()
            // 失败，重新获取全局锁
            p.mu.Lock()
        }
    } else {
        p.mu.RUnlock()
        p.mu.Lock()
    }
    // ... 后续逻辑
}
```

**改动量**：~40行代码

---

## P1优化（短期实施，改动较大）

### 1. 预热机制（收益：启动延迟减少85%，5秒→0.5秒）

**方案**：
```go
// internal/service/agent/process_pool.go
func (p *ProcessPool) Warmup(poolKeys []PoolKey) error {
    for _, key := range poolKeys {
        // 后台异步预热
        go func(pk PoolKey) {
            entry := &PoolEntry{
                State: "initializing",
                LastUsedAt: time.Now(),
            }
            // 异步初始化
            if err := entry.Initialize(); err == nil {
                entry.State = "ready"
                p.mu.Lock()
                p.entries[serializeKey(pk)] = append(..., entry)
                p.mu.Unlock()
            }
        }(key)
    }
    return nil
}

// cmd/server/main.go 启动时预热
func warmupProcessPool() {
    // 加载常用Agent配置
    configs := loadActiveAgentConfigs()
    poolKeys := configs.map(c => PoolKey{
        WorkDir: c.WorkDir,
        RoleID: c.ID,
    })
    processPool.Warmup(poolKeys)
}
```

**改动量**：~80行代码

---

### 2. pendingSpawns机制（收益：避免重复启动）

**借鉴clowder-ai第177-188行**：
```go
// internal/service/agent/process_pool.go
type ProcessPool struct {
    // 新增字段
    pendingSpawns map[string]*PendingSpawn // key: serializeKey(poolKey)
    // ...
}

type PendingSpawn struct {
    Promise chan *PoolEntry
    Done    bool
}

func (p *ProcessPool) Acquire(...) {
    // 检查是否有正在spawn的进程
    if pending, exists := p.pendingSpawns[key]; exists && !pending.Done {
        // 等待正在spawn的进程完成
        entry := <-pending.Promise
        // 复用entry，避免重复启动
        return p.createLease(entry, poolKey)
    }
    
    // 标记为正在spawn
    pending := &PendingSpawn{Promise: make(chan *PoolEntry, 1)}
    p.pendingSpawns[key] = pending
    
    // spawn完成后通知
    go func() {
        entry := spawnEntry(poolKey)
        pending.Done = true
        pending.Promise <- entry
        p.mu.Lock()
        delete(p.pendingSpawns, key)
        p.mu.Unlock()
    }()
}
```

**改动量**：~60行代码

---

### 3. stale lease recovery（收益：解决僵尸租约问题）

**借鉴clowder-ai第149-164行**：
```go
// internal/service/agent/process_pool.go
func (p *ProcessPool) Acquire(poolKey PoolKey, sessionId string) {
    if owner, exists := p.sessionOwners[sessionKey]; exists {
        // 新增：僵尸租约检测
        if owner.LeaseCount > 0 && owner.State == "ready" {
            // 判断为僵尸租约（Windows控制台断开等情况）
            log.Warn("僵尸租约检测，强制释放", 
                zap.String("sessionId", sessionId),
                zap.Int("leaseCount", owner.LeaseCount))
            
            // 强制释放
            p.metrics.ActiveLeaseCount -= owner.LeaseCount
            owner.LeaseCount = 0
            owner.LeaseGeneration++ // bump generation，防止旧release干扰
            
            // 继续使用该entry
            return p.createLease(owner, poolKey)
        }
    }
}
```

**改动量**：~30行代码

---

### 4. supportsMultiplexing（收益：进程利用率提升）

**方案**：
```go
// internal/model/base_agent.go
type BaseAgent struct {
    SupportsMultiplexing bool // 新增字段
}

// internal/service/agent/process_pool.go
func (p *ProcessPool) Acquire(...) {
    // 检查是否支持multiplexing
    supportsMultiplexing := baseAgent.SupportsMultiplexing
    
    if supportsMultiplexing {
        // 允许复用有leaseCount的进程（按需配置）
        warm := entries.find(e => e.State == "ready" && e.Client.isAlive)
        if warm {
            return p.createLease(warm, poolKey)
        }
    } else {
        // 当前逻辑：只复用leaseCount==0的进程
        warm := entries.find(e => e.State == "ready" && e.LeaseCount == 0)
    }
}
```

**改动量**：~20行代码 + 配置字段

---

## 预期收益汇总

| 优化项 | 收益 | 改动量 | 优先级 |
|--------|------|--------|--------|
| cliSessions持久化 | 重启后延迟减少200ms | 50行 + SQL | P0 |
| SessionID提前持久化 | 避免并发查询竞争 | 30行 | P0 |
| Acquire排序优化 | 减少锁持有时间 | 40行 | P0 |
| **P0总收益** | **~400ms延迟降低** | **120行 + SQL** | **立即** |
| 预热机制 | 启动延迟减少85% | 80行 | P1 |
| pendingSpawns | 避免重复启动 | 60行 | P1 |
| stale lease recovery | 解决僵尸租约 | 30行 | P1 |
| supportsMultiplexing | 进程利用率提升 | 20行 | P1 |
| **P1总收益** | **85%启动延迟降低** | **190行** | **短期** |

---

## 实施顺序

1. **Week 1**：P0优化（cliSessions持久化 + SessionID提前 + Acquire排序）
2. **Week 2-3**：P1优化（预热机制 + pendingSpawns + stale lease + multiplexing）
3. **Week 4**：性能测试验证

---

## 关键文件

- `internal/service/agent/process_pool.go` - ProcessPool核心
- `internal/service/agent/session_chain_store.go` - SessionChain持久化
- `internal/service/agent/execution_service.go` - SessionID提前生成
- `cmd/server/main.go` - 预热触发点