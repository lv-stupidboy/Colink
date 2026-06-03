// Logging configuration for ISDP
// Sets up centralized logging to logs/ directory

package config

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel 日志级别类型
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// ParseLogLevel 解析日志级别字符串
func ParseLogLevel(level string) zapcore.Level {
	switch LogLevel(level) {
	case LogLevelDebug:
		return zapcore.DebugLevel
	case LogLevelInfo:
		return zapcore.InfoLevel
	case LogLevelWarn:
		return zapcore.WarnLevel
	case LogLevelError:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// SetupLogging 初始化日志系统，将日志输出到logs目录
// logLevel 参数支持: debug, info, warn, error
func SetupLogging(logLevel string) (*zap.Logger, error) {
	// 确保logs目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, err
	}

	// 配置日志轮转
	logFile := &lumberjack.Logger{
		Filename:   "logs/server.log", // 统一日志文件位置
		MaxSize:    1,                 // MB
		MaxBackups: 10,
		MaxAge:     30,   // days
		Compress:   true, // disabled by default
	}

	// 设置编码器配置
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// 解析日志级别
	level := ParseLogLevel(logLevel)

	// 创建核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(logFile)),
		level,
	)

	return zap.New(core), nil
}
