package teampackagesync

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/google/uuid"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1, v2 string
		want   int
	}{
		{"", "", 0},
		{"", "1.0.0", -1},
		{"1.0.0", "", 1},
		{"v1.2.0", "1.1.9", 1},
		{"1.2", "1.2.0", 0},
		{"1.2.3", "1.2.10", -1},
		{"2.0.0", "10.0.0", -1},
		{"1.0.bad", "1.0.0", 0},
		{"1.0.1", "1.0.bad", 1},
	}
	for _, tt := range tests {
		if got := CompareVersions(tt.v1, tt.v2); got != tt.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

func TestSyncServiceParseMarketplaceJSON(t *testing.T) {
	root := t.TempDir()
	market := model.Marketplace{
		Name:        "Colink Market",
		Version:     "1.0.0",
		Description: "market",
		Plugins: []model.Plugin{
			{Name: "devmind", Version: "0.1.0", Category: "team", Repository: "git@example.com/repo.git", Source: "packages/devmind"},
		},
	}
	data, err := json.Marshal(market)
	if err != nil {
		t.Fatalf("marshal market: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "marketplace.json"), data, 0644); err != nil {
		t.Fatalf("write marketplace: %v", err)
	}

	service := NewSyncService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, config.TeamPackageSyncConfig{}, t.TempDir(), zap.NewNop(), nil)
	got, err := service.parseMarketplaceJSON(root)
	if err != nil {
		t.Fatalf("parseMarketplaceJSON returned error: %v", err)
	}
	if got.Name != "Colink Market" || len(got.Plugins) != 1 || got.Plugins[0].Name != "devmind" {
		t.Fatalf("marketplace = %#v", got)
	}

	if _, err := service.parseMarketplaceJSON(filepath.Join(root, "missing")); err == nil {
		t.Fatalf("missing marketplace should fail")
	}
	if err := os.WriteFile(filepath.Join(root, "marketplace.json"), []byte("{bad"), 0644); err != nil {
		t.Fatalf("write bad marketplace: %v", err)
	}
	if _, err := service.parseMarketplaceJSON(root); err == nil {
		t.Fatalf("invalid marketplace should fail")
	}
}

func TestSyncServiceCreateZipFromDir(t *testing.T) {
	root := t.TempDir()
	writeSyncFile(t, filepath.Join(root, "manifest.json"), `{"name":"devmind"}`)
	writeSyncFile(t, filepath.Join(root, "assets", "skill.md"), "skill")

	service := NewSyncService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, config.TeamPackageSyncConfig{}, t.TempDir(), zap.NewNop(), nil)
	data, err := service.createZipFromDir(root)
	if err != nil {
		t.Fatalf("createZipFromDir returned error: %v", err)
	}
	names := zipNames(t, data)
	for _, want := range []string{"manifest.json", "assets/", "assets/skill.md"} {
		if !containsName(names, want) {
			t.Fatalf("zip names %v missing %s", names, want)
		}
	}

	if _, err := service.createZipFromDir(filepath.Join(root, "missing")); err == nil {
		t.Fatalf("missing dir zip should fail")
	}
}

