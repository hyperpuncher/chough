package models

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractTarBz2StripsExpectedRoot(t *testing.T) {
	target := t.TempDir()
	if err := extractTarBz2("testdata/valid.tar.bz2", target, "test-model"); err != nil {
		t.Fatalf("extractTarBz2() error = %v", err)
	}
	files := append([]string{}, requiredModelFiles...)
	files = append(files, filepath.Join("nested", "info.txt"))
	for _, name := range files {
		info, err := os.Stat(filepath.Join(target, name))
		if err != nil {
			t.Fatalf("stat extracted %q: %v", name, err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("extracted %q is not a regular file", name)
		}
		if info.Mode().Perm()&0o022 != 0 {
			t.Fatalf("extracted %q is writable by group or others: %o", name, info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(target, "test-model")); !os.IsNotExist(err) {
		t.Fatalf("archive root was not stripped: err = %v", err)
	}
}

func TestExtractTarBz2RejectsUnsafeEntries(t *testing.T) {
	for _, test := range []struct {
		name    string
		archive string
		want    string
	}{
		{name: "absolute", archive: "absolute.tar.bz2", want: "absolute path"},
		{name: "traversal", archive: "traversal.tar.bz2", want: "escapes its root"},
		{name: "symlink", archive: "symlink.tar.bz2", want: "unsupported link type"},
		{name: "hard link", archive: "hardlink.tar.bz2", want: "unsupported link type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := t.TempDir()
			err := extractTarBz2(filepath.Join("testdata", test.archive), target, "test-model")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("extractTarBz2() error = %v, want containing %q", err, test.want)
			}
		})
	}
}
