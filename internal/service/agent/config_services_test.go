package agent

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestBaseAgentServiceLifecycleAndSanitization(t *testing.T) {
	ctx := context.Background()
	db := openAgentServiceTestDB(t)
	baseRepo := repo.NewBaseAgentRepository(db, repo.DBTypeSQLite)
	service := NewBaseAgentService(baseRepo)

	created, err := service.Create(ctx, &model.CreateBaseAgentRequest{
		Name:         "Hermes",
		Type:         model.BaseAgentType("hermes"),
		ApiURL:       "https://api",
		ApiToken:     "secret",
		DefaultModel: "qwen",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ApiToken != "" || created.MaxTokens != 4096 || created.TimeoutMinutes != 30 {
		t.Fatalf("created agent sanitized/defaulted incorrectly: %#v", created)
	}

	got, err := service.GetByID(ctx, created.ID)
	if err != nil || got.ApiToken != "" || got.Name != "Hermes" {
		t.Fatalf("GetByID = %#v err=%v", got, err)
	}
	// Mutating the sanitized result must not poison the cached secret-bearing object.
	got.Name = "mutated"
	gotAgain, err := service.GetByID(ctx, created.ID)
	if err != nil || gotAgain.Name != "Hermes" {
		t.Fatalf("cached GetByID = %#v err=%v", gotAgain, err)
	}

	if byType, err := service.GetByType(ctx, model.BaseAgentType("hermes")); err != nil || len(byType) != 1 || byType[0].ApiToken != "" {
		t.Fatalf("GetByType = %#v err=%v", byType, err)
	}
	if list, err := service.List(ctx); err != nil || len(list) != 1 || list[0].ApiToken != "" {
		t.Fatalf("List = %#v err=%v", list, err)
	}
	if active, err := service.ListActive(ctx); err != nil || len(active) != 1 {
		t.Fatalf("ListActive = %#v err=%v", active, err)
	}

	apiURL := ""
	token := ""
	modelName := "claude"
	gitBash := ""
	updated, err := service.Update(ctx, created.ID, &model.UpdateBaseAgentRequest{
		Name:           "Claude",
		ApiURL:         &apiURL,
		ApiToken:       &token,
		DefaultModel:   &modelName,
		GitBashPath:    &gitBash,
		MaxTokens:      8192,
		TimeoutMinutes: 5,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Claude" || updated.DefaultModel != "claude" || updated.ApiToken != "" || updated.MaxTokens != 8192 {
		t.Fatalf("updated agent = %#v", updated)
	}

	if err := service.SetDefault(ctx, created.ID); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if def, err := service.GetDefault(ctx); err != nil || def == nil || def.ID != created.ID {
		t.Fatalf("GetDefault = %#v err=%v", def, err)
	}
	if err := service.ClearDefault(ctx, created.ID); err != nil {
		t.Fatalf("ClearDefault: %v", err)
	}
	if def, err := service.GetDefault(ctx); err != nil || def != nil {
		t.Fatalf("GetDefault after clear = %#v err=%v", def, err)
	}
	if err := service.SetDefault(ctx, uuid.New()); !errors.Is(err, ErrBaseAgentNotFound) {
		t.Fatalf("SetDefault missing err = %v", err)
	}
	if len(service.GetTypes()) == 0 {
		t.Fatalf("GetTypes should return built-in types")
	}
	if err := service.TestConnection(ctx, created.ID); err == nil {
		t.Fatalf("TestConnection should fail without a real CLI")
	}
	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestConfigServiceLifecycleBatchAndBaseAgentFill(t *testing.T) {
	ctx := context.Background()
	db := openAgentServiceTestDB(t)
	configRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	baseRepo := repo.NewBaseAgentRepository(db, repo.DBTypeSQLite)
	configSvc := NewConfigService(configRepo, baseRepo)

	now := time.Now()
	baseA := &model.BaseAgent{ID: uuid.New(), Name: "Hermes", Type: model.BaseAgentType("hermes"), CliPath: "hermes", IsDefault: true, CreatedAt: now, UpdatedAt: now}
	baseB := &model.BaseAgent{ID: uuid.New(), Name: "OpenCode", Type: model.BaseAgentType("open_code"), CliPath: "opencode", CreatedAt: now, UpdatedAt: now}
	for _, base := range []*model.BaseAgent{baseA, baseB} {
		if err := baseRepo.Create(ctx, base); err != nil {
			t.Fatalf("create base %s: %v", base.Name, err)
		}
	}

	created, err := configSvc.Create(ctx, &model.CreateAgentRequest{
		Name:            "Planner",
		BaseAgentID:     baseA.ID,
		Description:     "plans",
		SystemPrompt:    "plan",
		IsDefault:       true,
		RequiresHuman:   true,
		MentionPatterns: []string{"@planner"},
	})
	if err != nil {
		t.Fatalf("Create config: %v", err)
	}
	if created.Role != model.AgentRoleAgent || !created.RequiresHuman {
		t.Fatalf("created config = %#v", created)
	}
	configSvc.InvalidateCache(created.ID)
	got, err := configSvc.GetByID(ctx, created.ID)
	if err != nil || got.BaseAgent == nil || got.BaseAgent.ID != baseA.ID {
		t.Fatalf("GetByID = %#v err=%v", got, err)
	}
	got.Name = "cache-mutated"
	gotAgain, err := configSvc.GetByID(ctx, created.ID)
	if err != nil || gotAgain.Name != "cache-mutated" {
		t.Fatalf("cache should return same config pointer before invalidation, got %#v err=%v", gotAgain, err)
	}
	configSvc.InvalidateCache(created.ID)
	gotFresh, err := configSvc.GetByID(ctx, created.ID)
	if err != nil || gotFresh.Name != "Planner" {
		t.Fatalf("fresh GetByID = %#v err=%v", gotFresh, err)
	}

	updated, err := configSvc.Update(ctx, created.ID, &model.CreateAgentRequest{Name: "Coder", Role: model.AgentRoleReviewer, BaseAgentID: baseB.ID, Description: "codes", SystemPrompt: "code", MentionPatterns: []string{"@coder"}})
	if err != nil {
		t.Fatalf("Update config: %v", err)
	}
	if updated.Role != model.AgentRoleReviewer || len(updated.MentionPatterns) != 1 {
		t.Fatalf("updated config = %#v", updated)
	}
	if err := configSvc.RefreshCache(ctx, created.ID); err != nil {
		t.Fatalf("RefreshCache: %v", err)
	}
	if byRole, err := configSvc.GetByRole(ctx, model.AgentRoleReviewer); err != nil || len(byRole) != 1 || byRole[0].BaseAgent == nil {
		t.Fatalf("GetByRole = %#v err=%v", byRole, err)
	}
	if def, err := configSvc.GetDefaultByRole(ctx, model.AgentRoleReviewer); err != nil || def.ID != created.ID {
		t.Fatalf("GetDefaultByRole = %#v err=%v", def, err)
	}
	if _, err := configSvc.GetDefaultByRole(ctx, model.AgentRoleHuman); !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("GetDefaultByRole missing err = %v", err)
	}
	if list, err := configSvc.List(ctx); err != nil || len(list) != 1 || list[0].BaseAgent == nil {
		t.Fatalf("List = %#v err=%v", list, err)
	}

	generator := &fakeConfigGenerator{failFor: map[uuid.UUID]bool{}}
	batch, err := configSvc.BatchGenerateConfig(ctx, []uuid.UUID{created.ID, uuid.New()}, "", generator)
	if err != nil {
		t.Fatalf("BatchGenerateConfig: %v", err)
	}
	if batch.Total != 2 || batch.Success != 1 || batch.Failed != 1 || generator.cliType != "claude_code" {
		t.Fatalf("batch generate = %#v generator=%#v", batch, generator)
	}
	generator.failFor[created.ID] = true
	batch, err = configSvc.BatchGenerateConfig(ctx, []uuid.UUID{created.ID}, "hermes", generator)
	if err != nil || batch.Success != 0 || batch.Failed != 1 || batch.Results[0].Error != "boom" {
		t.Fatalf("batch generate failure = %#v err=%v", batch, err)
	}

	updateResult, err := configSvc.BatchUpdateBaseAgent(ctx, []uuid.UUID{created.ID, uuid.New()}, baseA.ID)
	if err != nil {
		t.Fatalf("BatchUpdateBaseAgent: %v", err)
	}
	if updateResult.Success != 1 || updateResult.Failed != 1 || updateResult.Results[0].BaseAgentName != "Hermes" {
		t.Fatalf("batch update = %#v", updateResult)
	}
	if _, err := configSvc.BatchUpdateBaseAgent(ctx, []uuid.UUID{created.ID}, uuid.New()); err == nil {
		t.Fatalf("BatchUpdateBaseAgent missing base should fail")
	}
	if err := configSvc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete config: %v", err)
	}
	if _, err := configSvc.GetByID(ctx, created.ID); err == nil {
		t.Fatalf("deleted config should not be found")
	}
}

type fakeConfigGenerator struct {
	failFor map[uuid.UUID]bool
	cliType string
}

func (g *fakeConfigGenerator) GenerateAgentConfig(ctx context.Context, agentId uuid.UUID, cliType string) (int, int, int, int, int, error) {
	g.cliType = cliType
	if g.failFor[agentId] {
		return 0, 0, 0, 0, 0, errors.New("boom")
	}
	return 1, 2, 3, 4, 5, nil
}

func openAgentServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := []string{
		`CREATE TABLE base_agents (id TEXT PRIMARY KEY, name TEXT, type TEXT, api_url TEXT, api_token TEXT, default_model TEXT, cli_path TEXT, git_bash_path TEXT, max_tokens INTEGER, timeout_minutes INTEGER, is_default INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, base_agent_id TEXT, is_default INTEGER, is_system INTEGER, requires_human INTEGER, mention_patterns BLOB, config_generated_at TEXT, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}
