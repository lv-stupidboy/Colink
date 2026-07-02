package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardDisabled(t *testing.T) {
	guard, err := NewGuard(" ")
	if err != nil {
		t.Fatal(err)
	}
	if guard.Enabled() {
		t.Fatal("empty root should disable guard")
	}
	if guard.Root() != "" {
		t.Fatalf("disabled root=%q", guard.Root())
	}
	if got := guard.NormalizeStart("relative"); got != "relative" {
		t.Fatalf("NormalizeStart=%q", got)
	}
	if err := guard.Validate(""); err != nil {
		t.Fatalf("disabled guard should not validate paths: %v", err)
	}
	var nilGuard *Guard
	if nilGuard.Enabled() || nilGuard.Root() != "" {
		t.Fatal("nil guard should behave as disabled")
	}
}

func TestGuardValidateAndNormalize(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	guard, err := NewGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	if !guard.Enabled() {
		t.Fatal("guard should be enabled")
	}
	if guard.NormalizeStart("") != guard.Root() {
		t.Fatal("empty start should normalize to root")
	}
	if err := guard.Validate(child); err != nil {
		t.Fatalf("child path should be valid: %v", err)
	}
	if err := guard.Validate(" "); err == nil || !strings.Contains(err.Error(), "不能为空") {
		t.Fatalf("expected empty path validation error, got %v", err)
	}
	if err := guard.Validate(filepath.Dir(root)); err == nil || !strings.Contains(err.Error(), "工作空间内") {
		t.Fatalf("expected outside workspace error, got %v", err)
	}
}

func TestGuardValidateChild(t *testing.T) {
	root := t.TempDir()
	guard, err := NewGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	child, err := guard.ValidateChild(root, "new-folder")
	if err != nil {
		t.Fatal(err)
	}
	if child != filepath.Join(root, "new-folder") {
		t.Fatalf("child=%q", child)
	}
	for _, name := range []string{"", ".", "..", "a/b", `a\b`} {
		if _, err := guard.ValidateChild(root, name); err == nil {
			t.Fatalf("expected invalid child name %q to fail", name)
		}
	}
	if _, err := guard.ValidateChild(filepath.Dir(root), "child"); err == nil {
		t.Fatal("expected invalid parent to fail")
	}
}

func TestIsWithin(t *testing.T) {
	root := filepath.Clean("/tmp/workspace")
	if !isWithin(root, root) {
		t.Fatal("root should be within itself")
	}
	if !isWithin(root, filepath.Join(root, "child")) {
		t.Fatal("child should be within root")
	}
	if isWithin(root, filepath.Clean("/tmp/workspace-other")) {
		t.Fatal("sibling prefix path should not be within root")
	}
}
