package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestParseArgsAndSQLHelpers(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"migrate", "up", "--db", "data.db", "--version", "1.2.3", "--target", "12", "--backup", "--dry-run", "--json", "-v"}
	args := parseArgs()
	if args.Command != "up" || args.DBPath != "data.db" || args.MigrationsDir != "sql-change/v1.2.3/sqlite" || args.Target != 12 || !args.Backup || !args.DryRun || !args.JSONOutput || !args.Verbose {
		t.Fatalf("parseArgs = %#v", args)
	}

	if got := truncate("short", 10); got != "short" {
		t.Fatalf("truncate short = %q", got)
	}
	if got := truncate("0123456789", 4); got != "0123..." {
		t.Fatalf("truncate long = %q", got)
	}
	if got := extractUpSQL("CREATE TABLE demo(id INTEGER);"); !strings.Contains(got, "CREATE TABLE demo") {
		t.Fatalf("extract whole sql = %q", got)
	}
	gooseSQL := `-- +goose Up
-- +goose StatementBegin
CREATE TABLE up_table(id INTEGER);
-- +goose StatementEnd
-- +goose Down
DROP TABLE up_table;`
	up := extractUpSQL(gooseSQL)
	if !strings.Contains(up, "CREATE TABLE up_table") || strings.Contains(up, "StatementBegin") || strings.Contains(up, "DROP TABLE") {
		t.Fatalf("extract goose up = %q", up)
	}
}

func TestDBBackupOutputAndRunCommands(t *testing.T) {
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("SetDialect: %v", err)
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("openDB returned error: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE existing(id INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	db.Close()

	backupPath := createBackup(dbPath, false)
	if backupPath == "" {
		t.Fatalf("createBackup returned empty path")
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	if got := createBackup(filepath.Join(dir, "missing.db"), true); got != "" {
		t.Fatalf("missing backup path = %q", got)
	}

	jsonOut := captureMigrateStdout(t, func() {
		outputResult(true, Result{Success: true, Message: "ok", CurrentVersion: 1})
	})
	var result Result
	if err := json.Unmarshal([]byte(jsonOut), &result); err != nil || !result.Success || result.CurrentVersion != 1 {
		t.Fatalf("json output = %q result=%#v err=%v", jsonOut, result, err)
	}
	textOut := captureMigrateStdout(t, func() {
		outputResult(false, Result{Success: true, Message: "done"})
	})
	if !strings.Contains(textOut, "done") {
		t.Fatalf("text output = %q", textOut)
	}
	errOut := captureMigrateStdout(t, func() {
		outputError(true, "bad")
	})
	if !strings.Contains(errOut, `"success": false`) || !strings.Contains(errOut, `"bad"`) {
		t.Fatalf("error output = %q", errOut)
	}

	missingStatus := captureMigrateStdout(t, func() {
		runStatus(CLIArgs{DBPath: filepath.Join(dir, "missing-status.db"), JSONOutput: true})
	})
	if !strings.Contains(missingStatus, `"database not found"`) {
		t.Fatalf("missing status = %q", missingStatus)
	}
	missingVersion := captureMigrateStdout(t, func() {
		runVersion(CLIArgs{DBPath: filepath.Join(dir, "missing-version.db"), JSONOutput: false})
	})
	if !strings.Contains(missingVersion, "database not found") {
		t.Fatalf("missing version = %q", missingVersion)
	}

	sqlPath := filepath.Join(dir, "manual.sql")
	sqlBody := `-- +goose Up
CREATE TABLE manual(id INTEGER);
-- +goose Down
DROP TABLE manual;`
	if err := os.WriteFile(sqlPath, []byte(sqlBody), 0644); err != nil {
		t.Fatalf("write sql: %v", err)
	}
	dryRun := captureMigrateStdout(t, func() {
		runRun(CLIArgs{DBPath: dbPath, File: sqlPath, DryRun: true, JSONOutput: false})
	})
	if !strings.Contains(dryRun, "Dry-run") || !strings.Contains(dryRun, "CREATE TABLE manual") {
		t.Fatalf("dry run output = %q", dryRun)
	}
	runOut := captureMigrateStdout(t, func() {
		runRun(CLIArgs{DBPath: dbPath, File: sqlPath, Backup: true, JSONOutput: true})
	})
	if !strings.Contains(runOut, `"success": true`) || !strings.Contains(runOut, "manual.sql") {
		t.Fatalf("run output = %q", runOut)
	}
	db, err = openDB(dbPath)
	if err != nil {
		t.Fatalf("reopen DB: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='manual'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("manual table count=%d err=%v", count, err)
	}

	status := captureMigrateStdout(t, func() {
		runStatus(CLIArgs{DBPath: dbPath, JSONOutput: true})
	})
	if !strings.Contains(status, `"success": true`) {
		t.Fatalf("status output = %q", status)
	}
}

func captureMigrateStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}
