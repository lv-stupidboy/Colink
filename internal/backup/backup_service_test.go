package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupServiceBackupListRestoreAndRetention(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backups")
	dataDir := filepath.Join(root, "data")
	assetsDir := filepath.Join(dataDir, "agent-assets")
	dbPath := filepath.Join(root, "isdp.db")

	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("current-db"), 0644); err != nil {
		t.Fatalf("write db: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "asset.txt"), []byte("asset-v1"), 0644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	service := NewBackupService(BackupConfig{
		Path:      backupDir,
		DBPath:    dbPath,
		DataPath:  dataDir,
	})

	result, err := service.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup returned error: %v", err)
	}
	if !result.Success || result.DBFile == "" || result.AssetsFile == "" || result.Timestamp == "" {
		t.Fatalf("unexpected backup result: %+v", result)
	}

	if err := os.WriteFile(dbPath, []byte("changed-db"), 0644); err != nil {
		t.Fatalf("mutate db: %v", err)
	}
	waitForNextBackupSecond(t)
	if err := service.Restore(context.Background(), result.Timestamp); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	body, err := os.ReadFile(dbPath)
	if err != nil || string(body) != "current-db" {
		t.Fatalf("restored db body=%q err=%v", body, err)
	}

	backups, err := service.ListBackups(context.Background())
	if err != nil {
		t.Fatalf("ListBackups returned error: %v", err)
	}
	if len(backups) == 0 {
		t.Fatalf("expected at least one backup entry")
	}

	oldDB := filepath.Join(backupDir, "isdp-20000101-000000.db")
	oldAssets := filepath.Join(backupDir, "assets-20000101-000000.zip")
	if err := os.WriteFile(oldDB, []byte("old"), 0644); err != nil {
		t.Fatalf("write old db: %v", err)
	}
	if err := os.WriteFile(oldAssets, []byte("old"), 0644); err != nil {
		t.Fatalf("write old assets: %v", err)
	}
	oldTime := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(oldDB, oldTime, oldTime); err != nil {
		t.Fatalf("chtime old db: %v", err)
	}
	if err := os.Chtimes(oldAssets, oldTime, oldTime); err != nil {
		t.Fatalf("chtime old assets: %v", err)
	}

	service.config.Retention = 1
	service.cleanupOldBackups()
	if _, err := os.Stat(oldDB); !os.IsNotExist(err) {
		t.Fatalf("expected old db backup to be removed, err=%v", err)
	}
	if _, err := os.Stat(oldAssets); !os.IsNotExist(err) {
		t.Fatalf("expected old asset backup to be removed, err=%v", err)
	}
}

func waitForNextBackupSecond(t *testing.T) {
	t.Helper()

	before := time.Now().Format("20060102-150405")
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if time.Now().Format("20060102-150405") != before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for next backup timestamp second")
}

func TestBackupServiceValidationAndEmptyList(t *testing.T) {
	service := NewBackupService(BackupConfig{})
	if _, err := service.Backup(context.Background()); err == nil || !strings.Contains(err.Error(), "数据库路径未配置") {
		t.Fatalf("expected missing database path error, got %v", err)
	}
	if err := service.Restore(context.Background(), "20240101-000000"); err == nil || !strings.Contains(err.Error(), "数据库路径未配置") {
		t.Fatalf("expected missing database path error, got %v", err)
	}

	withMissingDir := NewBackupService(BackupConfig{Path: filepath.Join(t.TempDir(), "missing")})
	backups, err := withMissingDir.ListBackups(context.Background())
	if err != nil {
		t.Fatalf("ListBackups missing dir returned error: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("expected empty backup list, got %d", len(backups))
	}
}
