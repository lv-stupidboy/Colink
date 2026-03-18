package repo

import (
	"testing"
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