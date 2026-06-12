// internal/service/agent/plugins/claude_code/acp_adapter.go
// Claude ACP Adapter - 使用 claude-agent-acp CLI
package claude_code

import (
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/agent/plugins/acp"
)

// ClaudeACPAdapter 使用 ACP 协议的 Claude adapter
// 底层使用 claude-agent-acp CLI（@agentclientprotocol/claude-agent-acp）
type ClaudeACPAdapter struct {
	*acp.BaseACPAdapter
}

// NewClaudeACPAdapter 创建使用 ACP 协议的 Claude adapter
func NewClaudeACPAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "claude-agent-acp" // 默认使用 claude-agent-acp
	}

	config := acp.AcpAdapterConfig{
		CliPath: cliPath,
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			// claude-agent-acp 默认就是 ACP 模式，无需额外参数
			return []string{}
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			env := make([]string, 0, 4)

			// Claude Agent SDK 使用 ANTHROPIC_API_KEY 环境变量
			if baseAgent.ApiToken != "" {
				env = append(env, "ANTHROPIC_API_KEY="+baseAgent.ApiToken)
			}

			// Git Bash 路径（Windows）
			if baseAgent.GitBashPath != "" {
				env = append(env, "CLAUDE_GIT_BASH_PATH="+baseAgent.GitBashPath)
			}

			// 配置目录
			if req != nil && req.ConfigDir != "" {
				env = append(env, "CLAUDE_CONFIG_DIR="+req.ConfigDir)
			}

			return env
		},
		// claude-agent-acp 使用 configOptions 设置模型
		SkipModelConfig: func(req *agent.ExecutionRequest) bool {
			return false
		},
	}

	return &ClaudeACPAdapter{
		BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
	}
}