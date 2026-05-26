package atomicwriter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wpm.json")

	if err := WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // path is built from t.TempDir() + a constant
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("content = %q, want %q", got, "hello")
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !fi.Mode().IsRegular() {
		t.Fatalf("mode = %v, want a regular file", fi.Mode())
	}
	// Exact permission bits are umask-dependent (see the unix-only
	// TestWriteFileRespectsUmask); here we only assert the owner can still
	// read+write, which survives any standard umask.
	if perm := fi.Mode().Perm(); perm&0o600 != 0o600 {
		t.Fatalf("perm = %o, want at least owner rw (0600)", perm)
	}
}

func TestWriteFileOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wpm.lock")

	if err := WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}
	if err := WriteFile(path, []byte("new content"), 0o644); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // path is built from t.TempDir() + a constant
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new content" {
		t.Fatalf("content = %q, want %q", got, "new content")
	}
}

// TestWriteFileLeavesNoTempFiles guards the success-path cleanup: the only
// entry left in the directory must be the target file, never a leftover
// ".target.tmp-*" staging file.
func TestWriteFileLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wpm.json")

	if err := WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "wpm.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("directory entries = %v, want [wpm.json]", names)
	}
}
