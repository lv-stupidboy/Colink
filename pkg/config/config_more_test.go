package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDataAndAssetPaths(t *testing.T) {
	data := DataConfig{BasePath: "/data"}
	if data.GetDataPath() != "/data" ||
		data.GetLogsPath() != "/data/logs" ||
		data.GetConfigsPath() != "/data/configs" ||
		data.GetAgentAssetsPath() != "/data/agent-assets" ||
		data.GetAgentConfigsPath() != "/data/agent-configs" ||
		data.GetReposPath() != "/data/repos" ||
		data.GetDBPath() != "" {
		t.Fatalf("unexpected data paths: %#v", data)
	}
	empty := DataConfig{}
	if empty.GetLogsPath() != "" || empty.GetConfigsPath() != "" || empty.GetReposPath() != "" {
		t.Fatalf("empty base path should return empty derived paths")
	}

	cfg := Config{AgentAssets: AgentAssetsConfig{BasePath: "/assets"}}
	if cfg.GetSkillStoragePath() != "/assets/skills" ||
		cfg.GetSubagentStoragePath() != "/assets/subagents" ||
		cfg.GetCommandStoragePath() != "/assets/commands" ||
		cfg.GetRuleStoragePath() != "/assets/rules" ||
		cfg.GetSettingsStoragePath() != "/assets/settings" {
		t.Fatalf("unexpected asset paths")
	}
}

func TestConfigDefaultsAndIntervals(t *testing.T) {
	deployment := DeploymentConfig{}
	deployment.ApplyDefaults()
	if deployment.Type != DeploymentTypeWindows {
		t.Fatalf("deployment default=%s", deployment.Type)
	}

	reporter := ReporterConfig{Enabled: true, Endpoint: "https://example.test", Interval: "bad", RetryInterval: "bad"}
	reporter.ApplyDefaults()
	if !reporter.IsRunnable() || reporter.GetInterval() != 30*time.Minute || reporter.GetRetryInterval() != time.Minute {
		t.Fatalf("unexpected reporter config: %#v", reporter)
	}
	reporter.Interval = "10m"
	reporter.RetryInterval = "5s"
	if reporter.GetInterval() != 10*time.Minute || reporter.GetRetryInterval() != 5*time.Second {
		t.Fatalf("unexpected parsed reporter intervals")
	}

	messageReporter := MessageReporterConfig{Enabled: true, Endpoint: "https://example.test", Interval: "bad", RetryInterval: "bad"}
	messageReporter.ApplyDefaults()
	if messageReporter.BatchSize != 100 || !messageReporter.IsRunnable() || messageReporter.GetInterval() != 30*time.Minute || messageReporter.GetRetryInterval() != time.Minute {
		t.Fatalf("unexpected message reporter config: %#v", messageReporter)
	}

	teamSync := TeamPackageSyncConfig{}
	teamSync.ApplyDefaults()
	if teamSync.TempDir != "temp" {
		t.Fatalf("team sync temp dir=%q", teamSync.TempDir)
	}
}

func TestIMAndFeishuDefaults(t *testing.T) {
	feishu := FeishuConfig{}
	feishu.ApplyDefaults()
	if feishu.LarkCLIPath != "lark-cli" || feishu.EventMode != EventModeListener {
		t.Fatalf("unexpected feishu defaults: %#v", feishu)
	}

	platform := IMPlatformConfig{Type: "feishu"}
	platform.ApplyDefaults()
	if platform.RateLimitMax != 20 ||
		platform.RateLimitWindow != 60*time.Second ||
		platform.MaxRetries != 3 ||
		platform.LarkCLIPath != "lark-cli" ||
		platform.EventMode != EventModeListener {
		t.Fatalf("unexpected platform defaults: %#v", platform)
	}
}

