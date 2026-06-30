package teampackagesync

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/pkg/config"
	"go.uber.org/zap"
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
