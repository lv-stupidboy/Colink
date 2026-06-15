// internal/service/agent/plugins/claude_code/acp_adapter.go
// Claude ACP Adapter - 使用 claude-agent-acp CLI
package claude_code

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/agent/plugins/acp"
	"go.uber.org/zap"
)

// ClaudeACPAdapter 使用 ACP 协议的 Claude adapter
// 底层使用 claude-agent-acp CLI（@agentclientprotocol/claude-agent-acp）
type ClaudeACPAdapter struct {
	*acp.BaseACPAdapter
}

// loadUserMCPConfigACP 从用户级配置文件加载 MCP servers 配置
// 读取 ~/.claude.json 文件中的顶层 mcpServers 字段
// 返回格式：{"serverName": {"command": "...", "args": [...], "env": {...}}}
func loadUserMCPConfigACP() map[string]interface{} {
	// 获取用户主目录
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		logInfo("Claude ACP: Failed to get user home directory", zap.Error(err))
		return nil
	}

	// 用户级配置文件路径
	userConfigPath := filepath.Join(userHomeDir, ".claude.json")

	// 检查文件是否存在
	if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
		logInfo("Claude ACP: User config file not found", zap.String("path", userConfigPath))
		return nil
	}

	// 读取配置文件
	configData, err := os.ReadFile(userConfigPath)
	if err != nil {
		logInfo("Claude ACP: Failed to read user config file", zap.Error(err), zap.String("path", userConfigPath))
		return nil
	}

	// 解析 JSON
	var userConfig map[string]interface{}
	if err := json.Unmarshal(configData, &userConfig); err != nil {
		logInfo("Claude ACP: Failed to parse user config JSON", zap.Error(err), zap.String("path", userConfigPath))
		return nil
	}

	// 获取 mcpServers 字段
	mcpServers, ok := userConfig["mcpServers"]
	if !ok || mcpServers == nil {
		logInfo("Claude ACP: No mcpServers in user config", zap.String("path", userConfigPath))
		return nil
	}

	mcpServersMap, ok := mcpServers.(map[string]interface{})
	if !ok {
		logInfo("Claude ACP: mcpServers is not a map", zap.String("path", userConfigPath))
		return nil
	}

	// 记录找到的 MCP servers
	serverNames := make([]string, 0, len(mcpServersMap))
	for name := range mcpServersMap {
		serverNames = append(serverNames, name)
	}
	logInfo("Claude ACP: Loaded user MCP servers", zap.Strings("servers", serverNames), zap.String("path", userConfigPath))

	return mcpServersMap
}

// NewClaudeACPAdapter 创建使用 ACP 协议的 Claude adapter
func NewClaudeACPAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	// ACP 模式强制使用 claude-agent-acp，忽略数据库中的 cli_path
	// 因为 claude CLI 不支持 ACP 协议，只有 claude-agent-acp 支持
	cliPath := "claude-agent-acp"

	// 构建 Gateway 配置（用于第三方 API）
	// 如果 baseAgent.ApiURL 不为空，说明使用第三方 API（如阿里云百炼）
	var gatewayBaseURL string
	var gatewayHeaders map[string]string
	if baseAgent.ApiURL != "" {
		gatewayBaseURL = baseAgent.ApiURL
		gatewayHeaders = map[string]string{}
		if baseAgent.ApiToken != "" {
			// 第三方 API 通常使用 x-api-key header
			gatewayHeaders["x-api-key"] = baseAgent.ApiToken
		}
	}

	config := acp.AcpAdapterConfig{
		CliPath: cliPath,
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			// claude-agent-acp 默认就是 ACP 模式，无需额外参数
			return []string{}
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			env := make([]string, 0, 4)

			// 如果使用第三方 API，不传递 ANTHROPIC_API_KEY（通过 gateway authenticate 传递）
			// 如果使用 Anthropic 官方 API，需要传递 ANTHROPIC_API_KEY
			if gatewayBaseURL == "" && baseAgent.ApiToken != "" {
				env = append(env, "ANTHROPIC_API_KEY="+baseAgent.ApiToken)
			}

			// Git Bash 路径（Windows）
			if baseAgent.GitBashPath != "" {
				env = append(env, "CLAUDE_GIT_BASH_PATH="+baseAgent.GitBashPath)
			}

			// 配置目录
			if req != nil && req.ConfigDir != "" {
				env = append(env, "CLAUDE_CONFIG_DIR="+req.ConfigDir)
			}

			return env
		},
		// claude-agent-acp 使用 configOptions 设置模型
		SkipModelConfig: func(req *agent.ExecutionRequest) bool {
			return false
		},
		// 用户级 MCP 配置加载函数
		LoadUserMCPConfig: loadUserMCPConfigACP,
		// Gateway 配置（用于第三方 API）
		GatewayBaseURL: gatewayBaseURL,
		GatewayHeaders: gatewayHeaders,
	}

	return &ClaudeACPAdapter{
		BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
	}
}