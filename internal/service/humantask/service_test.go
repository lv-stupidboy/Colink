package humantask

import (
	"encoding/json"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestExtractExpectedOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "basic format with colon",
			input: "职责：负责前端页面开发\n交付物：完成登录页面代码",
			expected: "完成登录页面代码",
		},
		{
			name: "format with Chinese colon",
			input: "职责：负责后端API开发\n交付物：用户认证接口",
			expected: "用户认证接口",
		},
		{
			name: "format without newline",
			input: "职责：负责测试交付物：测试报告",
			expected: "测试报告",
		},
		{
			name: "multiline expected output",
			input: "职责：负责需求分析\n交付物：\n- 需求文档\n- 功能清单\n- 技术方案",
			expected: "- 需求文档\n- 功能清单\n- 技术方案", // 第二个正则处理多行情况
		},
		{
			name: "no expected output field",
			input: "职责：负责开发",
			expected: "",
		},
		{
			name: "empty system prompt",
			input: "",
			expected: "",
		},
		{
			name: "multiple colon format",
			input: "职责：负责UI设计\n交付物：设计稿和切图",
			expected: "设计稿和切图",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractExpectedOutput(tt.input)
			if result != tt.expected {
				t.Errorf("extractExpectedOutput(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildTaskCardMetadata(t *testing.T) {
	taskID := uuid.New()
	threadID := uuid.New()
	roleConfigID := uuid.New()

	task := &model.HumanTask{
		ID:              taskID,
		ThreadID:        threadID,
		RoleConfigID:    roleConfigID,
		RoleName:        "产品经理",
		TaskType:        model.HumanTaskTypeDispatch,
		TaskContent:     "请完成需求文档",
		ExpectedOutput:  "需求文档.docx",
		SourceAgentName: "需求分析 Agent",
		Status:          model.HumanTaskStatusPending,
	}

	metadata := buildTaskCardMetadata(task)

	if metadata.Type != "human_task" {
		t.Errorf("metadata.Type = %q, want %q", metadata.Type, "human_task")
	}
	if metadata.TaskID != taskID.String() {
		t.Errorf("metadata.TaskID = %q, want %q", metadata.TaskID, taskID.String())
	}
	if metadata.RoleName != "产品经理" {
		t.Errorf("metadata.RoleName = %q, want %q", metadata.RoleName, "产品经理")
	}
	if metadata.TaskType != model.HumanTaskTypeDispatch {
		t.Errorf("metadata.TaskType = %q, want %q", metadata.TaskType, model.HumanTaskTypeDispatch)
	}
	if metadata.ExpectedOutput != "需求文档.docx" {
		t.Errorf("metadata.ExpectedOutput = %q, want %q", metadata.ExpectedOutput, "需求文档.docx")
	}
	if metadata.SourceAgentName != "需求分析 Agent" {
		t.Errorf("metadata.SourceAgentName = %q, want %q", metadata.SourceAgentName, "需求分析 Agent")
	}
	if metadata.Status != "pending" {
		t.Errorf("metadata.Status = %q, want %q", metadata.Status, "pending")
	}
}

func TestTaskCardMetadataJSON(t *testing.T) {
	taskID := uuid.New()
	threadID := uuid.New()
	roleConfigID := uuid.New()

	task := &model.HumanTask{
		ID:              taskID,
		ThreadID:        threadID,
		RoleConfigID:    roleConfigID,
		RoleName:        "测试工程师",
		TaskType:        model.HumanTaskTypeReview,
		TaskContent:     "请审核代码",
		ExpectedOutput:  "审核报告",
		SourceAgentName: "开发 Agent",
		Status:          model.HumanTaskStatusPending,
	}

	metadata := buildTaskCardMetadata(task)

	// 验证可以正确序列化为 JSON
	jsonData, err := json.Marshal(metadata)
	if err != nil {
		t.Errorf("Failed to marshal metadata to JSON: %v", err)
	}

	// 验证 JSON 包含必要字段
	if len(jsonData) == 0 {
		t.Error("JSON output is empty")
	}

	// 验证可以反序列化
	var unmarshaled TaskCardMetadata
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal JSON: %v", err)
	}

	// 验证反序列化后的值正确
	if unmarshaled.TaskID != taskID.String() {
		t.Errorf("unmarshaled.TaskID = %q, want %q", unmarshaled.TaskID, taskID.String())
	}
	if unmarshaled.RoleName != "测试工程师" {
		t.Errorf("unmarshaled.RoleName = %q, want %q", unmarshaled.RoleName, "测试工程师")
	}
}