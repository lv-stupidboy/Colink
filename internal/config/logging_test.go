package config

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestParseLogLevel(t *testing.T) {
	cases := map[string]zapcore.Level{
		"debug":   zapcore.DebugLevel,
		"info":    zapcore.InfoLevel,
		"warn":    zapcore.WarnLevel,
		"error":   zapcore.ErrorLevel,
		"unknown": zapcore.InfoLevel,
		"":        zapcore.InfoLevel,
	}
	for input, want := range cases {
		if got := ParseLogLevel(input); got != want {
			t.Fatalf("ParseLogLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestSetupLoggingCreatesLogsDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	logger, err := SetupLogging("debug")
	if err != nil {
		t.Fatalf("SetupLogging returned error: %v", err)
	}
	defer logger.Sync()
	logger.Debug("hello")
	if info, err := os.Stat(filepath.Join("logs")); err != nil || !info.IsDir() {
		t.Fatalf("logs directory not created: info=%v err=%v", info, err)
	}
}