func TestUploadSizeAndIntervals(t *testing.T) {
	if (&SkillConfig{}).GetUseCountUpdateInterval() != time.Hour {
		t.Fatal("expected default skill use count interval")
	}
	if (&SkillConfig{UseCountUpdateInterval: "bad"}).GetUseCountUpdateInterval() != time.Hour {
		t.Fatal("expected invalid skill interval to default")
	}
	if (&SkillConfig{UseCountUpdateInterval: "2h"}).GetUseCountUpdateInterval() != 2*time.Hour {
		t.Fatal("expected parsed skill interval")
	}
	if (&SkillConfig{}).GetUploadMaxSize() != 10*1024*1024 ||
		(&SkillConfig{UploadMaxSize: 3}).GetUploadMaxSize() != 3*1024*1024 ||
		(&SubagentConfig{}).GetUploadMaxSize() != 2*1024*1024 ||
		(&CommandConfig{}).GetUploadMaxSize() != 2*1024*1024 ||
		(&RuleConfig{}).GetUploadMaxSize() != 2*1024*1024 {
		t.Fatal("unexpected upload size")
	}
}

func TestLoadAndValidateConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	mustWriteConfig(t, path, `
data:
  base_path: /data
database:
  type: sqlite
  path: /data/colink.db
agent_assets:
  base_path: /data/assets
agent_config:
  data_dir: /data/agent-configs
deployment:
  type: linux
im:
  platforms:
    - type: feishu
      enabled: true
      app_id: app
      app_secret: secret
      event_mode: listener
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.Type != DBTypeSQLite || cfg.Deployment.Type != DeploymentTypeLinux {
		t.Fatalf("unexpected loaded config: %#v", cfg)
	}
	if cfg.IM.Platforms[0].LarkCLIPath != "lark-cli" {
		t.Fatalf("expected IM defaults to apply: %#v", cfg.IM.Platforms[0])
	}
}

func TestValidateConfigFailures(t *testing.T) {
	base := Config{
		Data:        DataConfig{BasePath: "/data"},
		Database:    DatabaseConfig{Type: DBTypeSQLite, Path: "/data/db.sqlite"},
		AgentAssets: AgentAssetsConfig{BasePath: "/assets"},
		AgentConfig: AgentConfigConfig{DataDir: "/configs"},
		Deployment:  DeploymentConfig{Type: DeploymentTypeLinux},
	}
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"missing data", func(c *Config) { c.Data.BasePath = "" }, "data.base_path"},
		{"missing sqlite path", func(c *Config) { c.Database.Path = "" }, "database.path"},
		{"mysql missing host", func(c *Config) {
			c.Database = DatabaseConfig{Type: DBTypeMySQL, MySQL: MySQLConfig{Database: "db", Username: "u"}}
		}, "mysql.host"},
		{"mysql missing database", func(c *Config) {
			c.Database = DatabaseConfig{Type: DBTypeMySQL, MySQL: MySQLConfig{Host: "h", Username: "u"}}
		}, "mysql.database"},
		{"mysql missing username", func(c *Config) {
			c.Database = DatabaseConfig{Type: DBTypeMySQL, MySQL: MySQLConfig{Host: "h", Database: "db"}}
		}, "mysql.username"},
		{"bad deployment", func(c *Config) { c.Deployment.Type = "bad" }, "deployment.type"},
		{"missing assets", func(c *Config) { c.AgentAssets.BasePath = "" }, "agent_assets.base_path"},
		{"missing agent config", func(c *Config) { c.AgentConfig.DataDir = "" }, "agent_config.data_dir"},
		{"enabled im missing type", func(c *Config) { c.IM.Platforms = []IMPlatformConfig{{Enabled: true}} }, "type"},
		{"feishu missing app id", func(c *Config) { c.IM.Platforms = []IMPlatformConfig{{Enabled: true, Type: "feishu", AppSecret: "s"}} }, "app_id"},
		{"feishu missing app secret", func(c *Config) { c.IM.Platforms = []IMPlatformConfig{{Enabled: true, Type: "feishu", AppID: "a"}} }, "app_secret"},
		{"feishu webhook missing token", func(c *Config) {
			c.IM.Platforms = []IMPlatformConfig{{Enabled: true, Type: "feishu", AppID: "a", AppSecret: "s", EventMode: EventModeWebhook}}
		}, "verification_token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.mutate(&cfg)
			err := validateConfig(&cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}
