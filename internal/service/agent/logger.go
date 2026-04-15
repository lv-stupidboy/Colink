package agent

import (
	"go.uber.org/zap"
)

// sessionLogger 包级别的日志记录器
var sessionLogger *zap.Logger

// SetSessionLogger 设置会话日志记录器
func SetSessionLogger(logger *zap.Logger) {
	sessionLogger = logger
}

// LogInfo 记录信息级别日志（导出供插件使用）
func LogInfo(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Info(msg, fields...)
	}
}

// LogError 记录错误级别日志（导出供插件使用）
func LogError(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Error(msg, fields...)
	}
}

// LogDebug 记录调试级别日志（导出供插件使用）
func LogDebug(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Debug(msg, fields...)
	}
}

// LogWarn 记录警告级别日志（导出供插件使用）
func LogWarn(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Warn(msg, fields...)
	}
}

// 内部使用的小写函数别名（保持 agent 包内部代码兼容）
func logInfo(msg string, fields ...zap.Field) { LogInfo(msg, fields...) }
func logError(msg string, fields ...zap.Field) { LogError(msg, fields...) }
func logDebug(msg string, fields ...zap.Field) { LogDebug(msg, fields...) }
func logWarn(msg string, fields ...zap.Field) { LogWarn(msg, fields...) }