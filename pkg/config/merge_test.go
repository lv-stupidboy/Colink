package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeConfigCopiesTemplateWhenUserMissing(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.yaml")
	userPath := filepath.Join(dir, "nested", "config.yaml")
	mustWriteConfig(t, templatePath, "version: v1\nname: default\n")

	changed, err := MergeConfig(userPath, templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected missing user config to be created")
	}
	assertConfigContains(t, userPath, "name: default")
}

func TestMergeConfigMergesTemplateAndPreservesUserValues(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.yaml")
	userPath := filepath.Join(dir, "config.yaml")
	mustWriteConfig(t, templatePath, "# header\n\nversion: v2\nserver:\n  port: 8080\n  host: localhost\nfeature: true\n")
	mustWriteConfig(t, userPath, "version: v1\nserver:\n  port: 9090\ncustom: keep\n")

	changed, err := MergeConfig(userPath, templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected version mismatch to merge")
	}
	content := readConfig(t, userPath)
	for _, want := range []string{"# header", "version: v2", "port: 9090", "host: localhost", "custom: keep", "feature: true"} {
		if !strings.Contains(content, want) {
			t.Fatalf("merged config missing %q:\n%s", want, content)
		}
	}
}

func TestMergeConfigNoopsWhenVersionMatches(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.yaml")
	userPath := filepath.Join(dir, "config.yaml")
	mustWriteConfig(t, templatePath, "version: v1\nfeature: true\n")
	mustWriteConfig(t, userPath, "version: v1\ncustom: keep\n")

	changed, err := MergeConfig(userPath, templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected matching versions to skip merge")
	}
	assertConfigContains(t, userPath, "custom: keep")
}

func TestMergeConfigErrorsAndHelpers(t *testing.T) {
	dir := t.TempDir()
	if _, err := MergeConfig(filepath.Join(dir, "user.yaml"), filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("expected missing template to fail")
	}
	templatePath := filepath.Join(dir, "template.yaml")
	userPath := filepath.Join(dir, "user.yaml")
	mustWriteConfig(t, templatePath, "version: [")
	mustWriteConfig(t, userPath, "version: v1\n")
	if _, err := MergeConfig(userPath, templatePath); err == nil {
		t.Fatal("expected invalid template yaml to fail")
	}
	mustWriteConfig(t, templatePath, "version: v2\n")
	mustWriteConfig(t, userPath, "version: [")
	if _, err := MergeConfig(userPath, templatePath); err == nil {
		t.Fatal("expected invalid user yaml to fail")
	}

	if isMap(nil) || !isMap(map[string]interface{}{}) {
		t.Fatal("unexpected isMap result")
	}
	if got := getStringFromMap(map[string]interface{}{"version": 1}, "version"); got != "" {
		t.Fatalf("expected non-string map value to return empty, got %q", got)
	}
	if got := splitLines("a\nb\n"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("splitLines result=%v", got)
	}
	if got := extractHeaderComment([]byte("name: app\n# later")); got != "# later\n" {
		t.Fatalf("expected first encountered comment to be extracted, got %q", got)
	}
}

func mustWriteConfig(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertConfigContains(t *testing.T, path string, want string) {
	t.Helper()
	if content := readConfig(t, path); !strings.Contains(content, want) {
		t.Fatalf("%s missing %q:\n%s", path, want, content)
	}
}
