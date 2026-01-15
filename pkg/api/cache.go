package api

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	cacheVersion = "wpm-cache-v2"
	headerMagic  = 0x57504D43
	footerMagic  = 0x57504D5E
	lockTimeout  = 60 * time.Second
	maxLockAge   = 5 * time.Minute
)

type Transport struct {
	Base     http.RoundTripper
	sf       singleflight
	cacheDir string
}

type meta struct {
	Headers http.Header `json:"h"`
}

func CleanupStale(cacheDir string) error {
	tmpDir := filepath.Join(cacheDir, "tmp")
	entries, err := os.ReadDir(tmpDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	threshold := time.Now().Add(-15 * time.Minute)
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && info.ModTime().Before(threshold) {
			os.Remove(filepath.Join(tmpDir, e.Name()))
		}
	}
	return nil
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base().RoundTrip(req)
	}

	outReq := req.Clone(req.Context())

	cache := outReq.Header.Get("x-do-cache") == "true"
	force := outReq.Header.Get("x-cache-revalidate") == "true"

	outReq.Header.Del("x-do-cache")
	outReq.Header.Del("x-cache-revalidate")

	if (!cache && !force) || t.cacheDir == "" {
		return t.base().RoundTrip(outReq)
	}

	hash := sha256.Sum256([]byte(outReq.URL.String()))
	key := hex.EncodeToString(hash[:])

	// Sharding: cache/a1/b2/key
	finalPath := filepath.Join(t.cacheDir, key[:2], key[2:4], key)

	if !force {
		if body, h, err := t.open(finalPath); err == nil {
			h.Set("x-local-cache", "hit")
			return t.response(outReq, body, h), nil
		}
	}

	res, err, shared := t.sf.Do(key, func() (any, error) {
		return t.executeRequest(outReq, finalPath, force)
	})

	if err != nil {
		return t.base().RoundTrip(outReq)
	}

	resp := res.(*http.Response)

	// If this request was shared, another goroutine may have cached it
	// while we were waiting. In that case, we re-open the cache to
	// ensure we return a cached response.
	if shared {
		if body, h, err := t.open(finalPath); err == nil {
			h.Set("x-local-cache", "hit")
			return t.response(outReq, body, h), nil
		}

		// If file is not ready, fall back to fresh network request.
		// We do NOT cache this fallback to avoid race conditions.
		return t.base().RoundTrip(outReq)
	}

	return resp, nil
}

func (t *Transport) executeRequest(req *http.Request, path string, force bool) (*http.Response, error) {
	unlock, err := t.acquireLock(path + ".lock")
	if err != nil {
		return t.base().RoundTrip(req)
	}

	lockHeld := true
	defer func() {
		if lockHeld {
			unlock()
		}
	}()

	if !force {
		if body, h, err := t.open(path); err == nil {
			h.Set("x-local-cache", "hit")
			return t.response(req, body, h), nil
		}
	}

	if force {
		if body, h, err := t.open(path); err == nil {
			if lm := h.Get("Last-Modified"); lm != "" {
				req.Header.Set("If-Modified-Since", lm)
			}
			if et := h.Get("ETag"); et != "" {
				req.Header.Set("If-None-Match", et)
			}

			body.Close()
		}
	}

	resp, err := t.base().RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		resp.Body.Close()
		if b, h, err := t.open(path); err == nil {
			return t.response(req, b, h), nil
		}

		// Cache file missing/corrupt, redo the request without conditions
		req.Header.Del("If-None-Match")
		req.Header.Del("If-Modified-Since")
		resp, err = t.base().RoundTrip(req)
		if err != nil {
			return nil, err
		}
	}

	if resp.StatusCode == http.StatusOK {
		wrappedBody := t.write(resp.Body, path, resp.Header, unlock)
		resp.Body = wrappedBody
		lockHeld = false
	}

	return resp, nil
}

func (t *Transport) open(path string) (io.ReadCloser, http.Header, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	fail := func(e error) (io.ReadCloser, http.Header, error) {
		f.Close()
		return nil, nil, e
	}

	var magic uint32
	if err := binary.Read(f, binary.BigEndian, &magic); err != nil || magic != headerMagic {
		return fail(errors.New("bad magic"))
	}

	ver := make([]byte, len(cacheVersion))
	if _, err := io.ReadFull(f, ver); err != nil || string(ver) != cacheVersion {
		return fail(errors.New("version mismatch"))
	}

	var metaLen uint32
	if err := binary.Read(f, binary.BigEndian, &metaLen); err != nil {
		return fail(err)
	}

	rawMeta := make([]byte, metaLen)
	if _, err := io.ReadFull(f, rawMeta); err != nil {
		return fail(err)
	}

	var m meta
	if err := json.Unmarshal(rawMeta, &m); err != nil {
		return fail(err)
	}

	bodyStart, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fail(err)
	}

	stat, err := f.Stat()
	if err != nil {
		return fail(err)
	}

	// Ensure file ends with valid footer
	if stat.Size() < bodyStart+4 {
		return fail(errors.New("truncated"))
	}

	if _, err := f.Seek(-4, io.SeekEnd); err != nil {
		return fail(err)
	}

	var endMagic uint32
	if err := binary.Read(f, binary.BigEndian, &endMagic); err != nil || endMagic != footerMagic {
		return fail(errors.New("bad footer"))
	}

	if _, err := f.Seek(bodyStart, io.SeekStart); err != nil {
		return fail(err)
	}

	return &safeReader{
		f:             f,
		SectionReader: io.NewSectionReader(f, bodyStart, stat.Size()-bodyStart-4),
	}, m.Headers, nil
}

