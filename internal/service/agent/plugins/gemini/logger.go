// internal/service/agent/plugins/gemini/logger.go
// Logger helper functions - delegates to agent package
package gemini

import (
	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

// LogInfo 记录信息日志
func LogInfo(msg string, fields ...zap.Field) {
	agent.LogInfo(msg, fields...)
}

// LogError 记录错误日志
func LogError(msg string, fields ...zap.Field) {
	agent.LogError(msg, fields...)
}

// LogWarn 记录警告日志
func LogWarn(msg string, fields ...zap.Field) {
	agent.LogWarn(msg, fields...)
}

// LogDebug 记录调试日志
func LogDebug(msg string, fields ...zap.Field) {
	agent.LogDebug(msg, fields...)
}