// internal/service/agent/plugins/open_claw/config_generator.go
// OpenClaw openclaw.json 配置生成器
package open_claw

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

// OpenClawConfig represents the structure of openclaw.json.
type OpenClawConfig struct {
	Gateway *GatewayConfig `json:"gateway,omitempty"`
	Models  *ModelsConfig  `json:"models,omitempty"`
}

// GatewayConfig represents Gateway server configuration.
type GatewayConfig struct {
	Port int        `json:"port,omitempty"`
	Auth *AuthConfig `json:"auth,omitempty"`
}

// AuthConfig represents Gateway authentication configuration.
type AuthConfig struct {
	Mode string `json:"mode,omitempty"` // "none", "token", "password"
}

// ModelsConfig represents the models section of openclaw.json.
type ModelsConfig struct {
	Providers map[string]ProviderConfig `json:"providers,omitempty"`
}

// ProviderConfig represents a model provider configuration.
type ProviderConfig struct {
	BaseURL string         `json:"baseUrl,omitempty"`
	APIKey  string         `json:"apiKey,omitempty"`
	Models  []ModelDef     `json:"models,omitempty"`
}

// ModelDef represents a model definition.
type ModelDef struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Reasoning bool   `json:"reasoning,omitempty"`
}

// generateOpenClawConfig generates openclaw.json in the ConfigDir.
// OpenClaw reads model and gateway config from openclaw.json.
// 每次执行前检查并更新，确保 BaseAgent 信息变化时同步。
func generateOpenClawConfig(baseAgent *model.BaseAgent, req *agent.ExecutionRequest, gatewayPort int) {
	if req == nil || req.ConfigDir == "" {
		return
	}

	configPath := filepath.Join(req.ConfigDir, "openclaw.json")

	config := buildOpenClawConfig(baseAgent, gatewayPort)

	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		LogError("Failed to marshal OpenClaw config", zap.Error(err))
		return
	}

	// Check if file content is the same
	existingContent, err := os.ReadFile(configPath)
	if err == nil && string(existingContent) == string(content) {
		// Content unchanged, skip update
		return
	}

	// Ensure directory exists
	os.MkdirAll(req.ConfigDir, 0755)

	// Write file
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		LogError("Failed to generate OpenClaw openclaw.json", zap.Error(err))
	} else {
		LogInfo("Generated/Updated OpenClaw openclaw.json",
			zap.String("path", configPath),
			zap.String("model", baseAgent.DefaultModel),
			zap.Int("gatewayPort", gatewayPort))
	}
}

// buildOpenClawConfig builds the OpenClaw configuration structure.
func buildOpenClawConfig(baseAgent *model.BaseAgent, gatewayPort int) OpenClawConfig {
	config := OpenClawConfig{
		Gateway: &GatewayConfig{
			Port: gatewayPort,
			Auth: &AuthConfig{
				Mode: "none", // Local loopback, no auth needed
			},
		},
		Models: buildModelsConfig(baseAgent),
	}

	return config
}

// buildModelsConfig builds the models section based on BaseAgent settings.
func buildModelsConfig(baseAgent *model.BaseAgent) *ModelsConfig {
	if baseAgent.ApiURL != "" || baseAgent.ApiToken != "" {
		// Custom provider configuration
		return &ModelsConfig{
			Providers: map[string]ProviderConfig{
				"custom": {
					BaseURL: baseAgent.ApiURL,
					APIKey:  baseAgent.ApiToken,
					Models: []ModelDef{
						{
							ID:   baseAgent.DefaultModel,
							Name: baseAgent.DefaultModel,
						},
					},
				},
			},
		}
	}

	// Default: Anthropic provider
	// Check if model name suggests reasoning model
	isReasoning := isReasoningModel(baseAgent.DefaultModel)

	return &ModelsConfig{
		Providers: map[string]ProviderConfig{
			"anthropic": {
				Models: []ModelDef{
					{
						ID:        baseAgent.DefaultModel,
						Name:      baseAgent.DefaultModel,
						Reasoning: isReasoning,
					},
				},
			},
		},
	}
}

// isReasoningModel checks if the model name indicates a reasoning model.
func isReasoningModel(modelID string) bool {
	// Claude extended thinking models
	if len(modelID) >= 6 && modelID[:6] == "claude" {
		// All modern Claude models support reasoning
		return true
	}
	return false
}

// buildOpenClawEnv builds environment variables for OpenClaw ACP adapter.
//
// OpenClaw instance-level environment variables:
// - OPENCLAW_STATE_DIR: Override state directory (use agent's ConfigDir)
// - OPENCLAW_CONFIG_PATH: Override config file path
// - OPENCLAW_HIDE_BANNER: Suppress banner output
// - OPENCLAW_SUPPRESS_NOTES: Suppress startup notes
// - OPENCLAW_GATEWAY_TOKEN: Gateway auth token (if required)
//
// Note: OpenClaw reads model config from openclaw.json in the state directory.
func buildOpenClawEnv(baseAgent *model.BaseAgent, req *agent.ExecutionRequest, gatewayToken string) []string {
	env := make([]string, 0, 5)

	// Use agent's config directory as state directory
	if req != nil && req.ConfigDir != "" {
		env = append(env, "OPENCLAW_STATE_DIR="+req.ConfigDir)
		env = append(env, "OPENCLAW_CONFIG_PATH="+filepath.Join(req.ConfigDir, "openclaw.json"))
	}

	// Suppress banner and notes for cleaner ACP output
	env = append(env, "OPENCLAW_HIDE_BANNER=1")
	env = append(env, "OPENCLAW_SUPPRESS_NOTES=1")

	// Add Gateway token if available
	if gatewayToken != "" {
		env = append(env, "OPENCLAW_GATEWAY_TOKEN="+gatewayToken)
	}

	LogInfo("OpenClaw env configured",
		zap.String("model", baseAgent.DefaultModel),
		zap.String("stateDir", req.ConfigDir),
		zap.Bool("hasGatewayToken", gatewayToken != ""))

	return env
}

// maskURL masks sensitive URL for logging.
func maskURL(url string) string {
	if url == "" {
		return "<empty>"
	}
	if len(url) <= 20 {
		return url[:len(url)/2] + "****"
	}
	return url[:10] + "****" + url[len(url)-10:]
}

// GetGatewayPort returns the Gateway port from config or default.
func GetGatewayPort(req *agent.ExecutionRequest) int {
	// For now, use default port
	// In future, could read from BaseAgent settings
	return DefaultGatewayPort
}