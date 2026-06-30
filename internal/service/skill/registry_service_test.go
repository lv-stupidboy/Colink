package skill

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
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

func TestRegistryServiceAPISyncLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openRegistryServiceTestDB(t)
	registryRepo := repo.NewSkillRegistryRepository(db, repo.DBTypeSQLite)
	skillRepo := repo.NewSkillRepository(db, repo.DBTypeSQLite)
	scanner := NewSkillScanner(nil, nil, nil, nil, t.TempDir(), "", zap.NewNop())
	service := NewRegistryService(registryRepo, skillRepo, scanner)

	requests := 0
	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("Authorization = %q", got)
		}
		body, err := json.Marshal(map[string]any{
			"skills": []map[string]any{
				{"name": "review", "description": "remote review", "path": "skills/review", "tags": []string{"go", "review"}},
				{"name": "new-skill", "description": "new remote", "path": "skills/new"},
				{"name": "conflict", "description": "conflicting remote", "path": "remote/conflict"},
			},
		})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    r,
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	registry, err := service.Create(ctx, &model.CreateRegistryRequest{
		Name:        "api-registry",
		DisplayName: "API Registry",
		Type:        model.RegistryTypeAPI,
		URL:         "https://registry.test/skills",
		AuthConfig:  map[string]string{"token": "secret-token"},
	})
	if err != nil {
		t.Fatalf("Create registry: %v", err)
	}
	if registry.SyncInterval != 3600 || registry.SyncStatus != model.RegistrySyncPending {
		t.Fatalf("registry defaults not applied: %#v", registry)
	}
	if _, err := service.Create(ctx, &model.CreateRegistryRequest{Name: "api-registry", Type: model.RegistryTypeAPI, URL: "https://registry.test/skills"}); err == nil {
		t.Fatalf("duplicate registry name should fail")
	}

	now := time.Now()
	exact := &model.Skill{ID: uuid.New(), Name: "review", Description: "local review", Tags: []string{"old"}, SourceType: model.SkillSourceFederated, SourceRegistryID: registry.ID, SourcePath: "skills/review", Status: model.SkillStatusActive, CreatedAt: now, UpdatedAt: now}
	conflict := &model.Skill{ID: uuid.New(), Name: "conflict", Description: "personal conflict", SourceType: model.SkillSourcePersonal, SourcePath: "local/conflict", Status: model.SkillStatusActive, CreatedAt: now, UpdatedAt: now}
	userUpdate := &model.Skill{ID: uuid.New(), Name: "manual-target", Description: "manual", SourceType: model.SkillSourcePersonal, Status: model.SkillStatusActive, CreatedAt: now, UpdatedAt: now}
	for _, skill := range []*model.Skill{exact, conflict, userUpdate} {
		if err := skillRepo.Create(ctx, skill); err != nil {
			t.Fatalf("create skill %s: %v", skill.Name, err)
		}
	}

	preview, err := service.SyncPreview(ctx, registry.ID)
	if err != nil {
		t.Fatalf("SyncPreview: %v", err)
	}
	if len(preview.AutoUpdateSkills) != 1 || preview.AutoUpdateSkills[0].Name != "review" {
		t.Fatalf("auto preview = %#v", preview.AutoUpdateSkills)
	}
	if len(preview.ConflictSkills) != 1 || preview.ConflictSkills[0].LocalSkill.ID != conflict.ID {
		t.Fatalf("conflict preview = %#v", preview.ConflictSkills)
	}
	if len(preview.NewSkills) != 1 || preview.NewSkills[0].Name != "new-skill" {
		t.Fatalf("new preview = %#v", preview.NewSkills)
	}

	syncResult, err := service.Sync(ctx, registry.ID)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if syncResult.SkillsUpdated != 2 {
		t.Fatalf("sync result = %#v", syncResult)
	}
	updatedExact, err := skillRepo.FindByID(ctx, exact.ID)
	if err != nil || updatedExact.Description != "remote review" || len(updatedExact.Tags) != 2 {
		t.Fatalf("updated exact = %#v err=%v", updatedExact, err)
	}

	confirmResult, err := service.SyncConfirm(ctx, registry.ID, &model.SyncConfirmRequest{Operations: []*model.SyncOperation{
		{Action: "update", SkillName: "new-skill", TargetSkillID: userUpdate.ID, Description: "chosen remote"},
		{Action: "skip", SkillName: "conflict"},
		{Action: "update", SkillName: "missing", TargetSkillID: uuid.New(), Description: "missing"},
	}})
	if err != nil {
		t.Fatalf("SyncConfirm: %v", err)
	}
	if confirmResult.AutoUpdated != 1 || confirmResult.UserUpdated != 1 || confirmResult.UserSkipped != 1 || len(confirmResult.Skipped) != 2 {
		t.Fatalf("confirm result = %#v", confirmResult)
	}
	updatedTarget, err := skillRepo.FindByID(ctx, userUpdate.ID)
	if err != nil || updatedTarget.SourceType != model.SkillSourceFederated || updatedTarget.SourcePath != "skills/new" || updatedTarget.Description != "chosen remote" {
		t.Fatalf("updated target = %#v err=%v", updatedTarget, err)
	}

	list, total, err := service.List(ctx, &repo.RegistryListQuery{Type: string(model.RegistryTypeAPI), Size: 10})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("List = %#v total=%d err=%v", list, total, err)
	}
	changed, err := service.Update(ctx, registry.ID, &model.UpdateRegistryRequest{DisplayName: "Updated", Status: model.RegistryStatusInactive, SyncInterval: 10})
	if err != nil || changed.DisplayName != "Updated" || changed.Status != model.RegistryStatusInactive || changed.SyncInterval != 10 {
		t.Fatalf("Update = %#v err=%v", changed, err)
	}
	activeResults, err := service.SyncAll(ctx)
	if err != nil || len(activeResults) != 0 {
		t.Fatalf("SyncAll inactive = %#v err=%v", activeResults, err)
	}
	_, err = service.Update(ctx, registry.ID, &model.UpdateRegistryRequest{Status: model.RegistryStatusActive})
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	activeResults, err = service.SyncAll(ctx)
	if err != nil || len(activeResults) != 1 {
		t.Fatalf("SyncAll active = %#v err=%v", activeResults, err)
	}
	if requests == 0 {
		t.Fatalf("api server was not called")
	}
}

