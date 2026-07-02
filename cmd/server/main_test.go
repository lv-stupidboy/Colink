package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"
)

func TestVersionAndConfigPathHelpers(t *testing.T) {
	cases := map[string]string{
		"dev":                     "dev",
		"v1.1.0-20260411-191622": "1.1.0",
		"1.2.3-build":            "1.2.3",
		"v2.0.0":                 "2.0.0",
		"":                       "",
	}
	for input, want := range cases {
		if got := extractBaseVersion(input); got != want {
			t.Fatalf("extractBaseVersion(%q) = %q, want %q", input, got, want)
		}
	}
	if role := getRoleFromCatID("developer"); string(role) != "developer" {
		t.Fatalf("getRoleFromCatID = %q", role)
	}

	oldArgs := os.Args
	oldEnv := os.Getenv("ISDP_CONFIG")
	t.Cleanup(func() {
		os.Args = oldArgs
		_ = os.Setenv("ISDP_CONFIG", oldEnv)
	})

	os.Args = []string{"server", "-config", "/tmp/from-arg.yaml"}
	_ = os.Unsetenv("ISDP_CONFIG")
	if got := findConfigPath(); got != "/tmp/from-arg.yaml" {
		t.Fatalf("findConfigPath arg = %q", got)
	}
	os.Args = []string{"server"}
	_ = os.Setenv("ISDP_CONFIG", "/tmp/from-env.yaml")
	if got := findConfigPath(); got != "/tmp/from-env.yaml" {
		t.Fatalf("findConfigPath env = %q", got)
	}
	_ = os.Unsetenv("ISDP_CONFIG")
	if got := findConfigPath(); got != "configs/config.yaml" {
		t.Fatalf("findConfigPath fallback = %q", got)
	}
}

func TestHTTPMiddlewaresAndLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(corsMiddleware())
	router.Use(requestLogger(zap.NewNop()))
	router.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	router.GET("/bad", func(c *gin.Context) { c.String(http.StatusBadRequest, "bad") })

	req := httptest.NewRequest(http.MethodOptions, "/ok", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("OPTIONS status=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/ok?x=1", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("GET /ok status=%d body=%q", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/bad", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /bad status=%d", rec.Code)
	}

	logDir := t.TempDir()
	logger, err := initLogger("debug", "console", logDir)
	if err != nil {
		t.Fatalf("initLogger returned error: %v", err)
	}
	logger.Info("test log")
	if _, err := os.Stat(filepath.Join(logDir, "server.log")); err != nil {
		t.Fatalf("server log should exist: %v", err)
	}
	if getGoroutineID() == 0 {
		t.Fatalf("goroutine id should be non-zero")
	}
}

func TestLogMaintenanceAndDatabaseTableCheck(t *testing.T) {
	logDir := t.TempDir()
	oldLog := filepath.Join(logDir, "old.log")
	newLog := filepath.Join(logDir, "new.log")
	if err := os.WriteFile(oldLog, []byte("old"), 0644); err != nil {
		t.Fatalf("write old log: %v", err)
	}
	if err := os.WriteFile(newLog, []byte("new"), 0644); err != nil {
		t.Fatalf("write new log: %v", err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldLog, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old log: %v", err)
	}
	performLogMaintenance(zap.NewNop(), logDir)
	if _, err := os.Stat(oldLog); !os.IsNotExist(err) {
		t.Fatalf("old log should be removed, err=%v", err)
	}
	if _, err := os.Stat(newLog); err != nil {
		t.Fatalf("new log should remain: %v", err)
	}
	performLogMaintenance(zap.NewNop(), filepath.Join(logDir, "missing"))

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"projects", "threads", "messages", "agent_configs", "base_agents", "agent_invocations", "artifacts", "sandboxes", "workflow_templates", "skills", "commands", "subagents", "rules", "settings", "markets", "team_package_versions", "local_repos", "session_records", "mcp_servers", "agent_mcp_bindings", "goose_db_version"} {
		if _, err := db.Exec("CREATE TABLE " + table + " (id INTEGER, version_id INTEGER)"); err != nil {
			t.Fatalf("create table %s: %v", table, err)
		}
	}
	if _, err := db.Exec("INSERT INTO goose_db_version (version_id) VALUES (42)"); err != nil {
		t.Fatalf("insert goose version: %v", err)
	}
	checkDatabaseTables(db, zap.NewNop())

	emptyDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open empty sqlite: %v", err)
	}
	defer emptyDB.Close()
	checkDatabaseTables(emptyDB, zap.NewNop())
}
