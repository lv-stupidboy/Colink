// internal/service/agent/plugins/open_code/adapter.go
// OpenCode ACP Adapter - renamed from OpenCodeACPAdapter
package open_code

import (
	"encoding/json"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/agent/plugins/acp"
)

// openCodeProviderID 是注入到 OpenCode 配置中的自定义 provider 名。
// 模型在配置和 ACP set_config_option 两处都必须以 "<provider>/<model>" 形式引用，
// 否则 OpenCode 会把裸模型名当成 provider 解析（报 ProviderModelNotFoundError 或命中错误的能力配置）。
const openCodeProviderID = "colink"

// openCodeModelRef 返回带 provider 前缀的模型引用，如 "colink/qwen3.7-plus"。
func openCodeModelRef(defaultModel string) string {
	if defaultModel == "" {
		return ""
	}
	return openCodeProviderID + "/" + defaultModel
}

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
			env := make([]string, 0, 4)

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
		// OpenCode ACP 模式不读取 OPENCODE_CONFIG_CONTENT 来设置模型
		// 必须通过 session/set_model 设置，所以不跳过
		SkipModelConfig: func(req *agent.ExecutionRequest) bool {
			return false // 始终调用 session/set_model
		},
		// 设置模型时使用带 provider 前缀的引用，与注入配置中的 config.Model 保持一致
		ModelRef: func() string {
			return openCodeModelRef(baseAgent.DefaultModel)
		},
	}

	return &OpenCodeAdapter{
		BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
	}
}

// hasConfigContent 检查是否配置了 CONFIG_CONTENT
func hasConfigContent(baseAgent *model.BaseAgent) bool {
	return baseAgent.ApiURL != "" || baseAgent.ApiToken != "" || baseAgent.DefaultModel != ""
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
			openCodeProviderID: {
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
		provider := config.Provider[openCodeProviderID]
		provider.Models = map[string]openCodeModel{
			baseAgent.DefaultModel: {
				ID:   baseAgent.DefaultModel,
				Name: baseAgent.DefaultModel,
				// 声明模型支持图片/文件附件，否则 OpenCode 默认 false 会拦截图片不发给模型
				Attachment: true,
				// 关键：必须声明 input modalities 含 image，OpenCode 才会把图片真正发给模型；
				// 仅 attachment:true 不够（自定义 openai-compatible provider 不会从 models.dev 推断能力）
				Modalities: &openCodeModalities{
					Input:  []string{"text", "image"},
					Output: []string{"text"},
				},
			},
		}
		config.Provider[openCodeProviderID] = provider
		config.Model = openCodeModelRef(baseAgent.DefaultModel)
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
	Name    string                   `json:"name,omitempty"`
	Npm     string                   `json:"npm,omitempty"` // npm 包名，如 "@ai-sdk/openai-compatible"
	Env     []string                 `json:"env,omitempty"` // 环境变量列表
	Options openCodeProviderOptions  `json:"options,omitempty"`
	Models  map[string]openCodeModel `json:"models,omitempty"`
}

// openCodeProviderOptions Provider 选项
type openCodeProviderOptions struct {
	APIKey  string `json:"apiKey,omitempty"`
	BaseURL string `json:"baseURL,omitempty"`
}

// openCodeModel Model 配置
type openCodeModel struct {
	ID         string              `json:"id,omitempty"`
	Name       string              `json:"name,omitempty"`
	Attachment bool                `json:"attachment,omitempty"` // 是否支持图片/文件附件
	Modalities *openCodeModalities `json:"modalities,omitempty"` // 输入/输出模态（含 image 时才会发送图片）
}

// openCodeModalities 模型输入/输出模态
type openCodeModalities struct {
	Input  []string `json:"input,omitempty"`  // 如 ["text","image"]
	Output []string `json:"output,omitempty"` // 如 ["text"]
}
