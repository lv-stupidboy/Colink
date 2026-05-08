// internal/service/agent/plugins/hermes/plugin.go
// Hermes Agent Plugin - 自进化AI代理
package hermes

import (
	"github.com/anthropic/isdp/internal/service/agent"
)

func init() {
	agent.RegisterPlugin(agent.PluginMeta{
		Type:        Type,
		Name:        "Hermes",
		Description: "Hermes Agent via ACP - 自进化AI代理",
		Factory:     NewHermesAdapter,
		ConfigDir:   ".hermes",
		DefaultPath: "hermes",
	})
}