package subagent

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

func TestSubagentServiceCRUDAndContentFiles(t *testing.T) {
	ctx := context.Background()
	db := openSubagentTestDB(t)
	storage := t.TempDir()
	service := newSubagentTestService(db, storage)

	created, err := service.Create(ctx, &model.CreateSubagentRequest{
		Name:        "Review Bot",
		Description: "reviews changes",
		Content:     "check diffs",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == uuid.Nil || created.Content != "check diffs" {
		t.Fatalf("created subagent = %#v", created)
	}
	if body, err := os.ReadFile(filepath.Join(storage, "Review Bot.md")); err != nil || string(body) != "check diffs" {
		t.Fatalf("stored content = %q err=%v", body, err)
	}
	if _, err := service.Create(ctx, &model.CreateSubagentRequest{Name: "Review Bot", Content: "duplicate"}); !errors.Is(err, ErrSubagentNameExists) {
		t.Fatalf("duplicate create error = %v", err)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil || got.Content != "check diffs" {
		t.Fatalf("Get = %#v err=%v", got, err)
	}
	list, total, err := service.List(ctx, &model.SubagentListQuery{Search: "Review", Page: -1, PageSize: 200})
	if err != nil || total != 1 || len(list) != 1 || list[0].Content != "check diffs" {
		t.Fatalf("List = %#v total=%d err=%v", list, total, err)
	}

	updated, err := service.Update(ctx, created.ID, &model.UpdateSubagentRequest{Description: "updated", Content: "new content"})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Description != "updated" || updated.Content != "new content" {
		t.Fatalf("updated subagent = %#v", updated)
	}
	if body, err := os.ReadFile(filepath.Join(storage, "Review Bot.md")); err != nil || string(body) != "new content" {
		t.Fatalf("updated content = %q err=%v", body, err)
	}

	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(storage, "Review Bot.md")); !os.IsNotExist(err) {
		t.Fatalf("content file should be removed, err=%v", err)
	}
	if _, err := service.Get(ctx, created.ID); err == nil {
		t.Fatalf("deleted subagent should not be found")
	}
}

func TestSubagentServiceAgentBindingsAndDeleteGuard(t *testing.T) {
	ctx := context.Background()
	db := openSubagentTestDB(t)
	service := newSubagentTestService(db, t.TempDir())
	agentID := insertSubagentAgent(t, db, "Coder")
	subagentID := insertSubagentRecord(t, db, "Helper")

	if err := service.BindSubagents(ctx, agentID, []uuid.UUID{subagentID}); err != nil {
		t.Fatalf("BindSubagents returned error: %v", err)
	}
	bound, err := service.GetAgentSubagents(ctx, agentID)
	if err != nil || len(bound) != 1 || bound[0].ID != subagentID {
		t.Fatalf("GetAgentSubagents = %#v err=%v", bound, err)
	}
	if err := service.Delete(ctx, subagentID); err == nil || !strings.Contains(err.Error(), "已被以下Agent绑定") {
		t.Fatalf("delete bound subagent error = %v", err)
	}
	if err := service.UnbindSubagent(ctx, agentID, subagentID); err != nil {
		t.Fatalf("UnbindSubagent returned error: %v", err)
	}
	if err := service.UnbindSubagent(ctx, agentID, subagentID); err == nil || !strings.Contains(err.Error(), "绑定关系不存在") {
		t.Fatalf("unbind missing binding error = %v", err)
	}
	if err := service.Delete(ctx, subagentID); err != nil {
		t.Fatalf("Delete after unbind returned error: %v", err)
	}
	if err := service.BindSubagents(ctx, uuid.New(), []uuid.UUID{subagentID}); err == nil || !strings.Contains(err.Error(), "Agent角色不存在") {
		t.Fatalf("bind missing agent error = %v", err)
	}
	if err := service.BindSubagents(ctx, agentID, []uuid.UUID{uuid.New()}); err == nil || !strings.Contains(err.Error(), "子代理") {
		t.Fatalf("bind missing subagent error = %v", err)
	}
}

func TestSubagentServiceSkillBindings(t *testing.T) {
	ctx := context.Background()
	db := openSubagentTestDB(t)
	service := newSubagentTestService(db, t.TempDir())
	subagentID := insertSubagentRecord(t, db, "Tool User")
	skillID := insertSubagentSkill(t, db, "Search")

	if err := service.BindSkills(ctx, subagentID, []uuid.UUID{skillID}); err != nil {
		t.Fatalf("BindSkills returned error: %v", err)
	}
	skills, err := service.GetSkills(ctx, subagentID)
	if err != nil || len(skills) != 1 || skills[0].ID != skillID {
		t.Fatalf("GetSkills = %#v err=%v", skills, err)
	}
	if err := service.UnbindSkill(ctx, subagentID, skillID); err != nil {
		t.Fatalf("UnbindSkill returned error: %v", err)
	}
	if err := service.UnbindSkill(ctx, subagentID, skillID); err == nil || !strings.Contains(err.Error(), "绑定关系不存在") {
		t.Fatalf("unbind missing skill binding error = %v", err)
	}
	if err := service.BindSkills(ctx, subagentID, []uuid.UUID{uuid.New()}); err == nil || !strings.Contains(err.Error(), "技能") {
		t.Fatalf("bind missing skill error = %v", err)
	}
	if _, err := service.GetSkills(ctx, uuid.New()); err == nil || !strings.Contains(err.Error(), "子代理不存在") {
		t.Fatalf("get skills missing subagent error = %v", err)
	}
}

func newSubagentTestService(db *sql.DB, storage string) *Service {
	return NewService(
		repo.NewSubagentRepository(db, repo.DBTypeSQLite),
		repo.NewAgentSubagentBindingRepository(db, repo.DBTypeSQLite),
		repo.NewSubagentSkillBindingRepository(db, repo.DBTypeSQLite),
		repo.NewAgentConfigRepository(db, repo.DBTypeSQLite),
		repo.NewSkillRepository(db, repo.DBTypeSQLite),
		storage,
		zap.NewNop(),
	)
}

func openSubagentTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE subagents (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_subagent_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, subagent_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE subagent_skill_bindings (id TEXT PRIMARY KEY, subagent_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, base_agent_id TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, is_default BOOLEAN, is_system BOOLEAN, requires_human BOOLEAN, mention_patterns TEXT, config_generated_at TIMESTAMP, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT, description TEXT, tags BLOB, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertSubagentRecord(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO subagents (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id.String(), name, name+" desc", now, now)
	if err != nil {
		t.Fatalf("insert subagent: %v", err)
	}
	return id
}

func insertSubagentAgent(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO agent_configs (id, name, role, description, system_prompt, max_tokens, temperature, is_default, is_system, requires_human, mention_patterns, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, model.AgentRoleAgent, "", "system", 1000, 0.5, false, false, false, []byte(`[]`), now, now)
	if err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	return id
}

func insertSubagentSkill(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO skills (id, name, description, tags, source_type, use_count, status, is_public, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, name+" desc", []byte(`["search"]`), model.SkillSourcePlatform, 0, model.SkillStatusActive, true, now, now)
	if err != nil {
		t.Fatalf("insert skill: %v", err)
	}
	return id
}
