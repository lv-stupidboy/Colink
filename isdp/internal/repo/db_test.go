package repo

import (
	"testing"
)

func TestNewDB_InvalidType(t *testing.T) {
	cfg := DBConfig{
		Type: DBType("invalid"),
	}
	_, _, err := NewDB(cfg)
	if err == nil {
		t.Error("expected error for invalid database type")
	}
}

func TestNewDB_SQLiteType(t *testing.T) {
	cfg := DBConfig{
		Type: DBTypeSQLite,
		Path: ":memory:",
	}
	db, dialect, err := NewDB(cfg)
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
}