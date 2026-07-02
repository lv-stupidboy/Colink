package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestExportServiceExportsSelectedTablesAndManifest(t *testing.T) {
	db := openBackupTestDB(t)
	defer db.Close()
	seedBackupTestDB(t, db)

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	assetsDir := filepath.Join(dataDir, "agent-assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "note.txt"), []byte("asset"), 0644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	outputPath := filepath.Join(root, "export.zip")
	result, err := NewExportService(db, "sqlite").Export(context.Background(), &ExportRequest{
		Type:          ExportFull,
		Tables:        []string{"projects"},
		IncludeAssets: true,
		DataPath:      dataDir,
	}, outputPath)
	if err != nil {
		t.Fatalf("Export returned error: %v", err)
	}
	if !result.Success || result.Filename != "export.zip" || result.TableCount != 1 || result.RowCount != 2 || result.Size == 0 {
		t.Fatalf("unexpected export result: %+v", result)
	}

	reader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("open export zip: %v", err)
	}
	defer reader.Close()

	var manifest DatabaseManifest
	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
		if file.Name == "manifest.json" {
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("open manifest: %v", err)
			}
			if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
				rc.Close()
				t.Fatalf("decode manifest: %v", err)
			}
			rc.Close()
		}
	}
	if manifest.SchemaVersion != 7 || manifest.DatabaseType != "sqlite" || manifest.Tables["projects"].RowCount != 2 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !entries["data/projects.json"] || !entries["assets.zip"] {
		t.Fatalf("expected exported data and assets entries, got %#v", entries)
	}
}

func TestImportServicePreviewAndConfirm(t *testing.T) {
	source := openBackupTestDB(t)
	defer source.Close()
	seedBackupTestDB(t, source)

	root := t.TempDir()
	exportPath := filepath.Join(root, "export.zip")
	_, err := NewExportService(source, "sqlite").Export(context.Background(), &ExportRequest{
		Tables: []string{"projects"},
	}, exportPath)
	if err != nil {
		t.Fatalf("Export returned error: %v", err)
	}
	zipData, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}

	target := openBackupTestDB(t)
	defer target.Close()
	if _, err := target.Exec(`PRAGMA user_version = 9; CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create target schema: %v", err)
	}
	if _, err := target.Exec(`INSERT INTO projects (id, name) VALUES ('p1', 'Existing')`); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	service := NewImportService(target, "sqlite")
	preview, err := service.ImportPreview(zipData)
	if err != nil {
		t.Fatalf("ImportPreview returned error: %v", err)
	}
	if preview.SourceVersion != 7 || preview.CurrentVersion != 9 || !preview.Compatible || preview.NeedsMigration {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if preview.SourceTables["projects"].RowCount != 2 || !contains(preview.ExistingTables, "projects") {
		t.Fatalf("unexpected preview tables: %+v", preview)
	}

	result, err := service.ImportConfirm(context.Background(), zipData, &ImportConfirmRequest{
		ConflictResolution: "merge",
		SelectedTables:     []string{"projects"},
	}, root)
	if err != nil {
		t.Fatalf("ImportConfirm returned error: %v", err)
	}
	if !result.Success || result.RowCount != 2 || len(result.TablesImported) != 1 || result.TablesImported[0] != "projects" {
		t.Fatalf("unexpected import result: %+v", result)
	}

	rows, err := target.Query(`SELECT id, name FROM projects ORDER BY id`)
	if err != nil {
		t.Fatalf("query target: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		got[id] = name
	}
	if got["p1"] != "Existing" || got["p2"] != "Project Two" {
		t.Fatalf("unexpected imported rows: %#v", got)
	}
}

func TestImportServiceErrorsAndOverwrite(t *testing.T) {
	db := openBackupTestDB(t)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	service := NewImportService(db, "sqlite")

	if _, err := service.ImportPreview([]byte("not a zip")); err == nil {
		t.Fatalf("ImportPreview should fail for invalid zip")
	}

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	payload := map[string]any{
		"table":   "projects",
		"columns": []string{"id", "name"},
		"rows": []map[string]any{
			{"id": "p1", "name": "New Name"},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "projects.json"), data, 0644); err != nil {
		t.Fatalf("write data file: %v", err)
	}
	zipData, err := createZipFromDir(root)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO projects (id, name) VALUES ('p1', 'Old Name')`); err != nil {
		t.Fatalf("seed row: %v", err)
	}
	result, err := service.ImportConfirm(context.Background(), zipData, &ImportConfirmRequest{
		ConflictResolution: "overwrite",
		SelectedTables:     []string{"projects"},
	}, root)
	if err != nil {
		t.Fatalf("ImportConfirm overwrite returned error: %v", err)
	}
	if !result.Success || result.RowCount != 1 {
		t.Fatalf("unexpected overwrite result: %+v", result)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM projects WHERE id = 'p1'`).Scan(&name); err != nil {
		t.Fatalf("query overwritten row: %v", err)
	}
	if name != "New Name" {
		t.Fatalf("expected overwritten name, got %q", name)
	}
}

func openBackupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func seedBackupTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec(`PRAGMA user_version = 7; CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create source schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, name) VALUES ('p1', 'Project One'), ('p2', 'Project Two')`); err != nil {
		t.Fatalf("seed source rows: %v", err)
	}
}
