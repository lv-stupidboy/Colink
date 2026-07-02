package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipDirAndUnzipToRoundTrip(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "data.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write nested: %v", err)
	}

	archive := filepath.Join(root, "backup.zip")
	if err := zipDir(source, archive); err != nil {
		t.Fatalf("zipDir returned error: %v", err)
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	var sawReadme, sawNested bool
	for _, file := range reader.File {
		if file.Name == "source/README.md" {
			sawReadme = true
		}
		if file.Name == "source/nested/data.txt" {
			sawNested = true
		}
	}
	reader.Close()
	if !sawReadme || !sawNested {
		t.Fatalf("archive entries missing: readme=%v nested=%v", sawReadme, sawNested)
	}

	dest := filepath.Join(root, "restore")
	if err := unzipTo(archive, dest); err != nil {
		t.Fatalf("unzipTo returned error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dest, "source", "nested", "data.txt"))
	if err != nil || string(body) != "data" {
		t.Fatalf("restored body=%q err=%v", body, err)
	}
}

func TestZipDirAndUnzipToReturnErrors(t *testing.T) {
	root := t.TempDir()
	if err := zipDir(filepath.Join(root, "missing"), filepath.Join(root, "out.zip")); err == nil {
		t.Fatalf("zipDir should fail for missing source")
	}
	if err := unzipTo(filepath.Join(root, "missing.zip"), filepath.Join(root, "dest")); err == nil {
		t.Fatalf("unzipTo should fail for missing archive")
	}
}
