// internal/service/agent/plugins/open_code/plugin.go
package open_code

import (
	"github.com/anthropic/isdp/internal/service/agent"
)

func init() {
	agent.RegisterPlugin(agent.PluginMeta{
		Type:                Type,
		Name:                "OpenCode",
		Description:         "OpenCode CLI via ACP - 结构化输出",
		Factory:             NewOpenCodeAdapter,
		ConfigDir:           ".opencode",
		DefaultPath:         "opencode",
		ConfigGeneratorFactory: NewOpenCodeConfigGenerator,
	})
}