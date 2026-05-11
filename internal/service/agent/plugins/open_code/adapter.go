// internal/service/agent/plugins/open_code/adapter.go
// OpenCode ACP Adapter - renamed from OpenCodeACPAdapter
package open_code

import (
	"encoding/json"

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
			env := make([]string, 0, 3)

			// OPENCODE_API_URL 和 OPENCODE_API_KEY 已确认无效，不传递
			// 使用 OPENCODE_CONFIG_CONTENT 实现实例级配置
			// OPENCODE_PURE=1 阻止用户级插件加载

			if baseAgent.GitBashPath != "" {
				env = append(env, "OPENCODE_GIT_BASH_PATH="+baseAgent.GitBashPath)
			}

			if req != nil && req.ConfigDir != "" {
				env = append(env, "OPENCODE_CONFIG_DIR="+req.ConfigDir)
			}

			// 构造并传递实例级配置
			configContent := buildOpenCodeConfigContent(baseAgent)
			if configContent != "" {
				env = append(env, "OPENCODE_CONFIG_CONTENT="+configContent)
			}

			// 阻止用户级插件加载，确保隔离
			env = append(env, "OPENCODE_PURE=1")

			return env
		},
	}

	return &OpenCodeAdapter{
		BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
	}
}

// buildOpenCodeConfigContent 构造 OPENCODE_CONFIG_CONTENT JSON 配置
// 用于通过环境变量传递实例级的 Provider、Model、API URL、API Key 配置
func buildOpenCodeConfigContent(baseAgent *model.BaseAgent) string {
	if baseAgent.ApiURL == "" && baseAgent.ApiToken == "" && baseAgent.DefaultModel == "" {
		return ""
	}

	// 构造配置结构
	// 使用 "@ai-sdk/openai-compatible" 作为 npm 包，支持 OpenAI Compatible API
	config := openCodeConfig{
		Provider: map[string]openCodeProvider{
			"colink": {
				Name: "Colink Provider",
				Npm:  "@ai-sdk/openai-compatible",
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
	Npm     string                  `json:"npm,omitempty"`  // npm 包名，如 "@ai-sdk/openai-compatible"
	Env     []string                `json:"env,omitempty"`  // 环境变量列表
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