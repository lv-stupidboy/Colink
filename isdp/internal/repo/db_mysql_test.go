package repo

import (
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

func TestMySQLDSN(t *testing.T) {
	cfg := DBConfig{
		Type: DBTypeMySQL,
		MySQL: MySQLConfig{
			Host:     "localhost",
			Port:     3306,
			Database: "test_db",
			Username: "test_user",
			Password: "test_pass",
			Charset:  "utf8mb4",
		},
	}

	dsn := buildMySQLDSN(cfg.MySQL)
	expected := "test_user:test_pass@tcp(localhost:3306)/test_db?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=true"
	if dsn != expected {
		t.Errorf("DSN mismatch\n got: %s\n want: %s", dsn, expected)
	}
}