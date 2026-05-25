package config

import (
	"testing"
)

func TestMySQLConfigDefaults(t *testing.T) {
	cfg := &MySQLConfig{}
	cfg.ApplyDefaults()

	if cfg.Port != 3306 {
		t.Errorf("expected default port 3306, got %d", cfg.Port)
	}
	if cfg.Charset != "utf8mb4" {
		t.Errorf("expected default charset utf8mb4, got %s", cfg.Charset)
	}
	if cfg.MaxOpenConns != 10 {
		t.Errorf("expected default MaxOpenConns 10, got %d", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 5 {
		t.Errorf("expected default MaxIdleConns 5, got %d", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 300 {
		t.Errorf("expected default ConnMaxLifetime 300, got %d", cfg.ConnMaxLifetime)
	}
}

func TestDBTypeString(t *testing.T) {
	if string(DBTypeSQLite) != "sqlite" {
		t.Errorf("expected DBTypeSQLite to be 'sqlite', got %s", DBTypeSQLite)
	}
	if string(DBTypeMySQL) != "mysql" {
		t.Errorf("expected DBTypeMySQL to be 'mysql', got %s", DBTypeMySQL)
	}
}

func TestDatabaseConfigDefaults(t *testing.T) {
	cfg := &DatabaseConfig{}
	cfg.ApplyDefaults()

	if cfg.Type != DBTypeSQLite {
		t.Errorf("expected default type sqlite, got %s", cfg.Type)
	}
	// Path 不再有默认值，必须在配置文件中指定
	if cfg.Path != "" {
		t.Errorf("expected empty path (must be configured), got %s", cfg.Path)
	}
}

func TestGitURLConversionDisabled(t *testing.T) {
	cfg := &GitURLConversionConfig{Enabled: false}
	url := "https://gitee.com/colink_1/colinkmarketplace.git"
	result := cfg.ConvertHTTPToSSH(url)
	if result != url {
		t.Errorf("expected %s, got %s", url, result)
	}
}

func TestGitURLConversionNoRules(t *testing.T) {
	cfg := &GitURLConversionConfig{Enabled: true, Rules: nil}
	url := "https://gitee.com/colink_1/colinkmarketplace.git"
	result := cfg.ConvertHTTPToSSH(url)
	if result != url {
		t.Errorf("expected %s when no rules, got %s", url, result)
	}
}

func TestGitURLConversionNonHTTPS(t *testing.T) {
	cfg := &GitURLConversionConfig{
		Enabled: true,
		Rules:   []GitURLConversionRule{{Pattern: "https://gitee.com/", SSHHost: "git@gitee.com"}},
	}
	url := "git@gitee.com:colink_1/colinkmarketplace.git"
	result := cfg.ConvertHTTPToSSH(url)
	if result != url {
		t.Errorf("expected %s for non-HTTPS, got %s", url, result)
	}
}

func TestGitURLConversionMatching(t *testing.T) {
	cfg := &GitURLConversionConfig{
		Enabled: true,
		Rules: []GitURLConversionRule{
			{Pattern: "https://gitee.com/", SSHHost: "git@gitee.com"},
			{Pattern: "https://github.com/", SSHHost: "git@github.com"},
		},
	}
	tests := []struct {
		input    string
		expected string
	}{
		{"https://gitee.com/colink_1/colinkmarketplace.git", "git@gitee.com:colink_1/colinkmarketplace.git"},
		{"https://github.com/owner/repo.git", "git@github.com:owner/repo.git"},
		{"https://gitlab.com/user/project.git", "https://gitlab.com/user/project.git"},
	}
	for _, tt := range tests {
		result := cfg.ConvertHTTPToSSH(tt.input)
		if result != tt.expected {
			t.Errorf("ConvertHTTPToSSH(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}