package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

type fakeManifestServer struct {
	mu           sync.Mutex
	hits         map[string]int
	notModified  int
	body         string
	lastModified string
}

func (f *fakeManifestServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits[r.URL.Path]++
		body, lm := f.body, f.lastModified
		f.mu.Unlock()

		switch r.URL.Path {
		case "/pkg-a/latest":
			http.Redirect(w, r, "/pkg-a/1.0.0", http.StatusFound)
		case "/pkg-a/1.0.0":
			if lm != "" && r.Header.Get(HeaderIfModifiedSince) == lm {
				f.mu.Lock()
				f.notModified++
				f.mu.Unlock()
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set(HeaderLastModified, lm)
			w.Header().Set(HeaderContentType, "application/json")
			_, _ = w.Write([]byte(body))
		default:
			http.NotFound(w, r)
		}
	})
}

func (f *fakeManifestServer) hitCount(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func newCachingClient(t *testing.T, cacheDir string) (*Client, *fakeManifestServer, string) {
	t.Helper()
	reg := &fakeManifestServer{
		hits:         map[string]int{},
		body:         `{"name":"pkg-a","version":"1.0.0","type":"plugin"}`,
		lastModified: "Thu, 20 Aug 2026 00:46:05 GMT",
	}
	srv := httptest.NewServer(reg.handler())
	t.Cleanup(srv.Close)

	c, err := New(Options{Host: srv.URL, CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ct := &cacheTransport{dir: cacheDir}
	return c, reg, ct.entryPath(srv.URL + "/pkg-a/1.0.0")
}

func fetchManifest(t *testing.T, c *Client, versionOrTag string) {
	t.Helper()
	var out struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	err := c.Do(t.Context(), http.MethodGet, "/pkg-a/"+versionOrTag, nil, &out,
		WithHeader(HeaderAccept, MediaTypeManifestV1))
	if err != nil {
		t.Fatalf("fetch %s: %v", versionOrTag, err)
	}
	if out.Name != "pkg-a" || out.Version != "1.0.0" {
		t.Fatalf("manifest = %s@%s, want pkg-a@1.0.0", out.Name, out.Version)
	}
}

func ageManifestEntry(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the path is under t.TempDir()
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	e.FetchedAt = time.Now().Add(-time.Hour).Unix()
	data, err = json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestCacheFreshWindowSkipsNetwork(t *testing.T) {
	c, reg, entry := newCachingClient(t, t.TempDir())

	for range 3 {
		fetchManifest(t, c, "1.0.0")
	}
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 1 {
		t.Fatalf("server hits = %d, want 1", got)
	}
	if _, err := os.Stat(entry); err != nil {
		t.Fatalf("cache entry missing: %v", err)
	}
}

func TestCacheStaleRevalidatesWith304(t *testing.T) {
	c, reg, entry := newCachingClient(t, t.TempDir())

	fetchManifest(t, c, "1.0.0")
	ageManifestEntry(t, entry)
	fetchManifest(t, c, "1.0.0")

	if got := reg.hitCount("/pkg-a/1.0.0"); got != 2 {
		t.Fatalf("server hits = %d, want 2", got)
	}
	reg.mu.Lock()
	nm := reg.notModified
	reg.mu.Unlock()
	if nm != 1 {
		t.Fatalf("304 responses = %d, want the revalidation to be conditional", nm)
	}

	fetchManifest(t, c, "1.0.0")
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 2 {
		t.Fatalf("server hits = %d, want the 304 to refresh the window", got)
	}
}

func TestCacheStaleFetchesChangedBody(t *testing.T) {
	c, reg, entry := newCachingClient(t, t.TempDir())

	fetchManifest(t, c, "1.0.0")
	ageManifestEntry(t, entry)

	reg.mu.Lock()
	reg.body = `{"name":"pkg-a","version":"1.0.0","type":"theme"}`
	reg.lastModified = "Fri, 21 Aug 2026 10:00:00 GMT"
	reg.mu.Unlock()

	var out struct {
		Type string `json:"type"`
	}
	err := c.Do(t.Context(), http.MethodGet, "/pkg-a/1.0.0", nil, &out,
		WithHeader(HeaderAccept, MediaTypeManifestV1))
	if err != nil {
		t.Fatal(err)
	}
	if out.Type != "theme" {
		t.Fatalf("type = %s, want the updated body", out.Type)
	}

	fetchManifest(t, c, "1.0.0")
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 2 {
		t.Fatalf("server hits = %d, want the rewrite to restart the window", got)
	}
}

func TestCacheTagRedirectPopulatesVersionEntry(t *testing.T) {
	c, reg, entry := newCachingClient(t, t.TempDir())

	fetchManifest(t, c, "latest")
	if _, err := os.Stat(entry); err != nil {
		t.Fatalf("redirect target not cached under its version URL: %v", err)
	}

	fetchManifest(t, c, "1.0.0")
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 1 {
		t.Fatalf("version hits = %d, want the redirect-populated entry to serve", got)
	}
}

func TestCacheTagLookupAlwaysHitsRegistry(t *testing.T) {
	c, reg, _ := newCachingClient(t, t.TempDir())

	fetchManifest(t, c, "latest")
	fetchManifest(t, c, "latest")
	if got := reg.hitCount("/pkg-a/latest"); got != 2 {
		t.Fatalf("tag hits = %d, want 2", got)
	}
}

func TestCacheCorruptEntryRefetched(t *testing.T) {
	c, reg, entry := newCachingClient(t, t.TempDir())

	fetchManifest(t, c, "1.0.0")
	if err := os.WriteFile(entry, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	fetchManifest(t, c, "1.0.0")
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 2 {
		t.Fatalf("server hits = %d, want the corrupt entry treated as a miss", got)
	}

	fetchManifest(t, c, "1.0.0")
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 2 {
		t.Fatalf("server hits = %d, want the entry repaired", got)
	}
}

func TestCacheIgnoresOtherMediaTypes(t *testing.T) {
	c, reg, _ := newCachingClient(t, t.TempDir())

	for range 2 {
		var out struct{ Name string }
		if err := c.Do(t.Context(), http.MethodGet, "/pkg-a/1.0.0", nil, &out); err != nil {
			t.Fatal(err)
		}
	}
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 2 {
		t.Fatalf("server hits = %d, want plain json requests uncached", got)
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	c, reg, entry := newCachingClient(t, t.TempDir())

	fetch := func() error {
		var out struct{ Name string }
		return c.Do(t.Context(), http.MethodGet, "/pkg-a/1.0.0", nil, &out,
			WithHeader(HeaderAccept, MediaTypeManifestV1))
	}

	race := func() {
		var wg sync.WaitGroup
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := fetch(); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
	}

	race()
	ageManifestEntry(t, entry)
	race()

	if readEntry(entry) == nil {
		t.Fatal("entry unreadable after concurrent access")
	}
	before := reg.hitCount("/pkg-a/1.0.0")
	if err := fetch(); err != nil {
		t.Fatal(err)
	}
	if got := reg.hitCount("/pkg-a/1.0.0"); got != before {
		t.Fatalf("entry not fresh after concurrent revalidation, hits %d -> %d", before, got)
	}
}

func TestCacheFutureEntryTreatedAsStale(t *testing.T) {
	c, reg, entry := newCachingClient(t, t.TempDir())

	fetchManifest(t, c, "1.0.0")

	data, err := os.ReadFile(entry) //nolint:gosec // the path is under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatal(err)
	}
	e.FetchedAt = time.Now().Add(time.Hour).Unix()
	data, err = json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, data, 0o600); err != nil {
		t.Fatal(err)
	}

	fetchManifest(t, c, "1.0.0")
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 2 {
		t.Fatalf("server hits = %d, want a future entry to revalidate", got)
	}
}

func TestCacheDisabledWithoutDir(t *testing.T) {
	c, reg, _ := newCachingClient(t, "")

	fetchManifest(t, c, "1.0.0")
	fetchManifest(t, c, "1.0.0")
	if got := reg.hitCount("/pkg-a/1.0.0"); got != 2 {
		t.Fatalf("server hits = %d, want 2 with caching disabled", got)
	}
}
