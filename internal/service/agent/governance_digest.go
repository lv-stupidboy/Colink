package agent

import (
	"strings"
)

// GovernanceDigestVersion 治理摘要版本
const GovernanceDigestVersion = "v1.2.1"

// GovernanceDigest 治理规则摘要（嵌入每个 Agent 调用的 L0）
// 参考 clowder-ai GOVERNANCE_L0_DIGEST 设计：编译后约 150 tokens
// 单一真相源：docs/governance/shared-rules.md
var GovernanceDigest = `## ⚠️ 强制规则（必须遵守）

**完成工作后必须立即落盘 + 输出交接块** — 这是强制要求。

执行顺序（不可跳过）：

1. **落盘记录**：docs/{任务名}-时间戳.md
   - 同一任务所有Agent追加到同一文件
   - 格式：[{时间}] {Agent名} - 工作成果、决策、tradeoffs

2. **输出"xx完成"**

3. **输出交接块**（在回复中）：
<a2a-handoff>
### What
落盘记录**：docs/{任务名}-时间戳.md
文件路径 + 操作

### Why
约束/风险

### Tradeoff
放弃的备选

### Open
不确定问题

### Next
希望下游做什么
</a2a-handoff>

4. **@mention 触发下游**

**违规后果**：未落盘或缺少交接块 → 下游无上下文 → 任务失败归咎于你。

---

## 协作守则
出口检查：工作完成后立即执行
- 有下游：落盘 → "xx完成" → 交接块 → @mention → 触发下游
- 无下游：落盘 → "xx完成" → 结束`

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