func TestSyncServiceBuildPreviewResponseDetectsConflicts(t *testing.T) {
	ctx := context.Background()
	db := openSyncServiceTestDB(t)
	service := newSyncServiceWithRepos(db)
	now := time.Now()
	roleID := uuid.New()
	workflowID := uuid.New()

	mustSyncExec(t, db, `INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, routable_teams, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workflowID.String(), "DevMind Team", "old workflow", []byte(`[]`), []byte(`[]`), []byte(`[]`), "1h", 0, 0, []byte(`[]`), now, now)
	mustSyncExec(t, db, `INSERT INTO agent_configs (id, name, role, description, system_prompt, max_tokens, temperature, base_agent_id, is_default, is_system, requires_human, mention_patterns, config_generated_at, config_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		roleID.String(), "Planner", "agent", "old role", "system", 4096, 0.2, nil, 0, 0, 0, []byte(`["@planner"]`), nil, "", now, now)
	mustSyncExec(t, db, `INSERT INTO skills (id, name, description, tags, source_type, source_registry_id, source_path, author_id, project_id, use_count, status, is_public, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), "Review Skill", "old skill", []byte(`[]`), model.SkillSourcePersonal, nil, "", nil, nil, 0, model.SkillStatusActive, 1, now, now)
	mustSyncExec(t, db, `INSERT INTO commands (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), "Build", "old command", now, now)
	mustSyncExec(t, db, `INSERT INTO subagents (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), "Reviewer", "old subagent", now, now)
	mustSyncExec(t, db, `INSERT INTO rules (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), "Secure", "old rule", now, now)
	mustSyncExec(t, db, `INSERT INTO settings (id, name, description, directory_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), "Defaults", "old settings", "/tmp/defaults", now, now)

	manifest := model.TeamPackageManifest{
		Workflow: model.TeamPackageWorkflow{Name: "DevMind Team", Description: "new workflow"},
		Roles: []model.TeamPackageRole{{
			ID:          roleID.String(),
			Name:        "Planner",
			Role:        string(model.AgentRoleAgent),
			Description: "new role",
			Bindings: model.TeamPackageBindings{
				Skills:    []string{"Review Skill"},
				Commands:  []string{"Build"},
				Subagents: []string{"Reviewer"},
				Rules:     []string{"Secure"},
				Settings:  []string{"Defaults"},
			},
		}},
		Assets: model.TeamPackageAssets{
			Skills:    []model.AssetPackageSkillItem{{Name: "Review Skill", Description: "new skill"}},
			Commands:  []model.AssetPackageCommandItem{{Name: "Build", Description: "new command"}},
			Subagents: []model.AssetPackageSubagentItem{{Name: "Reviewer", Description: "new subagent"}},
			Rules:     []model.AssetPackageRuleItem{{Name: "Secure", Description: "new rule"}},
			Settings:  []model.AssetPackageSettingsItem{{Name: "Defaults", Description: "new settings"}},
		},
	}
	preview, err := service.buildPreviewResponse(ctx, "devmind", &RemotePackage{Name: "devmind", Version: "1.2.3", Description: "remote"}, manifest)
	if err != nil {
		t.Fatalf("buildPreviewResponse returned error: %v", err)
	}
	if preview.PackageName != "devmind" || preview.Version != "1.2.3" || preview.ConflictCount != 7 {
		t.Fatalf("preview summary = %+v", preview)
	}
	if !preview.Workflow.Exists || !preview.Roles[0].Exists || !preview.Assets.Skills[0].Exists ||
		!preview.Assets.Commands[0].Exists || !preview.Assets.Subagents[0].Exists ||
		!preview.Assets.Rules[0].Exists || !preview.Assets.Settings[0].Exists {
		t.Fatalf("preview did not detect conflicts: %+v", preview)
	}
	if len(preview.Roles[0].Assets) != 5 {
		t.Fatalf("role assets = %#v", preview.Roles[0].Assets)
	}
}

func TestSyncServiceVersionRecordsAndLocalVersions(t *testing.T) {
	ctx := context.Background()
	db := openSyncServiceTestDB(t)
	service := newSyncServiceWithRepos(db)
	workflowID := uuid.New()
	remote := &RemotePackage{Name: "devmind", Version: "1.0.0", Description: "first"}

	if err := service.updateVersionRecord(ctx, "devmind", remote, &model.ImportResult{}); err != nil {
		t.Fatalf("updateVersionRecord without workflow returned error: %v", err)
	}
	versions, err := service.GetLocalVersions(ctx)
	if err != nil {
		t.Fatalf("GetLocalVersions returned error: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("versions should stay empty without workflow id: %#v", versions)
	}

	result := &model.ImportResult{Details: []model.ImportDetail{{AssetType: "workflow", Status: "success", ID: workflowID.String()}}}
	if err := service.updateVersionRecord(ctx, "devmind", remote, result); err != nil {
		t.Fatalf("create version returned error: %v", err)
	}
	versions, err = service.GetLocalVersions(ctx)
	if err != nil {
		t.Fatalf("GetLocalVersions after create returned error: %v", err)
	}
	if len(versions) != 1 || versions[0].Name != "devmind" || versions[0].Version != "1.0.0" || versions[0].WorkflowID != workflowID {
		t.Fatalf("created version = %#v", versions)
	}

	remote.Version = "1.1.0"
	remote.Description = "updated"
	if err := service.updateVersionRecord(ctx, "devmind", remote, &model.ImportResult{Details: []model.ImportDetail{{AssetType: "workflow", Status: "skipped", ID: workflowID.String()}}}); err != nil {
		t.Fatalf("update version returned error: %v", err)
	}
	version, err := service.versionRepo.FindByName(ctx, "devmind")
	if err != nil {
		t.Fatalf("FindByName returned error: %v", err)
	}
	if version.Version != "1.1.0" || version.Description != "updated" || version.WorkflowID != workflowID || version.LastSyncedAt == nil {
		t.Fatalf("updated version = %#v", version)
	}

	if err := service.updateVersionRecord(ctx, "broken", remote, &model.ImportResult{Details: []model.ImportDetail{{AssetType: "workflow", Status: "success", ID: "not-a-uuid"}}}); err == nil {
		t.Fatalf("invalid workflow id should fail")
	}
}

func TestSyncCheckerSkipsDisabledAndManualMarkets(t *testing.T) {
	ctx := context.Background()
	db := openSyncServiceTestDB(t)
	marketRepo := repo.NewMarketRepository(db, repo.DBTypeSQLite)
	versionRepo := repo.NewTeamPackageVersionRepository(db, repo.DBTypeSQLite)
	service := newSyncServiceWithRepos(db)
	checker := NewSyncChecker(service, marketRepo, versionRepo, time.Hour, zap.NewNop())

	if checker.syncSvc != service || checker.marketRepo != marketRepo || checker.versionRepo != versionRepo || checker.interval != time.Hour {
		t.Fatalf("checker fields not initialized")
	}

	if err := marketRepo.Create(ctx, &model.Market{Name: "disabled", URL: "git@example.com/disabled.git", Branch: "main", Enabled: false, AutoUpdate: true}); err != nil {
		t.Fatalf("create disabled market: %v", err)
	}
	if err := marketRepo.Create(ctx, &model.Market{Name: "manual", URL: "git@example.com/manual.git", Branch: "main", Enabled: true, AutoUpdate: false}); err != nil {
		t.Fatalf("create manual market: %v", err)
	}
	if err := versionRepo.Create(ctx, &model.TeamPackageVersion{WorkflowID: uuid.New(), Name: "devmind", Category: "team", Version: "1.0.0"}); err != nil {
		t.Fatalf("create local version: %v", err)
	}

	checker.check()

	local, err := versionRepo.FindByName(ctx, "devmind")
	if err != nil || local == nil || local.Version != "1.0.0" {
		t.Fatalf("local version changed unexpectedly: %+v err=%v", local, err)
	}

	fastChecker := NewSyncChecker(service, marketRepo, versionRepo, time.Hour, zap.NewNop())
	done := make(chan struct{})
	go func() {
		fastChecker.runLoop()
		close(done)
	}()
	fastChecker.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runLoop did not stop")
	}
}

func writeSyncFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func zipNames(t *testing.T, data []byte) []string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, strings.ReplaceAll(file.Name, "\\", "/"))
	}
	return names
}

func containsName(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func openSyncServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE team_package_versions (id TEXT PRIMARY KEY, workflow_id TEXT, name TEXT, category TEXT, version TEXT, description TEXT, last_synced_at TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE workflow_templates (id TEXT PRIMARY KEY, name TEXT, description TEXT, agent_ids BLOB, transitions BLOB, checkpoints BLOB, estimated_time TEXT, is_system INTEGER, is_default INTEGER, routable_teams BLOB, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, base_agent_id TEXT, is_default INTEGER, is_system INTEGER, requires_human INTEGER, mention_patterns BLOB, config_generated_at TEXT, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT, description TEXT, tags BLOB, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE commands (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE subagents (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE rules (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE settings (id TEXT PRIMARY KEY, name TEXT, description TEXT, directory_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE markets (id TEXT PRIMARY KEY, name TEXT, url TEXT, branch TEXT, enabled INTEGER, auto_update INTEGER, check_interval TEXT, last_synced_at TEXT, last_error TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create sync test schema: %v", err)
		}
	}
	return db
}

func newSyncServiceWithRepos(db *sql.DB) *SyncService {
	return NewSyncService(
		repo.NewTeamPackageVersionRepository(db, repo.DBTypeSQLite),
		repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite),
		repo.NewAgentConfigRepository(db, repo.DBTypeSQLite),
		repo.NewSkillRepository(db, repo.DBTypeSQLite),
		repo.NewCommandRepository(db, repo.DBTypeSQLite),
		repo.NewSubagentRepository(db, repo.DBTypeSQLite),
		repo.NewRuleRepository(db, repo.DBTypeSQLite),
		repo.NewSettingsRepository(db, repo.DBTypeSQLite),
		nil,
		nil,
		config.TeamPackageSyncConfig{},
		os.TempDir(),
		zap.NewNop(),
		nil,
	)
}

func mustSyncExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %s: %v", query, err)
	}
}
