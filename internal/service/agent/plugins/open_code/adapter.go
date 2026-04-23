// internal/service/agent/plugins/open_code/adapter.go
// OpenCode ACP Adapter - renamed from OpenCodeACPAdapter
package open_code

import (
	"encoding/json"

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

				// 使用 OPENCODE_CONFIG_CONTENT 传递完整配置（官方支持的方式）
				// 参考：https://opencode.ai/docs/zh-cn/config/
				configContent := buildOpenCodeConfigContent(baseAgent)
				if configContent != "" {
					env = append(env, "OPENCODE_CONFIG_CONTENT="+configContent)
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

// buildOpenCodeConfigContent 构造 OPENCODE_CONFIG_CONTENT JSON 配置
// 用于通过环境变量传递实例级的 Provider、Model、API URL、API Key 配置
func buildOpenCodeConfigContent(baseAgent *model.BaseAgent) string {
	if baseAgent.ApiURL == "" && baseAgent.ApiToken == "" && baseAgent.DefaultModel == "" {
		return ""
	}

	// 构造配置结构
	config := openCodeConfig{
		Provider: map[string]openCodeProvider{
			"colink": {
				Name: "Colink Provider",
				Options: openCodeProviderOptions{
					APIKey:  baseAgent.ApiToken,
					BaseURL: baseAgent.ApiURL,
				},
			},
		},
	}

	// 如果指定了模型，配置模型和默认使用
	if baseAgent.DefaultModel != "" {
		provider := config.Provider["colink"]
		provider.Models = map[string]openCodeModel{
			baseAgent.DefaultModel: {
				ID:   baseAgent.DefaultModel,
				Name: baseAgent.DefaultModel,
			},
		}
		config.Provider["colink"] = provider
		config.Model = "colink/" + baseAgent.DefaultModel
	}

	// 序列化为 JSON
	data, err := json.Marshal(config)
	if err != nil {
		return ""
	}

	return string(data)
}

// openCodeConfig OpenCode 配置结构
type openCodeConfig struct {
	Provider map[string]openCodeProvider `json:"provider,omitempty"`
	Model    string                      `json:"model,omitempty"`
}

// openCodeProvider Provider 配置结构
type openCodeProvider struct {
	Name    string                  `json:"name,omitempty"`
	Options openCodeProviderOptions `json:"options,omitempty"`
	Models  map[string]openCodeModel `json:"models,omitempty"`
}

// openCodeProviderOptions Provider 选项
type openCodeProviderOptions struct {
	APIKey  string `json:"apiKey,omitempty"`
	BaseURL string `json:"baseURL,omitempty"`
}

// openCodeModel Model 配置
type openCodeModel struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}