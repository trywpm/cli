package cas

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	digestPrefix = "sha256:"
	maxBlobBytes = 128 << 20
)

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 256<<10)
		return &b
	},
}

type Store struct {
	root  string
	tmp   string
	limit int64
}

func New(dir string) (*Store, error) {
	tmp := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(tmp, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	sweepTemp(tmp)

	return &Store{root: dir, tmp: tmp, limit: maxBlobBytes}, nil
}

func sweepTemp(tmp string) {
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.Remove(filepath.Join(tmp, e.Name()))
	}
}

func (s *Store) Get(ctx context.Context, digest string, fetch func(context.Context) (io.ReadCloser, error)) (*os.File, error) {
	want, err := parseDigest(digest)
	if err != nil {
		return nil, err
	}

	path := s.blobPath(want)
	if f, ok := openBlob(path, want); ok {
		return f, nil
	}

	body, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()

	f, err := os.CreateTemp(s.tmp, "blob-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	if err := s.writeBlob(f, body, digest, want); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}

	moveBlob(f.Name(), path)

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func (s *Store) writeBlob(f *os.File, body io.Reader, digest string, want [sha256.Size]byte) error {
	h := sha256.New()
	n, err := copyBuf(io.MultiWriter(f, h), io.LimitReader(body, s.limit+1))
	if err != nil {
		return err
	}
	if n > s.limit {
		return fmt.Errorf("tarball for %s exceeds the %d byte cap", digest, s.limit)
	}

	var got [sha256.Size]byte
	h.Sum(got[:0])
	if got != want {
		return fmt.Errorf("digest mismatch: expected %s, got %s%s",
			digest, digestPrefix, base64.StdEncoding.EncodeToString(got[:]))
	}
	return nil
}

func openBlob(path string, want [sha256.Size]byte) (*os.File, bool) {
	f, err := os.Open(path) //nolint:gosec // path is a hex digest under the store root
	if err != nil {
		return nil, false
	}

	h := sha256.New()
	if _, err := copyBuf(h, f); err == nil {
		var got [sha256.Size]byte
		h.Sum(got[:0])
		if got == want {
			if _, err := f.Seek(0, io.SeekStart); err == nil {
				return f, true
			}
		}
	}

	_ = f.Close()
	_ = os.Remove(path)

	return nil, false
}

func moveBlob(tmpName, final string) {
	if os.MkdirAll(filepath.Dir(final), 0o750) == nil && os.Rename(tmpName, final) == nil {
		return
	}
	_ = os.Remove(tmpName)
}

func (s *Store) blobPath(sum [sha256.Size]byte) string {
	name := hex.EncodeToString(sum[:])
	return filepath.Join(s.root, "sha256", name[:2], name[2:4], name)
}

func parseDigest(digest string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte

	encoded, ok := strings.CutPrefix(digest, digestPrefix)
	if !ok {
		return sum, fmt.Errorf("unsupported digest %.60q: want a %q prefix", digest, digestPrefix)
	}
	if len(encoded) != 44 {
		return sum, fmt.Errorf("invalid digest: %d base64 characters, want 44", len(encoded))
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return sum, fmt.Errorf("invalid digest %q: %w", digest, err)
	}
	if len(raw) != sha256.Size {
		return sum, fmt.Errorf("invalid digest %q: %d bytes, want %d", digest, len(raw), sha256.Size)
	}

	return [sha256.Size]byte(raw), nil
}

func copyBuf(dst io.Writer, src io.Reader) (int64, error) {
	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	return io.CopyBuffer(dst, struct{ io.Reader }{src}, *buf)
}
