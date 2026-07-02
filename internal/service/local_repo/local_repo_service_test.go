package local_repo

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestLocalRepoServiceUploadListDeleteAndValidation(t *testing.T) {
	ctx := context.Background()
	db := openLocalRepoTestDB(t)
	repository := repo.NewLocalRepoRepository(db, repo.DBTypeSQLite)
	service := NewService(repository, nil, &config.GitURLConversionConfig{})
	targetPath := t.TempDir()

	uploaded, err := service.Upload(ctx, localRepoZip(t, map[string]string{
		"README.md":      "hello",
		"dir/file.txt":   "nested",
		"../escaped.txt": "escape",
	}), "demo.zip", &model.UploadRepoRequest{TargetPath: targetPath})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if uploaded.Name != "demo" || uploaded.Status != model.RepoStatusPending {
		t.Fatalf("uploaded repo = %#v", uploaded)
	}
	if body, err := os.ReadFile(filepath.Join(uploaded.LocalPath, "dir", "file.txt")); err != nil || string(body) != "nested" {
		t.Fatalf("uploaded nested file = %q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(targetPath, "escaped.txt")); !os.IsNotExist(err) {
		t.Fatalf("zip traversal file should not be written, err=%v", err)
	}

	repos, err := service.List(ctx)
	if err != nil || len(repos) != 1 {
		t.Fatalf("List = %#v err=%v", repos, err)
	}
	got, err := service.GetByID(ctx, uploaded.ID)
	if err != nil || got.Name != "demo" {
		t.Fatalf("GetByID = %#v err=%v", got, err)
	}

	if _, err := service.Upload(ctx, []byte("not zip"), "bad.zip", &model.UploadRepoRequest{Name: "bad", TargetPath: targetPath}); err == nil || !strings.Contains(err.Error(), "解压ZIP失败") {
		t.Fatalf("bad zip error = %v", err)
	}
	if _, err := service.Upload(ctx, localRepoZip(t, map[string]string{"a.txt": "a"}), "bad.zip", &model.UploadRepoRequest{Name: "../bad", TargetPath: targetPath}); err == nil || !strings.Contains(err.Error(), "路径分隔符") {
		t.Fatalf("bad repo name error = %v", err)
	}
	if _, err := service.Upload(ctx, localRepoZip(t, map[string]string{"a.txt": "a"}), "bad.zip", &model.UploadRepoRequest{Name: "bad"}); err == nil || !strings.Contains(err.Error(), "目标路径") {
		t.Fatalf("missing target error = %v", err)
	}

	if err := service.Delete(ctx, uploaded.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(uploaded.LocalPath); !os.IsNotExist(err) {
		t.Fatalf("local path should be removed, err=%v", err)
	}
	if _, err := service.GetByID(ctx, uploaded.ID); err == nil {
		t.Fatalf("deleted local repo should not be found")
	}
}

func TestLocalRepoServiceConfigureGitAndHelpers(t *testing.T) {
	ctx := context.Background()
	db := openLocalRepoTestDB(t)
	repository := repo.NewLocalRepoRepository(db, repo.DBTypeSQLite)
	service := NewService(repository, nil, &config.GitURLConversionConfig{
		Enabled: true,
		Rules: []config.GitURLConversionRule{{
			Pattern: "https://github.com/",
			SSHHost: "git@github.com",
		}},
	})

	localPath := t.TempDir()
	localRepo := &model.LocalRepo{
		ID:        uuid.New(),
		Name:      "repo",
		LocalPath: localPath,
		Status:    model.RepoStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repository.Create(ctx, localRepo); err != nil {
		t.Fatalf("create local repo: %v", err)
	}

	configured, err := service.ConfigureGit(ctx, localRepo.ID, &model.GitConfigRequest{
		GitUrl: "https://github.com/owner/repo.git",
		Branch: "main",
	})
	if err != nil {
		t.Fatalf("ConfigureGit returned error: %v", err)
	}
	if configured.GitUrl != "git@github.com:owner/repo.git" || configured.Branch == nil || *configured.Branch != "main" || configured.Status != model.RepoStatusReady {
		t.Fatalf("configured repo = %#v", configured)
	}
	if _, err := service.ConfigureGit(ctx, localRepo.ID, &model.GitConfigRequest{GitUrl: "https://example.com/owner/repo.git"}); err == nil || !strings.Contains(err.Error(), "SSH") {
		t.Fatalf("invalid git url error = %v", err)
	}
	if err := service.CreateFolder(ctx, localPath, "src"); err != nil {
		t.Fatalf("CreateFolder returned error: %v", err)
	}
	if err := service.CreateFolder(ctx, localPath, "src"); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("duplicate folder error = %v", err)
	}

	if !isSSHGitURL("git@github.com:owner/repo.git") || !isSSHGitURL("ssh://git@github.com/owner/repo.git") {
		t.Fatalf("valid SSH URLs rejected")
	}
	if isSSHGitURL("https://github.com/owner/repo.git") || isSSHGitURL("git@github.com") || isSSHGitURL("bad url") {
		t.Fatalf("invalid SSH URLs accepted")
	}
	if name := inferRepoNameFromGitUrl("git@github.com:owner/repo.git"); name != "repo" {
		t.Fatalf("inferRepoNameFromGitUrl = %q", name)
	}
	if name := inferRepoNameFromGitUrl("ssh://git@github.com/owner/repo.git?depth=1"); name != "repo.git" {
		t.Fatalf("inferRepoNameFromGitUrl ssh form = %q", name)
	}
	if name := inferRepoNameFromGitUrl("git@github.com:owner/repo.GIT"); name != "repo" {
		t.Fatalf("inferRepoNameFromGitUrl uppercase suffix = %q", name)
	}
	if !pathWithin(localPath, filepath.Join(localPath, "child")) || pathWithin(localPath, filepath.Dir(localPath)) {
		t.Fatalf("pathWithin returned unexpected result")
	}
}

func TestLocalRepoServiceBrowseCloneSyncAndSortingGuards(t *testing.T) {
	ctx := context.Background()
	db := openLocalRepoTestDB(t)
	repository := repo.NewLocalRepoRepository(db, repo.DBTypeSQLite)
	service := NewService(repository, nil, &config.GitURLConversionConfig{})

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "b-dir"), 0755); err != nil {
		t.Fatalf("mkdir b-dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "a-dir"), 0755); err != nil {
		t.Fatalf("mkdir a-dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".hidden"), 0755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("file"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	browse, err := service.BrowsePath(ctx, root)
	if err != nil {
		t.Fatalf("BrowsePath returned error: %v", err)
	}
	if !browse.IsValid || browse.CurrentPath != root || browse.ParentPath == "" {
		t.Fatalf("unexpected browse response: %+v", browse)
	}
	if len(browse.Entries) != 2 || browse.Entries[0].Name != "a-dir" || browse.Entries[1].Name != "b-dir" {
		t.Fatalf("unexpected browse entries: %+v", browse.Entries)
	}

	fileResp, err := service.BrowsePath(ctx, filepath.Join(root, "file.txt"))
	if err != nil {
		t.Fatalf("BrowsePath file returned error: %v", err)
	}
	if fileResp.IsValid || fileResp.Error != "路径不是目录" {
		t.Fatalf("expected non-directory browse error, got %+v", fileResp)
	}
	missingResp, err := service.BrowsePath(ctx, filepath.Join(root, "missing"))
	if err != nil {
		t.Fatalf("BrowsePath missing returned error: %v", err)
	}
	if missingResp.IsValid || missingResp.Error != "路径不存在" {
		t.Fatalf("expected missing path browse error, got %+v", missingResp)
	}

	if err := service.CreateFolder(ctx, filepath.Join(root, "missing-parent"), "child"); err == nil || !strings.Contains(err.Error(), "父目录不存在") {
		t.Fatalf("missing parent error = %v", err)
	}
	if err := service.CreateFolder(ctx, filepath.Join(root, "file.txt"), "child"); err == nil || !strings.Contains(err.Error(), "父路径不是目录") {
		t.Fatalf("file parent error = %v", err)
	}

	if _, err := service.Clone(ctx, &model.CloneRepoRequest{GitUrl: "https://example.com/owner/repo.git", TargetPath: root}); err == nil || !strings.Contains(err.Error(), "SSH") {
		t.Fatalf("clone invalid URL error = %v", err)
	}
	if _, err := service.Clone(ctx, &model.CloneRepoRequest{GitUrl: "git@github.com:owner/repo.git"}); err == nil || !strings.Contains(err.Error(), "目标路径") {
		t.Fatalf("clone missing target error = %v", err)
	}
	if _, err := service.GetRemoteBranches("https://example.com/owner/repo.git"); err == nil || !strings.Contains(err.Error(), "SSH") {
		t.Fatalf("remote branches invalid URL error = %v", err)
	}

	localRepo := &model.LocalRepo{
		ID:        uuid.New(),
		Name:      "nogit",
		LocalPath: root,
		Status:    model.RepoStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repository.Create(ctx, localRepo); err != nil {
		t.Fatalf("create local repo: %v", err)
	}
	if _, err := service.Sync(ctx, localRepo.ID); err == nil || !strings.Contains(err.Error(), "GIT地址未配置") {
		t.Fatalf("sync without git error = %v", err)
	}

	files := []model.FileInfo{
		{Name: "z-file", IsDir: false},
		{Name: "b-dir", IsDir: true},
		{Name: "a-dir", IsDir: true},
	}
	sortFiles(files)
	if files[0].Name != "a-dir" || files[1].Name != "b-dir" || files[2].Name != "z-file" {
		t.Fatalf("sortFiles result = %+v", files)
	}
}

func openLocalRepoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE local_repos (id TEXT PRIMARY KEY, name TEXT, git_url TEXT, local_path TEXT, branch TEXT, last_commit TEXT, status TEXT, error_message TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`)
	if err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return db
}

func localRepoZip(t *testing.T, files map[string]string) []byte {
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
