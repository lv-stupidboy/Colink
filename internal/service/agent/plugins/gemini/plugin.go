// internal/service/agent/plugins/gemini/plugin.go
// Gemini CLI plugin registration
package gemini

import "github.com/anthropic/isdp/internal/service/agent"

func init() {
	agent.RegisterPlugin(agent.PluginMeta{
		Type:        Type,
		Name:        "Gemini",
		Description: "Google Gemini CLI via ACP - Free tier AI agent",
		Factory:     NewGeminiAdapter,
		ConfigDir:   ".gemini",
		DefaultPath: "gemini",
	})
}