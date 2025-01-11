package archive

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Errors used or returned by this file.
var (
	ErrNotDirectory      = errors.New("not a directory")
	ErrDirNotExists      = errors.New("no such directory")
	ErrCannotCopyDir     = errors.New("cannot copy directory")
	ErrInvalidCopySource = errors.New("invalid copy source content")
)

func copyWithBuffer(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := copyPool.Get().(*[]byte)
	written, err = io.CopyBuffer(dst, src, *buf)
	copyPool.Put(buf)
	return
}

var copyPool = sync.Pool{
	New: func() interface{} { s := make([]byte, 32*1024); return &s },
}

// specifiesCurrentDir returns whether the given path specifies
// a "current directory", i.e., the last path segment is `.`.
func specifiesCurrentDir(path string) bool {
	return filepath.Base(path) == "."
}

// SplitPathDirEntry splits the given path between its directory name and its
// basename by first cleaning the path but preserves a trailing "." if the
// original path specified the current directory.
func SplitPathDirEntry(path string) (dir, base string) {
	cleanedPath := filepath.Clean(filepath.FromSlash(path))

	if specifiesCurrentDir(path) {
		cleanedPath += string(os.PathSeparator) + "."
	}

	return filepath.Dir(cleanedPath), filepath.Base(cleanedPath)
}
