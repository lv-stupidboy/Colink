package agent

import "strings"

func BuildMemoryToolGovernance() string {
	var sb strings.Builder
	sb.WriteString("\n## 记忆维护规则\n\n")
	sb.WriteString("Colink 记忆采用 auto memory index 机制：启动上下文只注入团队/项目 MEMORY.md 索引中的前 30 条链接项，不会自动加载链接指向的 topic 文件正文。\n")
	sb.WriteString("- 根据 MEMORY.md 中的标题、文件名和摘要判断某个 topic 是否与当前任务相关。\n")
	sb.WriteString("- 当某个 topic 相关时，先用标准文件工具读取链接的 `.md` topic 文件，再依赖其中的细节。\n")
	sb.WriteString("- 不要预先读取全部 topic 文件；没有读取 topic 文件前，不要声称知道该文件的详细内容。\n\n")
	sb.WriteString("你可以维护团队记忆和项目记忆，但只有在你显式调用记忆 MCP 工具并收到成功结果后，才能在回复中说记忆已经保存或更新。\n\n")
	sb.WriteString("- 写团队记忆：调用 `memory.add`，传 `type=team`。适用于用户偏好、Agent 角色职责、上游/下游关系、团队协作规则、跨项目复用约定。\n")
	sb.WriteString("- 写项目记忆：调用 `memory.add`，传 `type=project`。适用于当前 workspace 的端口约束、项目命令、目录结构、技术栈、测试规则、实现约定。\n")
	sb.WriteString("- 查记忆：优先根据注入的 MEMORY.md 索引用标准文件工具读取相关 topic 文件；也可以调用 `memory.search`，需要限定范围时传 `type=team` 或 `type=project`。\n")
	sb.WriteString("- 保存前由当前 Agent 压缩，不要把完整聊天记录、推理过程、寒暄、确认话术、一次性状态写入记忆。\n")
	sb.WriteString("- 优先传结构化字段：`topic` 是归档主题，`facts` 是长期事实/规则/关系，`usage` 是使用场景。`content` 只作为原始上下文或降级备用。\n")
	sb.WriteString("- 如果你在回答中总结了新的稳定事实，并且这些事实值得长期复用，应先调用记忆工具保存；工具成功后再告诉用户已保存。\n")
	sb.WriteString("- 如果你没有调用记忆工具，不要声称已经保存，也不要假设系统会在后台自动保存。可以询问用户是否需要保存。\n")
	sb.WriteString("- Agent 角色记忆必须包含角色名、职责、上游关系和下游关系；缺失关系时明确写“未配置上游”或“未配置下游”。\n")
	sb.WriteString("- 对团队角色分工这类系统配置事实，如果你判断后续协作需要复用，应写入团队记忆，例如 topic=`agent_roles`。\n")
	return sb.String()
}
