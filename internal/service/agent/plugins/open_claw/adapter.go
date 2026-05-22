// internal/service/agent/plugins/open_claw/adapter.go
// OpenClaw ACP Adapter - Gateway-backed ACP Bridge
package open_claw

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/agent/plugins/acp"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// OpenClawAdapter implements AgentAdapter using ACP protocol via Gateway.
// Unlike Hermes/OpenCode, OpenClaw requires a Gateway daemon running first.
// The ACP bridge connects to Gateway over WebSocket.
type OpenClawAdapter struct {
	*acp.BaseACPAdapter // Reuse ACP implementation
	baseAgent   *model.BaseAgent
	gatewayMgr  *GatewayManager
	gatewayPort int
}

// NewOpenClawAdapter creates a new OpenClaw adapter.
func NewOpenClawAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "openclaw"
	}

	// 使用全局 GatewayManager 单例，确保所有适配器共享同一个 Gateway
	gatewayMgr := GetGlobalGatewayManager()
	gatewayPort := DefaultGatewayPort

	config := acp.AcpAdapterConfig{
		CliPath: cliPath,
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			// Build ACP bridge command arguments
			// Read token dynamically each time (token may change)
			gatewayToken := gatewayMgr.GetGatewayToken()

			sessionKey := buildSessionKey(req)
			args := []string{
				"acp",
				"--url", fmt.Sprintf("ws://127.0.0.1:%d", gatewayPort),
				"--session", sessionKey,
				"--reset-session", // Reset session for each new invocation
			}
			// Add token if available
			if gatewayToken != "" {
				args = append(args, "--token", gatewayToken)
				acp.LogInfo("OpenClaw: using gateway token for ACP bridge",
					zap.String("token", maskToken(gatewayToken)))
			} else {
				acp.LogWarn("OpenClaw: no gateway token found, ACP may fail if auth required")
			}
			return args
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			// Read token dynamically each time
			gatewayToken := gatewayMgr.GetGatewayToken()

			// Generate openclaw.json before each execution
			generateOpenClawConfig(baseAgent, req, gatewayPort)
			return buildOpenClawEnv(baseAgent, req, gatewayToken)
		},
	}

	base := acp.NewBaseACPAdapter(config, baseAgent)

	return &OpenClawAdapter{
		BaseACPAdapter: base,
		baseAgent:      baseAgent,
		gatewayMgr:     gatewayMgr,
		gatewayPort:    gatewayPort,
	}
}

// buildSessionKey builds a unique session key for Colink invocations.
// Uses Gateway's default agent "main" with unique invocation ID.
// Format: agent:main:<invocationID>
func buildSessionKey(req *agent.ExecutionRequest) string {
	if req.InvocationID != uuid.Nil {
		return "agent:main:" + req.InvocationID.String()
	}
	// Fallback to new UUID if InvocationID is not set
	return "agent:main:" + uuid.New().String()
}

// ExecuteWithStream executes the agent with streaming output.
// Unlike Hermes, OpenClaw requires Gateway daemon running first.
// This method ensures Gateway is ready before starting ACP bridge.
func (a *OpenClawAdapter) ExecuteWithStream(ctx context.Context, req *agent.ExecutionRequest, onChunk func(agent.Chunk)) (*agent.ExecutionResult, error) {
	// 1. Start Gateway daemon if not running
	if err := a.gatewayMgr.StartGateway(ctx); err != nil {
		return nil, fmt.Errorf("OpenClaw: failed to start gateway: %w", err)
	}

	// 2. Wait for Gateway ready (使用默认超时 10 秒)
	if err := a.gatewayMgr.WaitForReady(ctx, DefaultGatewayTimeout); err != nil {
		return nil, fmt.Errorf("OpenClaw: gateway not ready: %w", err)
	}

	acp.LogInfo("OpenClaw: Gateway ready, starting ACP bridge",
		zap.Int("port", a.gatewayPort),
		zap.String("url", a.gatewayMgr.GetGatewayURL()))

	// 3. Generate config before execution
	generateOpenClawConfig(a.baseAgent, req, a.gatewayPort)

	// 4. Execute via BaseACPAdapter (reuse Hermes implementation)
	return a.BaseACPAdapter.ExecuteWithStream(ctx, req, onChunk)
}

// Execute executes the agent without streaming (delegates to ExecuteWithStream).
func (a *OpenClawAdapter) Execute(ctx context.Context, req *agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	return a.ExecuteWithStream(ctx, req, nil)
}

// StartSession starts a persistent session.
// Requires Gateway daemon running.
func (a *OpenClawAdapter) StartSession(ctx context.Context, sessionID string, req *agent.ExecutionRequest) error {
	// 1. Start Gateway daemon if not running
	if err := a.gatewayMgr.StartGateway(ctx); err != nil {
		return fmt.Errorf("OpenClaw: failed to start gateway: %w", err)
	}

	// 2. Wait for Gateway ready (使用默认超时 10 秒)
	if err := a.gatewayMgr.WaitForReady(ctx, DefaultGatewayTimeout); err != nil {
		return fmt.Errorf("OpenClaw: gateway not ready: %w", err)
	}

	// 3. Generate config
	generateOpenClawConfig(a.baseAgent, req, a.gatewayPort)

	// 4. Start session via BaseACPAdapter
	return a.BaseACPAdapter.StartSession(ctx, sessionID, req)
}

// CheckHealth checks if OpenClaw CLI and Gateway are available.
func (a *OpenClawAdapter) CheckHealth(ctx context.Context) error {
	// First check if Gateway can be started
	if err := a.gatewayMgr.StartGateway(ctx); err != nil {
		return fmt.Errorf("OpenClaw: gateway health check failed: %w", err)
	}

	// Wait for Gateway ready (健康检查使用更长超时 15 秒)
	if err := a.gatewayMgr.WaitForReady(ctx, 15*time.Second); err != nil {
		return fmt.Errorf("OpenClaw: gateway not ready after 15s: %w", err)
	}

	// Check ACP bridge health via BaseACPAdapter
	return a.BaseACPAdapter.CheckHealth(ctx)
}