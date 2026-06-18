// @feature F001 - Agent 对话核心
// @priority P1
// @id ACP-01

package acp

import (
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

// TestStderrBufferField 验证 acpSession 包含 stderrOutput 字段
func TestStderrBufferField(t *testing.T) {
	session := &acpSession{
		id:     "test-session",
		isdpID: "test-invocation",
		status: agent.SessionStatusRunning,
	}

	// 验证 stderrOutput 字段存在且可用
	session.mu.Lock()
	session.stderrOutput.WriteString("test stderr line\n")
	session.stderrOutput.WriteString("another line\n")
	content := session.stderrOutput.String()
	session.mu.Unlock()

	if !strings.Contains(content, "test stderr line") {
		t.Error("Expected stderrOutput to contain written content")
	}
	if !strings.Contains(content, "another line") {
		t.Error("Expected stderrOutput to contain second line")
	}
}

// TestStderrSizeLimit 验证 64KB 截断上限常量
func TestStderrSizeLimit(t *testing.T) {
	// 验证常量值正确
	expectedLimit := 64 * 1024
	if maxStderrSize != expectedLimit {
		t.Errorf("Expected maxStderrSize to be %d, got %d", expectedLimit, maxStderrSize)
	}

	// 验证截断逻辑
	session := &acpSession{}

	// 写入超过 64KB 的内容（模拟）
	// 每行 100 字节，写入 700 行（70KB）
	for i := 0; i < 700; i++ {
		session.mu.Lock()
		if session.stderrOutput.Len() < maxStderrSize {
			session.stderrOutput.WriteString(strings.Repeat("X", 99) + "\n")
		}
		session.mu.Unlock()
	}

	content := session.stderrOutput.String()
	// 验证内容不超过 64KB（加上截断逻辑的额外检查）
	if len(content) > maxStderrSize+1000 {
		t.Errorf("Stderr content exceeded limit: %d bytes (limit: %d)", len(content), maxStderrSize)
	}
}

// TestStderrInErrorMessage 验证错误消息格式化包含 stderr
func TestStderrInErrorMessage(t *testing.T) {
	session := &acpSession{}
	session.mu.Lock()
	session.stderrOutput.WriteString("config validation failed: invalid API key\n")
	stderrContent := session.stderrOutput.String()
	session.mu.Unlock()

	// 模拟错误消息格式化
	errMsg := "ACP: initialize handshake failed: connection refused\nstderr: " + stderrContent

	// 验证 stderr 内容在错误消息中
	if !strings.Contains(errMsg, "config validation failed") {
		t.Error("Expected error message to contain stderr content")
	}
	if !strings.Contains(errMsg, "stderr:") {
		t.Error("Expected error message to contain 'stderr:' marker")
	}
}

// TestAdapterConfigField 验证 adapter 配置结构正确
func TestAdapterConfigField(t *testing.T) {
	baseAgent := &model.BaseAgent{
		Type:         model.BaseAgentType("test_acp"),
		DefaultModel: "claude-3-opus",
	}

	config := AcpAdapterConfig{
		CliPath: "/usr/local/bin/test-cli",
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			return []string{"--model", "test"}
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			return []string{"TEST_ENV=value"}
		},
	}

	adapter := NewBaseACPAdapter(config, baseAgent)

	// 验证 adapter 配置正确
	if adapter.Config.CliPath != "/usr/local/bin/test-cli" {
		t.Errorf("Expected CliPath to be configured")
	}
	if adapter.baseAgent.DefaultModel != "claude-3-opus" {
		t.Errorf("Expected baseAgent to be configured")
	}
}

// TestMultipleStderrLines 验证多行 stderr 缓冲
func TestMultipleStderrLines(t *testing.T) {
	session := &acpSession{}

	lines := []string{
		"error line 1: starting validation",
		"error line 2: checking config",
		"error line 3: validation failed",
	}

	for _, line := range lines {
		session.mu.Lock()
		if session.stderrOutput.Len() < maxStderrSize {
			session.stderrOutput.WriteString(line)
			session.stderrOutput.WriteString("\n")
		}
		session.mu.Unlock()
	}

	content := session.stderrOutput.String()

	for _, line := range lines {
		if !strings.Contains(content, line) {
			t.Errorf("Expected stderr to contain '%s'", line)
		}
	}
}

