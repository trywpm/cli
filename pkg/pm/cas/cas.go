package cas

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"go.wpm.so/cli/pkg/atomicwriter"
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

func (s *Store) Get(ctx context.Context, digest, ref string, fetch func(context.Context) (io.ReadCloser, error)) (*os.File, error) {
	want, err := parseDigest(digest)
	if err != nil {
		return nil, err
	}

	path := s.blobPath(want)
	if f, ok := openBlob(path, want); ok {
		s.annotate(want, ref)
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
	s.annotate(want, ref)

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

// annotate records ref for a digest with append-if-absent semantics, so a
// blob shared by identical releases collects every name it serves under.
func (s *Store) annotate(sum [sha256.Size]byte, ref string) {
	if ref == "" {
		return
	}

	path := s.refsPath(sum)
	refs := readRefs(path)
	if slices.Contains(refs, ref) {
		return
	}
	writeRefs(path, append(refs, ref))
}

func (s *Store) refsPath(sum [sha256.Size]byte) string {
	return filepath.Join(s.root, "meta", hex.EncodeToString(sum[:]))
}

func readRefs(path string) []string {
	data, err := os.ReadFile(path) //nolint:gosec // the path is a hex digest under the store root
	if err != nil {
		return nil
	}

	var refs []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if line = strings.TrimSpace(line); validRef(line) {
			refs = append(refs, line)
		}
	}
	return refs
}

func validRef(ref string) bool {
	if len(ref) == 0 || len(ref) > 240 {
		return false
	}
	at := strings.IndexByte(ref, '@')
	if at <= 0 || at == len(ref)-1 {
		return false
	}
	for i := range len(ref) {
		if ref[i] <= ' ' || ref[i] > '~' {
			return false
		}
	}
	return true
}

func validMetaName(name string) bool {
	if len(name) != hex.EncodedLen(sha256.Size) {
		return false
	}
	for _, c := range name {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func writeRefs(path string, refs []string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	slices.Sort(refs)
	_ = atomicwriter.WriteFile(path, []byte(strings.Join(slices.Compact(refs), "\n")+"\n"), 0o600)
}

// Refs returns every ref recorded for digest, for display.
func Refs(dir, digest string) []string {
	if !validMetaName(digest) {
		return nil
	}
	return readRefs(filepath.Join(dir, "meta", digest))
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

type VerifyReport struct {
	Blobs   int
	Removed int
	Bytes   int64
}

// Verify re-hashes every blob against its own name, removes entries that no
// longer match, and sweeps temp files left by crashed runs.
func Verify(ctx context.Context, dir string) VerifyReport {
	var blobs, removed, kept atomic.Int64

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(max(runtime.NumCPU(), 1))

	blobRoot := filepath.Join(dir, "sha256")
	_ = filepath.WalkDir(blobRoot, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return fs.SkipAll
		}
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}

		g.Go(func() error {
			f, err := os.Open(path) //nolint:gosec // the path comes from walking our own store
			if err != nil {
				return nil
			}
			h := sha256.New()
			_, copyErr := copyBuf(h, f)
			_ = f.Close()

			if copyErr == nil && hex.EncodeToString(h.Sum(nil)) == filepath.Base(path) {
				blobs.Add(1)
				kept.Add(info.Size())
				return nil
			}
			if os.Remove(path) == nil {
				removed.Add(1)
			}
			return nil
		})
		return nil
	})
	_ = g.Wait()
	sweepTemp(filepath.Join(dir, "tmp"))
	sweepOrphanRefs(dir)

	return VerifyReport{
		Blobs:   int(blobs.Load()),
		Removed: int(removed.Load()),
		Bytes:   kept.Load(),
	}
}

func sweepOrphanRefs(dir string) {
	entries, err := os.ReadDir(filepath.Join(dir, "meta"))
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !validMetaName(name) {
			_ = os.Remove(filepath.Join(dir, "meta", name))
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "sha256", name[:2], name[2:4], name)); err != nil {
			_ = os.Remove(filepath.Join(dir, "meta", name))
		}
	}
}

func copyBuf(dst io.Writer, src io.Reader) (int64, error) {
	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	return io.CopyBuffer(dst, struct{ io.Reader }{src}, *buf)
}
