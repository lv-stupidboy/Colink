// Logging configuration for ISDP
// Sets up centralized logging to logs/ directory

package config

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// SetupLogging 初始化日志系统，将日志输出到logs目录
func SetupLogging() (*zap.Logger, error) {
	// 确保logs目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, err
	}

	// 配置日志轮转
	logFile := &lumberjack.Logger{
		Filename:   "logs/server.log", // 统一日志文件位置
		MaxSize:    1,                 // MB
		MaxBackups: 10,
		MaxAge:     30,                // days
		Compress:   true,              // disabled by default
	}

	// 设置编码器配置
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// 创建核心
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(logFile)),
		zapcore.InfoLevel,
	)

	return zap.New(core), nil
}