package configgen

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestDownloaderDownloadSkillCleanAndSubagentFallback(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	skillStore := filepath.Join(root, "skills-store")
	subagentStore := filepath.Join(root, "subagents-store")
	target := filepath.Join(root, "target")
	skillID := uuid.New()
	skillDir := filepath.Join(skillStore, skillID.String(), "nested")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("skill body"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(target, "skills", "old"), 0755); err != nil {
		t.Fatalf("mkdir old skills: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(target, "rules", "old"), 0755); err != nil {
		t.Fatalf("mkdir old rules: %v", err)
	}

	downloader := NewDownloader(skillStore, subagentStore, zap.NewNop())
	if err := downloader.CleanConfigDir(target); err != nil {
		t.Fatalf("CleanConfigDir returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills")); !os.IsNotExist(err) {
		t.Fatalf("skills dir should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "rules")); !os.IsNotExist(err) {
		t.Fatalf("rules dir should be removed, err=%v", err)
	}

	skill := &model.Skill{ID: skillID, Name: "Code Review"}
	path, err := downloader.DownloadSkill(ctx, skill, "open_code", target)
	if err != nil {
		t.Fatalf("DownloadSkill returned error: %v", err)
	}
	if path != filepath.Join(target, "skills", "Code Review") {
		t.Fatalf("download path = %q", path)
	}
	body, err := os.ReadFile(filepath.Join(path, "nested", "README.md"))
	if err != nil || string(body) != "skill body" {
		t.Fatalf("downloaded body = %q err=%v", body, err)
	}

	results := downloader.DownloadSkills(ctx, []*model.Skill{
		skill,
		{ID: uuid.New(), Name: "Missing"},
	}, "open_code", target)
	if len(results) != 2 || results[0].Error != nil || results[1].Error == nil {
		t.Fatalf("DownloadSkills results = %#v", results)
	}

	subagentTarget := filepath.Join(target, "agents")
	if err := os.MkdirAll(subagentTarget, 0755); err != nil {
		t.Fatalf("mkdir subagent target: %v", err)
	}
	if err := downloader.CopySubagentToDir(&model.Subagent{
		Name:        "Ops Helper",
		Description: "handles ops",
		Content:     "run checks",
	}, subagentTarget); err != nil {
		t.Fatalf("CopySubagentToDir returned error: %v", err)
	}
	subagentBody, err := os.ReadFile(filepath.Join(subagentTarget, "Ops-Helper.md"))
	if err != nil {
		t.Fatalf("read generated subagent: %v", err)
	}
	if !strings.Contains(string(subagentBody), "description: handles ops") || !strings.Contains(string(subagentBody), "run checks") {
		t.Fatalf("subagent fallback body = %q", subagentBody)
	}
}

func TestDownloaderCopySubagentPrefersStoredFile(t *testing.T) {
	root := t.TempDir()
	subagentStore := filepath.Join(root, "subagents")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(subagentStore, 0755); err != nil {
		t.Fatalf("mkdir subagent store: %v", err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subagentStore, "Stored-Agent.md"), []byte("stored content"), 0644); err != nil {
		t.Fatalf("write stored subagent: %v", err)
	}

	downloader := NewDownloader("", subagentStore, zap.NewNop())
	if err := downloader.CopySubagentToDir(&model.Subagent{Name: "Stored Agent", Content: "db content"}, target); err != nil {
		t.Fatalf("CopySubagentToDir returned error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(target, "Stored-Agent.md"))
	if err != nil || string(body) != "stored content" {
		t.Fatalf("stored body = %q err=%v", body, err)
	}
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}
