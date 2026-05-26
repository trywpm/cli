package atomicwriter

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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

	tmp, tmpName, err := createTemp(dir, filepath.Base(path), perm)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
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

// createTemp opens a new, uniquely-named file in dir with O_CREATE|O_EXCL so it
// never follows a pre-existing symlink or clobbers an existing file. perm is
// passed straight to the open syscall so the kernel masks it by the process
// umask, exactly as os.WriteFile does.
func createTemp(dir, base string, perm os.FileMode) (*os.File, string, error) {
	for range 10000 {
		suffix, err := randomSuffix()
		if err != nil {
			return nil, "", err
		}

		name := filepath.Join(dir, "."+base+".tmp-"+suffix)
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm) //nolint:gosec // name is confined to dir; O_EXCL blocks clobbering and symlink-following
		if err == nil {
			return f, name, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return nil, "", err
	}
	return nil, "", errors.New("exhausted attempts to create a unique temp file")
}

func randomSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
