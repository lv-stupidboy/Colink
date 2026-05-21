// internal/service/agent/plugins/open_code/adapter.go
// OpenCode ACP Adapter - renamed from OpenCodeACPAdapter
package open_code

import (
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/agent/plugins/acp"
)

// OpenCodeAdapter implements AgentAdapter using ACP protocol.
// Renamed from OpenCodeACPAdapter (ACP suffix removed).
type OpenCodeAdapter struct {
	*acp.BaseACPAdapter
}

// NewOpenCodeAdapter creates a new OpenCode adapter.
// Renamed from NewOpenCodeACPAdapter.
func NewOpenCodeAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "opencode"
	}

	config := acp.AcpAdapterConfig{
		CliPath: cliPath,
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			return []string{"acp"}
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			// 每次执行前生成/更新 opencode.json 配置文件
			// OpenCode CLI 从配置目录读取配置，而非环境变量
			generateOpenCodeConfig(baseAgent, req)

			env := make([]string, 0, 4)

			// 设置配置目录，OpenCode 会读取其中的 opencode.json
			if req != nil && req.ConfigDir != "" {
				env = append(env, "OPENCODE_CONFIG_DIR="+req.ConfigDir)
			}

			// Git Bash path (Windows)
			if baseAgent.GitBashPath != "" {
				env = append(env, "OPENCODE_GIT_BASH_PATH="+baseAgent.GitBashPath)
			}

			// 阻止用户级插件加载，确保隔离
			env = append(env, "OPENCODE_PURE=1")

			return env
		},
		// 如果通过配置文件配置了模型（有自定义 API），跳过 session/set_model
		// 没有自定义 API 时，OpenCode 使用默认配置，模型由 ACP session/set_model 或 CLI 默认处理
		SkipModelConfig: func(req *agent.ExecutionRequest) bool {
			// 有自定义 API 配置时，配置文件中已包含模型设置，跳过 ACP 的 set_model
			return baseAgent.ApiURL != "" && baseAgent.ApiToken != "" && baseAgent.DefaultModel != ""
		},
	}

	return &OpenCodeAdapter{
		BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
	}
}