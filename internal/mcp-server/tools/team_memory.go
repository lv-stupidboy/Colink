package tools

import (
	"encoding/json"
	"fmt"

	"github.com/anthropic/isdp/internal/mcp-server/client"
)

// TeamMemoryTool 团队级记忆工具 - 管理团队约定、协作规则、角色分工等长期共享记忆
// 绑定到 WorkflowTemplate（工作流模板），同一团队的所有 Agent 可见
type TeamMemoryTool struct {
	APIURL        string
	InvocationID  string
	CallbackToken string
}

func (t *TeamMemoryTool) Name() string {
	return "team_memory"
}

func (t *TeamMemoryTool) Description() string {
	return `团队级记忆工具 - 管理团队约定、协作规则、角色分工等长期共享记忆。

绑定到 WorkflowTemplate（工作流模板），同一团队的所有 Agent 可见。

**适用场景**:
- 团队约定：代码审查规则、提交规范
- 协作规则：角色分工、接力顺序
- 会议纪要：重要决策、共识结论
- 技术约定：使用的技术栈、框架版本

**不适用场景**（使用 project_memory）:
- 项目规范：跨团队共享的规范
- 技术栈约定：整个项目的技术决策

ACTIONS:
- add: 添加新团队记忆条目
- replace: 替换现有条目（通过 old_text 子串匹配）
- remove: 删除条目（通过 old_text 子串匹配）
- search: 搜索团队记忆内容

WHEN TO SAVE:
- 团队成员达成共识的约定
- 协作流程中的重要决策
- 发现的团队级事实

SKIP: 临时任务状态、单个 Agent 的执行记录`
}

func (t *TeamMemoryTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"add", "replace", "remove", "search"},
				"description": "操作类型",
			},
			"teamId": map[string]interface{}{
				"type":        "string",
				"description": "团队ID（WorkflowTemplate ID）",
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

func (t *TeamMemoryTool) Execute(args map[string]interface{}) (interface{}, error) {
	action, ok := args["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action argument must be a string")
	}

	// 调用 ISDP callback API
	client := client.NewAuthClient(t.APIURL, t.InvocationID, t.CallbackToken)

	reqBody := map[string]interface{}{
		"action": action,
		"scope":  "team", // 固定为 team scope
	}

	// 添加可选参数
	if teamId, ok := args["teamId"].(string); ok {
		reqBody["scopeId"] = teamId
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
		return nil, fmt.Errorf("failed to execute team_memory operation: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}