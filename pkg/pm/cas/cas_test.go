package cas

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
)

type fakeFetch struct {
	body  []byte
	calls int
}

func (ff *fakeFetch) fn(context.Context) (io.ReadCloser, error) {
	ff.calls++
	return io.NopCloser(bytes.NewReader(ff.body)), nil
}

func digestOf(b []byte) string {
	return digest.FromBytes(b).String()
}

func dOf(b []byte) digest.Digest {
	return digest.FromBytes(b)
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func read(t *testing.T, f *os.File) []byte {
	t.Helper()
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return b
}

func TestHexDigestLayout(t *testing.T) {
	s := newStore(t)
	body := []byte("a real tarball body")
	d := dOf(body)

	if d.Algorithm() != digest.SHA256 || len(d.Encoded()) != 64 {
		t.Fatalf("digest is not hex sha256: %s", d)
	}

	f, err := s.Get(t.Context(), d.String(), "pkg@1.0.0", func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = f.Close()

	enc := d.Encoded()
	want := filepath.Join(s.root, "sha256", enc[:2], enc[2:4], enc)
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("blob absent from algorithm-derived path %s: %v", want, err)
	}
}

func TestGetFetchesOnceThenHitsCache(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("a tarball, more or less")}
	dgst := digestOf(ff.body)

	for i := range 2 {
		f, err := s.Get(t.Context(), dgst, "", ff.fn)
		if err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
		if got := read(t, f); !bytes.Equal(got, ff.body) {
			t.Fatalf("Get %d content = %q, want %q", i, got, ff.body)
		}
	}
	if ff.calls != 1 {
		t.Fatalf("fetch ran %d times, want 1", ff.calls)
	}
}

func TestGetRejectsDigestMismatch(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("what the server actually sent")}
	claimed := digestOf([]byte("what the manifest promised"))

	_, err := s.Get(t.Context(), claimed, "", ff.fn)
	if err == nil {
		t.Fatal("Get accepted a body that did not match the digest")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("error = %v, want a digest mismatch", err)
	}
	assertNoTempFiles(t, s)

	if _, err := os.Stat(blobPath(s.root, digest.Digest(claimed))); err == nil {
		t.Fatal("mismatched bytes were stored under the claimed digest")
	}
}

func TestGetRefetchesCorruptEntry(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("good content that gets edited on disk later")}
	dgst := digestOf(ff.body)

	f, err := s.Get(t.Context(), dgst, "", ff.fn)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = f.Close()

	path := blobPath(s.root, digest.Digest(dgst))
	stored, err := os.ReadFile(path) //nolint:gosec // path comes from the store under t.TempDir()
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	stored[len(stored)/2] ^= 0xFF
	if err := os.WriteFile(path, stored, 0o600); err != nil { //nolint:gosec // same path
		t.Fatalf("WriteFile: %v", err)
	}

	f, err = s.Get(t.Context(), dgst, "", ff.fn)
	if err != nil {
		t.Fatalf("Get after corruption: %v", err)
	}
	if got := read(t, f); !bytes.Equal(got, ff.body) {
		t.Fatalf("content = %q, want %q", got, ff.body)
	}
	if ff.calls != 2 {
		t.Fatalf("fetch ran %d times, want 2", ff.calls)
	}

	repaired, err := os.ReadFile(path) //nolint:gosec // same path
	if err != nil {
		t.Fatalf("ReadFile after repair: %v", err)
	}
	if !bytes.Equal(repaired, ff.body) {
		t.Fatal("blob on disk still corrupt after refetch")
	}
}

func TestGetRejectsOversizedBody(t *testing.T) {
	s := newStore(t)
	s.limit = 64
	ff := &fakeFetch{body: bytes.Repeat([]byte("x"), 128)}

	_, err := s.Get(t.Context(), digestOf(ff.body), "", ff.fn)
	if err == nil {
		t.Fatal("Get accepted a body over the size cap")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want a size cap error", err)
	}
	assertNoTempFiles(t, s)
}

