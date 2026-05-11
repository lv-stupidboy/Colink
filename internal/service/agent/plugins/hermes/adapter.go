// internal/service/agent/plugins/hermes/adapter.go
// Hermes ACP Adapter - 自进化AI代理
package hermes

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/agent/plugins/acp"
	"go.uber.org/zap"
)

// HermesAdapter implements AgentAdapter using ACP protocol.
// Inherits from BaseACPAdapter for ACP protocol handling.
type HermesAdapter struct {
	*acp.BaseACPAdapter
	baseAgent *model.BaseAgent
}

// NewHermesAdapter creates a new Hermes adapter.
func NewHermesAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "hermes"
	}

	config := acp.AcpAdapterConfig{
		CliPath: cliPath,
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			return []string{"acp"}
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			// 每次执行前生成/更新 config.yaml
			// Hermes ACP 不从环境变量读取模型配置，必须通过 config.yaml
			generateHermesConfig(baseAgent, req)
			return buildHermesEnv(baseAgent, req)
		},
	}

	base := acp.NewBaseACPAdapter(config, baseAgent)

	return &HermesAdapter{
		BaseACPAdapter: base,
		baseAgent:      baseAgent,
	}
}

// generateHermesConfig generates Hermes config.yaml in the ConfigDir.
// Hermes ACP adapter reads model config from config.yaml, not from environment variables.
// 每次执行前检查并更新，确保 BaseAgent 信息变化时同步。
func generateHermesConfig(baseAgent *model.BaseAgent, req *agent.ExecutionRequest) {
	if req == nil || req.ConfigDir == "" {
		return
	}

	configPath := filepath.Join(req.ConfigDir, "config.yaml")

	// 构建 config.yaml 内容
	var content string
	if baseAgent.ApiURL != "" || baseAgent.ApiToken != "" {
		// 自定义 provider 配置
		content = fmt.Sprintf(`model:
  default: "%s"
  provider: custom
  base_url: "%s"
`, baseAgent.DefaultModel, baseAgent.ApiURL)
	} else {
		// 使用默认配置
		content = fmt.Sprintf(`model:
  default: "%s"
`, baseAgent.DefaultModel)
	}

	// 检查现有文件内容是否相同
	existingContent, err := os.ReadFile(configPath)
	if err == nil && string(existingContent) == content {
		// 内容相同，无需更新
		return
	}

	// 确保目录存在
	os.MkdirAll(req.ConfigDir, 0755)

	// 写入文件
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		acp.LogError("Failed to generate Hermes config.yaml", zap.Error(err))
	} else {
		acp.LogInfo("Generated/Updated Hermes config.yaml",
			zap.String("path", configPath),
			zap.String("model", baseAgent.DefaultModel))
	}
}

// buildHermesEnv builds environment variables for Hermes ACP adapter.
//
// Hermes instance-level environment variables (confirmed from source code):
// - HERMES_INFERENCE_MODEL: Override model selection (not used by ACP)
// - HERMES_INFERENCE_PROVIDER: Override provider selection (e.g., "custom")
// - CUSTOM_BASE_URL: Custom API endpoint URL for custom provider
// - OPENAI_API_KEY: API key for custom provider (fallback chain includes this)
// - HERMES_HOME: Override config directory (use agent's ConfigDir)
//
// IMPORTANT: Hermes ACP prioritizes config.yaml over environment variables for model config.
// We use HERMES_HOME to point to the agent's config directory (req.ConfigDir),
// which contains config.yaml with model settings.
//
// Reference: hermes_cli/runtime_provider.py lines 300-315, 578-614
func buildHermesEnv(baseAgent *model.BaseAgent, req *agent.ExecutionRequest) []string {
	env := make([]string, 0, 8)

	// 自定义 API 配置（环境变量用于 runtime provider）
	if baseAgent.ApiURL != "" || baseAgent.ApiToken != "" {
		// 使用 custom provider
		env = append(env, "HERMES_INFERENCE_PROVIDER=custom")

		// 设置自定义 endpoint URL
		if baseAgent.ApiURL != "" {
			env = append(env, "CUSTOM_BASE_URL="+baseAgent.ApiURL)
		}

		// 设置 API key（Hermes custom provider 使用 OPENAI_API_KEY）
		if baseAgent.ApiToken != "" {
			env = append(env, "OPENAI_API_KEY="+baseAgent.ApiToken)
		}
	}

	// 使用角色的配置目录作为 HERMES_HOME
	// config.yaml 在这个目录下，包含模型配置
	if req != nil && req.ConfigDir != "" {
		env = append(env, "HERMES_HOME="+req.ConfigDir)
	}

	acp.LogInfo("Hermes env configured",
		zap.String("model", baseAgent.DefaultModel),
		zap.String("provider", "custom"),
		zap.String("baseURL", maskURL(baseAgent.ApiURL)),
		zap.String("apiKey", maskToken(baseAgent.ApiToken)),
		zap.String("hermesHome", req.ConfigDir))

	return env
}

// maskURL masks sensitive URL for logging
func maskURL(url string) string {
	if url == "" {
		return "<empty>"
	}
	if len(url) <= 20 {
		return url[:len(url)/2] + "****"
	}
	return url[:10] + "****" + url[len(url)-10:]
}

// maskToken masks sensitive token for logging
func maskToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}