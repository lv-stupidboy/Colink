// internal/service/agent/plugins/open_code/adapter.go
// OpenCode ACP Adapter - renamed from OpenCodeACPAdapter
package open_code

import (
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

// OpenCodeAdapter implements AgentAdapter using ACP protocol.
// Renamed from OpenCodeACPAdapter (ACP suffix removed).
type OpenCodeAdapter struct {
	*BaseACPAdapter
}

// NewOpenCodeAdapter creates a new OpenCode adapter.
// Renamed from NewOpenCodeACPAdapter.
func NewOpenCodeAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "opencode"
	}

	base := &BaseACPAdapter{
		config: acpAdapterConfig{
			cliPath: cliPath,
			buildArgs: func(req *agent.ExecutionRequest) []string {
				return []string{"acp"}
			},
			buildEnv: func(req *agent.ExecutionRequest) []string {
				env := make([]string, 0, 4)

				if baseAgent.ApiURL != "" {
					env = append(env, "OPENCODE_API_URL="+baseAgent.ApiURL)
				}

				if baseAgent.ApiToken != "" {
					env = append(env, "OPENCODE_API_KEY="+baseAgent.ApiToken)
				}

				if baseAgent.GitBashPath != "" {
					env = append(env, "OPENCODE_GIT_BASH_PATH="+baseAgent.GitBashPath)
				}

				if req != nil && req.ConfigDir != "" {
					env = append(env, "OPENCODE_CONFIG_DIR="+req.ConfigDir)
				}

				return env
			},
		},
		baseAgent: baseAgent,
		sessions:  make(map[string]*acpSession),
	}

	return &OpenCodeAdapter{
		BaseACPAdapter: base,
	}
}