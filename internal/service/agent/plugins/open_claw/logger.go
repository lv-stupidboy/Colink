// internal/service/agent/plugins/open_claw/logger.go
// OpenClaw 插件日志工具
package open_claw

import (
	"go.uber.org/zap"
)

var (
	logLogger *zap.Logger
)

func init() {
	logLogger = zap.NewNop()
}

// SetLogger 设置日志器（由 adapter_registry.go 调用）
func SetLogger(logger *zap.Logger) {
	logLogger = logger
}

// LogInfo 记录信息日志
func LogInfo(msg string, fields ...zap.Field) {
	logLogger.Info(msg, fields...)
}

// LogError 记录错误日志
func LogError(msg string, fields ...zap.Field) {
	logLogger.Error(msg, fields...)
}

// LogWarn 记录警告日志
func LogWarn(msg string, fields ...zap.Field) {
	logLogger.Warn(msg, fields...)
}

// LogDebug 记录调试日志
func LogDebug(msg string, fields ...zap.Field) {
	logLogger.Debug(msg, fields...)
}