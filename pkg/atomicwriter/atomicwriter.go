package atomicwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// WriteFile atomically writes data to path with the given permissions. It is a
// drop-in replacement for os.WriteFile that survives an interrupt or power loss
// mid-write.
func WriteFile(path string, data []byte, perm os.FileMode) (retErr error) {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if retErr != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	// Sync the directory to guarantee the POSIX rename is flushed to disk.
	// Windows handles NTFS journaling automatically and rejects directory syncs.
	if runtime.GOOS != "windows" {
		if d, err := os.Open(dir); err == nil { //nolint:gosec // we need to open the directory to fsync it
			_ = d.Sync()
			_ = d.Close()
		}
	}

	return nil
}
