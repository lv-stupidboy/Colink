package configgen

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
	_ "modernc.org/sqlite"
	"go.uber.org/zap"
)

func TestServiceConfigDirAndFileCopyHelpers(t *testing.T) {
	root := t.TempDir()
	commandStore := filepath.Join(root, "commands")
	ruleStore := filepath.Join(root, "rules")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(commandStore, 0755); err != nil {
		t.Fatalf("mkdir command store: %v", err)
	}
	if err := os.MkdirAll(ruleStore, 0755); err != nil {
		t.Fatalf("mkdir rule store: %v", err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandStore, "review.md"), []byte("command body"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ruleStore, "secure.md"), []byte("rule body"), 0644); err != nil {
		t.Fatalf("write rule: %v", err)
	}

	service := &Service{
		commandStoragePath: commandStore,
		ruleStoragePath:    ruleStore,
		logger:             zap.NewNop(),
	}

	if got := service.getConfigDir("/repo", "unknown_agent"); !strings.HasSuffix(got, filepath.Join("/repo", ".claude")) {
		t.Fatalf("fallback config dir = %q", got)
	}
	if got := service.getConfigDirName("claude_code"); got != ".claude" {
		t.Fatalf("claude config dir name = %q", got)
	}

	if err := service.copyCommandFile(&model.Command{Name: "review"}, target); err != nil {
		t.Fatalf("copyCommandFile returned error: %v", err)
	}
	assertFileBody(t, filepath.Join(target, "review.md"), "command body")

	if err := service.copyRuleFile(&model.Rule{Name: "secure"}, target); err != nil {
		t.Fatalf("copyRuleFile returned error: %v", err)
	}
	assertFileBody(t, filepath.Join(target, "secure.md"), "rule body")

	if err := service.copyCommandFile(&model.Command{Name: "missing"}, target); err == nil {
		t.Fatalf("copyCommandFile should fail for missing command source")
	}
	if err := service.copyRuleFile(&model.Rule{Name: "missing"}, target); err == nil {
		t.Fatalf("copyRuleFile should fail for missing rule source")
	}
}

func TestServiceCopySettingsDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "settings")
	target := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "config.yaml"), []byte("root: true"), 0644); err != nil {
		t.Fatalf("write source config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "env"), []byte("dev"), 0644); err != nil {
		t.Fatalf("write nested config: %v", err)
	}

	service := &Service{logger: zap.NewNop()}
	if err := service.copySettingsDirectory(&model.Settings{Name: "defaults", DirectoryPath: source}, target); err != nil {
		t.Fatalf("copySettingsDirectory returned error: %v", err)
	}
	assertFileBody(t, filepath.Join(target, "config.yaml"), "root: true")
	assertFileBody(t, filepath.Join(target, "nested", "env"), "dev")

	if err := service.copySettingsDirectory(&model.Settings{Name: "empty"}, target); err == nil || !strings.Contains(err.Error(), "Settings目录路径为空") {
		t.Fatalf("expected empty path error, got %v", err)
	}
	if err := service.copySettingsDirectory(&model.Settings{Name: "missing", DirectoryPath: filepath.Join(root, "missing")}, target); err == nil {
		t.Fatalf("expected missing settings directory error")
	}
}

func TestServiceCacheInvalidateCallbackAndAutoGeneratorNilBindings(t *testing.T) {
	service := &Service{}
	agentID := uuid.New()
	var invalidated uuid.UUID
	service.SetCacheInvalidateCallback(func(id uuid.UUID) {
		invalidated = id
	})
	service.onCacheInvalidate(agentID)
	if invalidated != agentID {
		t.Fatalf("cache invalidation id = %s, want %s", invalidated, agentID)
	}

	generator := NewAutoGenerator(nil, nil, nil, &BindingRepositories{}, zap.NewNop())
	ctx := context.Background()
	if got, err := generator.GetAffectedAgentsBySkill(ctx, uuid.New()); err != nil || got != nil {
		t.Fatalf("skill affected agents = %#v err=%v", got, err)
	}
	if got, err := generator.GetAffectedAgentsByCommand(ctx, uuid.New()); err != nil || got != nil {
		t.Fatalf("command affected agents = %#v err=%v", got, err)
	}
	if got, err := generator.GetAffectedAgentsBySubagent(ctx, uuid.New()); err != nil || got != nil {
		t.Fatalf("subagent affected agents = %#v err=%v", got, err)
	}
	if got, err := generator.GetAffectedAgentsByRule(ctx, uuid.New()); err != nil || got != nil {
		t.Fatalf("rule affected agents = %#v err=%v", got, err)
	}
	if got, err := generator.GetAffectedAgentsBySettings(ctx, uuid.New()); err != nil || got != nil {
		t.Fatalf("settings affected agents = %#v err=%v", got, err)
	}
}

