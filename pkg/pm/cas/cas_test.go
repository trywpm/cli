package cas

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	sum := sha256.Sum256(b)
	return digestPrefix + base64.StdEncoding.EncodeToString(sum[:])
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

func TestGetFetchesOnceThenHitsCache(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("a tarball, more or less")}
	digest := digestOf(ff.body)

	for i := range 2 {
		f, err := s.Get(t.Context(), digest, ff.fn)
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

	_, err := s.Get(t.Context(), claimed, ff.fn)
	if err == nil {
		t.Fatal("Get accepted a body that did not match the digest")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("error = %v, want a digest mismatch", err)
	}
	assertNoTempFiles(t, s)

	sum, _ := parseDigest(claimed)
	if _, err := os.Stat(s.blobPath(sum)); err == nil {
		t.Fatal("mismatched bytes were stored under the claimed digest")
	}
}

func TestGetRefetchesCorruptEntry(t *testing.T) {
	s := newStore(t)
	ff := &fakeFetch{body: []byte("good content that gets edited on disk later")}
	digest := digestOf(ff.body)

	f, err := s.Get(t.Context(), digest, ff.fn)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = f.Close()

	sum, err := parseDigest(digest)
	if err != nil {
		t.Fatalf("parseDigest: %v", err)
	}
	path := s.blobPath(sum)
	stored, err := os.ReadFile(path) //nolint:gosec // path comes from the store under t.TempDir()
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	stored[len(stored)/2] ^= 0xFF
	if err := os.WriteFile(path, stored, 0o600); err != nil { //nolint:gosec // same path
		t.Fatalf("WriteFile: %v", err)
	}

	f, err = s.Get(t.Context(), digest, ff.fn)
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

	_, err := s.Get(t.Context(), digestOf(ff.body), ff.fn)
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

	_, err := s.Get(t.Context(), digestOf([]byte("x")), func(context.Context) (io.ReadCloser, error) {
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
		f, err := s.Get(t.Context(), digestOf(ff.body), ff.fn)
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

func TestParseDigest(t *testing.T) {
	valid := digestOf([]byte("anything"))

	cases := []struct {
		name   string
		digest string
		ok     bool
	}{
		{"valid", valid, true},
		{"no_prefix", strings.TrimPrefix(valid, digestPrefix), false},
		{"wrong_algorithm", "sha512:" + strings.TrimPrefix(valid, digestPrefix), false},
		{"not_base64", digestPrefix + strings.Repeat("!", 44), false},
		{"too_long", digestPrefix + strings.Repeat("A", 64), false},
		{"too_short", digestPrefix + base64.StdEncoding.EncodeToString([]byte("short")), false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseDigest(tc.digest)
			if tc.ok && err != nil {
				t.Fatalf("parseDigest(%q) = %v, want no error", tc.digest, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("parseDigest(%q) accepted an invalid digest", tc.digest)
			}
		})
	}
}

func TestBlobPathStaysUnderRoot(t *testing.T) {
	s := newStore(t)

	var sum [sha256.Size]byte
	for i := range sum {
		sum[i] = byte(i)
	}

	path := s.blobPath(sum)
	rel, err := filepath.Rel(s.root, path)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	if !filepath.IsLocal(rel) {
		t.Fatalf("blob path %q escapes the store root", path)
	}
	if filepath.Base(path) != "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" {
		t.Fatalf("blob name = %q, want the hex digest", filepath.Base(path))
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

	f, err := s.Get(t.Context(), digestOf(good.body), good.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	f, err = s.Get(t.Context(), digestOf(bad.body), bad.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	sum, _ := parseDigest(digestOf(bad.body))
	if err := os.WriteFile(s.blobPath(sum), []byte("flipped"), 0o600); err != nil {
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

	f, err = s.Get(t.Context(), digestOf(good.body), good.fn)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if good.calls != 1 {
		t.Fatalf("good blob refetched after verify, calls = %d", good.calls)
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

	f, err := s.Get(b.Context(), digest, fetch)
	if err != nil {
		b.Fatal(err)
	}
	_ = f.Close()

	b.ReportAllocs()
	for b.Loop() {
		hit, err := s.Get(b.Context(), digest, fetch)
		if err != nil {
			b.Fatal(err)
		}
		_ = hit.Close()
	}
}
