// internal/service/agent/plugins/gemini/adapter.go
// Gemini CLI ACP Adapter
package gemini

import (
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/agent/plugins/acp"
	"go.uber.org/zap"
)

// GeminiAdapter implements AgentAdapter using ACP protocol.
// Gemini CLI uses stdio JSON-RPC via `gemini --acp` command.
type GeminiAdapter struct {
	*acp.BaseACPAdapter
	baseAgent *model.BaseAgent
}

// NewGeminiAdapter creates a new Gemini adapter.
func NewGeminiAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "gemini"
	}

	config := acp.AcpAdapterConfig{
		CliPath: cliPath,
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			args := []string{"--acp"}
			// Gemini CLI supports --model parameter
			if baseAgent.DefaultModel != "" {
				args = append(args, "--model", baseAgent.DefaultModel)
			}
			return args
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			env := make([]string, 0, 4)

			// API Key configuration
			// Gemini CLI supports two authentication modes:
			// 1. GEMINI_API_KEY - for Gemini API (free tier)
			// 2. GOOGLE_API_KEY + GOOGLE_GENAI_USE_VERTEXAI=true - for Vertex AI
			if baseAgent.ApiToken != "" {
				if baseAgent.ApiURL != "" && strings.Contains(baseAgent.ApiURL, "vertex") {
					// Vertex AI mode
					env = append(env, "GOOGLE_API_KEY="+baseAgent.ApiToken)
					env = append(env, "GOOGLE_GENAI_USE_VERTEXAI=true")
				} else {
					// Gemini API mode (free tier)
					env = append(env, "GEMINI_API_KEY="+baseAgent.ApiToken)
				}
			}

			// Custom API URL (if provided and not Vertex)
			if baseAgent.ApiURL != "" && !strings.Contains(baseAgent.ApiURL, "vertex") {
				env = append(env, "GEMINI_API_URL="+baseAgent.ApiURL)
			}

			acp.LogInfo("Gemini env configured",
				zap.String("model", baseAgent.DefaultModel),
				zap.Bool("vertexAI", strings.Contains(baseAgent.ApiURL, "vertex")),
				zap.String("configDir", req.ConfigDir))

			return env
		},
	}

	return &GeminiAdapter{
		BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
		baseAgent:      baseAgent,
	}
}