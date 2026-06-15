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
// 读取两个配置文件并合并：
// 1. ~/.claude.json - 旧的用户配置文件
// 2. ~/.claude/.claude.json - claude mcp add 命令保存的配置文件
// 返回格式：{"serverName": {"command": "...", "args": [...], "env": {...}}}
func loadUserMCPConfigACP() map[string]interface{} {
	// 获取用户主目录
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		logInfo("Claude ACP: Failed to get user home directory", zap.Error(err))
		return nil
	}

	// 合并后的 MCP servers 配置
	result := make(map[string]interface{})

	// 配置文件路径列表（按优先级顺序，后面的会覆盖前面的）
	configPaths := []string{
		filepath.Join(userHomeDir, ".claude.json"),               // 旧的用户配置文件
		filepath.Join(userHomeDir, ".claude", ".claude.json"),     // claude mcp add 保存的配置
	}

	for _, configPath := range configPaths {
		mcpServers := loadMCPConfigFromFile(configPath)
		if mcpServers != nil {
			for name, config := range mcpServers {
				result[name] = config
			}
		}
	}

	if len(result) == 0 {
		return nil
	}

	// 记录找到的 MCP servers
	serverNames := make([]string, 0, len(result))
	for name := range result {
		serverNames = append(serverNames, name)
	}
	logInfo("Claude ACP: Loaded user MCP servers", zap.Strings("servers", serverNames))

	return result
}

// loadMCPConfigFromFile 从单个配置文件加载 MCP servers 配置
func loadMCPConfigFromFile(configPath string) map[string]interface{} {
	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logInfo("Claude ACP: Config file not found", zap.String("path", configPath))
		return nil
	}

	// 读取配置文件
	configData, err := os.ReadFile(configPath)
	if err != nil {
		logInfo("Claude ACP: Failed to read config file", zap.Error(err), zap.String("path", configPath))
		return nil
	}

	// 解析 JSON
	var userConfig map[string]interface{}
	if err := json.Unmarshal(configData, &userConfig); err != nil {
		logInfo("Claude ACP: Failed to parse config JSON", zap.Error(err), zap.String("path", configPath))
		return nil
	}

	// 获取 mcpServers 字段
	mcpServers, ok := userConfig["mcpServers"]
	if !ok || mcpServers == nil {
		logInfo("Claude ACP: No mcpServers in config", zap.String("path", configPath))
		return nil
	}

	mcpServersMap, ok := mcpServers.(map[string]interface{})
	if !ok {
		logInfo("Claude ACP: mcpServers is not a map", zap.String("path", configPath))
		return nil
	}

	// 记录该文件中的 MCP servers
	serverNames := make([]string, 0, len(mcpServersMap))
	for name := range mcpServersMap {
		serverNames = append(serverNames, name)
	}
	logInfo("Claude ACP: Loaded MCP servers from file", zap.Strings("servers", serverNames), zap.String("path", configPath))

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
			env := make([]string, 0, 6)

			// 如果使用第三方 API，不传递 ANTHROPIC_API_KEY（通过 gateway authenticate 传递）
			// 如果使用 Anthropic 官方 API，需要传递 ANTHROPIC_API_KEY
			if gatewayBaseURL == "" && baseAgent.ApiToken != "" {
				env = append(env, "ANTHROPIC_API_KEY="+baseAgent.ApiToken)
			}

			// 通过环境变量设置模型（支持自定义模型）
			// claude-agent-acp 不支持 session/set_model，configOptions 会验证模型列表
			if baseAgent.DefaultModel != "" {
				env = append(env, "ANTHROPIC_MODEL="+baseAgent.DefaultModel)
			}

			// 通过 ANTHROPIC_MCP 环境变量传递用户级 MCP 配置
			// 这样 CLI 在启动时就加载 MCP 配置，无需在 RPC 请求中传递
			userMCP := loadUserMCPConfigACP()
			if userMCP != nil && len(userMCP) > 0 {
				mcpConfig := map[string]interface{}{
					"mcpServers": userMCP,
				}
				mcpJSON, err := json.Marshal(mcpConfig)
				if err == nil {
					env = append(env, "ANTHROPIC_MCP="+string(mcpJSON))
					logInfo("Claude ACP: Set ANTHROPIC_MCP env", zap.Int("serverCount", len(userMCP)))
				}
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
		// 跳过 session 级别的模型配置
		// 模型已通过 ANTHROPIC_MODEL 环境变量在 CLI 启动时设置
		SkipModelConfig: func(req *agent.ExecutionRequest) bool {
			return true
		},
		// 不再通过 RPC 请求传递 MCP 配置，已通过 ANTHROPIC_MCP 环境变量设置
		LoadUserMCPConfig: nil,
		// Gateway 配置（用于第三方 API）
		GatewayBaseURL: gatewayBaseURL,
		GatewayHeaders: gatewayHeaders,
	}

	return &ClaudeACPAdapter{
		BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
	}
}