func (t *Transport) write(src io.ReadCloser, finalPath string, h http.Header, unlock func()) io.ReadCloser {
	rootDir := filepath.Dir(filepath.Dir(filepath.Dir(finalPath)))
	tmpDir := filepath.Join(rootDir, "tmp")

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		unlock()
		return src
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		unlock()
		return src
	}

	tmpName := fmt.Sprintf("%x.%d.tmp", sha256.Sum256([]byte(finalPath)), time.Now().UnixNano())
	tmpPath := filepath.Join(tmpDir, tmpName)

	f, err := os.Create(tmpPath)
	if err != nil {
		unlock()
		return src
	}

	if err := t.writeMeta(f, h); err != nil {
		f.Close()
		os.Remove(tmpPath)
		unlock()
		return src
	}

	return &writer{
		src:    src,
		dst:    f,
		tmp:    tmpPath,
		final:  finalPath,
		unlock: unlock,
	}
}

func (t *Transport) writeMeta(w io.Writer, h http.Header) error {
	if err := binary.Write(w, binary.BigEndian, uint32(headerMagic)); err != nil {
		return err
	}
	if _, err := w.Write([]byte(cacheVersion)); err != nil {
		return err
	}

	metaHeaders := http.Header{}
	for _, key := range []string{"ETag", "Last-Modified", "Content-Type", "Cache-Control"} {
		if values, ok := h[key]; ok {
			for _, v := range values {
				metaHeaders.Add(key, v)
			}
		}
	}

	h = metaHeaders

	b, err := json.Marshal(meta{Headers: h})
	if err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(b))); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

type writer struct {
	src         io.ReadCloser
	dst         *os.File
	tmp         string
	final       string
	unlock      func()
	hitEOF      bool
	cacheFailed bool
}

func (w *writer) Read(p []byte) (int, error) {
	n, err := w.src.Read(p)

	if n > 0 && !w.cacheFailed {
		if _, wErr := w.dst.Write(p[:n]); wErr != nil {
			// On write failure, stop caching and clean up
			w.cacheFailed = true
			w.dst.Close()
			os.Remove(w.tmp)
		}
	}

	if err == io.EOF {
		w.hitEOF = true
	}
	return n, err
}

func (w *writer) Close() error {
	defer w.unlock()

	srcErr := w.src.Close()
	success := srcErr == nil && w.hitEOF && !w.cacheFailed

	var footerErr error
	if success {
		footerErr = binary.Write(w.dst, binary.BigEndian, uint32(footerMagic))
	}

	if !w.cacheFailed {
		dstErr := w.dst.Close()

		if success && footerErr == nil && dstErr == nil {
			if err := renameFile(w.tmp, w.final); err != nil {
				os.Remove(w.tmp)
			}
		} else {
			os.Remove(w.tmp)
		}
	} else {
		os.Remove(w.tmp)
	}

	return srcErr
}

// rename with retry for Windows compatibility
func renameFile(src, dst string) error {
	for range 5 {
		err := os.Rename(src, dst)
		if err == nil {
			return nil
		}

		// On Windows, if the file is locked by another process (Reader),
		// both Remove and Rename will fail with "Access Denied".
		// We retry in hopes the Reader finishes or the AV scanner releases it.
		time.Sleep(50 * time.Millisecond)
	}

	// Final attempt: remove destination first
	os.Remove(dst)
	return os.Rename(src, dst)
}

func (t *Transport) acquireLock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// O_EXCL ensures atomic check-and-create
			f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0666)
			if err == nil {
				f.Close()
				return func() { os.Remove(path) }, nil
			}

			// If the lock file exists, check its age
			if os.IsExist(err) {
				if info, statErr := os.Stat(path); statErr == nil {
					if time.Since(info.ModTime()) > maxLockAge {
						os.Remove(path)
						continue
					}
				}
			} else {
				return nil, err
			}
		}
	}
}

func (t *Transport) response(req *http.Request, body io.ReadCloser, h http.Header) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       body,
		Header:     h,
		Request:    req,
		Status:     http.StatusText(http.StatusOK),
	}
}

func (t *Transport) base() http.RoundTripper {
	if t.Base == nil {
		return http.DefaultTransport
	}
	return t.Base
}

type safeReader struct {
	*io.SectionReader
	f *os.File
}

func (s *safeReader) Close() error {
	return s.f.Close()
}

type singleflight struct {
	mu sync.Mutex
	m  map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	val any
	err error
}

func (g *singleflight) Do(key string, fn func() (any, error)) (any, error, bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err, false
}
