// internal/service/agent/plugins/acp/logger.go
// Logger helper functions - delegates to agent package
package acp

import (
	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

// LogInfo records info level log via agent package
// Exported for reuse by other ACP-based plugins (e.g., Hermes).
func LogInfo(msg string, fields ...zap.Field) {
	agent.LogInfo(msg, fields...)
}

// LogError records error level log via agent package
// Exported for reuse by other ACP-based plugins (e.g., Hermes).
func LogError(msg string, fields ...zap.Field) {
	agent.LogError(msg, fields...)
}

// LogDebug records debug level log via agent package
// Exported for reuse by other ACP-based plugins (e.g., Hermes).
func LogDebug(msg string, fields ...zap.Field) {
	agent.LogDebug(msg, fields...)
}

// LogWarn records warning level log via agent package
// Exported for reuse by other ACP-based plugins (e.g., Hermes).
func LogWarn(msg string, fields ...zap.Field) {
	agent.LogWarn(msg, fields...)
}