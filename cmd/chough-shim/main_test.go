package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindTargetBinaryFromAdjacentDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "chough-bin.exe")
	writeTestFile(t, target)

	got, err := findTargetBinaryFrom(filepath.Join(dir, "chough.exe"))
	if err != nil {
		t.Fatalf("findTargetBinaryFrom() error = %v", err)
	}
	if got != target {
		t.Fatalf("target = %q, want %q", got, target)
	}
}

func TestFindTargetBinaryFromResolvedSymlinkDirectory(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	linkDir := filepath.Join(root, "link")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realExecutable := filepath.Join(realDir, "chough.exe")
	writeTestFile(t, realExecutable)
	linkedExecutable := filepath.Join(linkDir, "chough.exe")
	if err := os.Symlink(realExecutable, linkedExecutable); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	target := filepath.Join(realDir, "chough-bin.exe")
	writeTestFile(t, target)

	got, err := findTargetBinaryFrom(linkedExecutable)
	if err != nil {
		t.Fatalf("findTargetBinaryFrom() error = %v", err)
	}
	if got != target {
		t.Fatalf("target = %q, want %q", got, target)
	}
}

func TestFindTargetBinaryFromReportsMissingTarget(t *testing.T) {
	_, err := findTargetBinaryFrom(filepath.Join(t.TempDir(), "chough.exe"))
	if err == nil || !strings.Contains(err.Error(), "chough-bin.exe not found") {
		t.Fatalf("findTargetBinaryFrom() error = %v", err)
	}
}

func TestUniqueDirs(t *testing.T) {
	dirs := uniqueDirs([]string{"a", "a", "", filepath.Join("a", "..", "b")})
	if len(dirs) != 2 || dirs[0] != "a" || dirs[1] != "b" {
		t.Fatalf("uniqueDirs() = %q", dirs)
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
}
