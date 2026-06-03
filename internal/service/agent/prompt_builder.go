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
func BuildPrompt(layers *ContextLayers, input string) string {
	var sb strings.Builder

	if layers != nil {
		if layers.Layer0 != "" {
			sb.WriteString("<system>\n")
			sb.WriteString(layers.Layer0)
			sb.WriteString("\n</system>\n\n")
		}
		if layers.Layer1 != "" {
			sb.WriteString("<conversation>\n")
			sb.WriteString(layers.Layer1)
			sb.WriteString("\n</conversation>\n\n")
		}
		if layers.Layer2 != "" {
			sb.WriteString("<artifacts>\n")
			sb.WriteString(layers.Layer2)
			sb.WriteString("\n</artifacts>\n\n")
		}
		if layers.Layer3 != "" {
			sb.WriteString("<environment>\n")
			sb.WriteString(layers.Layer3)
			sb.WriteString("\n</environment>\n\n")
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
	if layers != nil && layers.MemoryContext != "" {
		sb.WriteString("\n\n")
		sb.WriteString(layers.MemoryContext)
	}
	sb.WriteString("\n</")
	sb.WriteString(tag)
	sb.WriteString(">\n")

	return sb.String()
}
