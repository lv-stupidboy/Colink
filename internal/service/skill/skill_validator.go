package skill

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationResult 校验结果
type ValidationResult struct {
	Valid       bool     `json:"valid"`        // 是否合规
	Errors      []string `json:"errors"`       // 错误列表
	Suggestions []string `json:"suggestions"`  // 补充建议
}

// SkillValidator Skill 输出校验器
// 参考 clowder-ai cross-cat-handoff 校验规则
type SkillValidator struct {
	registry *SkillRegistry
}

// NewSkillValidator 创建 Skill 校验器
func NewSkillValidator(registry *SkillRegistry) *SkillValidator {
	return &SkillValidator{
		registry: registry,
	}
}

// ValidateHandoff 校验 a2a-handoff 输出格式
// 规则（参考 clowder-ai cross-cat-handoff）：
// 1. 检测到 @mention 时必须存在 <a2a-handoff> 块
// 2. 五件套字段必须完整（What/Why/Tradeoff/Open Questions/Next Action）
// 3. 每个部分至少一句话（内容长度 >= 10）
// 注意：文件路径是场景相关的，不强制要求
func (v *SkillValidator) ValidateHandoff(output string) ValidationResult {
	result := ValidationResult{
		Valid:       true,
		Errors:      []string{},
		Suggestions: []string{},
	}

	// 1. 检测是否存在 @mention（行首）
	hasMention := v.detectMention(output)

	// 2. 检测是否存在 <a2a-handoff> 块
	handoffContent, hasHandoff := v.extractHandoffBlock(output)

	// 规则 1: @mention 时必须存在 handoff 块
	if hasMention && !hasHandoff {
		result.Valid = false
		result.Errors = append(result.Errors, "检测到 @mention 但缺少 <a2a-handoff> 交接块")
		result.Suggestions = append(result.Suggestions, "请在回复开头添加 <a2a-handoff> 五件套交接信息")
		return result
	}

	// 如果没有 @mention，不需要校验
	if !hasMention {
		return result
	}

	// 规则 2: 五件套字段完整性
	v.validateFiveParts(handoffContent, &result)

	// 规则 3: 文件路径校验（放宽为警告，不强制）
	// 某些场景（如需求分析、新建项目）没有文件路径是正常的
	v.checkFilePathSuggestion(handoffContent, &result)

	return result
}

// detectMention 检测行首 @mention
func (v *SkillValidator) detectMention(output string) bool {
	lines := strings.Split(output, "\n")
	// 支持中文字符：使用 Unicode 字符类
	mentionRe := regexp.MustCompile(`^@[\p{L}\w-]+`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if mentionRe.MatchString(trimmed) {
			return true
		}
	}

	return false
}

// DetectMentionWithLog 检测并记录日志（调试用）
func (v *SkillValidator) DetectMentionWithLog(output string) bool {
	result := v.detectMention(output)
	// 调试输出：显示前几行内容
	lines := strings.Split(output, "\n")
	sample := ""
	for i, line := range lines {
		if i < 5 {
			sample += line + "\\n"
		}
	}
	fmt.Printf("[SkillValidator] detectMention result: %v, sample lines: %s\n", result, sample)
	return result
}

// extractHandoffBlock 提取 <a2a-handoff> 块内容
func (v *SkillValidator) extractHandoffBlock(output string) (string, bool) {
	re := regexp.MustCompile(`(?s)<a2a-handoff>(.*?)</a2a-handoff>`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", false
	}

	content := strings.TrimSpace(matches[1])
	if content == "" {
		return "", false
	}

	return content, true
}

