package workspace

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"
)

type ProjectLock struct {
	fileLock *flock.Flock
}

// AcquireLock blocks until the caller can take an exclusive lock on the
// project, or the ctx is cancelled. Any command that reads then mutates
// wpm.json or wpm.lock MUST hold this lock for the entire
// read-modify-write window.
func AcquireLock(ctx context.Context, cwd string, printWaitMsg func()) (*ProjectLock, error) {
	wpmDir := filepath.Join(cwd, ".wpm")
	if err := os.MkdirAll(wpmDir, 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create workspace directory")
	}

	_ = os.WriteFile(filepath.Join(wpmDir, ".gitignore"), []byte("*\n"), 0o644)

	fileLock := flock.New(filepath.Join(wpmDir, "install.lock"))

	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire workspace lock")
	}
	if locked {
		return &ProjectLock{fileLock: fileLock}, nil
	}

	if printWaitMsg != nil {
		printWaitMsg()
	}

	locked, err = fileLock.TryLockContext(ctx, 200*time.Millisecond)
	if err != nil {
		return nil, errors.Wrap(err, "failed while waiting for workspace lock")
	}
	if !locked {
		return nil, errors.Wrap(ctx.Err(), "operation cancelled while waiting for workspace lock")
	}

	return &ProjectLock{fileLock: fileLock}, nil
}

// Release unlocks and closes the underlying file descriptor. Must be
// called exactly once. Safe to call on a nil receiver.
//
// We intentionally do NOT remove the lock file from disk: deleting an
// in-use lock file races against concurrent processes that opened the
// same path before our delete, producing two processes locking different
// inodes for the same path.
func (l *ProjectLock) Release() error {
	if l == nil || l.fileLock == nil {
		return nil
	}
	return l.fileLock.Close()
}
