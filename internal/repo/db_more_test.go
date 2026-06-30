package repo

import (
	"database/sql"
	"testing"
	"time"
)

func TestDBFactoriesAndBaseRepository(t *testing.T) {
	path := t.TempDir() + "/colink.db"
	db, dialect, err := NewDB(DBConfig{Type: DBTypeSQLite, Path: path})
	if err != nil {
		t.Fatalf("NewDB sqlite returned error: %v", err)
	}
	defer db.Close()
	if dialect.Placeholder() != "?" || dialect.QuoteIdentifier() != `"` || dialect.JSONContainsExpr("tags") != "tags LIKE ?" || dialect.JSONContainsParam("go") != `%"go"%` {
		t.Fatalf("unexpected sqlite dialect")
	}

	legacyDB, err := NewDBFromConfig(DBConfig{Type: DBTypeSQLite, Path: t.TempDir() + "/legacy.db"})
	if err != nil {
		t.Fatalf("NewDBFromConfig returned error: %v", err)
	}
	legacyDB.Close()
	sqliteDB, err := NewSQLiteDB(t.TempDir() + "/direct.db")
	if err != nil {
		t.Fatalf("NewSQLiteDB returned error: %v", err)
	}
	sqliteDB.Close()

	if _, _, err := NewDB(DBConfig{Type: "unknown"}); err == nil {
		t.Fatalf("unsupported DB type should fail")
	}
	if _, _, err := NewDB(DBConfig{Type: DBTypeSQLite}); err == nil {
		t.Fatalf("empty sqlite path should fail")
	}

	base := NewBaseRepository(db, DBTypeSQLite)
	if base.DB() != db || base.DBType() != DBTypeSQLite {
		t.Fatalf("base repository = %#v", base)
	}
}

func TestScanTimeHelpers(t *testing.T) {
	want := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	var got time.Time
	if err := ScanTime(DBTypeSQLite, fakeScanner{value: want.Format(time.RFC3339)}, &got); err != nil {
		t.Fatalf("ScanTime sqlite returned error: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("ScanTime sqlite = %s, want %s", got, want)
	}
	if err := ScanTime(DBTypeMySQL, fakeScanner{value: want}, &got); err != nil {
		t.Fatalf("ScanTime mysql returned error: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("ScanTime mysql = %s, want %s", got, want)
	}

	var ptr *time.Time
	if err := ScanTimeNull(DBTypeSQLite, fakeScanner{value: want.Format(time.RFC3339)}, &ptr); err != nil {
		t.Fatalf("ScanTimeNull sqlite returned error: %v", err)
	}
	if ptr == nil || !ptr.Equal(want) {
		t.Fatalf("ScanTimeNull sqlite = %v, want %s", ptr, want)
	}
	if err := ScanTimeNull(DBTypeSQLite, fakeScanner{value: nil}, &ptr); err != nil {
		t.Fatalf("ScanTimeNull sqlite nil returned error: %v", err)
	}
	if ptr != nil {
		t.Fatalf("ScanTimeNull sqlite nil = %v", ptr)
	}
	if err := ScanTimeNull(DBTypeMySQL, fakeScanner{value: sql.NullTime{Time: want, Valid: true}}, &ptr); err != nil {
		t.Fatalf("ScanTimeNull mysql returned error: %v", err)
	}
	if ptr == nil || !ptr.Equal(want) {
		t.Fatalf("ScanTimeNull mysql = %v, want %s", ptr, want)
	}
	if err := ScanTimeNull(DBTypeMySQL, fakeScanner{value: sql.NullTime{}}, &ptr); err != nil {
		t.Fatalf("ScanTimeNull mysql nil returned error: %v", err)
	}
	if ptr != nil {
		t.Fatalf("ScanTimeNull mysql nil = %v", ptr)
	}
	if err := ScanTime(DBTypeSQLite, fakeScanner{err: sql.ErrNoRows}, &got); err != sql.ErrNoRows {
		t.Fatalf("ScanTime should return scanner error, got %v", err)
	}
}

type fakeScanner struct {
	value any
	err   error
}

func (s fakeScanner) Scan(dest ...interface{}) error {
	if s.err != nil {
		return s.err
	}
	switch d := dest[0].(type) {
	case *SQLiteTimeScanner:
		return d.Scan(s.value)
	case *time.Time:
		*d = s.value.(time.Time)
	case *sql.NullTime:
		*d = s.value.(sql.NullTime)
	default:
		panic("unsupported fake scan destination")
	}
	return nil
}
