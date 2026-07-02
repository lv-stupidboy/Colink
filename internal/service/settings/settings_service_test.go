package settings

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

func TestSettingsServiceCRUDFilesAndBindings(t *testing.T) {
	ctx := context.Background()
	db := openSettingsTestDB(t)
	service := NewService(
		repo.NewSettingsRepository(db, repo.DBTypeSQLite),
		repo.NewAgentSettingsBindingRepository(db, repo.DBTypeSQLite),
		repo.NewAgentConfigRepository(db, repo.DBTypeSQLite),
		t.TempDir(),
		zap.NewNop(),
	)

	created, err := service.Create(ctx, &model.CreateSettingsRequest{Name: "runtime", Description: "runtime settings"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.DirectoryPath == "" {
		t.Fatalf("DirectoryPath should be set")
	}
	if _, err := os.Stat(created.DirectoryPath); err != nil {
		t.Fatalf("settings dir missing: %v", err)
	}
	if _, err := service.Create(ctx, &model.CreateSettingsRequest{Name: "runtime"}); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("duplicate create error = %v", err)
	}

	got, err := service.GetByID(ctx, created.ID)
	if err != nil || got.Name != "runtime" {
		t.Fatalf("GetByID = %#v err=%v", got, err)
	}
	byName, err := service.GetByName(ctx, "runtime")
	if err != nil || byName.ID != created.ID {
		t.Fatalf("GetByName = %#v err=%v", byName, err)
	}
	list, total, err := service.List(ctx, &model.SettingsListQuery{Search: "runtime", Page: 1, PageSize: 10})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("List = %#v total=%d err=%v", list, total, err)
	}

	updated, err := service.Update(ctx, created.ID, &model.UpdateSettingsRequest{Description: "updated"})
	if err != nil || updated.Description != "updated" {
		t.Fatalf("Update = %#v err=%v", updated, err)
	}

	withFiles, err := service.CreateFromFile(ctx, &CreateFromFileRequest{
		Name:        "hermes",
		Description: "Hermes config",
		Files: []FileData{
			{RelativePath: "config.yaml", Content: strings.NewReader("model: qwen")},
			{RelativePath: "mcp/mock.json", Content: strings.NewReader(`{"mock":true}`)},
		},
	})
	if err != nil {
		t.Fatalf("CreateFromFile returned error: %v", err)
	}
	if got := readSettingsFile(t, filepath.Join(withFiles.DirectoryPath, "mcp", "mock.json")); got != `{"mock":true}` {
		t.Fatalf("uploaded nested file = %q", got)
	}

	dir, err := service.ReadDirectoryContent(ctx, withFiles.ID, "")
	if err != nil {
		t.Fatalf("ReadDirectoryContent root returned error: %v", err)
	}
	if len(dir.Files) != 1 || dir.Files[0].Name != "config.yaml" || len(dir.Subdirs) != 1 || dir.Subdirs[0].Name != "mcp" {
		t.Fatalf("root dir content = %#v", dir)
	}
	subdir, err := service.ReadDirectoryContent(ctx, withFiles.ID, "mcp")
	if err != nil || len(subdir.Files) != 1 || subdir.Files[0].Name != "mock.json" {
		t.Fatalf("subdir content = %#v err=%v", subdir, err)
	}
	fileContent, err := service.ReadFileContent(ctx, withFiles.ID, "config.yaml")
	if err != nil || string(fileContent) != "model: qwen" {
		t.Fatalf("ReadFileContent = %q err=%v", string(fileContent), err)
	}
	if _, err := service.ReadFileContent(ctx, withFiles.ID, "mcp"); err == nil || !strings.Contains(err.Error(), "不是文件") {
		t.Fatalf("directory file read error = %v", err)
	}
	if _, err := service.ReadDirectoryContent(ctx, withFiles.ID, "config.yaml"); err == nil || !strings.Contains(err.Error(), "不是目录") {
		t.Fatalf("file dir read error = %v", err)
	}
	if _, err := service.ReadDirectoryContent(ctx, uuid.New(), ""); err == nil {
		t.Fatalf("missing settings dir read should fail")
	}

	agentID := insertSettingsAgent(t, db, "Admin")
	if err := service.BindSettings(ctx, agentID, []uuid.UUID{withFiles.ID}); err != nil {
		t.Fatalf("BindSettings returned error: %v", err)
	}
	boundSettings, err := service.GetBoundSettings(ctx, agentID)
	if err != nil || len(boundSettings) != 1 || boundSettings[0].Name != "hermes" {
		t.Fatalf("GetBoundSettings = %#v err=%v", boundSettings, err)
	}
	boundAgents, err := service.GetBoundAgents(ctx, withFiles.ID)
	if err != nil || len(boundAgents) != 1 || boundAgents[0].Name != "Admin" {
		t.Fatalf("GetBoundAgents = %#v err=%v", boundAgents, err)
	}
	if err := service.Delete(ctx, withFiles.ID); err == nil || !strings.Contains(err.Error(), "Admin") {
		t.Fatalf("Delete bound settings error = %v", err)
	}
	if err := service.UnbindSettings(ctx, agentID, withFiles.ID); err != nil {
		t.Fatalf("UnbindSettings returned error: %v", err)
	}
	if err := service.UnbindSettings(ctx, agentID, withFiles.ID); !errors.Is(err, errors.New("绑定关系不存在")) && (err == nil || !strings.Contains(err.Error(), "绑定关系不存在")) {
		t.Fatalf("missing unbind error = %v", err)
	}
	dirPath := withFiles.DirectoryPath
	if err := service.Delete(ctx, withFiles.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Fatalf("settings directory should be removed, err=%v", err)
	}
}

func openSettingsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE settings (id TEXT PRIMARY KEY, name TEXT UNIQUE, description TEXT, directory_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_settings_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, settings_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, base_agent_id TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, is_default BOOLEAN, is_system BOOLEAN, requires_human BOOLEAN, mention_patterns TEXT, config_generated_at TIMESTAMP, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertSettingsAgent(t *testing.T, db *sql.DB, name string) uuid.UUID {
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

func readSettingsFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
