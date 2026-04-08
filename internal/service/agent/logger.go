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

// logInfo 记录信息级别日志
func logInfo(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Info(msg, fields...)
	}
}

// logError 记录错误级别日志
func logError(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Error(msg, fields...)
	}
}

// logDebug 记录调试级别日志
func logDebug(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Debug(msg, fields...)
	}
}

// logWarn 记录警告级别日志
func logWarn(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Warn(msg, fields...)
	}
}