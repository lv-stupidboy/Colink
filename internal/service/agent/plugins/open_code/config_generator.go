// internal/service/agent/plugins/open_code/config_generator.go
// OpenCode opencode.json 配置生成器
package open_code

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

// OpenCodeConfig represents the structure of opencode.json.
// OpenCode CLI reads this configuration from ~/.opencode/opencode.json or OPENCODE_CONFIG_DIR.
type OpenCodeConfig struct {
	Provider map[string]OpenCodeProvider `json:"provider,omitempty"`
	Model    string                      `json:"model,omitempty"`
}

// OpenCodeProvider represents a provider configuration.
type OpenCodeProvider struct {
	Name    string                     `json:"name,omitempty"`
	Npm     string                     `json:"npm,omitempty"`
	Env     []string                   `json:"env,omitempty"`
	Options OpenCodeProviderOptions    `json:"options,omitempty"`
	Models  map[string]OpenCodeModelDef `json:"models,omitempty"`
}

// OpenCodeProviderOptions represents provider options.
type OpenCodeProviderOptions struct {
	APIKey  string `json:"apiKey,omitempty"`
	BaseURL string `json:"baseURL,omitempty"`
}

// OpenCodeModelDef represents a model definition.
type OpenCodeModelDef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// generateOpenCodeConfig generates opencode.json in the ConfigDir.
// OpenCode CLI reads configuration from the config directory.
// 每次执行前检查并更新，确保 BaseAgent 信息变化时同步。
// 如果没有自定义 API 配置，不生成配置文件，让 OpenCode 使用默认配置。
func generateOpenCodeConfig(baseAgent *model.BaseAgent, req *agent.ExecutionRequest) {
	if req == nil || req.ConfigDir == "" {
		return
	}

	configPath := filepath.Join(req.ConfigDir, "opencode.json")

	// 如果没有自定义 API 配置，删除配置文件（如果存在），让 OpenCode 使用默认配置
	if baseAgent.ApiURL == "" && baseAgent.ApiToken == "" {
		// 删除已有的配置文件
		if _, err := os.Stat(configPath); err == nil {
			if err := os.Remove(configPath); err != nil {
				logError("Failed to remove OpenCode opencode.json", zap.Error(err))
			} else {
				logInfo("Removed OpenCode opencode.json (no custom API), using default config")
			}
		}
		return
	}

	config := buildOpenCodeConfigJSON(baseAgent)

	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logError("Failed to marshal OpenCode config", zap.Error(err))
		return
	}

	// Check if file content is the same (避免重复写入)
	existingContent, err := os.ReadFile(configPath)
	if err == nil && string(existingContent) == string(content) {
		// Content unchanged, skip update
		return
	}

	// Ensure directory exists
	os.MkdirAll(req.ConfigDir, 0755)

	// Write file
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		logError("Failed to generate OpenCode opencode.json", zap.Error(err))
	} else {
		logInfo("Generated/Updated OpenCode opencode.json",
			zap.String("path", configPath),
			zap.String("model", baseAgent.DefaultModel))
	}
}

// buildOpenCodeConfigJSON builds the OpenCode configuration structure.
// Uses "custom" as provider name.
// 只在有自定义 API 配置时才生成配置。
func buildOpenCodeConfigJSON(baseAgent *model.BaseAgent) OpenCodeConfig {
	// 自定义 provider 配置
	config := OpenCodeConfig{
		Provider: map[string]OpenCodeProvider{
			"custom": {
				Name: "Custom Provider",
				Npm:  "@ai-sdk/openai-compatible",
				Options: OpenCodeProviderOptions{
					APIKey:  baseAgent.ApiToken,
					BaseURL: baseAgent.ApiURL,
				},
			},
		},
	}

	// 如果指定了模型，添加模型定义和默认使用
	if baseAgent.DefaultModel != "" {
		provider := config.Provider["custom"]
		provider.Models = map[string]OpenCodeModelDef{
			baseAgent.DefaultModel: {
				ID:   baseAgent.DefaultModel,
				Name: baseAgent.DefaultModel,
			},
		}
		config.Provider["custom"] = provider
		config.Model = "custom/" + baseAgent.DefaultModel
	}

	return config
}

// logInfo logs info message
func logInfo(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Info(msg, fields...)
	}
}

// logError logs error message
func logError(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Error(msg, fields...)
	}
}
