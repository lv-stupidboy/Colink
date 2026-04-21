package agent

import (
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
)

// BuildStaticLayer0 构建静态 L0 层（角色定义 + 治理摘要）
// 参考 clowder-ai SystemPromptBuilder 中的 L0 结构
func BuildStaticLayer0(config *model.AgentConfig) string {
	var sb strings.Builder

	// 版本标记
	sb.WriteString(BuildGovernanceDigestWithVersion())

	// 角色定义
	sb.WriteString("\n<role>\n")
	sb.WriteString(fmt.Sprintf("## 角色: %s\n", config.Name))
	if config.Description != "" {
		sb.WriteString(fmt.Sprintf("描述: %s\n", config.Description))
	}
	sb.WriteString("</role>\n\n")

	// 系统提示（如果存在）
	if config.SystemPrompt != "" {
		sb.WriteString("<system_prompt>\n")
		sb.WriteString(config.SystemPrompt)
		sb.WriteString("\n</system_prompt>\n")
	}

	return sb.String()
}

// BuildStaticLayer0Minimal 构建最小 L0 层（仅角色定义，不含治理摘要）
// 用于不需要 A2A 协作的简单场景
func BuildStaticLayer0Minimal(config *model.AgentConfig) string {
	var sb strings.Builder

	// 角色定义
	sb.WriteString("<role>\n")
	sb.WriteString(fmt.Sprintf("## 角色: %s\n", config.Name))
	if config.Description != "" {
		sb.WriteString(fmt.Sprintf("描述: %s\n", config.Description))
	}
	sb.WriteString("</role>\n\n")

	// 系统提示（如果存在）
	if config.SystemPrompt != "" {
		sb.WriteString("<system_prompt>\n")
		sb.WriteString(config.SystemPrompt)
		sb.WriteString("\n</system_prompt>\n")
	}

	return sb.String()
}