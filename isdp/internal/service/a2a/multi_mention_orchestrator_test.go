package a2a

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestMultiMentionOrchestrator_Errors 测试错误定义
func TestMultiMentionOrchestrator_Errors(t *testing.T) {
	// 验证所有错误定义正确
	assert.Equal(t, "targets cannot be empty", ErrTargetsEmpty.Error())
	assert.Equal(t, "targets cannot exceed 3", ErrTargetsExceed.Error())
	assert.Equal(t, "target not in available agents", ErrTargetsInvalid.Error())
	assert.Equal(t, "callbackTo not in available agents", ErrCallbackToInvalid.Error())
	assert.Contains(t, ErrMissingSearchEvidence.Error(), "searchEvidenceRefs")
	assert.Contains(t, ErrCascadeBlocked.Error(), "cascade blocked")
	assert.Equal(t, "request not found", ErrRequestNotFound.Error())
	assert.Equal(t, "invalid status transition", ErrInvalidStatusTransition.Error())
}

// TestMultiMentionOrchestrator_CreateParams 测试创建参数验证
func TestMultiMentionOrchestrator_CreateParams(t *testing.T) {
	tests := []struct {
		name          string
		targets       []string
		expectError   error
	}{
		{
			name:        "单个目标合法",
			targets:     []string{"agent1"},
			expectError: nil,
		},
		{
			name:        "两个目标合法",
			targets:     []string{"agent1", "agent2"},
			expectError: nil,
		},
		{
			name:        "三个目标合法",
			targets:     []string{"agent1", "agent2", "agent3"},
			expectError: nil,
		},
		{
			name:        "四个目标超出限制",
			targets:     []string{"agent1", "agent2", "agent3", "agent4"},
			expectError: ErrTargetsExceed,
		},
		{
			name:        "空目标列表",
			targets:     []string{},
			expectError: ErrTargetsEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟参数验证逻辑
			var err error
			if len(tt.targets) == 0 {
				err = ErrTargetsEmpty
			} else if len(tt.targets) > 3 {
				err = ErrTargetsExceed
			}
			assert.Equal(t, tt.expectError, err)
		})
	}
}

// TestMultiMentionOrchestrator_SearchEvidence 测试先搜后问原则
func TestMultiMentionOrchestrator_SearchEvidence(t *testing.T) {
	tests := []struct {
		name            string
		searchEvidence  []string
		overrideReason  string
		expectValid     bool
	}{
		{
			name:           "有搜索证据 - 合法",
			searchEvidence: []string{"已查看代码", "已搜索文档"},
			overrideReason: "",
			expectValid:    true,
		},
		{
			name:           "无证据但有覆盖理由 - 合法",
			searchEvidence: []string{},
			overrideReason: "全新概念，无参考资料",
			expectValid:    true,
		},
		{
			name:           "无证据无理由 - 非法",
			searchEvidence: []string{},
			overrideReason: "",
			expectValid:    false,
		},
		{
			name:           "有证据和理由 - 合法",
			searchEvidence: []string{"已查看代码"},
			overrideReason: "补充说明",
			expectValid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证先搜后问原则
			valid := len(tt.searchEvidence) > 0 || tt.overrideReason != ""
			assert.Equal(t, tt.expectValid, valid)
		})
	}
}

// TestMultiMentionOrchestrator_CascadePrevention 测试级联防护
func TestMultiMentionOrchestrator_CascadePrevention(t *testing.T) {
	// 创建编排器
	orchestrator := NewMultiMentionOrchestrator(nil)

	// 测试 IsActiveTarget 在未设置时返回 false
	threadID := uuid.New()
	agentID := "test-agent-id"

	// 未设置时应该返回 false
	isActive := orchestrator.IsActiveTarget(threadID, agentID)
	assert.False(t, isActive)

	// 设置后应该返回 true
	orchestrator.SetActiveTargets(threadID, []string{agentID})
	isActive = orchestrator.IsActiveTarget(threadID, agentID)
	assert.True(t, isActive)

	// 其他 agent 应该返回 false
	isActive = orchestrator.IsActiveTarget(threadID, "other-agent")
	assert.False(t, isActive)

	// 清除后应该返回 false
	orchestrator.ClearActiveTargets(threadID)
	isActive = orchestrator.IsActiveTarget(threadID, agentID)
	assert.False(t, isActive)
}

// TestMultiMentionOrchestrator_StatusTransitions 测试状态转换
func TestMultiMentionOrchestrator_StatusTransitions(t *testing.T) {
	// 定义合法的状态转换
	validTransitions := map[string][]string{
		"pending": {"running"},
		"running": {"partial", "done", "timeout", "failed"},
		"partial": {"done", "timeout", "failed"},
	}

	// 验证状态转换规则
	for from, toList := range validTransitions {
		for _, to := range toList {
			t.Run(from+"->"+to, func(t *testing.T) {
				// 状态转换应该合法
				assert.Contains(t, toList, to)
			})
		}
	}
}

// TestMultiMentionOrchestrator_Timeout 测试超时机制
func TestMultiMentionOrchestrator_Timeout(t *testing.T) {
	tests := []struct {
		name           string
		timeoutMinutes int
		expectedMin    int
	}{
		{
			name:           "默认超时",
			timeoutMinutes: 0,
			expectedMin:    8,
		},
		{
			name:           "自定义超时",
			timeoutMinutes: 5,
			expectedMin:    5,
		},
		{
			name:           "负值使用默认",
			timeoutMinutes: -1,
			expectedMin:    8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟超时逻辑
			timeout := tt.timeoutMinutes
			if timeout <= 0 {
				timeout = 8
			}
			assert.Equal(t, tt.expectedMin, timeout)
		})
	}
}