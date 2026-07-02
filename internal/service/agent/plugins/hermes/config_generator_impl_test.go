package hermes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestHermesGenerateConfigCopiesAssets(t *testing.T) {
	root := t.TempDir()
	skillID := uuid.New()
	skillStorage := filepath.Join(root, "skill-store")
	subagentStorage := filepath.Join(root, "subagent-store")
	commandStorage := filepath.Join(root, "command-store")
	ruleStorage := filepath.Join(root, "rule-store")
	settingsStorage := filepath.Join(root, "settings")
	configPath := filepath.Join(root, "config")

	mustWrite(t, filepath.Join(skillStorage, skillID.String(), "SKILL.md"), "skill content")
	mustWrite(t, filepath.Join(subagentStorage, "reviewer.md"), "subagent from file")
	mustWrite(t, filepath.Join(commandStorage, "ship.md"), "command content")
	mustWrite(t, filepath.Join(ruleStorage, "style.md"), "rule content")
	mustWrite(t, filepath.Join(settingsStorage, "nested", "config.json"), "{}")
	mustWrite(t, filepath.Join(configPath, "stale.txt"), "delete me")

	generator := NewHermesConfigGenerator(skillStorage, subagentStorage, commandStorage, ruleStorage, zap.NewNop())
	result, err := generator.GenerateConfig(context.Background(), &agent.ConfigGenerateRequest{
		AgentRoleID:   uuid.New(),
		ConfigPath:    configPath,
		CleanExisting: true,
		Skills:        []*model.Skill{{ID: skillID, Name: "autoplan"}},
		Commands:      []*model.Command{{Name: "ship"}},
		Subagents:     []*model.Subagent{{Name: "reviewer", Description: "reviews"}},
		Rules:         []*model.Rule{{Name: "style"}},
		Settings:      []*model.Settings{{Name: "base", DirectoryPath: settingsStorage}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SkillsCount != 1 || result.CommandsCount != 1 || result.SubagentsCount != 1 || result.RulesCount != 1 || result.SettingsCount != 1 {
		t.Fatalf("unexpected result counts: %#v", result)
	}
	assertFile(t, filepath.Join(configPath, "skills", "autoplan", "SKILL.md"), "skill content")
	assertFile(t, filepath.Join(configPath, "commands", "ship.md"), "command content")
	assertFile(t, filepath.Join(configPath, "agents", "reviewer.md"), "subagent from file")
	assertFile(t, filepath.Join(configPath, "rules", "style.md"), "rule content")
	assertFile(t, filepath.Join(configPath, "nested", "config.json"), "{}")
	if _, err := os.Stat(filepath.Join(configPath, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed, err=%v", err)
	}
}

func TestHermesGenerateConfigFallbackAndPreview(t *testing.T) {
	root := t.TempDir()
	generator := NewHermesConfigGenerator(
		filepath.Join(root, "missing-skills"),
		filepath.Join(root, "missing-subagents"),
		filepath.Join(root, "missing-commands"),
		filepath.Join(root, "missing-rules"),
		zap.NewNop(),
	)
	configPath := filepath.Join(root, "config")
	result, err := generator.GenerateConfig(context.Background(), &agent.ConfigGenerateRequest{
		AgentRoleID: uuid.New(),
		ConfigPath:  configPath,
		Skills:      []*model.Skill{{ID: uuid.New(), Name: "missing"}},
		Commands:    []*model.Command{{Name: "missing-command"}},
		Subagents:   []*model.Subagent{{Name: "fallback agent", Description: "fallback desc", Content: "fallback body"}},
		Rules:       []*model.Rule{{Name: "missing-rule"}},
		Settings:    []*model.Settings{{Name: "missing-settings", DirectoryPath: filepath.Join(root, "missing-settings")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SkillsCount != 0 || result.CommandsCount != 0 || result.SubagentsCount != 1 || result.RulesCount != 0 || result.SettingsCount != 0 {
		t.Fatalf("unexpected result counts: %#v", result)
	}
	assertFile(t, filepath.Join(configPath, "agents", "fallback-agent.md"), "---\nname: fallback agent\ndescription: fallback desc\n---\n\nfallback body")

	preview, err := generator.PreviewConfig(context.Background(), &agent.ConfigPreviewRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Files) != 0 {
		t.Fatalf("expected empty preview, got %#v", preview.Files)
	}
}

func TestHermesCopyHelpers(t *testing.T) {
	root := t.TempDir()
	generator := NewHermesConfigGenerator("", "", "", "", zap.NewNop()).(*HermesConfigGenerator)
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	mustWrite(t, filepath.Join(source, "nested", "file.txt"), "new")
	mustWrite(t, filepath.Join(target, "old.txt"), "old")

	if err := generator.copyDir(source, target); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(target, "nested", "file.txt"), "new")
	if _, err := os.Stat(filepath.Join(target, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected old target to be removed, err=%v", err)
	}

	copied := filepath.Join(root, "copied.txt")
	if err := generator.copyFile(filepath.Join(target, "nested", "file.txt"), copied); err != nil {
		t.Fatal(err)
	}
	assertFile(t, copied, "new")

	if err := generator.copySettingsDirectory(&model.Settings{Name: "empty"}, root); err == nil {
		t.Fatal("expected empty settings path to fail")
	}
	if err := generator.copyDirContents(filepath.Join(root, "missing"), target); err == nil {
		t.Fatal("expected missing source directory to fail")
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s=%q, want %q", path, content, want)
	}
}
