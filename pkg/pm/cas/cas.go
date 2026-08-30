package cas

import (
	"context"
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
	"time"

	"github.com/opencontainers/go-digest"
	"golang.org/x/sync/errgroup"

	"go.wpm.so/cli/pkg/atomicwriter"
	"go.wpm.so/cli/pkg/unsafeconv"
)

const (
	tmpDirName   = "tmp"
	metaDirName  = "meta"
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
	tmp := filepath.Join(dir, tmpDirName)
	if err := os.MkdirAll(tmp, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	sweepTemp(tmp)

	return &Store{
		root:  dir,
		tmp:   tmp,
		limit: maxBlobBytes,
	}, nil
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

func (s *Store) Get(ctx context.Context, dgst, ref string, fetch func(context.Context) (io.ReadCloser, error)) (*os.File, error) {
	d, err := digest.Parse(dgst)
	if err != nil {
		return nil, fmt.Errorf("invalid digest %.80q: %w", dgst, err)
	}

	path := blobPath(s.root, d)
	if f, ok := openBlob(path, d); ok {
		s.annotate(d, ref)
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

	if err := s.writeBlob(f, body, d); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}

	moveBlob(f.Name(), path)
	s.annotate(d, ref)

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func (s *Store) writeBlob(f *os.File, body io.Reader, want digest.Digest) error {
	digester := want.Algorithm().Digester()
	n, err := copyBuf(io.MultiWriter(f, digester.Hash()), io.LimitReader(body, s.limit+1))
	if err != nil {
		return err
	}
	if n > s.limit {
		return fmt.Errorf("tarball for %s exceeds the %d byte cap", want, s.limit)
	}

	if got := digester.Digest(); got != want {
		return fmt.Errorf("digest mismatch: expected %s, got %s", want, got)
	}
	return nil
}

func openBlob(path string, want digest.Digest) (*os.File, bool) {
	f, err := os.Open(path) //nolint:gosec // the path is a validated digest under the store root
	if err != nil {
		return nil, false
	}

	verifier := want.Verifier()
	if _, err := copyBuf(verifier, f); err == nil && verifier.Verified() {
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			return f, true
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
func (s *Store) annotate(d digest.Digest, ref string) {
	if !validRef(ref) {
		return
	}

	path := refsPath(s.root, d)
	if refFileContains(path, ref) {
		return
	}
	writeRefs(path, append(readRefs(path), ref))
}

func refFileContains(path, ref string) bool {
	f, err := os.Open(path) //nolint:gosec // the path is a validated digest under the store root
	if err != nil {
		return false
	}

	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	n, _ := io.ReadFull(f, *buf)
	_ = f.Close()

	for line := range strings.SplitSeq(unsafeconv.UnsafeBytesToString((*buf)[:n]), "\n") {
		if strings.TrimSpace(line) == ref {
			return true
		}
	}
	return false
}

func refsPath(root string, d digest.Digest) string {
	return filepath.Join(root, metaDirName, d.Algorithm().String(), d.Encoded())
}

func readRefs(path string) []string {
	data, err := os.ReadFile(path) //nolint:gosec // the path is a validated digest under the store root
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

func writeRefs(path string, refs []string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	slices.Sort(refs)
	_ = atomicwriter.WriteFile(path, []byte(strings.Join(slices.Compact(refs), "\n")+"\n"), 0o600)
}

// Refs returns every ref recorded for a digest, for display.
func Refs(dir string, d digest.Digest) []string {
	if d.Validate() != nil {
		return nil
	}
	return readRefs(refsPath(dir, d))
}

func blobPath(root string, d digest.Digest) string {
	enc := d.Encoded()
	return filepath.Join(root, d.Algorithm().String(), enc[:2], enc[2:4], enc)
}

type BlobInfo struct {
	Digest  digest.Digest
	Path    string
	Size    int64
	ModTime time.Time
}

func Blobs(dir string) []BlobInfo {
	var blobs []BlobInfo
	for _, algo := range algoDirs(dir) {
		_ = filepath.WalkDir(filepath.Join(dir, algo.String()), func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return nil
			}
			d := digest.NewDigestFromEncoded(algo, e.Name())
			if d.Validate() != nil {
				return nil
			}
			if info, err := e.Info(); err == nil {
				blobs = append(blobs, BlobInfo{Digest: d, Path: path, Size: info.Size(), ModTime: info.ModTime()})
			}
			return nil
		})
	}
	return blobs
}

func algoDirs(dir string) []digest.Algorithm {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var algos []digest.Algorithm
	for _, e := range entries {
		if algo := digest.Algorithm(e.Name()); e.IsDir() && algo.Available() {
			algos = append(algos, algo)
		}
	}
	return algos
}

type VerifyReport struct {
	Blobs   int
	Removed int
	Bytes   int64
}

// Verify re-hashes every blob against its own digest, removes entries that no
// longer match, and sweeps leftovers from crashed runs.
func Verify(ctx context.Context, dir string) VerifyReport {
	var blobs, removed, kept atomic.Int64

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(max(runtime.NumCPU(), 1))

	for _, blob := range Blobs(dir) {
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error {
			f, err := os.Open(blob.Path)
			if err != nil {
				return nil
			}
			verifier := blob.Digest.Verifier()
			_, copyErr := copyBuf(verifier, f)
			_ = f.Close()

			if copyErr == nil && verifier.Verified() {
				blobs.Add(1)
				kept.Add(blob.Size)
				return nil
			}
			if os.Remove(blob.Path) == nil {
				removed.Add(1)
			}
			return nil
		})
	}
	_ = g.Wait()
	sweepTemp(filepath.Join(dir, tmpDirName))
	sweepOrphanRefs(dir)

	return VerifyReport{
		Blobs:   int(blobs.Load()),
		Removed: int(removed.Load()),
		Bytes:   kept.Load(),
	}
}

func sweepOrphanRefs(dir string) {
	metaRoot := filepath.Join(dir, metaDirName)
	entries, err := os.ReadDir(metaRoot)
	if err != nil {
		return
	}

	for _, e := range entries {
		algo := digest.Algorithm(e.Name())
		if !e.IsDir() || !algo.Available() {
			_ = os.RemoveAll(filepath.Join(metaRoot, e.Name()))
			continue
		}

		files, err := os.ReadDir(filepath.Join(metaRoot, e.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			metaPath := filepath.Join(metaRoot, e.Name(), f.Name())
			d := digest.NewDigestFromEncoded(algo, f.Name())
			if d.Validate() != nil {
				_ = os.Remove(metaPath)
				continue
			}
			if _, err := os.Stat(blobPath(dir, d)); err != nil {
				_ = os.Remove(metaPath)
			}
		}
	}
}

func copyBuf(dst io.Writer, src io.Reader) (int64, error) {
	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	return io.CopyBuffer(dst, struct{ io.Reader }{src}, *buf)
}
