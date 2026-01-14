package archive

import (
	"errors"
	"io"
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
	New: func() any { s := make([]byte, 32*1024); return &s },
}
