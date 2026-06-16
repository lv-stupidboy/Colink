package agent

import "strings"

// BuildPromptFromRequest builds the exact prompt sent to CLI adapters.
func BuildPromptFromRequest(req *ExecutionRequest) string {
	if req == nil {
		return ""
	}
	return BuildPrompt(req.Context, req.Input)
}

// BuildPrompt keeps the stored full prompt and the actual CLI prompt in sync.
// 支持场景化注入：根据 ContextLayers 中各字段是否为空决定是否注入
func BuildPrompt(layers *ContextLayers, input string) string {
	var sb strings.Builder

	if layers != nil {
		// Layer 0: 系统提示（角色定义）- 仅 New 场景注入
		if layers.Layer0 != "" {
			sb.WriteString("<system>\n")
			sb.WriteString(layers.Layer0)
			sb.WriteString("\n</system>\n\n")
		}

		// ChainHistory: A2A 链路历史 - 仅 A2A 场景注入
		if layers.ChainHistory != "" {
			sb.WriteString("<a2a-context>\n")
			sb.WriteString(layers.ChainHistory)
			sb.WriteString("\n</a2a-context>\n\n")
		}

		// Layer 1: Thread 历史 - 不再注入（保留字段用于兼容）
		// Resume 场景 CLI 内部已有；New 场景没有历史；A2A 场景用 ChainHistory
		// 故意不处理 Layer1

		// Layer 2: 工作产物 - 除"单个角色 Resume"外都注入
		if layers.Layer2 != "" {
			sb.WriteString("<artifacts>\n")
			sb.WriteString(layers.Layer2)
			sb.WriteString("\n</artifacts>\n\n")
		}

		// Layer 3: 环境信息 - 除"单个角色 Resume"外都注入
		if layers.Layer3 != "" {
			sb.WriteString("<environment>\n")
			sb.WriteString(layers.Layer3)
			sb.WriteString("\n</environment>\n\n")
		}

		// MemoryContext: 记忆索引 - 除"单个角色 Resume"外都注入
		if layers.MemoryContext != "" {
			sb.WriteString("<memory>\n")
			sb.WriteString(layers.MemoryContext)
			sb.WriteString("\n</memory>\n\n")
		}
	}

	tag := "user"
	if strings.Contains(input, "## 协作规则") || strings.Contains(input, "Direct message from") {
		tag = "a2a_input"
	}

	sb.WriteString("<")
	sb.WriteString(tag)
	sb.WriteString(">\n")
	sb.WriteString(input)
	sb.WriteString("\n</")
	sb.WriteString(tag)
	sb.WriteString(">\n")

	return sb.String()
}
