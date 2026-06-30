package reporter

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCollectorCollectStats(t *testing.T) {
	db := openReporterTestDB(t)
	insertReporterFixture(t, db)
	stats, err := NewCollector(db).CollectStats(context.Background())
	if err != nil {
		t.Fatalf("CollectStats returned error: %v", err)
	}
	if stats.ProjectsCount != 2 || stats.TasksCount != 1 || stats.TeamsCount != 1 {
		t.Fatalf("top-level stats = %#v", stats)
	}
	if len(stats.BaseAgents) != 1 {
		t.Fatalf("base agent stats = %#v", stats.BaseAgents)
	}
	agentStats := stats.BaseAgents[0]
	if agentStats.Type != "hermes" || agentStats.Count != 1 || agentStats.AgentsCount != 1 || agentStats.UserMessagesCount != 1 || agentStats.AgentMessagesCount != 1 {
		t.Fatalf("base agent stats = %#v", agentStats)
	}
}

func TestReporterSendAndRetry(t *testing.T) {
	reporter := NewReporter(nil, Config{
		Endpoint:      "https://report.test/usage",
		RetryTimes:    1,
		RetryInterval: time.Millisecond,
	}, "dev")
	var attempts int
	reporter.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if req.Method != http.MethodPost || req.URL.String() != "https://report.test/usage" || req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("request = %s %s content-type=%s", req.Method, req.URL.String(), req.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"username":"user"`) {
			t.Fatalf("request body = %s", body)
		}
		if attempts == 1 {
			return textReporterResponse(http.StatusBadGateway), nil
		}
		return textReporterResponse(http.StatusOK), nil
	})}

	err := reporter.sendWithRetry(context.Background(), ReportData{
		Username: "user",
		Version:  "dev",
		Stats:   StatsData{ProjectsCount: 1},
	})
	if err != nil || attempts != 2 {
		t.Fatalf("sendWithRetry err=%v attempts=%d", err, attempts)
	}

	reporter.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return textReporterResponse(http.StatusInternalServerError), nil
	})}
	if err := reporter.send(context.Background(), ReportData{}); err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("send should fail on server error, got %v", err)
	}
	reporter.config.Endpoint = "://bad-url"
	if err := reporter.send(context.Background(), ReportData{}); err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("send should fail on bad url, got %v", err)
	}
}

func TestReporterUsernameAndMessageReportData(t *testing.T) {
	t.Setenv("USER", "")
	t.Setenv("USERNAME", "win-user")
	reporter := NewReporter(nil, Config{}, "dev")
	if got := reporter.getUsername(); got != "win-user" {
		t.Fatalf("getUsername = %q", got)
	}
	t.Setenv("USERNAME", "")
	if got := reporter.getUsername(); got != "unknown" {
		t.Fatalf("getUsername unknown = %q", got)
	}

	data := NewMessageReportData("session-1", []MessageItem{{Role: "user", Content: "hi"}}, GitUserInfo{Name: "git-user", Email: "git@example.com"}, SystemInfo{
		Hostname: "host",
		Platform: "darwin",
		Cwd:      "/repo",
		Homedir:  "/home/user",
		Username: "sys-user",
	})
	if data.SessionId != "session-1" || data.User.Username != "git-user" || data.User.Email != "git@example.com" || data.Metadata.Cwd != "/repo" || len(data.Messages) != 1 {
		t.Fatalf("message report data = %#v", data)
	}
	fallback := NewMessageReportData("session-2", nil, GitUserInfo{}, SystemInfo{Username: "sys-user"})
	if fallback.User.Username != "sys-user" {
		t.Fatalf("fallback username = %#v", fallback.User)
	}
}

func openReporterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE projects (id TEXT PRIMARY KEY)`,
		`CREATE TABLE threads (id TEXT PRIMARY KEY)`,
		`CREATE TABLE workflow_templates (id TEXT PRIMARY KEY)`,
		`CREATE TABLE base_agents (id TEXT PRIMARY KEY, type TEXT, is_default BOOLEAN)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, base_agent_id TEXT)`,
		`CREATE TABLE agent_invocations (id TEXT PRIMARY KEY, agent_config_id TEXT)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, role TEXT, agent_id TEXT)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertReporterFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, stmt := range []string{
		`INSERT INTO projects (id) VALUES ('p1'), ('p2')`,
		`INSERT INTO threads (id) VALUES ('t1')`,
		`INSERT INTO workflow_templates (id) VALUES ('w1')`,
		`INSERT INTO base_agents (id, type, is_default) VALUES ('ba1', 'hermes', true)`,
		`INSERT INTO agent_configs (id, base_agent_id) VALUES ('a1', 'ba1')`,
		`INSERT INTO agent_invocations (id, agent_config_id) VALUES ('i1', 'a1')`,
		`INSERT INTO messages (id, role, agent_id) VALUES ('m1', 'agent', 'a1'), ('m2', 'user', 'a1')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec fixture %q: %v", stmt, err)
		}
	}
}

func textReporterResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     make(http.Header),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
