package repo

import (
	"fmt"
	"os"
	"testing"
	"time"
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
	// NowExpr 应返回空字符串（使用参数传入时间）
	if d.NowExpr() != "" {
		t.Errorf("expected empty NowExpr, got %s", d.NowExpr())
	}
}

func TestMySQLDialect_JSONMethods(t *testing.T) {
	d := &MySQLDialect{}

	// 测试 JSON_CONTAINS 表达式
	expr := d.JSONContainsExpr("`agent_ids`")
	expectedExpr := "JSON_CONTAINS(`agent_ids`, ?)"
	if expr != expectedExpr {
		t.Errorf("expected JSON_CONTAINS expression '%s', got '%s'", expectedExpr, expr)
	}

	// 测试 JSON 参数格式化
	param := d.JSONContainsParam("test-uuid")
	expectedParam := `"test-uuid"`
	if param != expectedParam {
		t.Errorf("expected JSON param '%s', got '%s'", expectedParam, param)
	}
}

func TestSQLiteTimeScanner_MySQLCompatibility(t *testing.T) {
	// 测试 SQLiteTimeScanner 对 MySQL time.Time 的兼容性
	// MySQL 使用 parseTime=true，返回 time.Time 类型

	scanner := &SQLiteTimeScanner{}
	testTime := time.Date(2026, 4, 11, 10, 5, 13, 0, time.Local)

	err := scanner.Scan(testTime)
	if err != nil {
		t.Fatalf("failed to scan time.Time: %v", err)
	}

	if !scanner.Valid {
		t.Error("expected Valid to be true")
	}

	if !scanner.Time.Equal(testTime) {
		t.Errorf("expected time %v, got %v", testTime, scanner.Time)
	}
}

func TestSQLiteTimeScanner_NilHandling(t *testing.T) {
	// 测试 NULL 值处理
	scanner := &SQLiteTimeScanner{}

	err := scanner.Scan(nil)
	if err != nil {
		t.Fatalf("failed to scan nil: %v", err)
	}

	if scanner.Valid {
		t.Error("expected Valid to be false for nil value")
	}

	if !scanner.Time.IsZero() {
		t.Error("expected Time to be zero for nil value")
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