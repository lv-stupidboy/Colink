package skill

import (
	"context"
	"database/sql"
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

func TestServiceLifecycleBindingsAndDeleteGuards(t *testing.T) {
	ctx := context.Background()
	db := openSkillServiceTestDB(t)
	storagePath := t.TempDir()
	service := NewService(
		repo.NewSkillRepository(db, repo.DBTypeSQLite),
		repo.NewAgentSkillBindingRepository(db, repo.DBTypeSQLite),
		repo.NewAgentConfigRepository(db, repo.DBTypeSQLite),
		nil,
		nil,
		nil,
		nil,
		storagePath,
		zap.NewNop(),
	)

	personal, err := service.Create(ctx, &model.CreateSkillRequest{
		Name:        "review",
		Description: "review code",
		Tags:        []string{"Go", "代码审查"},
		SourceType:  model.SkillSourcePersonal,
		IsPublic:    false,
	})
	if err != nil {
		t.Fatalf("Create personal returned error: %v", err)
	}
	if personal.IsPublic || personal.Status != model.SkillStatusActive {
		t.Fatalf("personal skill = %#v", personal)
	}
	platform, err := service.Create(ctx, &model.CreateSkillRequest{
		Name:       "platform-review",
		SourceType: model.SkillSourcePlatform,
		IsPublic:   false,
	})
	if err != nil {
		t.Fatalf("Create platform returned error: %v", err)
	}
	if !platform.IsPublic {
		t.Fatalf("platform skill should be public: %#v", platform)
	}

	updated, err := service.Update(ctx, personal.ID, &model.UpdateSkillRequest{
		Description: "updated",
		Tags:        []string{"Go", "安全审计"},
		Status:      string(model.SkillStatusDeprecated),
		IsPublic:    true,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Description != "updated" || !updated.IsPublic || updated.Status != model.SkillStatusDeprecated {
		t.Fatalf("updated skill = %#v", updated)
	}
	if _, err := service.GetByID(ctx, personal.ID); err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if _, err := service.GetByName(ctx, "review"); err != nil {
		t.Fatalf("GetByName returned error: %v", err)
	}
	list, total, err := service.List(ctx, &model.SkillListQuery{Search: "review", PageSize: 10})
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("List total=%d list=%#v err=%v", total, list, err)
	}
	tags, err := service.GetAllTags(ctx)
	if err != nil || !containsSkillTag(tags, "Go") || !containsSkillTag(tags, "安全审计") {
		t.Fatalf("GetAllTags = %#v err=%v", tags, err)
	}
	if len(service.GetBuiltInTagCategories()) == 0 {
		t.Fatalf("expected built-in tag categories")
	}

	agentID := insertSkillServiceAgent(t, db, "Review Agent")
	if err := service.BindSkills(ctx, agentID, []uuid.UUID{personal.ID, platform.ID}); err != nil {
		t.Fatalf("BindSkills returned error: %v", err)
	}
	bound, err := service.GetBoundSkills(ctx, agentID)
	if err != nil || len(bound) != 2 {
		t.Fatalf("GetBoundSkills = %#v err=%v", bound, err)
	}
	agents, err := service.GetBoundAgents(ctx, personal.ID)
	if err != nil || len(agents) != 1 || agents[0].Name != "Review Agent" {
		t.Fatalf("GetBoundAgents = %#v err=%v", agents, err)
	}
	if err := service.Delete(ctx, personal.ID); err == nil || !strings.Contains(err.Error(), "无法删除技能") {
		t.Fatalf("Delete bound skill error = %v", err)
	}
	if err := service.UnbindSkill(ctx, agentID, personal.ID); err != nil {
		t.Fatalf("UnbindSkill returned error: %v", err)
	}
	if err := service.BindSkills(ctx, agentID, []uuid.UUID{platform.ID}); err != nil {
		t.Fatalf("BindSkills replace returned error: %v", err)
	}
	bound, err = service.GetBoundSkills(ctx, agentID)
	if err != nil || len(bound) != 1 || bound[0].ID != platform.ID {
		t.Fatalf("GetBoundSkills after replace = %#v err=%v", bound, err)
	}

	skillDir := filepath.Join(storagePath, personal.ID.String())
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := service.Delete(ctx, personal.ID); err != nil {
		t.Fatalf("Delete unbound returned error: %v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("skill dir should be removed, err=%v", err)
	}
	if _, err := service.Update(ctx, uuid.New(), &model.UpdateSkillRequest{Description: "missing"}); err == nil {
		t.Fatalf("Update should fail for missing skill")
	}
	if err := service.BindSkills(ctx, uuid.New(), []uuid.UUID{platform.ID}); err == nil {
		t.Fatalf("BindSkills should fail for missing agent")
	}
	if err := service.BindSkills(ctx, agentID, []uuid.UUID{uuid.New()}); err == nil {
		t.Fatalf("BindSkills should fail for missing skill")
	}
	if err := service.IncrementUse(ctx, platform.ID); err != nil {
		t.Fatalf("IncrementUse returned error: %v", err)
	}
}

func TestUseCountUpdaterCountsProjectsAndResetsUnusedSkills(t *testing.T) {
	ctx := context.Background()
	db := openSkillServiceTestDB(t)
	skillRepo := repo.NewSkillRepository(db, repo.DBTypeSQLite)
	projectRepo := repo.NewProjectRepository(db, repo.DBTypeSQLite)
	bindingRepo := repo.NewAgentSkillBindingRepository(db, repo.DBTypeSQLite)
	updater := NewUseCountUpdater(skillRepo, projectRepo, bindingRepo)
	updater.SetLogger(nil)

	agentA := insertSkillServiceAgent(t, db, "Agent A")
	agentB := insertSkillServiceAgent(t, db, "Agent B")
	workflowA := uuid.New()
	workflowB := uuid.New()
	updater.SetWorkflowService(fakeWorkflowAgentLister{
		agents: map[uuid.UUID][]uuid.UUID{
			workflowA: {agentA, agentB},
			workflowB: {agentA},
		},
	})

	used := insertSkillServiceSkill(t, db, "used", 99)
	unused := insertSkillServiceSkill(t, db, "unused", 7)
	insertSkillServiceProject(t, db, "project-a", workflowA)
	insertSkillServiceProject(t, db, "project-b", workflowB)
	insertSkillServiceProject(t, db, "project-without-workflow", uuid.Nil)
	mustSkillServiceExec(t, db, `INSERT INTO agent_skill_bindings (id, agent_role_id, skill_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), agentA.String(), used.String(), time.Now())
	mustSkillServiceExec(t, db, `INSERT INTO agent_skill_bindings (id, agent_role_id, skill_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), agentB.String(), used.String(), time.Now())

	updater.UpdateAll(ctx)

	gotUsed := getSkillServiceUseCount(t, db, used)
	gotUnused := getSkillServiceUseCount(t, db, unused)
	if gotUsed != 2 {
		t.Fatalf("used skill count = %d, want 2 projects", gotUsed)
	}
	if gotUnused != 0 {
		t.Fatalf("unused skill count = %d, want 0", gotUnused)
	}
	updater.Stop()
}

func openSkillServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT, description TEXT, tags BLOB, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, base_agent_id TEXT, is_default INTEGER, is_system INTEGER, requires_human INTEGER, mention_patterns BLOB, config_generated_at TEXT, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_skill_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT, description TEXT, type TEXT, mode TEXT, status TEXT, local_path TEXT, git_repo TEXT, config BLOB, workflow_template_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertSkillServiceAgent(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO agent_configs (id, name, role, description, system_prompt, max_tokens, temperature, is_default, is_system, requires_human, mention_patterns, config_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "reviewer", "", "system", 4096, 0.2, 0, 0, 0, []byte(`[]`), "", now, now)
	if err != nil {
		t.Fatalf("insert agent config: %v", err)
	}
	return id
}

func insertSkillServiceSkill(t *testing.T, db *sql.DB, name string, useCount int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	mustSkillServiceExec(t, db, `INSERT INTO skills (id, name, description, tags, source_type, source_registry_id, source_path, author_id, project_id, use_count, status, is_public, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "skill", []byte(`[]`), model.SkillSourcePersonal, nil, "", nil, nil, useCount, model.SkillStatusActive, 1, now, now)
	return id
}

func insertSkillServiceProject(t *testing.T, db *sql.DB, name string, workflowID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	var workflow any
	if workflowID != uuid.Nil {
		workflow = workflowID.String()
	}
	mustSkillServiceExec(t, db, `INSERT INTO projects (id, name, description, type, mode, status, local_path, git_repo, config, workflow_template_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "", model.ProjectTypeApp, model.ProjectModeNew, model.ProjectStatusDeveloping, t.TempDir(), nil, []byte(`{}`), workflow, now, now)
	return id
}

func getSkillServiceUseCount(t *testing.T, db *sql.DB, skillID uuid.UUID) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT use_count FROM skills WHERE id = ?`, skillID.String()).Scan(&count); err != nil {
		t.Fatalf("query use_count: %v", err)
	}
	return count
}

func mustSkillServiceExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %s: %v", query, err)
	}
}

type fakeWorkflowAgentLister struct {
	agents map[uuid.UUID][]uuid.UUID
	err    error
}

func (f fakeWorkflowAgentLister) GetAgentIDs(ctx context.Context, templateID uuid.UUID) ([]uuid.UUID, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.agents[templateID], nil
}

func containsSkillTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}
