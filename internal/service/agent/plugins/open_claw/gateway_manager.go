// internal/service/agent/plugins/open_claw/gateway_manager.go
// OpenClaw Gateway daemon 管理
package open_claw

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DefaultGatewayPort is the default WebSocket port for OpenClaw Gateway.
const DefaultGatewayPort = 18789

// GatewayManager manages the OpenClaw Gateway daemon lifecycle.
// It ensures Gateway is running before ACP bridge connects.
type GatewayManager struct {
	port       int
	stateDir   string
	configPath string
	cmd        *exec.Cmd
	mu         sync.Mutex
}

// NewGatewayManager creates a new GatewayManager.
func NewGatewayManager(port int, configDir string) *GatewayManager {
	stateDir := configDir
	if stateDir == "" {
		stateDir = filepath.Join(os.Getenv("HOME"), ".openclaw")
	}
	configPath := filepath.Join(stateDir, "openclaw.json")

	return &GatewayManager{
		port:       port,
		stateDir:   stateDir,
		configPath: configPath,
	}
}

// StartGateway starts the Gateway daemon if not already running.
// Returns nil if Gateway is already running or successfully started.
func (g *GatewayManager) StartGateway(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if Gateway is already running
	if g.isGatewayRunning() {
		LogInfo("OpenClaw: Gateway already running",
			zap.Int("port", g.port))
		return nil
	}

	// Start Gateway daemon
	cmd := exec.CommandContext(ctx, "openclaw", "gateway")
	hideCommandLineWindow(cmd)

	// Set environment variables
	cmd.Env = g.buildGatewayEnv()

	// Gateway runs in background, we don't need to capture output
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("OpenClaw: failed to start gateway: %w", err)
	}

	g.cmd = cmd

	LogInfo("OpenClaw: Gateway started",
		zap.Int("port", g.port),
		zap.String("stateDir", g.stateDir),
		zap.Int("pid", cmd.Process.Pid))

	return nil
}

// WaitForReady waits for Gateway to be ready (listening on port).
// Returns error if Gateway is not ready within the timeout.
func (g *GatewayManager) WaitForReady(ctx context.Context, timeout time.Duration) error {
 deadline := time.Now().Add(timeout)

 for {
	 if time.Now().After(deadline) {
		 return fmt.Errorf("OpenClaw: gateway not ready after %v", timeout)
	 }

	 if g.isGatewayRunning() {
		 return nil
	 }

	 // Wait a bit before checking again
	 select {
	 case <-ctx.Done():
		 return ctx.Err()
	 case <-time.After(100 * time.Millisecond):
	 }
 }
}

// isGatewayRunning checks if Gateway is listening on the port.
func (g *GatewayManager) isGatewayRunning() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", g.port), 1*time.Second)
	if err != nil {
	 return false
	}
	conn.Close()
	return true
}

// StopGateway stops the Gateway daemon if started by this manager.
// Note: We typically don't stop Gateway as it can be shared across instances.
func (g *GatewayManager) StopGateway() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cmd == nil || g.cmd.Process == nil {
		return nil
	}

	// Gracefully stop Gateway
	if err := g.cmd.Process.Signal(os.Interrupt); err != nil {
		// Force kill if interrupt fails
		g.cmd.Process.Kill()
	}

	done := make(chan error, 1)
	go func() {
	 done <- g.cmd.Wait()
	}()

	select {
	case <-done:
	 LogInfo("OpenClaw: Gateway stopped", zap.Int("port", g.port))
	case <-time.After(5 * time.Second):
	 g.cmd.Process.Kill()
	 LogWarn("OpenClaw: Gateway killed after timeout")
	}

	g.cmd = nil
	return nil
}

// GetGatewayURL returns the WebSocket URL for the Gateway.
func (g *GatewayManager) GetGatewayURL() string {
	return fmt.Sprintf("ws://127.0.0.1:%d", g.port)
}

// GetGatewayToken reads the Gateway auth token from ~/.openclaw/openclaw.json.
// Returns empty string if no token is configured or file doesn't exist.
func (g *GatewayManager) GetGatewayToken() string {
	// Gateway token is stored in user's home directory config
	homeConfigPath := filepath.Join(os.Getenv("HOME"), ".openclaw", "openclaw.json")

	content, err := os.ReadFile(homeConfigPath)
	if err != nil {
		LogDebug("OpenClaw: no home config found for gateway token",
			zap.String("path", homeConfigPath))
		return ""
	}

	var config struct {
		Gateway struct {
			Auth struct {
				Mode  string `json:"mode"`
				Token string `json:"token"`
			} `json:"auth"`
		} `json:"gateway"`
	}

	if err := json.Unmarshal(content, &config); err != nil {
		LogWarn("OpenClaw: failed to parse gateway config", zap.Error(err))
		return ""
	}

	if config.Gateway.Auth.Token != "" {
		LogInfo("OpenClaw: found gateway token",
			zap.String("mode", config.Gateway.Auth.Mode),
			zap.String("token", maskToken(config.Gateway.Auth.Token)))
		return config.Gateway.Auth.Token
	}

	return ""
}

// maskToken masks sensitive token for logging.
func maskToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// buildGatewayEnv builds environment variables for Gateway daemon.
func (g *GatewayManager) buildGatewayEnv() []string {
	env := os.Environ()

 // OpenClaw state directory
 if g.stateDir != "" {
	 env = append(env, "OPENCLAW_STATE_DIR="+g.stateDir)
 }

 // Config file path
 if g.configPath != "" {
	 env = append(env, "OPENCLAW_CONFIG_PATH="+g.configPath)
 }

 // Suppress banner for cleaner ACP output
 env = append(env, "OPENCLAW_HIDE_BANNER=1")
 env = append(env, "OPENCLAW_SUPPRESS_NOTES=1")

 return env
}