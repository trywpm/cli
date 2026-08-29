package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPErrorRendersOnlyTheCause(t *testing.T) {
	cases := []struct {
		name string
		err  HTTPError
		want string
	}{
		{"registry message", HTTPError{Message: "version not found", StatusCode: 404}, "version not found"},
		{"status text fallback", HTTPError{StatusCode: 404}, "not found"},
		{"unknown status keeps the code", HTTPError{StatusCode: 599}, "unexpected registry response (HTTP 599)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHTTPErrorComposesWhenWrapped(t *testing.T) {
	err := fmt.Errorf("failed to fetch package akismet@9.9.9: %w",
		&HTTPError{Message: "version not found", StatusCode: 404})
	want := "failed to fetch package akismet@9.9.9: version not found"
	if err.Error() != want {
		t.Fatalf("wrapped chain = %q, want %q", err.Error(), want)
	}
}

func TestHandleHTTPErrorParsesBodies(t *testing.T) {
	serve := func(contentType, body string, status int) error {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if contentType != "" {
				w.Header().Set(HeaderContentType, contentType)
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		defer srv.Close()
		c, err := New(Options{Host: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		return c.Do(t.Context(), http.MethodGet, "/pkg/1.0.0", nil, nil)
	}

	if got := serve("application/json", `{"error":"  version not found  "}`, http.StatusNotFound).Error(); got != "version not found" {
		t.Fatalf("json body = %q", got)
	}
	if got := serve("text/html", "<html>boom</html>", http.StatusForbidden).Error(); got != "forbidden" {
		t.Fatalf("non json body = %q", got)
	}
	if got := serve("application/json", "not json", http.StatusConflict).Error(); got != "conflict" {
		t.Fatalf("garbage json body = %q", got)
	}
}
