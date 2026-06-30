package command

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

func TestCommandServiceCRUDContentAndBindings(t *testing.T) {
	ctx := context.Background()
	db := openCommandTestDB(t)
	service := NewService(
		repo.NewCommandRepository(db, repo.DBTypeSQLite),
		repo.NewCommandSkillBindingRepository(db, repo.DBTypeSQLite),
		repo.NewAgentCommandBindingRepository(db, repo.DBTypeSQLite),
		repo.NewAgentConfigRepository(db, repo.DBTypeSQLite),
		repo.NewSkillRepository(db, repo.DBTypeSQLite),
		t.TempDir(),
		zap.NewNop(),
	)

	if service.GetStoragePath() == "" {
		t.Fatalf("storage path should be set")
	}
	if _, err := service.Create(ctx, &model.CreateCommandRequest{Name: "BadName"}); err == nil {
		t.Fatalf("invalid command name should fail")
	}

	cmd, err := service.Create(ctx, &model.CreateCommandRequest{Name: "build-app", Description: "build", Content: "npm run build"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if cmd.Content != "npm run build" {
		t.Fatalf("created content = %q", cmd.Content)
	}
	if _, err := os.Stat(filepath.Join(service.GetStoragePath(), "build-app.md")); err != nil {
		t.Fatalf("content file missing: %v", err)
	}
	if _, err := service.Create(ctx, &model.CreateCommandRequest{Name: "build-app"}); !errors.Is(err, ErrCommandNameExists) {
		t.Fatalf("duplicate error = %v", err)
	}

	got, err := service.Get(ctx, cmd.ID)
	if err != nil || got.Content != "npm run build" {
		t.Fatalf("Get = %#v err=%v", got, err)
	}
	byName, err := service.GetByName(ctx, "build-app")
	if err != nil || byName.ID != cmd.ID || byName.Content != "npm run build" {
		t.Fatalf("GetByName = %#v err=%v", byName, err)
	}
	list, total, err := service.List(ctx, &model.CommandListQuery{Search: "build", Page: 1, PageSize: 10})
	if err != nil || total != 1 || len(list) != 1 || list[0].Content != "npm run build" {
		t.Fatalf("List = %#v total=%d err=%v", list, total, err)
	}

	updated, err := service.Update(ctx, cmd.ID, &model.UpdateCommandRequest{Description: "updated", Content: "go test ./..."})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Description != "updated" || updated.Content != "go test ./..." {
		t.Fatalf("updated command = %#v", updated)
	}
	if got := stringMustRead(t, filepath.Join(service.GetStoragePath(), "build-app.md")); got != "go test ./..." {
		t.Fatalf("updated file content = %q", got)
	}

	skillID := insertCommandSkill(t, db, "review-skill")
	if err := service.BindSkills(ctx, cmd.ID, []uuid.UUID{skillID}); err != nil {
		t.Fatalf("BindSkills returned error: %v", err)
	}
	skills, err := service.GetSkills(ctx, cmd.ID)
	if err != nil || len(skills) != 1 || skills[0].Name != "review-skill" {
		t.Fatalf("GetSkills = %#v err=%v", skills, err)
	}
	if err := service.UnbindSkill(ctx, cmd.ID, skillID); err != nil {
		t.Fatalf("UnbindSkill returned error: %v", err)
	}
	if err := service.UnbindSkill(ctx, cmd.ID, skillID); err == nil || !strings.Contains(err.Error(), "绑定关系不存在") {
		t.Fatalf("missing unbind error = %v", err)
	}

	agentID := insertCommandAgent(t, db, "Coder")
	if err := service.BindCommandsToAgent(ctx, agentID, []uuid.UUID{cmd.ID}); err != nil {
		t.Fatalf("BindCommandsToAgent returned error: %v", err)
	}
	agentCommands, err := service.GetAgentCommands(ctx, agentID)
	if err != nil || len(agentCommands) != 1 || agentCommands[0].Name != "build-app" {
		t.Fatalf("GetAgentCommands = %#v err=%v", agentCommands, err)
	}
	if err := service.Delete(ctx, cmd.ID); err == nil || !strings.Contains(err.Error(), "Coder") {
		t.Fatalf("Delete bound command error = %v", err)
	}
	if err := service.UnbindCommandFromAgent(ctx, agentID, cmd.ID); err != nil {
		t.Fatalf("UnbindCommandFromAgent returned error: %v", err)
	}
	if err := service.UnbindCommandFromAgent(ctx, agentID, cmd.ID); err == nil || !strings.Contains(err.Error(), "绑定关系不存在") {
		t.Fatalf("missing command unbind error = %v", err)
	}
	if err := service.Delete(ctx, cmd.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(service.GetStoragePath(), "build-app.md")); !os.IsNotExist(err) {
		t.Fatalf("content file should be deleted, err=%v", err)
	}
}

func openCommandTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE commands (id TEXT PRIMARY KEY, name TEXT UNIQUE, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT UNIQUE, description TEXT, tags TEXT, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE command_skill_bindings (id TEXT PRIMARY KEY, command_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_command_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, command_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, base_agent_id TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, is_default BOOLEAN, is_system BOOLEAN, requires_human BOOLEAN, mention_patterns TEXT, config_generated_at TIMESTAMP, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertCommandSkill(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO skills (id, name, description, tags, source_type, use_count, status, is_public, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "skill", `["test"]`, model.SkillSourcePersonal, 0, model.SkillStatusActive, false, now, now)
	if err != nil {
		t.Fatalf("insert skill: %v", err)
	}
	return id
}

func insertCommandAgent(t *testing.T, db *sql.DB, name string) uuid.UUID {
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

func stringMustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
