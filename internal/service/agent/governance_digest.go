package agent

import (
	"strings"
)

// GovernanceDigestVersion 治理摘要版本
const GovernanceDigestVersion = "v1.0.0"

// GovernanceDigest 治理规则摘要（嵌入每个 Agent 调用的 L0）
// 参考 clowder-ai GOVERNANCE_L0_DIGEST 设计：编译后约 150 tokens
// 单一真相源：docs/governance/shared-rules.md
var GovernanceDigest = `## 协作守则
- 出口检查：回复前问"到我这里结束了吗？"→ 三问短路 → @ 或不
- @mention：必须行首单独成行，不能嵌入句子

## 质量约束
- Bug 先定位根因，不盲目试错
- 不确定先确认再继续
- Scope 越界时记录并停止

## 交接规范
- @ 下游时开头输出 <a2a-handoff> 五件套（What/Why/Tradeoff/Open/Next）
- 交接块 Token ≤ 800

## Token 预算
- 窗口：200K（Claude）
- A2A 深度：动态计算，上限 15 层`

// BuildGovernanceDigest 编译治理规则摘要
// 返回约 150 tokens 的精简版本，嵌入 Layer0
// 参考 clowder-ai SystemPromptBuilder 中的 GOVERNANCE_L0_DIGEST
func BuildGovernanceDigest() string {
	return GovernanceDigest
}

// BuildGovernanceDigestWithVersion 带版本的治理摘要
func BuildGovernanceDigestWithVersion() string {
	var sb strings.Builder
	sb.WriteString("<!-- GOVERNANCE_DIGEST_VERSION: ")
	sb.WriteString(GovernanceDigestVersion)
	sb.WriteString(" -->\n\n")
	sb.WriteString(GovernanceDigest)
	sb.WriteString("\n")
	return sb.String()
}

// GovernanceDigestTokens 估算治理摘要 Token 数
func GovernanceDigestTokens() int {
	return EstimateTokens(GovernanceDigest)
}

// ValidateGovernanceDigest 验证摘要 Token 数是否符合约束
// 目标：≤ 200 tokens（留有余量）
func ValidateGovernanceDigest() bool {
	tokens := GovernanceDigestTokens()
	return tokens <= 200
}

// GetGovernanceRules 获取完整治理规则文件路径
func GetGovernanceRulesPath() string {
	return "docs/governance/shared-rules.md"
}