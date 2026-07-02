// internal/service/agent/process_pool_test.go
package agent

import (
	"testing"

	"github.com/google/uuid"
)

func TestSerializeKey(t *testing.T) {
	key := PoolKey{
		WorkDir: "/path/to/project",
		RoleID:  uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
	result := serializeKey(key)
	expected := "/path/to/project::00000000-0000-0000-0000-000000000001"
	if result != expected {
		t.Errorf("serializeKey() = %s, want %s", result, expected)
	}
}

func TestSerializeSessionKey(t *testing.T) {
	key := PoolKey{
		WorkDir: "/path/to/project",
		RoleID:  uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
	result := serializeSessionKey(key, "session-123")
	expected := "/path/to/project::00000000-0000-0000-0000-000000000001::session-123"
	if result != expected {
		t.Errorf("serializeSessionKey() = %s, want %s", result, expected)
	}
}

func TestNewProcessPool(t *testing.T) {
	// 测试默认值
	pool := NewProcessPool(PoolConfig{})
	if pool.config.MaxLiveProcesses != 10 {
		t.Errorf("default MaxLiveProcesses = %d, want 10", pool.config.MaxLiveProcesses)
	}
	if pool.config.IdleTtlMs != 1800000 {
		t.Errorf("default IdleTtlMs = %d, want 1800000", pool.config.IdleTtlMs)
	}
	if pool.config.HealthCheckMs != 30000 {
		t.Errorf("default HealthCheckMs = %d, want 30000", pool.config.HealthCheckMs)
	}

	// 测试自定义值
	pool2 := NewProcessPool(PoolConfig{
		MaxLiveProcesses:  5,
		IdleTtlMs:         60000,
		HealthCheckMs:     10000,
	})
	if pool2.config.MaxLiveProcesses != 5 {
		t.Errorf("custom MaxLiveProcesses = %d, want 5", pool2.config.MaxLiveProcesses)
	}
}

func TestProcessPoolAcquireColdStart(t *testing.T) {
	pool := NewProcessPool(PoolConfig{
		MaxLiveProcesses: 10,
	})

	key := PoolKey{
		WorkDir: "/test",
		RoleID:  uuid.New(),
	}

	lease, err := pool.Acquire(key, "", false)
	if err != nil {
		t.Errorf("Acquire() error = %v", err)
	}
	if lease == nil {
		t.Error("Acquire() returned nil lease")
	}

	// 检查 metrics
	metrics := pool.GetMetrics()
	if metrics.ColdStartCount != 1 {
		t.Errorf("ColdStartCount = %d, want 1", metrics.ColdStartCount)
	}
	if metrics.LiveProcessCount != 1 {
		t.Errorf("LiveProcessCount = %d, want 1", metrics.LiveProcessCount)
	}
	if metrics.ActiveLeaseCount != 1 {
		t.Errorf("ActiveLeaseCount = %d, want 1", metrics.ActiveLeaseCount)
	}

	// 释放租约
	lease.Release()

	metrics = pool.GetMetrics()
	if metrics.ActiveLeaseCount != 0 {
		t.Errorf("after Release, ActiveLeaseCount = %d, want 0", metrics.ActiveLeaseCount)
	}
	if metrics.IdleProcessCount != 1 {
		t.Errorf("after Release, IdleProcessCount = %d, want 1", metrics.IdleProcessCount)
	}
}

func TestProcessPoolRememberSession(t *testing.T) {
	pool := NewProcessPool(PoolConfig{})

	key := PoolKey{
		WorkDir: "/test",
		RoleID:  uuid.New(),
	}

	lease, err := pool.Acquire(key, "", false)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	// 记录 session
	pool.RememberSession(key, "session-123", lease)

	// 验证 sessionOwners
	sessionKey := serializeSessionKey(key, "session-123")
	pool.mu.RLock()
	_, exists := pool.sessionOwners[sessionKey]
	pool.mu.RUnlock()

	if !exists {
		t.Error("RememberSession() did not add entry to sessionOwners")
	}

	// 空 sessionId 不记录
	pool.RememberSession(key, "", lease)
}

func TestProcessPoolClose(t *testing.T) {
	pool := NewProcessPool(PoolConfig{})

	key := PoolKey{
		WorkDir: "/test",
		RoleID:  uuid.New(),
	}

	_, err := pool.Acquire(key, "", false)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	pool.Close()

	if !pool.closed {
		t.Error("Close() did not set closed flag")
	}

	// 再次 Acquire 应失败
	_, err = pool.Acquire(key, "", false)
	if err != ErrPoolClosed {
		t.Errorf("Acquire after Close() error = %v, want ErrPoolClosed", err)
	}
}

func TestProcessPoolMetrics(t *testing.T) {
	pool := NewProcessPool(PoolConfig{})
	metrics := pool.GetMetrics()

	// 初始状态
	if metrics.WarmHitCount != 0 {
		t.Errorf("initial WarmHitCount = %d, want 0", metrics.WarmHitCount)
	}
	if metrics.LiveProcessCount != 0 {
		t.Errorf("initial LiveProcessCount = %d, want 0", metrics.LiveProcessCount)
	}
}