// validateFiveParts 校验五件套完整性
func (v *SkillValidator) validateFiveParts(handoffContent string, result *ValidationResult) {
	requiredParts := []string{
		"### What",
		"### Why",
		"### Tradeoff",
		"### Open Questions",
		"### Next Action",
	}

	for _, part := range requiredParts {
		if !strings.Contains(handoffContent, part) {
			result.Valid = false
			result.Errors = append(result.Errors, "缺少必填字段: "+part)
			result.Suggestions = append(result.Suggestions, "请补充 "+part+" 部分")
		} else {
			// 检查字段是否有内容（至少一句话）
			partContent := v.extractPartContent(handoffContent, part)
			if len(strings.TrimSpace(partContent)) < 10 {
				result.Valid = false
				result.Errors = append(result.Errors, part+" 内容过短，需要至少一句话")
			}
		}
	}
}

// validateFilePaths 校验 What 部分包含文件路径（已废弃，改用 checkFilePathSuggestion）
// 保留用于向后兼容
func (v *SkillValidator) validateFilePaths(handoffContent string, result *ValidationResult) {
	v.checkFilePathSuggestion(handoffContent, result)
}

// checkFilePathSuggestion 检查文件路径建议（不强制，只提示）
// 参考 clowder-ai：交付件应落地，下游 Agent 才能看到详情
func (v *SkillValidator) checkFilePathSuggestion(handoffContent string, result *ValidationResult) {
	whatContent := v.extractPartContent(handoffContent, "### What")

	// 文件路径正则：支持多种格式
	filePatterns := []string{
		`[a-zA-Z0-9_\-/]+\.(go|ts|tsx|js|jsx|py|java|kt|rs|c|cpp|h|sql|yaml|yml|json|md|css|html)`,
		`\./[^\s]+`,
		`internal/[^\s]+`,
		`src/[^\s]+`,
		`docs/[^\s]+`, // 交付件存放路径
	}

	hasFilePath := false
	for _, pattern := range filePatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(whatContent) {
			hasFilePath = true
			break
		}
	}

	// 只作为建议，不阻止路由
	if !hasFilePath {
		result.Suggestions = append(result.Suggestions, "建议：请将分析结果/审查报告写入 docs/{任务名}-{时间戳}.md，并在 What 中列出文件路径")
	}
}

// extractPartContent 提取指定部分的内容
func (v *SkillValidator) extractPartContent(handoffContent, partHeader string) string {
	// 查找部分标题
	idx := strings.Index(handoffContent, partHeader)
	if idx == -1 {
		return ""
	}

	// 从标题后开始
	start := idx + len(partHeader)

	// 查找下一个 ### 标记作为结束
	nextPart := strings.Index(handoffContent[start:], "### ")
	if nextPart != -1 {
		return strings.TrimSpace(handoffContent[start : start+nextPart])
	}

	// 查找 </a2a-handoff> 作为结束
	endTag := strings.Index(handoffContent[start:], "</a2a-handoff>")
	if endTag != -1 {
		return strings.TrimSpace(handoffContent[start : start+endTag])
	}

	return strings.TrimSpace(handoffContent[start:])
}

// ShouldRejectRouting 是否应该拒绝触发下游
// 当校验不通过时，返回 true 和错误信息
func (v *SkillValidator) ShouldRejectRouting(output string) (bool, string) {
	result := v.ValidateHandoff(output)

	if result.Valid {
		return false, ""
	}

	// 构建错误信息
	errorMsg := "❌ A2A 交接格式校验失败\n\n"
	for _, err := range result.Errors {
		errorMsg += "- " + err + "\n"
	}

	if len(result.Suggestions) > 0 {
		errorMsg += "\n💡 建议:\n"
		for _, suggestion := range result.Suggestions {
			errorMsg += "- " + suggestion + "\n"
		}
	}

	return true, errorMsg
}

// GetRejectionPrompt 获取拒绝提示（用于返回给 Agent）
func (v *SkillValidator) GetRejectionPrompt(output string) string {
	shouldReject, errorMsg := v.ShouldRejectRouting(output)

	if !shouldReject {
		return ""
	}

	return errorMsg + "\n请补充交接信息后重新发送。"
}