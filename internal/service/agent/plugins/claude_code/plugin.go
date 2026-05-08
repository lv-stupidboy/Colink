// internal/service/agent/plugins/claude_code/plugin.go
package claude_code

import (
	"github.com/anthropic/isdp/internal/service/agent"
)

func init() {
	agent.RegisterPlugin(agent.PluginMeta{
		Type:                Type,
		Name:                "ClaudeCode",
		Description:         "Anthropic Claude CLI - 使用 claude 命令行工具",
		Factory:             NewClaudeAdapter,
		ConfigDir:           ".claude",
		DefaultPath:         "claude",
		ConfigGeneratorFactory: NewClaudeConfigGenerator,
	})
}