package rule

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

func TestRuleServiceCRUDContentAndBindings(t *testing.T) {
	ctx := context.Background()
	db := openRuleTestDB(t)
	service := NewService(
		repo.NewRuleRepository(db, repo.DBTypeSQLite),
		repo.NewAgentRuleBindingRepository(db, repo.DBTypeSQLite),
		repo.NewAgentConfigRepository(db, repo.DBTypeSQLite),
		t.TempDir(),
		zap.NewNop(),
	)

	if service.GetStoragePath() == "" {
		t.Fatalf("storage path should be set")
	}
	if _, err := service.Create(ctx, &model.CreateRuleRequest{Name: "BadRule"}); err == nil {
		t.Fatalf("invalid rule name should fail")
	}

	rule, err := service.Create(ctx, &model.CreateRuleRequest{Name: "test-rule", Description: "desc", Content: "Always test changes."})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if rule.Content != "Always test changes." {
		t.Fatalf("created content = %q", rule.Content)
	}
	if _, err := service.Create(ctx, &model.CreateRuleRequest{Name: "test-rule"}); !errors.Is(err, ErrRuleNameExists) {
		t.Fatalf("duplicate error = %v", err)
	}

	got, err := service.Get(ctx, rule.ID)
	if err != nil || got.Content != "Always test changes." {
		t.Fatalf("Get = %#v err=%v", got, err)
	}
	byName, err := service.GetByName(ctx, "test-rule")
	if err != nil || byName.ID != rule.ID || byName.Content != "Always test changes." {
		t.Fatalf("GetByName = %#v err=%v", byName, err)
	}
	list, total, err := service.List(ctx, &model.RuleListQuery{Search: "test", Page: 1, PageSize: 10})
	if err != nil || total != 1 || len(list) != 1 || list[0].Content != "Always test changes." {
		t.Fatalf("List = %#v total=%d err=%v", list, total, err)
	}

	updated, err := service.Update(ctx, rule.ID, &model.UpdateRuleRequest{Description: "updated", Content: "Never skip verification."})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Description != "updated" || updated.Content != "Never skip verification." {
		t.Fatalf("updated rule = %#v", updated)
	}
	if got := stringMustReadRule(t, filepath.Join(service.GetStoragePath(), "test-rule.md")); got != "Never skip verification." {
		t.Fatalf("updated file content = %q", got)
	}

	agentID := insertRuleAgent(t, db, "Reviewer")
	if err := service.BindRulesToAgent(ctx, agentID, []uuid.UUID{rule.ID}); err != nil {
		t.Fatalf("BindRulesToAgent returned error: %v", err)
	}
	rules, err := service.GetAgentRules(ctx, agentID)
	if err != nil || len(rules) != 1 || rules[0].Name != "test-rule" {
		t.Fatalf("GetAgentRules = %#v err=%v", rules, err)
	}
	if err := service.Delete(ctx, rule.ID); err == nil || !strings.Contains(err.Error(), "Reviewer") {
		t.Fatalf("Delete bound rule error = %v", err)
	}
	if err := service.UnbindRuleFromAgent(ctx, agentID, rule.ID); err != nil {
		t.Fatalf("UnbindRuleFromAgent returned error: %v", err)
	}
	if err := service.UnbindRuleFromAgent(ctx, agentID, rule.ID); err == nil || !strings.Contains(err.Error(), "绑定关系不存在") {
		t.Fatalf("missing rule unbind error = %v", err)
	}
	if err := service.Delete(ctx, rule.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(service.GetStoragePath(), "test-rule.md")); !os.IsNotExist(err) {
		t.Fatalf("content file should be deleted, err=%v", err)
	}
}

func TestRuleIsValidName(t *testing.T) {
	tests := map[string]bool{
		"rule-one": true,
		"r1":       true,
		"":         false,
		"1-rule":   false,
		"Rule":     false,
		"rule_one": false,
	}
	for name, want := range tests {
		if got := isValidName(name); got != want {
			t.Fatalf("isValidName(%q) = %v, want %v", name, got, want)
		}
	}
}

func openRuleTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE rules (id TEXT PRIMARY KEY, name TEXT UNIQUE, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_rule_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, rule_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, base_agent_id TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, is_default BOOLEAN, is_system BOOLEAN, requires_human BOOLEAN, mention_patterns TEXT, config_generated_at TIMESTAMP, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertRuleAgent(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO agent_configs (id, name, role, description, system_prompt, max_tokens, temperature, is_default, is_system, requires_human, mention_patterns, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, model.AgentRoleAgent, "agent", "prompt", 1000, 0.2, false, false, false, `[]`, now, now)
	if err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	return id
}

func stringMustReadRule(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
