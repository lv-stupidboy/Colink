// internal/service/agent/plugins/open_code/logger.go
// Logger helper functions - delegates to agent package
package open_code

import (
	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

// logInfo records info level log via agent package
func logInfo(msg string, fields ...zap.Field) {
	agent.LogInfo(msg, fields...)
}

// logError records error level log via agent package
func logError(msg string, fields ...zap.Field) {
	agent.LogError(msg, fields...)
}

// logDebug records debug level log via agent package
func logDebug(msg string, fields ...zap.Field) {
	agent.LogDebug(msg, fields...)
}

// logWarn records warning level log via agent package
func logWarn(msg string, fields ...zap.Field) {
	agent.LogWarn(msg, fields...)
}