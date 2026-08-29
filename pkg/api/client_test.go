package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func newClient(t *testing.T, handler http.Handler, opts Options) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	opts.Host = srv.URL
	c, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestDefaultHeadersAndCallerOverride(t *testing.T) {
	var got http.Header
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}), Options{UserAgent: "wpm-test"})

	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	for header, want := range map[string]string{
		HeaderAccept:         "application/json",
		HeaderAcceptEncoding: encodingZstd,
		HeaderUserAgent:      "wpm-test",
	} {
		if v := got.Get(header); v != want {
			t.Fatalf("%s = %q, want %q", header, v, want)
		}
	}

	err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil, WithHeader(HeaderAcceptEncoding, "identity"))
	if err != nil {
		t.Fatal(err)
	}
	if v := got.Get(HeaderAcceptEncoding); v != "identity" {
		t.Fatalf("caller-set Accept-Encoding = %q, want identity", v)
	}
}

func TestAuthTokenSentToLoopback(t *testing.T) {
	var got string
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(HeaderAuthorization)
	}), Options{AuthToken: "sekret"})

	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	if got != "Bearer sekret" {
		t.Fatalf("Authorization = %q, want the bearer token", got)
	}
}

func TestAuthTokenRefusedOverCleartext(t *testing.T) {
	if _, err := New(Options{Host: "http://example.com", AuthToken: "sekret"}); err == nil {
		t.Fatal("New accepted a token over http to a remote host")
	}
}

func TestRedirectOutsideHostRefused(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://attacker.example/x", http.StatusFound)
	}), Options{})

	err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing redirect") {
		t.Fatalf("err = %v, want a refused redirect", err)
	}
}

func TestZstdResponseDecoded(t *testing.T) {
	payload := `{"ok":true}`
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		enc, err := zstd.NewWriter(nil)
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Encoding", encodingZstd)
		w.Header().Set(HeaderContentType, "application/json")
		_, _ = w.Write(enc.EncodeAll([]byte(payload), nil))
	}), Options{})

	var out struct{ Ok bool }
	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Ok {
		t.Fatal("zstd body did not decode")
	}
}

func TestJSONResponseSanitized(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentType, "application/json")
		_, _ = w.Write([]byte("{\"v\":\"a\x08b\"}"))
	}), Options{})

	var raw string
	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &raw); err == nil {
		if strings.ContainsRune(raw, '\x08') {
			t.Fatal("control character survived sanitizing")
		}
	}
}

func TestDoDecodesResponses(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(HeaderContentType, "application/json")
		_, _ = io.WriteString(w, `{"name":"akismet"}`)
	}), Options{})

	var s string
	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &s); err != nil {
		t.Fatal(err)
	}
	if s != `{"name":"akismet"}` {
		t.Fatalf("*string = %q", s)
	}

	var obj struct{ Name string }
	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &obj); err != nil {
		t.Fatal(err)
	}
	if obj.Name != "akismet" {
		t.Fatalf("decoded name = %q", obj.Name)
	}
}

func TestDecodeFailureNamesTheRequest(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentType, "text/html")
		_, _ = io.WriteString(w, "<html>Sign in to WiFi</html>")
	}), Options{})

	var out struct{ Ok bool }
	err := c.Do(t.Context(), http.MethodGet, "/keys.json", nil, &out)
	if err == nil || !strings.Contains(err.Error(), "GET /keys.json") {
		t.Fatalf("err = %v, want the failing request named", err)
	}
}

func TestErrorResponseBecomesHTTPError(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentType, "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"no such package"}`)
	}), Options{})

	err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil)
	httpErr, ok := errors.AsType[*HTTPError](err)
	if !ok {
		t.Fatalf("err = %T, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusNotFound || !strings.Contains(httpErr.Message, "no such package") {
		t.Fatalf("HTTPError = %+v", httpErr)
	}
}

func TestStreamReturnsBodyAndRefusesErrors(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = io.WriteString(w, "raw bytes")
	}), Options{})

	rc, err := c.Stream(t.Context(), http.MethodGet, "/blob", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "raw bytes" {
		t.Fatalf("body = %q", b)
	}

	if _, err := c.Stream(t.Context(), http.MethodGet, "/bad", nil); err == nil {
		t.Fatal("Stream returned no error for a 403")
	}
}

func TestGetRetriesTransientStatus(t *testing.T) {
	var hits atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) < 3 {
			w.Header().Set(HeaderRetryAfter, "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}), Options{})

	var out string
	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 3 {
		t.Fatalf("hits = %d, want 3", hits.Load())
	}
}

func TestNonGetNeverRetries(t *testing.T) {
	var hits atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}), Options{})

	if err := c.Do(t.Context(), http.MethodPut, "/x", strings.NewReader("body"), nil); err == nil {
		t.Fatal("expected an error")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1 (PUT must not retry)", hits.Load())
	}
}

func TestClientErrorStatusNeverRetries(t *testing.T) {
	var hits atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}), Options{})

	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil); err == nil {
		t.Fatal("expected an error")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1 (4xx must not retry)", hits.Load())
	}
}

func TestRedirectRefusalNotRetried(t *testing.T) {
	var hits atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "https://attacker.example/x", http.StatusFound)
	}), Options{})

	if err := c.Do(t.Context(), http.MethodGet, "/x", nil, nil); err == nil {
		t.Fatal("expected a refused redirect")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want a policy refusal to skip retries", hits.Load())
	}
}

func TestGetWithBodyNeverRetries(t *testing.T) {
	var hits atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}), Options{})

	if err := c.Do(t.Context(), http.MethodGet, "/x", strings.NewReader("body"), nil); err == nil {
		t.Fatal("expected an error")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want a GET with a body to skip retries", hits.Load())
	}
}

func TestNormalizeBaseURLLowercasesHost(t *testing.T) {
	a, err := normalizeBaseURL("https://Registry.WPM.so")
	if err != nil {
		t.Fatal(err)
	}
	b, err := normalizeBaseURL("registry.wpm.so")
	if err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatalf("normalized hosts differ, %q vs %q", a, b)
	}
}

func TestAbsoluteURLPathRefused(t *testing.T) {
	c := newClient(t, http.NotFoundHandler(), Options{})
	if err := c.Do(t.Context(), http.MethodGet, "https://evil.example/x", nil, nil); err == nil {
		t.Fatal("absolute URL accepted as path")
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"registry.wpm.so", "https://registry.wpm.so", true},
		{"http://localhost:8080", "http://localhost:8080", true},
		{"https://reg.example/path/", "https://reg.example/path", true},
		{"", "", false},
		{"ftp://reg.example", "", false},
		{"https://", "", false},
	}
	for _, tc := range cases {
		u, err := normalizeBaseURL(tc.in)
		if tc.ok != (err == nil) {
			t.Fatalf("normalizeBaseURL(%q) error = %v, want ok=%v", tc.in, err, tc.ok)
		}
		if err == nil && u.String() != tc.want {
			t.Fatalf("normalizeBaseURL(%q) = %q, want %q", tc.in, u, tc.want)
		}
	}
}
