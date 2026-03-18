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