package repo

import (
	"testing"
	"time"
)

func TestSQLiteDialect(t *testing.T) {
	d := &SQLiteDialect{}

	if d.Placeholder() != "?" {
		t.Errorf("expected placeholder '?', got %s", d.Placeholder())
	}
	if d.QuoteIdentifier() != "\"" {
		t.Errorf("expected quote '\"', got %s", d.QuoteIdentifier())
	}
	if d.AutoIncrement() != "AUTOINCREMENT" {
		t.Errorf("expected AUTOINCREMENT, got %s", d.AutoIncrement())
	}
	// NowExpr 应返回空字符串（使用参数传入时间）
	if d.NowExpr() != "" {
		t.Errorf("expected empty NowExpr, got %s", d.NowExpr())
	}
}

func TestSQLiteDialect_JSONMethods(t *testing.T) {
	d := &SQLiteDialect{}

	// 测试 LIKE 表达式（SQLite 不支持 JSON_CONTAINS）
	expr := d.JSONContainsExpr("agent_ids")
	expectedExpr := "agent_ids LIKE ?"
	if expr != expectedExpr {
		t.Errorf("expected LIKE expression '%s', got '%s'", expectedExpr, expr)
	}

	// 测试 LIKE 参数格式化
	param := d.JSONContainsParam("test-uuid")
	expectedParam := `%"test-uuid"%`
	if param != expectedParam {
		t.Errorf("expected LIKE param '%s', got '%s'", expectedParam, param)
	}
}

func TestSQLiteTimeScanner_StringParsing(t *testing.T) {
	// 测试 SQLite TEXT 时间格式解析
	scanner := &SQLiteTimeScanner{}

	// 测试标准格式 "2006-01-02 15:04:05"
	err := scanner.Scan("2026-04-11 10:05:13")
	if err != nil {
		t.Fatalf("failed to scan string: %v", err)
	}

	if !scanner.Valid {
		t.Error("expected Valid to be true")
	}

	expectedTime := time.Date(2026, 4, 11, 10, 5, 13, 0, time.UTC)
	if scanner.Time.Year() != expectedTime.Year() ||
		scanner.Time.Month() != expectedTime.Month() ||
		scanner.Time.Day() != expectedTime.Day() ||
		scanner.Time.Hour() != expectedTime.Hour() ||
		scanner.Time.Minute() != expectedTime.Minute() ||
		scanner.Time.Second() != expectedTime.Second() {
		t.Errorf("parsed time mismatch: got %v, expected approx %v", scanner.Time, expectedTime)
	}
}

func TestSQLiteTimeScanner_EmptyString(t *testing.T) {
	// 测试空字符串处理
	scanner := &SQLiteTimeScanner{}

	err := scanner.Scan("")
	if err != nil {
		t.Fatalf("failed to scan empty string: %v", err)
	}

	if scanner.Valid {
		t.Error("expected Valid to be false for empty string")
	}
}

func TestNewSQLiteDB_Connection(t *testing.T) {
	cfg := DBConfig{
		Type: DBTypeSQLite,
		Path: ":memory:",
	}

	db, dialect, err := newSQLiteDB(cfg)
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	defer db.Close()

	if dialect == nil {
		t.Error("expected dialect to be non-nil")
	}
	if _, ok := dialect.(*SQLiteDialect); !ok {
		t.Error("expected SQLiteDialect")
	}

	// 验证外键约束已启用
	var enabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&enabled)
	if err != nil {
		t.Fatalf("failed to check foreign keys: %v", err)
	}
	if enabled != 1 {
		t.Error("expected foreign keys to be enabled")
	}
}