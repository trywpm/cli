//go:build unix

package atomicwriter

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestWriteFileRespectsUmask verifies that WriteFile respects the process
// umask when creating the temp file.
//
// syscall.Umask is process-global, so this test must not run in parallel.
func TestWriteFileRespectsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	dir := t.TempDir()
	path := filepath.Join(dir, "wpm.json")

	if err := WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Fatalf("perm = %o, want 0600 (0644 masked by umask 077)", got)
	}
}
