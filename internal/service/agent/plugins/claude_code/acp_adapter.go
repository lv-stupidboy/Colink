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
	// ACP 模式强制使用 claude-agent-acp，忽略数据库中的 cli_path
	// 因为 claude CLI 不支持 ACP 协议，只有 claude-agent-acp 支持
	cliPath := "claude-agent-acp"

	// 构建 Gateway 配置（用于第三方 API）
	// 如果 baseAgent.ApiURL 不为空，说明使用第三方 API（如阿里云百炼）
	var gatewayBaseURL string
	var gatewayHeaders map[string]string
	if baseAgent.ApiURL != "" {
		gatewayBaseURL = baseAgent.ApiURL
		gatewayHeaders = map[string]string{}
		if baseAgent.ApiToken != "" {
			// 第三方 API 通常使用 x-api-key header
			gatewayHeaders["x-api-key"] = baseAgent.ApiToken
		}
	}

	config := acp.AcpAdapterConfig{
		CliPath: cliPath,
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			// claude-agent-acp 默认就是 ACP 模式，无需额外参数
			return []string{}
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			env := make([]string, 0, 5)

			// 如果使用第三方 API，不传递 ANTHROPIC_API_KEY（通过 gateway authenticate 传递）
			// 如果使用 Anthropic 官方 API，需要传递 ANTHROPIC_API_KEY
			if gatewayBaseURL == "" && baseAgent.ApiToken != "" {
				env = append(env, "ANTHROPIC_API_KEY="+baseAgent.ApiToken)
			}

			// 通过环境变量设置模型（支持自定义模型）
			// claude-agent-acp 不支持 session/set_model，configOptions 会验证模型列表
			if baseAgent.DefaultModel != "" {
				env = append(env, "ANTHROPIC_MODEL="+baseAgent.DefaultModel)
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
		// 跳过 session 级别的模型配置
		// 模型已通过 ANTHROPIC_MODEL 环境变量在 CLI 启动时设置
		SkipModelConfig: func(req *agent.ExecutionRequest) bool {
			return true
		},
		// Gateway 配置（用于第三方 API）
		GatewayBaseURL: gatewayBaseURL,
		GatewayHeaders: gatewayHeaders,
	}

	return &ClaudeACPAdapter{
		BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
	}
}