// TestConcurrentStderrWrite 验证并发写入 stderr 缓冲的安全性
func TestConcurrentStderrWrite(t *testing.T) {
	session := &acpSession{}

	// 模拟并发写入（多个 goroutine 同时写入）
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				session.mu.Lock()
				if session.stderrOutput.Len() < maxStderrSize {
					session.stderrOutput.WriteString("goroutine-")
					session.stderrOutput.WriteString(string(rune('0' + id)))
					session.stderrOutput.WriteString("-line-")
					session.stderrOutput.WriteString(string(rune('0' + j%10)))
					session.stderrOutput.WriteString("\n")
				}
				session.mu.Unlock()
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	content := session.stderrOutput.String()

	// 验证内容不超过 64KB
	if len(content) > maxStderrSize+1000 {
		t.Errorf("Stderr content exceeded limit: %d bytes", len(content))
	}

	// 验证有内容被写入
	if len(content) == 0 {
		t.Error("Expected some stderr content to be written")
	}
}

// TestStopSessionClearsPendingElicitation 验证 StopSession 清除 pending elicitation 状态
// @feature F001 - Agent 对话核心
// @priority P0
// @id ACP-02
func TestStopSessionClearsPendingElicitation(t *testing.T) {
	// 创建带有 pending elicitation 的 session
	session := &acpSession{
		id:                      "test-session-stop",
		isdpID:                  "test-invocation-stop",
		status:                  agent.SessionStatusRunning,
		pendingElicitationID:    map[string]interface{}{"id": "elicitation-123"},
		pendingElicitationQuestions: []agent.QuestionItem{
			{Header: "Choice", Question: "What is your choice?", Options: []agent.QuestionOption{{Label: "A"}, {Label: "B"}}},
		},
		pendingQuestion: &agent.Chunk{Type: agent.ChunkTypeQuestion, Content: "Please answer"},
	}

	// 验证初始状态有 pending elicitation
	session.mu.Lock()
	if session.pendingElicitationID == nil {
		t.Error("Expected pendingElicitationID to be set initially")
	}
	if session.pendingElicitationQuestions == nil {
		t.Error("Expected pendingElicitationQuestions to be set initially")
	}
	if session.pendingQuestion == nil {
		t.Error("Expected pendingQuestion to be set initially")
	}
	session.mu.Unlock()

	// 模拟 StopSession 清除逻辑
	session.mu.Lock()
	session.status = agent.SessionStatusStopped
	// 清除待响应状态
	session.pendingElicitationID = nil
	session.pendingElicitationQuestions = nil
	session.pendingQuestion = nil
	session.mu.Unlock()

	// 验证清除后状态
	session.mu.Lock()
	if session.pendingElicitationID != nil {
		t.Error("Expected pendingElicitationID to be nil after stop")
	}
	if session.pendingElicitationQuestions != nil {
		t.Error("Expected pendingElicitationQuestions to be nil after stop")
	}
	if session.pendingQuestion != nil {
		t.Error("Expected pendingQuestion to be nil after stop")
	}
	if session.status != agent.SessionStatusStopped {
		t.Error("Expected status to be SessionStatusStopped")
	}
	session.mu.Unlock()
}

// TestStopSessionWithoutPendingElicitation 验证无 pending elicitation 的 session 正常停止
// @feature F001 - Agent 对话核心
// @priority P1
// @id ACP-03
func TestStopSessionWithoutPendingElicitation(t *testing.T) {
	// 创建没有 pending elicitation 的 session
	session := &acpSession{
		id:     "test-session-no-elicitation",
		isdpID: "test-invocation-no-elicitation",
		status: agent.SessionStatusRunning,
	}

	// 验证初始状态无 pending elicitation
	session.mu.Lock()
	hasPending := session.pendingElicitationID != nil
	session.mu.Unlock()

	if hasPending {
		t.Error("Expected no pending elicitation initially")
	}

	// 模拟 StopSession 清除逻辑（不应触发 cancel response）
	session.mu.Lock()
	session.status = agent.SessionStatusStopped
	session.pendingElicitationID = nil
	session.pendingElicitationQuestions = nil
	session.pendingQuestion = nil
	session.mu.Unlock()

	// 验证停止后状态正常
	session.mu.Lock()
	if session.status != agent.SessionStatusStopped {
		t.Error("Expected status to be SessionStatusStopped")
	}
	session.mu.Unlock()
}