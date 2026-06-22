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
			// --port 26307 固定 HTTP API 端口，让 ISDP 能通过 localhost 调用
			// OpenCode 的 /question REST API 来 reply question 工具的用户答案
			return []string{"acp", "--port", "26307"}
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

			// 显式开启 question 工具：OpenCode 在 ACP 模式下会把 process.env.OPENCODE_CLIENT
			// 设成 "acp"（cli/cmd/acp.ts:23），而 tool/registry.ts:196 里
			//   questionEnabled = ["app","cli","desktop"].includes(flags.client) || flags.enableQuestionTool
			// "acp" 不在白名单里，question 工具默认不注册——LLM 看不到这个工具，
			// 用户问"用 question 工具问我"时模型只能回 "我没有 question 工具"。
			// 这个标志就是 OpenCode 给"非 cli 客户端但仍想用 question"留的口子。
			env = append(env, "OPENCODE_ENABLE_QUESTION_TOOL=1")

			return env
		},
		// OpenCode ACP 模式不读取 OPENCODE_CONFIG_CONTENT 来设置模型
		// 必须通过 session/set_config_option 设置
		// 关键：set_config_option 应使用裸模型名（不带 provider 前缀）
		// 否则 OpenCode 无法正确匹配模型配置，导致返回空响应
		SkipModelConfig: func(req *agent.ExecutionRequest) bool {
			return false // 始终调用 session/set_config_option
		},
		// 不设置 ModelRef，使用默认的 baseAgent.DefaultModel（裸模型名）
		// 配置文件中的 model 字段使用 "colink/model" 格式，但 set_config_option 使用 "model" 格式
		// ModelRefForSetModel 用于 1.3.3 等旧版 OpenCode：它们只有 session/set_model
		// 没有 set_config_option，而 set_model 的 parseModelSelection 需要 providerID/modelID 拆分。
		ModelRefForSetModel: func() string {
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
		// OpenCode 全局放行：等价于 Claude CLI 的 --dangerously-skip-permissions。
		// 让 OpenCode 自身跳过 session/request_permission，避免 read 工具访问
		// workspace 外路径（external_directory 默认 ask）时挂死。
		Permission: "allow",
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
	// Permission 走 OpenCode 官方的"全局放行"语法（详见 OpenCode permissions 文档）：
	//   "permission": "allow"               // 等价于所有工具 *: allow
	//   "permission": {"*": "allow"}        // 等价写法
	// 我们在 ISDP 内通过 ACP 与 OpenCode 一起跑，并不需要 CLI 再向用户弹权限确认；
	// 把这里设成 "allow" 之后 OpenCode 不会再发 session/request_permission，
	// read 等工具读 workspace 外文件（external_directory 默认是 ask）也不会卡住。
	// 客户端侧 handleServerRequest 仍然保留作为兜底（万一某些工具/配置仍触发请求）。
	Permission interface{} `json:"permission,omitempty"`
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
