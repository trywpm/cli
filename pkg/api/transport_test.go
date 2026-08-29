package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func zstdCompress(tb testing.TB, b []byte) []byte {
	tb.Helper()
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		tb.Fatal(err)
	}
	return enc.EncodeAll(b, nil)
}

// A zstd body holding a control character proves the decode then sanitize
// order. Sanitizing before decoding would corrupt the compressed stream and
// skipping the sanitizer would leak the control byte through.
func TestTransportOrderDecodeThenSanitize(t *testing.T) {
	compressed := zstdCompress(t, []byte("{\"v\":\"a\x08b\"}"))
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentEncoding, encodingZstd)
		w.Header().Set(HeaderContentType, "application/json")
		_, _ = w.Write(compressed)
	}), Options{})

	var raw string
	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &raw); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if strings.ContainsRune(raw, '\x08') {
		t.Fatal("control character survived: sanitizer did not run on decoded bytes")
	}
	var obj struct{ V string }
	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &obj); err != nil {
		t.Fatalf("decoded JSON unparseable after sanitize: %v", err)
	}
}

// Every redirect hop re-enters the transport stack, so defaults and the token
// must be present on the second hop, not just the first.
func TestRedirectHopKeepsHeaders(t *testing.T) {
	var hopUA, hopAuth string
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		hopUA = r.Header.Get(HeaderUserAgent)
		hopAuth = r.Header.Get(HeaderAuthorization)
	}), Options{UserAgent: "wpm-test", AuthToken: "sekret"})

	if err := c.Do(t.Context(), http.MethodGet, "/start", nil, nil); err != nil {
		t.Fatal(err)
	}
	if hopUA != "wpm-test" {
		t.Fatalf("redirected hop User-Agent = %q", hopUA)
	}
	if hopAuth != "Bearer sekret" {
		t.Fatalf("redirected hop Authorization = %q", hopAuth)
	}
}

func TestZstdBodyDoubleCloseSafe(t *testing.T) {
	compressed := zstdCompress(t, []byte(`{"ok":true}`))
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentEncoding, encodingZstd)
		w.Header().Set(HeaderContentType, "application/octet-stream")
		_, _ = w.Write(compressed)
	}), Options{})

	rc, err := c.Stream(t.Context(), http.MethodGet, "/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(rc); err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatal("second Close errored")
	}

	// The pool must hand out working decoders afterwards.
	var out struct{ Ok bool }
	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &out); err != nil || !out.Ok {
		t.Fatalf("pool poisoned after double close: %v", err)
	}
}

func TestNoGoroutineOrFDLeaks(t *testing.T) {
	compressed := zstdCompress(t, []byte(`{"ok":true}`))
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentEncoding, encodingZstd)
		w.Header().Set(HeaderContentType, "application/json")
		_, _ = w.Write(compressed)
	}), Options{})

	warm := func(n int) {
		var wg sync.WaitGroup
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var out struct{ Ok bool }
				if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &out); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
	}

	// Idle connections carry stdlib goroutines and fds, so close them before
	// each measurement removes pool noise, so any residual growth is a real
	// per-request leak (a decoder goroutine, an unclosed body).
	warm(32)
	c.http.CloseIdleConnections()
	time.Sleep(100 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()
	fdsBefore := countFDs(t)

	for range 10 {
		warm(32)
	}

	// Teardown of server-side conn handlers is asynchronous, so poll for the
	// count to come back down instead of sleeping once. A real per-request
	// leak is ~320 goroutines and never settles.
	deadline := time.Now().Add(3 * time.Second)
	var goroutinesAfter, fdsAfter int
	for {
		c.http.CloseIdleConnections()
		time.Sleep(50 * time.Millisecond)
		goroutinesAfter, fdsAfter = runtime.NumGoroutine(), countFDs(t)
		if goroutinesAfter <= goroutinesBefore+2 || time.Now().After(deadline) {
			break
		}
	}

	if goroutinesAfter > goroutinesBefore+2 {
		t.Fatalf("goroutines grew %d -> %d across 320 requests", goroutinesBefore, goroutinesAfter)
	}
	if fdsBefore > 0 && fdsAfter > fdsBefore+2 {
		t.Fatalf("fds grew %d -> %d", fdsBefore, fdsAfter)
	}
}

func countFDs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0
	}
	return len(entries)
}

func BenchmarkDoJSON(b *testing.B) {
	body := []byte(`{"name":"akismet","version":"5.7.2","type":"plugin","dist":{"digest":"sha256:hmBCVLbYU0UkrrgEE3xDhAd9jmVq57tMgICXB0XZrGA="}}`)
	c := newClientB(b, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentType, "application/json")
		_, _ = w.Write(body)
	}))

	b.ReportAllocs()
	for b.Loop() {
		var out struct{ Name string }
		if err := c.Do(b.Context(), http.MethodGet, "/x", nil, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDoZstd(b *testing.B) {
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		b.Fatal(err)
	}
	body := enc.EncodeAll([]byte(`{"name":"akismet","version":"5.7.2","type":"plugin","dist":{"digest":"sha256:hmBCVLbYU0UkrrgEE3xDhAd9jmVq57tMgICXB0XZrGA="}}`), nil)
	c := newClientB(b, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentEncoding, encodingZstd)
		w.Header().Set(HeaderContentType, "application/json")
		_, _ = w.Write(body)
	}))

	b.ReportAllocs()
	for b.Loop() {
		var out struct{ Name string }
		if err := c.Do(b.Context(), http.MethodGet, "/x", nil, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func newClientB(b *testing.B, handler http.Handler) *Client {
	b.Helper()
	srv := httptest.NewServer(handler)
	b.Cleanup(srv.Close)
	c, err := New(Options{Host: srv.URL})
	if err != nil {
		b.Fatal(err)
	}
	return c
}
