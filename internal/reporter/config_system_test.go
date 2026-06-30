package reporter

import (
	"runtime"
	"testing"
	"time"
)

func TestDefaultConfigAndIsRunnable(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Enabled || cfg.Endpoint != "" || cfg.Interval != 30*time.Minute || cfg.RetryTimes != 3 || cfg.RetryInterval != time.Minute {
		t.Fatalf("DefaultConfig = %#v", cfg)
	}
	if cfg.IsRunnable() {
		t.Fatalf("disabled config should not be runnable")
	}
	cfg.Enabled = true
	if cfg.IsRunnable() {
		t.Fatalf("config without endpoint should not be runnable")
	}
	cfg.Endpoint = "https://example.invalid/report"
	if !cfg.IsRunnable() {
		t.Fatalf("enabled config with endpoint should be runnable")
	}
}

func TestGetSystemInfoAndGitUserInfo(t *testing.T) {
	t.Setenv("USER", "colink-user")
	t.Setenv("USERNAME", "")
	info := GetSystemInfo()
	if info.Platform != runtime.GOOS || info.Username != "colink-user" || info.Cwd == "" {
		t.Fatalf("system info = %#v", info)
	}

	gitInfo := GetGitUserInfo()
	if gitInfo.Name == "\n" || gitInfo.Email == "\n" {
		t.Fatalf("git info should be trimmed: %#v", gitInfo)
	}
}
