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
	if cfg.Path != "./data/isdp.db" {
		t.Errorf("expected default path ./data/isdp.db, got %s", cfg.Path)
	}
}