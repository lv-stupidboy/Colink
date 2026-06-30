package teampackage

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
)

func TestActionHelpers(t *testing.T) {
	assetActions := []model.TeamPackageAssetAction{
		{AssetType: "skill", Name: "review", Action: "skip"},
		{AssetType: "command", Name: "build", Action: "rename"},
	}
	if got := getAssetAction(assetActions, "skill", "review"); got != "skip" {
		t.Fatalf("getAssetAction = %q", got)
	}
	if got := getAssetAction(assetActions, "skill", "missing"); got != "overwrite" {
		t.Fatalf("default asset action = %q", got)
	}

	roleActions := []model.TeamPackageRoleAction{{Name: "coder", Action: "skip"}}
	if got := getRoleAction(roleActions, "coder"); got != "skip" {
		t.Fatalf("getRoleAction = %q", got)
	}
	if got := getRoleAction(roleActions, "architect"); got != "overwrite" {
		t.Fatalf("default role action = %q", got)
	}
}

func TestCopyFileAndDir(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	writeTeamFile(t, filepath.Join(src, "a.txt"), "hello")
	writeTeamFile(t, filepath.Join(src, "nested", "b.txt"), "world")
	if err := os.Chmod(filepath.Join(src, "a.txt"), 0600); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir returned error: %v", err)
	}
	if got := readTeamFile(t, filepath.Join(dst, "a.txt")); got != "hello" {
		t.Fatalf("copied a.txt = %q", got)
	}
	if got := readTeamFile(t, filepath.Join(dst, "nested", "b.txt")); got != "world" {
		t.Fatalf("copied b.txt = %q", got)
	}
	if info, err := os.Stat(filepath.Join(dst, "a.txt")); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("copied mode info=%v err=%v", info, err)
	}

	if err := copyFile(src, filepath.Join(root, "bad")); err == nil {
		t.Fatalf("copyFile should reject directory source")
	}
	if err := copyDir(filepath.Join(src, "a.txt"), filepath.Join(root, "bad-dir")); err == nil {
		t.Fatalf("copyDir should reject file source")
	}
	if err := copyDir(filepath.Join(root, "missing"), filepath.Join(root, "missing-dst")); err == nil {
		t.Fatalf("copyDir missing source should fail")
	}
}

func TestCreateAndExtractZip(t *testing.T) {
	root := t.TempDir()
	writeTeamFile(t, filepath.Join(root, "manifest.json"), `{"name":"team"}`)
	writeTeamFile(t, filepath.Join(root, "assets", "skill.md"), "skill")

	data, err := createZip(root)
	if err != nil {
		t.Fatalf("createZip returned error: %v", err)
	}
	names := teamZipNames(t, data)
	for _, want := range []string{"./", "manifest.json", "assets/", "assets/skill.md"} {
		if !containsTeamName(names, want) {
			t.Fatalf("zip names %v missing %s", names, want)
		}
	}

	dst := filepath.Join(t.TempDir(), "extract")
	if err := extractZip(bytes.NewReader(data), dst); err != nil {
		t.Fatalf("extractZip returned error: %v", err)
	}
	if got := readTeamFile(t, filepath.Join(dst, "assets", "skill.md")); got != "skill" {
		t.Fatalf("extracted file = %q", got)
	}

	if err := extractZip(strings.NewReader("not zip"), t.TempDir()); err == nil {
		t.Fatalf("invalid zip should fail")
	}
	if err := extractZip(bytes.NewReader(makeZip(t, map[string]string{"../escape.txt": "bad"})), t.TempDir()); err == nil || !strings.Contains(err.Error(), "路径遍历") {
		t.Fatalf("zip slip error = %v", err)
	}
}

func TestExtractZipLimits(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < 1001; i++ {
		w, err := zw.Create("f" + strings.Repeat("x", i%3) + "/" + strings.Repeat("y", i%5) + ".txt")
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := extractZip(bytes.NewReader(buf.Bytes()), t.TempDir()); err == nil || !strings.Contains(err.Error(), "数量超过限制") {
		t.Fatalf("file count limit error = %v", err)
	}
}

func writeTeamFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readTeamFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func teamZipNames(t *testing.T, data []byte) []string {
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

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func containsTeamName(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
