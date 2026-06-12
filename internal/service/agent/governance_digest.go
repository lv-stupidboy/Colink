package agent

import (
	"strings"
)

// GovernanceDigestVersion 治理摘要版本
const GovernanceDigestVersion = "v1.3.1"

// GovernanceDigest 治理规则摘要（嵌入每个 Agent 调用的 L0）
// 参考 clowder-ai GOVERNANCE_L0_DIGEST 设计：编译后约 150 tokens
// 单一真相源：docs/governance/shared-rules.md
// v1.3.0: 合并协作规则，增加下游接力判断强制要求
// v1.3.1: 强化 @mention 格式规则，增加正确/错误示例
var GovernanceDigest = `## ⚠️ 强制规则（必须遵守）

**完成工作后必须判断是否需要下游接力：**
需要 → 另起一行，行首写 @mention（严禁嵌入句子）
不需要 → 回复"无需下游：{原因}"

**@mention 格式（强制）**：
正确：
@前端开发工程师 请实现登录页面

错误（无效，不会触发）：
确认后我将@前端开发工程师进行实现

**落盘记录**：docs/{任务名}-时间戳.md

**交接块**（触发下游时必须）：
<a2a-handoff>
### What | ### Why | ### Next
</a2a-handoff>`

// BuildGovernanceDigest 编译治理规则摘要
// 返回约 150 tokens 的精简版本，嵌入 Layer0
func BuildGovernanceDigest() string {
	return GovernanceDigest
}

// BuildGovernanceDigestWithVersion 带版本的治理摘要
func BuildGovernanceDigestWithVersion() string {
	var sb strings.Builder
	sb.WriteString("<!-- GOVERNANCE_DIGEST_VERSION: ")
	sb.WriteString(GovernanceDigestVersion)
	sb.WriteString(" -->\n\n")
	sb.WriteString(BuildGovernanceDigest())
	sb.WriteString("\n")
	return sb.String()
}

// GovernanceDigestTokens 估算治理摘要 Token 数
func GovernanceDigestTokens() int {
	return EstimateTokens(BuildGovernanceDigest())
}

// ValidateGovernanceDigest 验证摘要 Token 数是否符合约束
// 目标：≤ 200 tokens（留有余量）
func ValidateGovernanceDigest() bool {
	tokens := GovernanceDigestTokens()
	return tokens <= 200
}

// GetGovernanceRulesPath 获取完整治理规则文件路径
func GetGovernanceRulesPath() string {
	return "docs/governance/shared-rules.md"
}