func TestPreviewAgentConfigAggregatesBoundAssets(t *testing.T) {
	ctx := context.Background()
	db := openConfiggenTestDB(t)
	now := time.Now()

	agentID := uuid.New()
	directSkillID := uuid.New()
	sharedSkillID := uuid.New()
	commandID := uuid.New()
	subagentID := uuid.New()
	ruleID := uuid.New()
	settingsID := uuid.New()

	agentRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	skillRepo := repo.NewSkillRepository(db, repo.DBTypeSQLite)
	bindingRepo := repo.NewAgentSkillBindingRepository(db, repo.DBTypeSQLite)
	subagentRepo := repo.NewSubagentRepository(db, repo.DBTypeSQLite)
	agentSubagentBindingRepo := repo.NewAgentSubagentBindingRepository(db, repo.DBTypeSQLite)
	commandRepo := repo.NewCommandRepository(db, repo.DBTypeSQLite)
	ruleRepo := repo.NewRuleRepository(db, repo.DBTypeSQLite)
	agentCommandBindingRepo := repo.NewAgentCommandBindingRepository(db, repo.DBTypeSQLite)
	agentRuleBindingRepo := repo.NewAgentRuleBindingRepository(db, repo.DBTypeSQLite)
	commandSkillBindingRepo := repo.NewCommandSkillBindingRepository(db, repo.DBTypeSQLite)
	subagentSkillBindingRepo := repo.NewSubagentSkillBindingRepository(db, repo.DBTypeSQLite)
	settingsRepo := repo.NewSettingsRepository(db, repo.DBTypeSQLite)
	agentSettingsBindingRepo := repo.NewAgentSettingsBindingRepository(db, repo.DBTypeSQLite)

	must(t, agentRepo.Create(ctx, &model.AgentRoleConfig{
		ID:           agentID,
		Name:         "Planner",
		Role:         model.AgentRoleAgent,
		Description:  "plans work",
		SystemPrompt: "plan",
		CreatedAt:    now,
		UpdatedAt:    now,
	}))
	must(t, skillRepo.Create(ctx, &model.Skill{ID: directSkillID, Name: "Direct Skill", Description: "direct", SourceType: model.SkillSourcePersonal, Status: model.SkillStatusActive, CreatedAt: now, UpdatedAt: now}))
	must(t, skillRepo.Create(ctx, &model.Skill{ID: sharedSkillID, Name: "Shared Skill", Description: "shared", SourceType: model.SkillSourcePersonal, Status: model.SkillStatusActive, CreatedAt: now, UpdatedAt: now}))
	must(t, commandRepo.Create(ctx, &model.Command{ID: commandID, Name: "Ship", Description: "ship cmd", CreatedAt: now, UpdatedAt: now}))
	must(t, subagentRepo.Create(ctx, &model.Subagent{ID: subagentID, Name: "Reviewer", Description: "review bot", CreatedAt: now, UpdatedAt: now}))
	must(t, ruleRepo.Create(ctx, &model.Rule{ID: ruleID, Name: "Secure", Description: "security", CreatedAt: now, UpdatedAt: now}))
	must(t, settingsRepo.Create(ctx, &model.Settings{ID: settingsID, Name: "Defaults", Description: "default settings", CreatedAt: now, UpdatedAt: now}))

	must(t, bindingRepo.Create(ctx, &model.AgentSkillBinding{ID: uuid.New(), AgentRoleID: agentID, SkillID: directSkillID, CreatedAt: now}))
	must(t, agentCommandBindingRepo.Create(ctx, &model.AgentCommandBinding{ID: uuid.New(), AgentRoleID: agentID, CommandID: commandID, CreatedAt: now}))
	must(t, commandSkillBindingRepo.Create(ctx, &model.CommandSkillBinding{ID: uuid.New(), CommandID: commandID, SkillID: sharedSkillID, CreatedAt: now}))
	must(t, agentSubagentBindingRepo.Create(ctx, &model.AgentSubagentBinding{ID: uuid.New(), AgentRoleID: agentID, SubagentID: subagentID, CreatedAt: now}))
	must(t, subagentSkillBindingRepo.Create(ctx, &model.SubagentSkillBinding{ID: uuid.New(), SubagentID: subagentID, SkillID: sharedSkillID, CreatedAt: now}))
	must(t, agentRuleBindingRepo.Create(ctx, &model.AgentRuleBinding{ID: uuid.New(), AgentRoleID: agentID, RuleID: ruleID, CreatedAt: now}))
	must(t, agentSettingsBindingRepo.Create(ctx, &model.AgentSettingsBinding{ID: uuid.New(), AgentRoleID: agentID, SettingsID: settingsID, CreatedAt: now}))

	service := &Service{
		agentRepo:                agentRepo,
		skillRepo:                skillRepo,
		bindingRepo:              bindingRepo,
		subagentRepo:             subagentRepo,
		agentSubagentBindingRepo: agentSubagentBindingRepo,
		commandRepo:              commandRepo,
		ruleRepo:                 ruleRepo,
		agentCommandBindingRepo:  agentCommandBindingRepo,
		agentRuleBindingRepo:     agentRuleBindingRepo,
		commandSkillBindingRepo:  commandSkillBindingRepo,
		subagentSkillBindingRepo: subagentSkillBindingRepo,
		settingsRepo:             settingsRepo,
		agentSettingsBindingRepo: agentSettingsBindingRepo,
		logger:                   zap.NewNop(),
	}

	preview, err := service.PreviewAgentConfig(ctx, agentID)
	if err != nil {
		t.Fatalf("PreviewAgentConfig returned error: %v", err)
	}
	if preview.AgentID != agentID.String() || preview.AgentName != "Planner" {
		t.Fatalf("unexpected preview identity: %+v", preview)
	}
	if preview.SkillsCount != 2 || preview.CommandsCount != 1 || preview.SubagentsCount != 1 || preview.RulesCount != 1 || preview.SettingsCount != 1 {
		t.Fatalf("unexpected preview counts: %+v", preview)
	}
	if !previewHasName(preview.Skills, "Direct Skill") || !previewHasName(preview.Skills, "Shared Skill") {
		t.Fatalf("expected direct and indirect skills, got %+v", preview.Skills)
	}
	if !previewHasName(preview.Commands, "Ship") || !previewHasName(preview.Subagents, "Reviewer") || !previewHasName(preview.Rules, "Secure") || !previewHasName(preview.Settings, "Defaults") {
		t.Fatalf("missing bound assets: %+v", preview)
	}

	assets := service.getFilteredAssets(ctx, agentID)
	if len(assets.Skills) != 2 || len(assets.Commands) != 1 || len(assets.Subagents) != 1 || len(assets.Rules) != 1 || len(assets.Settings) != 1 {
		t.Fatalf("unexpected filtered assets: %+v", assets)
	}
}

func assertFileBody(t *testing.T, path, want string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(body) != want {
		t.Fatalf("%s body = %q, want %q", path, body, want)
	}
}

func openConfiggenTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := []string{
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, base_agent_id TEXT, is_default INTEGER, is_system INTEGER, requires_human INTEGER, mention_patterns BLOB, config_generated_at TEXT, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT, description TEXT, tags BLOB, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE commands (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE subagents (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE rules (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE settings (id TEXT PRIMARY KEY, name TEXT, description TEXT, directory_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_skill_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_command_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, command_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE command_skill_bindings (id TEXT PRIMARY KEY, command_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_subagent_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, subagent_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE subagent_skill_bindings (id TEXT PRIMARY KEY, subagent_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_rule_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, rule_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_settings_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, settings_id TEXT, created_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func previewHasName(items []PreviewAssetItem, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
