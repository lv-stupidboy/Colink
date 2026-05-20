// internal/service/agent/plugins/open_claw/plugin.go
// OpenClaw Agent Plugin - ACP Bridge backed by Gateway
package open_claw

import (
	"github.com/anthropic/isdp/internal/service/agent"
)

func init() {
	agent.RegisterPlugin(agent.PluginMeta{
		Type:                  Type,
		Name:                  "OpenClaw",
		Description:           "OpenClaw Agent via ACP Bridge - AI Agent Platform",
		Factory:               NewOpenClawAdapter,
		ConfigDir:             ".openclaw",
		DefaultPath:           "openclaw",
		ConfigGeneratorFactory: NewOpenClawConfigGenerator,
	})
}