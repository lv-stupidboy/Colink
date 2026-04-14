package agent

import (
	"github.com/anthropic/isdp/internal/model"
)

type OpenCodeACPAdapter struct {
	*BaseACPAdapter
}

func NewOpenCodeACPAdapter(baseAgent *model.BaseAgent) *OpenCodeACPAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "opencode"
	}

	base := &BaseACPAdapter{
		config: acpAdapterConfig{
			cliPath: cliPath,
			buildArgs: func(req *ExecutionRequest) []string {
				return []string{"acp"}
			},
			buildEnv: func(req *ExecutionRequest) []string {
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

	return &OpenCodeACPAdapter{
		BaseACPAdapter: base,
	}
}
