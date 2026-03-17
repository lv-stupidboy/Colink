package agent

// ExecutionContext 执行上下文枚举，区分不同的执行场景
type ExecutionContext string

const (
	ExecutionContextWorkflow  ExecutionContext = "workflow"  // 工作流场景
	ExecutionContextDebug     ExecutionContext = "debug"     // 调试场景
	ExecutionContextInteractive ExecutionContext = "interactive" // 交互式场景
)