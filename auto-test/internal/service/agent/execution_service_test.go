// auto-test/internal/service/agent/execution_service_test.go
package agent_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/**
 * SV-01: Execution Service 测试
 * P0 用例：SV-01-02, SV-01-14
 */

// MockAdapter 实现 AgentAdapter 接口用于测试
type MockAdapter struct {
	mu sync.Mutex

	// 控制行为
	ExecuteDelay   time.Duration
	ExecuteError   error
	ExecuteResult  *agent.ExecutionResult
	StreamChunks   []agent.Chunk

	// 会话状态
	Sessions       map[string]agent.SessionStatus
	SessionResults map[string]string

	// 记录调用
	ExecuteCalls    int
	StartSessionCalls int
	ResumeSessionCalls int
	StopSessionCalls  int
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		Sessions:       make(map[string]agent.SessionStatus),
		SessionResults: make(map[string]string),
		ExecuteResult:  &agent.ExecutionResult{Output: "mock output", SessionID: uuid.New().String()},
	}
}

// Execute 单次执行
func (m *MockAdapter) Execute(ctx context.Context, req *agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	m.mu.Lock()
	m.ExecuteCalls++
	m.mu.Unlock()

	if m.ExecuteDelay > 0 {
		select {
		case <-time.After(m.ExecuteDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.ExecuteError != nil {
		return nil, m.ExecuteError
	}

	return m.ExecuteResult, nil
}

// ExecuteWithStream 流式执行
func (m *MockAdapter) ExecuteWithStream(ctx context.Context, req *agent.ExecutionRequest, onChunk func(agent.Chunk)) (*agent.ExecutionResult, error) {
	m.mu.Lock()
	m.ExecuteCalls++
	m.mu.Unlock()

	if m.ExecuteDelay > 0 {
		select {
		case <-time.After(m.ExecuteDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.ExecuteError != nil {
		// 发送错误 chunk
		onChunk(agent.Chunk{Type: agent.ChunkTypeError, Content: m.ExecuteError.Error()})
		return nil, m.ExecuteError
	}

	// 发送预定义的 chunks
	for _, chunk := range m.StreamChunks {
		onChunk(chunk)
	}

	return m.ExecuteResult, nil
}

// StartSession 启动会话
func (m *MockAdapter) StartSession(ctx context.Context, sessionID string, req *agent.ExecutionRequest) error {
	m.mu.Lock()
	m.StartSessionCalls++
	m.Sessions[sessionID] = agent.SessionStatusRunning
	m.mu.Unlock()
	return nil
}

// ResumeSession 恢复会话
func (m *MockAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(agent.Chunk)) error {
	m.mu.Lock()
	m.ResumeSessionCalls++
	m.mu.Unlock()

	if m.ExecuteDelay > 0 {
		select {
		case <-time.After(m.ExecuteDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// 发送响应 chunks
	onChunk(agent.Chunk{Type: agent.ChunkTypeText, Content: "response to: " + input})
	onChunk(agent.Chunk{Type: agent.ChunkTypeText, Content: "", Done: true})

	m.mu.Lock()
	m.SessionResults[sessionID] = input
	m.mu.Unlock()

	return nil
}

// StopSession 停止会话
func (m *MockAdapter) StopSession(sessionID string) error {
	m.mu.Lock()
	m.StopSessionCalls++
	m.Sessions[sessionID] = agent.SessionStatusStopped
	m.mu.Unlock()
	return nil
}

// GetSessionStatus 获取会话状态
func (m *MockAdapter) GetSessionStatus(sessionID string) agent.SessionStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if status, ok := m.Sessions[sessionID]; ok {
		return status
	}
	return agent.SessionStatusIdle
}

// CheckHealth 检查健康状态
func (m *MockAdapter) CheckHealth(ctx context.Context) error {
	return nil
}

// GetCurrentProcess 返回 nil（Mock 不需要真实进程）
func (m *MockAdapter) GetCurrentProcess() interface{} {
	return nil
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id SV-01-02
func TestMockAdapter_Execute(t *testing.T) {
	adapter := NewMockAdapter()

	req := &agent.ExecutionRequest{
		Config: &model.AgentRoleConfig{
			ID:   uuid.New(),
			Name: "Test Agent",
			Role: model.AgentRoleAgent,
		},
		Input: "test input",
	}

	ctx := context.Background()
	result, err := adapter.Execute(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "mock output", result.Output)
	assert.NotEmpty(t, result.SessionID)
	assert.Equal(t, 1, adapter.ExecuteCalls)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id SV-01-02
func TestMockAdapter_ExecuteWithStream(t *testing.T) {
	adapter := NewMockAdapter()
	adapter.StreamChunks = []agent.Chunk{
		{Type: agent.ChunkTypeText, Content: "Hello"},
		{Type: agent.ChunkTypeText, Content: " World"},
		{Type: agent.ChunkTypeText, Content: "", Done: true},
	}

	req := &agent.ExecutionRequest{
		Config: &model.AgentRoleConfig{
			ID:   uuid.New(),
			Name: "Stream Test Agent",
		},
		Input: "stream test",
	}

	var receivedChunks []agent.Chunk
	onChunk := func(chunk agent.Chunk) {
		receivedChunks = append(receivedChunks, chunk)
	}

	ctx := context.Background()
	result, err := adapter.ExecuteWithStream(ctx, req, onChunk)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, receivedChunks, 3)
	assert.Equal(t, "Hello", receivedChunks[0].Content)
	assert.Equal(t, " World", receivedChunks[1].Content)
	assert.True(t, receivedChunks[2].Done)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id SV-01-14
func TestMockAdapter_ExecuteTimeout(t *testing.T) {
	adapter := NewMockAdapter()
	adapter.ExecuteDelay = 5 * time.Second // 5秒延迟

	req := &agent.ExecutionRequest{
		Config: &model.AgentRoleConfig{
			ID:   uuid.New(),
			Name: "Timeout Test Agent",
		},
		Input: "timeout test",
	}

	// 设置 1 秒超时
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := adapter.Execute(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "Should return timeout error")
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id SV-01-14
func TestMockAdapter_ExecuteError(t *testing.T) {
	adapter := NewMockAdapter()
	adapter.ExecuteError = errors.New("mock execution error")

	req := &agent.ExecutionRequest{
		Config: &model.AgentRoleConfig{
			ID:   uuid.New(),
			Name: "Error Test Agent",
		},
		Input: "error test",
	}

	ctx := context.Background()
	result, err := adapter.Execute(ctx, req)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock execution error")
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id SV-01-14
func TestMockAdapter_ExecuteWithStreamError(t *testing.T) {
	adapter := NewMockAdapter()
	adapter.ExecuteError = errors.New("stream error")

	req := &agent.ExecutionRequest{
		Config: &model.AgentRoleConfig{
			ID:   uuid.New(),
			Name: "Stream Error Agent",
		},
		Input: "stream error test",
	}

	var receivedChunks []agent.Chunk
	onChunk := func(chunk agent.Chunk) {
		receivedChunks = append(receivedChunks, chunk)
	}

	ctx := context.Background()
	result, err := adapter.ExecuteWithStream(ctx, req, onChunk)

	assert.Nil(t, result)
	assert.Error(t, err)

	// 应该收到错误 chunk
	assert.Len(t, receivedChunks, 1)
	assert.Equal(t, agent.ChunkTypeError, receivedChunks[0].Type)
	assert.Contains(t, receivedChunks[0].Content, "stream error")
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id SV-01-03
func TestMockAdapter_SessionLifecycle(t *testing.T) {
	adapter := NewMockAdapter()
	sessionID := uuid.New().String()

	req := &agent.ExecutionRequest{
		Config: &model.AgentRoleConfig{
			ID:   uuid.New(),
			Name: "Session Test Agent",
		},
		Input: "session test",
	}

	ctx := context.Background()

	// 1. 启动会话
	err := adapter.StartSession(ctx, sessionID, req)
	require.NoError(t, err)
	assert.Equal(t, agent.SessionStatusRunning, adapter.GetSessionStatus(sessionID))
	assert.Equal(t, 1, adapter.StartSessionCalls)

	// 2. 恢复会话
	var chunks []agent.Chunk
	err = adapter.ResumeSession(ctx, sessionID, "new input", func(chunk agent.Chunk) {
		chunks = append(chunks, chunk)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, adapter.ResumeSessionCalls)
	assert.Len(t, chunks, 2)

	// 3. 停止会话
	err = adapter.StopSession(sessionID)
	require.NoError(t, err)
	assert.Equal(t, agent.SessionStatusStopped, adapter.GetSessionStatus(sessionID))
	assert.Equal(t, 1, adapter.StopSessionCalls)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id SV-01-04
func TestMockAdapter_SessionStatusUnknown(t *testing.T) {
	adapter := NewMockAdapter()

	// 未创建的会话应返回 Idle
	status := adapter.GetSessionStatus("unknown-session")
	assert.Equal(t, agent.SessionStatusIdle, status)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id SV-01-05
func TestMockAdapter_CheckHealth(t *testing.T) {
	adapter := NewMockAdapter()

	ctx := context.Background()
	err := adapter.CheckHealth(ctx)
	assert.NoError(t, err, "Mock adapter should always be healthy")
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id SV-01-06
func TestMockAdapter_ConcurrentExecute(t *testing.T) {
	adapter := NewMockAdapter()

	// 并发执行 10 次
	var wg sync.WaitGroup
	errors := make([]error, 10)
	results := make([]*agent.ExecutionResult, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := &agent.ExecutionRequest{
				Config: &model.AgentRoleConfig{
					ID:   uuid.New(),
					Name: "Concurrent Agent",
				},
				Input: "concurrent test",
			}
			ctx := context.Background()
			result, err := adapter.Execute(ctx, req)
			results[idx] = result
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// 所有执行应该成功
	for i := 0; i < 10; i++ {
		assert.NoError(t, errors[i])
		assert.NotNil(t, results[i])
	}

	// 检查调用计数
	adapter.mu.Lock()
	callCount := adapter.ExecuteCalls
	adapter.mu.Unlock()
	assert.Equal(t, 10, callCount, "Should have 10 execute calls")
}

// @feature F001 - Agent 对话核心
// @priority P2
// @id SV-01-15
func TestChunkTypes(t *testing.T) {
	tests := []struct {
		name     string
		chunk    agent.Chunk
		wantType agent.ChunkType
	}{
		{"text chunk", agent.Chunk{Type: agent.ChunkTypeText, Content: "hello"}, agent.ChunkTypeText},
		{"error chunk", agent.Chunk{Type: agent.ChunkTypeError, Content: "error"}, agent.ChunkTypeError},
		{"status chunk", agent.Chunk{Type: agent.ChunkTypeStatus, Content: "running"}, agent.ChunkTypeStatus},
		{"thinking chunk", agent.Chunk{Type: agent.ChunkTypeThinking, Content: "thinking..."}, agent.ChunkTypeThinking},
		{"tool_use chunk", agent.Chunk{Type: agent.ChunkTypeToolUse, ToolName: "bash"}, agent.ChunkTypeToolUse},
		{"tool_result chunk", agent.Chunk{Type: agent.ChunkTypeToolResult, Content: "result"}, agent.ChunkTypeToolResult},
		{"usage chunk", agent.Chunk{Type: agent.ChunkTypeUsage}, agent.ChunkTypeUsage},
		{"question chunk", agent.Chunk{Type: agent.ChunkTypeQuestion}, agent.ChunkTypeQuestion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantType, tt.chunk.Type)
		})
	}
}

// @feature F001 - Agent 对话核心
// @priority P2
// @id SV-01-16
func TestSessionStatusConstants(t *testing.T) {
	// 验证 SessionStatus 常量定义
	assert.Equal(t, agent.SessionStatus("idle"), agent.SessionStatusIdle)
	assert.Equal(t, agent.SessionStatus("running"), agent.SessionStatusRunning)
	assert.Equal(t, agent.SessionStatus("paused"), agent.SessionStatusPaused)
	assert.Equal(t, agent.SessionStatus("completed"), agent.SessionStatusCompleted)
	assert.Equal(t, agent.SessionStatus("failed"), agent.SessionStatusFailed)
	assert.Equal(t, agent.SessionStatus("stopped"), agent.SessionStatusStopped)
}

// @feature F001 - Agent 对话核心
// @priority P2
// @id SV-01-17
func TestSessionStrategyConstants(t *testing.T) {
	// 验证 SessionStrategy 常量定义
	assert.Equal(t, agent.SessionStrategy("new"), agent.SessionStrategyNew)
	assert.Equal(t, agent.SessionStrategy("resume"), agent.SessionStrategyResume)
}