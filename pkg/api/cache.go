package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.wpm.so/cli/pkg/atomicwriter"
	"go.wpm.so/cli/pkg/unsafeconv"
)

const (
	freshFor            = 5 * time.Minute
	MediaTypeManifestV1 = "application/vnd.wpm.install-v1+json"
)

type cacheTransport struct {
	base http.RoundTripper
	dir  string
}

type cacheEntry struct {
	FetchedAt    int64           `json:"fetchedAt"`
	LastModified string          `json:"lastModified,omitempty"`
	ETag         string          `json:"etag,omitempty"`
	ContentType  string          `json:"contentType,omitempty"`
	Body         json.RawMessage `json:"body"`
}

func (t *cacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.cacheable(req) {
		return t.base.RoundTrip(req)
	}

	entryPath := t.entryPath(req.URL.String())
	entry := readEntry(entryPath)
	if entry != nil {
		if age := time.Since(time.Unix(entry.FetchedAt, 0)); age >= 0 && age < freshFor {
			return entry.response(req), nil
		}
	}

	if entry != nil {
		if entry.LastModified != "" {
			req.Header.Set(HeaderIfModifiedSince, entry.LastModified)
		}
		if entry.ETag != "" {
			req.Header.Set(HeaderIfNoneMatch, entry.ETag)
		}
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotModified && entry != nil {
		drainBody(resp.Body)
		_ = resp.Body.Close()
		entry.FetchedAt = time.Now().Unix()
		writeEntry(entryPath, entry)
		return entry.response(req), nil
	}

	if resp.StatusCode == http.StatusOK {
		return store(entryPath, resp)
	}
	return resp, nil
}

func (t *cacheTransport) cacheable(req *http.Request) bool {
	return t.dir != "" && req.Method == http.MethodGet && req.Header.Get(HeaderAccept) == MediaTypeManifestV1
}

func store(entryPath string, resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	if len(body) > maxResponseBodySize {
		resp.Body = &wrappedBody{
			Reader: io.MultiReader(bytes.NewReader(body), resp.Body),
			Closer: resp.Body,
		}
		return resp, nil
	}
	_ = resp.Body.Close()

	writeEntry(entryPath, &cacheEntry{
		FetchedAt:    time.Now().Unix(),
		LastModified: resp.Header.Get(HeaderLastModified),
		ETag:         resp.Header.Get(HeaderEtag),
		ContentType:  resp.Header.Get(HeaderContentType),
		Body:         body,
	})

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}

func (e *cacheEntry) response(req *http.Request) *http.Response {
	h := http.Header{}
	if e.ContentType != "" {
		h.Set(HeaderContentType, e.ContentType)
	}
	if e.LastModified != "" {
		h.Set(HeaderLastModified, e.LastModified)
	}
	if e.ETag != "" {
		h.Set(HeaderEtag, e.ETag)
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        http.StatusText(http.StatusOK),
		Header:        h,
		Body:          io.NopCloser(bytes.NewReader(e.Body)),
		ContentLength: int64(len(e.Body)),
		Request:       req,
	}
}

func (t *cacheTransport) entryPath(url string) string {
	sum := sha256.Sum256(unsafeconv.UnsafeStringToBytes(url))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(t.dir, name[:2], name[2:4], name+".json")
}

func readEntry(file string) *cacheEntry {
	data, err := os.ReadFile(file) //nolint:gosec // the path is a hex digest under the cache dir
	if err != nil {
		return nil
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil || len(e.Body) == 0 {
		return nil
	}
	return &e
}

func writeEntry(file string, e *cacheEntry) {
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	_ = atomicwriter.WriteFile(file, data, 0o600)
}
