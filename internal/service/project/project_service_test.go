package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectFileSystemHelpers(t *testing.T) {
	service := NewService(nil, nil, nil)
	root := t.TempDir()
	writeProjectFile(t, filepath.Join(root, "b.txt"), "b")
	writeProjectFile(t, filepath.Join(root, "a.txt"), "a")
	writeProjectFile(t, filepath.Join(root, ".hidden"), "hidden")
	if err := os.Mkdir(filepath.Join(root, "dir"), 0755); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "$system"), 0755); err != nil {
		t.Fatalf("mkdir system: %v", err)
	}

	if !pathWithin(root, filepath.Join(root, "dir", "x")) || !pathWithin(root, root) {
		t.Fatalf("pathWithin should accept descendants")
	}
	if pathWithin(root, filepath.Dir(root)) {
		t.Fatalf("pathWithin should reject parent")
	}

	files, err := service.ListFilesByPath(context.Background(), root, "")
	if err != nil {
		t.Fatalf("ListFilesByPath returned error: %v", err)
	}
	if len(files.Files) != 4 || files.Files[0].Name != "$system" || files.Files[1].Name != "dir" || files.Files[2].Name != "a.txt" || files.Files[3].Name != "b.txt" {
		t.Fatalf("files = %#v", files.Files)
	}
	if _, err := service.ListFilesByPath(context.Background(), "", ""); err == nil || !strings.Contains(err.Error(), "基础路径不能为空") {
		t.Fatalf("empty base path error = %v", err)
	}
	if _, err := service.ListFilesByPath(context.Background(), root, "missing"); err == nil || !strings.Contains(err.Error(), "路径不存在") {
		t.Fatalf("missing subpath error = %v", err)
	}

	browse, err := service.BrowsePath(context.Background(), root)
	if err != nil {
		t.Fatalf("BrowsePath returned error: %v", err)
	}
	if !browse.IsValid || len(browse.Entries) != 1 || browse.Entries[0].Name != "dir" {
		t.Fatalf("browse = %#v", browse)
	}
	browseFile, err := service.BrowsePath(context.Background(), filepath.Join(root, "a.txt"))
	if err != nil || browseFile.Error != "路径不是目录" {
		t.Fatalf("browse file = %#v err=%v", browseFile, err)
	}

	valid, err := service.ValidatePath(context.Background(), root)
	if err != nil || !valid.IsValid || !valid.Exists || !valid.IsDir || !valid.CanCreate {
		t.Fatalf("ValidatePath existing = %#v err=%v", valid, err)
	}
	newPath := filepath.Join(root, "new-project")
	valid, err = service.ValidatePath(context.Background(), newPath)
	if err != nil || !valid.IsValid || !valid.CanCreate || valid.Exists {
		t.Fatalf("ValidatePath creatable = %#v err=%v", valid, err)
	}
	invalid, err := service.ValidatePath(context.Background(), "")
	if err != nil || invalid.Error != "路径不能为空" {
		t.Fatalf("ValidatePath empty = %#v err=%v", invalid, err)
	}

	if err := service.CreateFolder(context.Background(), root, "created"); err != nil {
		t.Fatalf("CreateFolder returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "created")); err != nil {
		t.Fatalf("created folder missing: %v", err)
	}
	if err := service.CreateFolder(context.Background(), root, "created"); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("duplicate folder error = %v", err)
	}
	if err := service.CreateFolder(context.Background(), filepath.Join(root, "a.txt"), "child"); err == nil || !strings.Contains(err.Error(), "父路径不是目录") {
		t.Fatalf("file parent error = %v", err)
	}

	content, err := service.GetFileContent(context.Background(), root, "a.txt")
	if err != nil || content.Content != "a" || content.IsBinary || content.Truncated {
		t.Fatalf("GetFileContent text = %#v err=%v", content, err)
	}
	writeProjectFile(t, filepath.Join(root, "image.png"), "binary-ish")
	binary, err := service.GetFileContent(context.Background(), root, "image.png")
	if err != nil || !binary.IsBinary || binary.Content != "" {
		t.Fatalf("GetFileContent binary = %#v err=%v", binary, err)
	}
	writeProjectFile(t, filepath.Join(root, "large.txt"), strings.Repeat("x", maxFileSize+8))
	large, err := service.GetFileContent(context.Background(), root, "large.txt")
	if err != nil || !large.Truncated || len(large.Content) != maxFileSize {
		t.Fatalf("GetFileContent large = len %d truncated=%v err=%v", len(large.Content), large.Truncated, err)
	}
	if _, err := service.GetFileContent(context.Background(), root, "dir"); err == nil || !strings.Contains(err.Error(), "路径是目录") {
		t.Fatalf("directory content error = %v", err)
	}
	if _, err := service.GetFileContent(context.Background(), "", "a.txt"); err == nil || !strings.Contains(err.Error(), "基础路径不能为空") {
		t.Fatalf("empty base content error = %v", err)
	}

	items := files.Files
	sortFiles(items)
	if !items[0].IsDir || items[0].Name > items[1].Name {
		t.Fatalf("sortFiles result = %#v", items)
	}
	if err := checkWritable(root); err != nil {
		t.Fatalf("checkWritable returned error: %v", err)
	}
}

func writeProjectFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
