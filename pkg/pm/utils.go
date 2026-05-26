package pm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// WriteFileAtomic writes data to path atomically: it writes to a temp file in
// the same directory, fsyncs it, then renames over the target.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) (retErr error) {
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

// DetectIndentation scans the first few lines to find the indentation style.
//
// Defaults to 2 spaces if it can't decide.
func DetectIndentation(data []byte) string {
	lines := strings.SplitSeq(string(data), "\n")

	for line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			continue
		}

		var buf bytes.Buffer
		for _, r := range line {
			if r == ' ' || r == '\t' {
				buf.WriteRune(r)
			} else {
				break
			}
		}

		if buf.Len() > 0 {
			return buf.String()
		}
	}

	return "  "
}