func TestGetPropagatesFetchError(t *testing.T) {
	s := newStore(t)
	boom := errors.New("network down")

	_, err := s.Get(t.Context(), digestOf([]byte("x")), "", func(context.Context) (io.ReadCloser, error) {
		return nil, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Get = %v, want the fetch error", err)
	}
	assertNoTempFiles(t, s)
}

func TestGetWorksWhenCacheCannotRetain(t *testing.T) {
	s := newStore(t)
	if err := os.WriteFile(filepath.Join(s.root, "sha256"), []byte("in the way"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ff := &fakeFetch{body: []byte("bytes that cannot be cached")}
	for i := range 2 {
		f, err := s.Get(t.Context(), digestOf(ff.body), "", ff.fn)
		if err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
		if got := read(t, f); !bytes.Equal(got, ff.body) {
			t.Fatalf("Get %d content = %q, want %q", i, got, ff.body)
		}
	}
	if ff.calls != 2 {
		t.Fatalf("fetch ran %d times, want 2 with retention broken", ff.calls)
	}
	assertNoTempFiles(t, s)
}

func TestGetRejectsInvalidDigests(t *testing.T) {
	s := newStore(t)
	fetch := func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	cases := []struct {
		name string
		dgst string
	}{
		{"empty", ""},
		{"no_algorithm", strings.Repeat("a", 64)},
		{"unknown_algorithm", "md5:" + strings.Repeat("a", 32)},
		{"old_base64_form", "sha256:NHUrqGHcaDSCCUPpr31j15TVsIZTtA53W7oCZXrnMNw="},
		{"short_hex", "sha256:" + strings.Repeat("a", 32)},
		{"uppercase_hex", "sha256:" + strings.Repeat("A", 64)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Get(t.Context(), tc.dgst, "", fetch); err == nil {
				t.Fatalf("Get accepted invalid digest %q", tc.dgst)
			}
		})
	}
}

func TestBlobPathStaysUnderRoot(t *testing.T) {
	s := newStore(t)

	enc := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	path := blobPath(s.root, digest.NewDigestFromEncoded(digest.SHA256, enc))

	rel, err := filepath.Rel(s.root, path)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if !filepath.IsLocal(rel) {
		t.Fatalf("blob path %q escapes the store root", path)
	}
	if filepath.Base(path) != enc {
		t.Fatalf("blob name = %q, want the encoded digest", filepath.Base(path))
	}
	if !strings.Contains(path, string(filepath.Separator)+"sha256"+string(filepath.Separator)) {
		t.Fatalf("blob path %q lacks the algorithm segment", path)
	}
}

func TestNewClearsCrashLeftovers(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(tmp, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	leftover := filepath.Join(tmp, "blob-leftover")
	if err := os.WriteFile(leftover, []byte("partial"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := os.Stat(leftover); err == nil {
		t.Fatal("crash leftover survived New")
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("temp directory itself was removed: %v", err)
	}
	assertNoTempFiles(t, s)
}

func TestVerifyRemovesCorruptAndSweepsTemps(t *testing.T) {
	s := newStore(t)
	good := &fakeFetch{body: []byte("stays intact")}
	bad := &fakeFetch{body: []byte("gets corrupted on disk")}

	f, err := s.Get(t.Context(), digestOf(good.body), "", good.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	f, err = s.Get(t.Context(), digestOf(bad.body), "", bad.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(blobPath(s.root, dOf(bad.body)), []byte("flipped"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.tmp, "blob-stray"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := Verify(t.Context(), s.root)
	if report.Blobs != 1 || report.Removed != 1 {
		t.Fatalf("report = %+v, want 1 kept 1 removed", report)
	}
	assertNoTempFiles(t, s)

	f, err = s.Get(t.Context(), digestOf(good.body), "", good.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if good.calls != 1 {
		t.Fatalf("good blob refetched after verify, calls = %d", good.calls)
	}
}

func TestGetAnnotatesRefs(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("shared bytes")}
	dgst := digestOf(ff.body)

	f, err := s.Get(t.Context(), dgst, "akismet@5.7.2", ff.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	f, err = s.Get(t.Context(), dgst, "akismet-fork@5.7.2", ff.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	refs := readRefs(refsPath(s.root, digest.Digest(dgst)))
	if len(refs) != 2 || refs[0] != "akismet-fork@5.7.2" || refs[1] != "akismet@5.7.2" {
		t.Fatalf("refs = %v, want both names sorted", refs)
	}
	if ff.calls != 1 {
		t.Fatalf("fetch ran %d times, want the second name to annotate a hit", ff.calls)
	}
}

func TestVerifySweepsOrphanRefs(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("blob that gets removed behind the store")}
	dgst := digestOf(ff.body)

	f, err := s.Get(t.Context(), dgst, "pkg-a@1.0.0", ff.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.Remove(blobPath(s.root, digest.Digest(dgst))); err != nil {
		t.Fatal(err)
	}
	Verify(t.Context(), s.root)

	if _, err := os.Stat(refsPath(s.root, digest.Digest(dgst))); err == nil {
		t.Fatal("orphan ref file survived verify")
	}
}

func TestMetaDirDeletedDegradesAndHeals(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("survives losing its name")}
	dgst := digestOf(ff.body)

	f, err := s.Get(t.Context(), dgst, "pkg-a@1.0.0", ff.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.RemoveAll(filepath.Join(s.root, "meta")); err != nil {
		t.Fatal(err)
	}

	if refs := readRefs(refsPath(s.root, digest.Digest(dgst))); refs != nil {
		t.Fatalf("refs = %v after meta wipe, want none", refs)
	}
	f, err = s.Get(t.Context(), dgst, "pkg-a@1.0.0", ff.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if ff.calls != 1 {
		t.Fatalf("fetch ran %d times, want the heal to ride a cache hit", ff.calls)
	}
	if refs := readRefs(refsPath(s.root, digest.Digest(dgst))); len(refs) != 1 || refs[0] != "pkg-a@1.0.0" {
		t.Fatalf("refs = %v, want the name healed", refs)
	}
}

func TestJunkInMetaDirIsHarmless(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("real blob")}
	f, err := s.Get(t.Context(), digestOf(ff.body), "pkg-a@1.0.0", ff.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	metaRoot := filepath.Join(s.root, "meta")
	if err := os.WriteFile(filepath.Join(metaRoot, "x"), []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}

	Verify(t.Context(), s.root)
	if _, err := os.Stat(filepath.Join(metaRoot, "x")); err == nil {
		t.Fatal("junk meta file survived verify")
	}
}

func TestReadRefsDropsGarbage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refs")
	content := "pkg-a@1.0.0\n\x1b[31mevil\x1b[0m@1.0.0\n@\nnoversion\n\x00\x01binary\npkg-b@2.0.0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	refs := readRefs(path)
	if len(refs) != 2 || refs[0] != "pkg-a@1.0.0" || refs[1] != "pkg-b@2.0.0" {
		t.Fatalf("refs = %q, want only the two clean refs", refs)
	}
}

func assertNoTempFiles(t *testing.T, s *Store) {
	t.Helper()
	entries, err := os.ReadDir(s.tmp)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp directory holds %d leftover entries", len(entries))
	}
}

func BenchmarkGetHit(b *testing.B) {
	s, err := New(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	payload := bytes.Repeat([]byte("x"), 1<<20)
	digest := digestOf(payload)
	fetch := func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}

	f, err := s.Get(b.Context(), digest, "pkg-bench@1.0.0", fetch)
	if err != nil {
		b.Fatal(err)
	}
	_ = f.Close()

	b.ReportAllocs()
	for b.Loop() {
		hit, err := s.Get(b.Context(), digest, "pkg-bench@1.0.0", fetch)
		if err != nil {
			b.Fatal(err)
		}
		_ = hit.Close()
	}
}
