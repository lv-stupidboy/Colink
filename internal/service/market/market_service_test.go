package market

import (
	"context"
	"database/sql"
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

func TestMarketServiceCRUDAndValidation(t *testing.T) {
	ctx := context.Background()
	db := openMarketTestDB(t)
	service := NewService(repo.NewMarketRepository(db, repo.DBTypeSQLite), repo.NewTeamPackageVersionRepository(db, repo.DBTypeSQLite), t.TempDir(), zap.NewNop(), &config.GitURLConversionConfig{})

	created, err := service.AddMarket(ctx, AddMarketRequest{
		Name: "  Colink\ufffdٷӳ  ",
		URL:  "git@github.com:colink-ai/market.git",
	})
	if err != nil {
		t.Fatalf("AddMarket returned error: %v", err)
	}
	if created.ID == uuid.Nil || created.Name != "Colink官方市场" || created.Branch != "main" || !created.Enabled || created.AutoUpdate {
		t.Fatalf("created market = %#v", created)
	}

	list, err := service.ListMarkets(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "Colink官方市场" {
		t.Fatalf("ListMarkets = %#v err=%v", list, err)
	}
	got, err := service.GetMarketByID(ctx, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("GetMarketByID = %#v err=%v", got, err)
	}

	name := "Internal Market"
	url := "git@github.com:colink-ai/internal.git"
	branch := "develop"
	enabled := false
	autoUpdate := true
	cron := "*/5 * * * *"
	updated, err := service.UpdateMarket(ctx, created.ID, UpdateMarketRequest{
		Name:          &name,
		URL:           &url,
		Branch:        &branch,
		Enabled:       &enabled,
		AutoUpdate:    &autoUpdate,
		CheckInterval: &cron,
	})
	if err != nil {
		t.Fatalf("UpdateMarket returned error: %v", err)
	}
	if updated.Name != name || updated.URL != url || updated.Branch != branch || updated.Enabled || !updated.AutoUpdate || updated.CheckInterval != cron {
		t.Fatalf("updated market = %#v", updated)
	}

	badCron := "not cron"
	if _, err := service.UpdateMarket(ctx, created.ID, UpdateMarketRequest{CheckInterval: &badCron}); err == nil || !strings.Contains(err.Error(), "invalid cron") {
		t.Fatalf("bad cron error = %v", err)
	}
	if _, err := service.UpdateMarket(ctx, uuid.New(), UpdateMarketRequest{Name: &name}); err == nil {
		t.Fatalf("missing market update should fail")
	}
	if err := service.DeleteMarket(ctx, created.ID); err != nil {
		t.Fatalf("DeleteMarket returned error: %v", err)
	}
	if got, err := service.GetMarketByID(ctx, created.ID); err != nil || got != nil {
		t.Fatalf("deleted market = %#v err=%v", got, err)
	}
}

func TestMarketCacheAndHelpers(t *testing.T) {
	cache := NewMarketCache(20 * time.Millisecond)
	if got := cache.GetTeamPackages(); got != nil {
		t.Fatalf("empty cache = %#v", got)
	}
	packages := []model.MarketPackage{{Name: "team", Version: "1.0.0"}}
	cache.SetTeamPackages(packages)
	if got := cache.GetTeamPackages(); len(got) != 1 || got[0].Name != "team" {
		t.Fatalf("fresh cache = %#v", got)
	}
	time.Sleep(30 * time.Millisecond)
	if got := cache.GetTeamPackages(); got != nil {
		t.Fatalf("expired cache should be nil, got %#v", got)
	}
	if got := cache.GetExpiredTeamPackages(); len(got) != 1 || got[0].Name != "team" {
		t.Fatalf("expired fallback cache = %#v", got)
	}
	cache.InvalidateTeamPackages()
	if got := cache.GetExpiredTeamPackages(); got != nil {
		t.Fatalf("invalidated cache = %#v", got)
	}

	validCrons := []string{"", "* * * * *", "*/5 0-2 1 1 0", "1,2 3 4 5 6"}
	for _, cron := range validCrons {
		if !ValidateCron(cron) {
			t.Fatalf("expected valid cron %q", cron)
		}
	}
	invalidCrons := []string{"* * * *", "60 * * * *", "* 24 * * *", "* * 32 * *", "* * * 13 *", "* * * * 7"}
	for _, cron := range invalidCrons {
		if ValidateCron(cron) {
			t.Fatalf("expected invalid cron %q", cron)
		}
	}

	if compareVersions("1.2.0", "1.2.1") >= 0 {
		t.Fatalf("compareVersions should detect older patch")
	}
	if compareVersions("2.0.0", "1.9.9") <= 0 {
		t.Fatalf("compareVersions should detect newer major")
	}
	if compareVersions("1.0.0-beta", "1.0.0") != 0 {
		t.Fatalf("compareVersions should ignore suffix text")
	}
	if parts := parseVersionParts("3.4.x"); parts != [3]int{3, 4, 0} {
		t.Fatalf("parseVersionParts = %#v", parts)
	}
	if cleanDisplayName(" \u200bMarket\u0000 ") != "Market" {
		t.Fatalf("cleanDisplayName did not strip hidden/control chars")
	}
}

func TestMarketServiceRefreshPackagesUsesCacheInvalidation(t *testing.T) {
	ctx := context.Background()
	db := openMarketTestDB(t)
	service := NewService(repo.NewMarketRepository(db, repo.DBTypeSQLite), repo.NewTeamPackageVersionRepository(db, repo.DBTypeSQLite), t.TempDir(), zap.NewNop(), &config.GitURLConversionConfig{})
	service.cache.SetTeamPackages([]model.MarketPackage{{Name: "stale"}})
	if err := service.RefreshPackages(ctx); err != nil {
		t.Fatalf("RefreshPackages with no markets returned error: %v", err)
	}
	if got := service.cache.GetExpiredTeamPackages(); got != nil {
		t.Fatalf("RefreshPackages should invalidate empty cache, got %#v", got)
	}
	if packages, err := service.GetTeamPackages(ctx, false); err != nil || len(packages) != 0 {
		t.Fatalf("GetTeamPackages no markets = %#v err=%v", packages, err)
	}
}

func openMarketTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE markets (id TEXT PRIMARY KEY, name TEXT, url TEXT, branch TEXT, enabled BOOLEAN, auto_update BOOLEAN, check_interval TEXT, last_synced_at TEXT, last_error TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE team_package_versions (id TEXT PRIMARY KEY, workflow_id TEXT, name TEXT, category TEXT, version TEXT, description TEXT, last_synced_at TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}