func TestRegistryServiceErrorPaths(t *testing.T) {
	ctx := context.Background()
	db := openRegistryServiceTestDB(t)
	registryRepo := repo.NewSkillRegistryRepository(db, repo.DBTypeSQLite)
	skillRepo := repo.NewSkillRepository(db, repo.DBTypeSQLite)
	service := NewRegistryService(registryRepo, skillRepo, nil)

	if _, err := service.GetByID(ctx, uuid.New()); err == nil {
		t.Fatalf("GetByID missing should fail")
	}
	if _, err := service.Update(ctx, uuid.New(), &model.UpdateRegistryRequest{DisplayName: "missing"}); err == nil {
		t.Fatalf("Update missing should fail")
	}
	if _, err := service.Sync(ctx, uuid.New()); err == nil {
		t.Fatalf("Sync missing should fail")
	}

	badType := &model.SkillRegistry{ID: uuid.New(), Name: "bad", Type: model.RegistryType("bad"), URL: "http://example.invalid", SyncStatus: model.RegistrySyncPending, Status: model.RegistryStatusActive, CreatedAt: time.Now()}
	if err := registryRepo.Create(ctx, badType); err != nil {
		t.Fatalf("create bad type: %v", err)
	}
	if result, err := service.Sync(ctx, badType.ID); err == nil || result == nil || result.Error == "" {
		t.Fatalf("Sync unsupported = %#v err=%v", result, err)
	}

	apiDown := &model.SkillRegistry{ID: uuid.New(), Name: "api-down", Type: model.RegistryTypeAPI, URL: "://bad-url", SyncStatus: model.RegistrySyncPending, Status: model.RegistryStatusActive, CreatedAt: time.Now()}
	if err := registryRepo.Create(ctx, apiDown); err != nil {
		t.Fatalf("create bad api: %v", err)
	}
	if _, err := service.SyncPreview(ctx, apiDown.ID); err == nil {
		t.Fatalf("SyncPreview bad api should fail")
	}
	if _, err := service.SyncConfirm(ctx, apiDown.ID, &model.SyncConfirmRequest{}); err == nil {
		t.Fatalf("SyncConfirm bad api should fail")
	}

	custom := &model.SkillRegistry{ID: uuid.New(), Name: "custom", Type: model.RegistryTypeCustom, URL: "ssh://example/skills", SyncStatus: model.RegistrySyncPending, Status: model.RegistryStatusActive, CreatedAt: time.Now()}
	if err := registryRepo.Create(ctx, custom); err != nil {
		t.Fatalf("create custom: %v", err)
	}
	if _, err := service.Sync(ctx, custom.ID); err == nil {
		t.Fatalf("Sync custom without scanner should fail")
	}
	if err := service.Delete(ctx, custom.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := service.GetByName(ctx, "custom"); err == nil {
		t.Fatalf("deleted registry should not be found")
	}

	withScanner := NewRegistryService(registryRepo, skillRepo, NewSkillScanner(nil, nil, nil, nil, filepath.Join(t.TempDir(), "storage"), "", zap.NewNop()))
	if _, _, err := withScanner.cloneAndScanGitRepo(ctx, custom); err == nil {
		t.Fatalf("cloneAndScanGitRepo should fail for unreachable repo")
	}
}

func openRegistryServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := []string{
		`CREATE TABLE skill_registries (id TEXT PRIMARY KEY, name TEXT UNIQUE, display_name TEXT, type TEXT, url TEXT, auth_config BLOB, sync_interval INTEGER, last_sync_at TIMESTAMP, sync_status TEXT, skill_count INTEGER, status TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT, description TEXT, tags BLOB, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
