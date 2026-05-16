package tools

import (
	"encoding/json"
	"fmt"

	"github.com/anthropic/isdp/internal/mcp-server/client"
)

// ProjectMemoryTool 项目级记忆工具 - 管理项目规范、技术栈约定、架构决策等跨团队共享记忆
// 绑定到 Project（项目），当前项目下的所有团队可见
type ProjectMemoryTool struct {
	APIURL        string
	InvocationID  string
	CallbackToken string
}

func (t *ProjectMemoryTool) Name() string {
	return "project_memory"
}

func (t *ProjectMemoryTool) Description() string {
	return `项目级记忆工具 - 管理项目规范、技术栈约定、架构决策等跨团队共享记忆。

绑定到 Project（项目），当前项目下的所有团队可见。

**适用场景**:
- 项目规范：API 返回格式、错误码规范
- 技术栈约定：使用的框架、库版本、语言版本
- 架构决策：系统架构、模块划分、数据流设计
- 代码风格：命名规范、目录结构、注释规范

**不适用场景**（使用 team_memory）:
- 团队约定：单个团队内部的协作规则
- 角色分工：特定团队的 Agent 分工

ACTIONS:
- add: 添加新项目记忆条目
- replace: 替换现有条目（通过 old_text 子串匹配）
- remove: 删除条目（通过 old_text 子串匹配）
- search: 搜索项目记忆内容

WHEN TO SAVE:
- 项目级的技术决策
- 跨团队共享的规范
- 架构层面的约定

SKIP: 单个团队的内部约定、临时状态`
}

func (t *ProjectMemoryTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"add", "replace", "remove", "search"},
				"description": "操作类型",
			},
			"projectId": map[string]interface{}{
				"type":        "string",
				"description": "项目ID",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "记忆内容（add/replace 时必需）",
			},
			"oldText": map[string]interface{}{
				"type":        "string",
				"description": "匹配旧条目的子串（replace/remove 时必需）",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "搜索查询词（search 时必需）",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"preference", "decision", "convention", "technical"},
				"description": "内容分类（可选）",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ProjectMemoryTool) Execute(args map[string]interface{}) (interface{}, error) {
	action, ok := args["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action argument must be a string")
	}

	// 调用 ISDP callback API
	client := client.NewAuthClient(t.APIURL, t.InvocationID, t.CallbackToken)

	reqBody := map[string]interface{}{
		"action": action,
		"scope":  "project", // 固定为 project scope
	}

	// 添加可选参数
	if projectId, ok := args["projectId"].(string); ok {
		reqBody["scopeId"] = projectId
	}
	if content, ok := args["content"].(string); ok {
		reqBody["content"] = content
	}
	if oldText, ok := args["oldText"].(string); ok {
		reqBody["oldText"] = oldText
	}
	if query, ok := args["query"].(string); ok {
		reqBody["query"] = query
	}
	if category, ok := args["category"].(string); ok {
		reqBody["category"] = category
	}

	respBody, err := client.CallAPI("POST", "/memory", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to execute project_memory operation: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}