package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestSkillRegistryRepositoryLifecycleAndQueries(t *testing.T) {
	ctx := context.Background()
	db := openSkillRegistryRepoTestDB(t)
	repository := NewSkillRegistryRepository(db, DBTypeSQLite)

	lastSync := time.Now().Add(-time.Hour).Truncate(time.Second)
	registry := &model.SkillRegistry{
		ID:           uuid.New(),
		Name:         "github-public",
		DisplayName:  "GitHub Public",
		Type:         model.RegistryTypeGitHub,
		URL:          "https://github.com/example/skills",
		AuthConfig:   map[string]string{"token": "secret"},
		SyncInterval: 60,
		LastSyncAt:   &lastSync,
		SyncStatus:   model.RegistrySyncPending,
		SkillCount:   3,
		Status:       model.RegistryStatusActive,
		CreatedAt:    time.Now().Add(-time.Minute),
	}
	if err := repository.Create(ctx, registry); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	inactive := &model.SkillRegistry{
		ID:           uuid.New(),
		Name:         "gitlab-private",
		DisplayName:  "GitLab Private",
		Type:         model.RegistryTypeGitLab,
		URL:          "https://gitlab.com/example/skills",
		AuthConfig:   map[string]string{},
		SyncInterval: 120,
		SyncStatus:   model.RegistrySyncFailed,
		Status:       model.RegistryStatusInactive,
		CreatedAt:    time.Now(),
	}
	if err := repository.Create(ctx, inactive); err != nil {
		t.Fatalf("Create inactive returned error: %v", err)
	}

	byID, err := repository.FindByID(ctx, registry.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if byID.Name != registry.Name || byID.AuthConfig["token"] != "secret" || byID.LastSyncAt == nil {
		t.Fatalf("FindByID = %#v", byID)
	}
	byName, err := repository.FindByName(ctx, "gitlab-private")
	if err != nil {
		t.Fatalf("FindByName returned error: %v", err)
	}
	if byName.Type != model.RegistryTypeGitLab || byName.Status != model.RegistryStatusInactive {
		t.Fatalf("FindByName = %#v", byName)
	}

	list, total, err := repository.List(ctx, &RegistryListQuery{Search: "Public", Page: 0, Size: 0})
	if err != nil {
		t.Fatalf("List search returned error: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != registry.ID {
		t.Fatalf("List search total=%d list=%#v", total, list)
	}
	filtered, total, err := repository.List(ctx, &RegistryListQuery{Type: string(model.RegistryTypeGitLab), Status: string(model.RegistryStatusInactive), Size: 200})
	if err != nil {
		t.Fatalf("List filtered returned error: %v", err)
	}
	if total != 1 || len(filtered) != 1 || filtered[0].ID != inactive.ID {
		t.Fatalf("List filtered total=%d list=%#v", total, filtered)
	}

	registry.DisplayName = "GitHub Updated"
	registry.URL = "https://github.com/updated/skills"
	registry.SyncInterval = 30
	registry.Status = model.RegistryStatusInactive
	registry.AuthConfig = map[string]string{"token": "updated"}
	if err := repository.Update(ctx, registry); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	updated, err := repository.FindByID(ctx, registry.ID)
	if err != nil {
		t.Fatalf("FindByID updated returned error: %v", err)
	}
	if updated.DisplayName != "GitHub Updated" || updated.AuthConfig["token"] != "updated" || updated.Status != model.RegistryStatusInactive {
		t.Fatalf("updated = %#v", updated)
	}

	if err := repository.UpdateSyncStatus(ctx, registry.ID, model.RegistrySyncSuccess, 9); err != nil {
		t.Fatalf("UpdateSyncStatus returned error: %v", err)
	}
	synced, err := repository.FindByID(ctx, registry.ID)
	if err != nil {
		t.Fatalf("FindByID synced returned error: %v", err)
	}
	if synced.SyncStatus != model.RegistrySyncSuccess || synced.SkillCount != 9 || synced.LastSyncAt == nil {
		t.Fatalf("synced = %#v", synced)
	}

	inactiveList, err := repository.FindByStatus(ctx, model.RegistryStatusInactive)
	if err != nil {
		t.Fatalf("FindByStatus returned error: %v", err)
	}
	if len(inactiveList) != 2 {
		t.Fatalf("FindByStatus = %#v", inactiveList)
	}
	all, err := repository.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("FindAll = %#v", all)
	}

	if err := repository.Delete(ctx, registry.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := repository.FindByID(ctx, registry.ID); err == nil {
		t.Fatalf("FindByID should fail after delete")
	}
	if _, err := repository.FindByName(ctx, "missing"); err == nil {
		t.Fatalf("FindByName should fail for missing registry")
	}
}

func openSkillRegistryRepoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE skill_registries (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE,
		display_name TEXT,
		type TEXT,
		url TEXT,
		auth_config BLOB,
		sync_interval INTEGER,
		last_sync_at TIMESTAMP,
		sync_status TEXT,
		skill_count INTEGER,
		status TEXT,
		created_at TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return db
}
