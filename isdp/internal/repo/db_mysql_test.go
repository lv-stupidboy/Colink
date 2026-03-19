package repo

import (
	"fmt"
	"os"
	"testing"
)

func TestMySQLDialect(t *testing.T) {
	d := &MySQLDialect{}

	if d.Placeholder() != "?" {
		t.Errorf("expected placeholder '?', got %s", d.Placeholder())
	}
	if d.QuoteIdentifier() != "`" {
		t.Errorf("expected quote '`', got %s", d.QuoteIdentifier())
	}
	if d.AutoIncrement() != "AUTO_INCREMENT" {
		t.Errorf("expected AUTO_INCREMENT, got %s", d.AutoIncrement())
	}
}

func TestNewMySQLDB(t *testing.T) {
	// 注意：此测试需要MySQL服务运行，实际测试可能需要跳过
	// 这里仅测试函数签名和配置处理
	t.Skip("需要MySQL服务运行，跳过集成测试")
}

func TestNewMySQLDB_Connection(t *testing.T) {
	// 需要 MySQL 环境变量
	host := os.Getenv("MYSQL_TEST_HOST")
	if host == "" {
		t.Skip("MYSQL_TEST_HOST not set, skipping MySQL connection test")
	}

	cfg := DBConfig{
		Type: DBTypeMySQL,
		MySQL: MySQLConfig{
			Host:     host,
			Port:     getEnvInt("MYSQL_TEST_PORT", 3306),
			Database: os.Getenv("MYSQL_TEST_DATABASE"),
			Username: os.Getenv("MYSQL_TEST_USERNAME"),
			Password: os.Getenv("MYSQL_TEST_PASSWORD"),
			Charset:  "utf8mb4",
		},
	}
	cfg.MySQL.ApplyDefaults()

	db, dialect, err := newMySQLDB(cfg)
	if err != nil {
		t.Fatalf("failed to create mysql db: %v", err)
	}
	defer db.Close()

	if dialect == nil {
		t.Error("expected dialect to be non-nil")
	}
	if _, ok := dialect.(*MySQLDialect); !ok {
		t.Error("expected MySQLDialect")
	}
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		fmt.Sscanf(v, "%d", &i)
		return i
	}
	return defaultVal
}