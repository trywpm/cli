package api

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	headerMagic = 0x57504D43
	footerMagic = 0x57504D5E

	// Meta length sanity check: 512KB should be more than enough for headers.
	maxMetaLen = 512 * 1024

	// Bump cache version to invalidate old caches after metadata format change.
	cacheVersion = "wpm-cache-v1"
)

var cacheableHeaders = []string{
	HeaderEtag,
	HeaderLastModified,
	HeaderContentType,
	HeaderCacheControl,
	HeaderContentEncoding,
}

type Transport struct {
	Base     http.RoundTripper
	cacheDir string
}

type meta struct {
	Headers http.Header `json:"h"`
}

func CleanupStale(cacheDir string) error {
	tmpDir := filepath.Join(cacheDir, "tmp")
	entries, err := os.ReadDir(tmpDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	threshold := time.Now().Add(-15 * time.Minute)
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && info.ModTime().Before(threshold) {
			_ = os.Remove(filepath.Join(tmpDir, e.Name()))
		}
	}
	return nil
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base().RoundTrip(req)
	}

	outReq := req.Clone(req.Context())

	cache := outReq.Header.Get(HeaderSaveCache) == "true"
	force := outReq.Header.Get(HeaderCacheRevalidate) == "true"

	outReq.Header.Del(HeaderSaveCache)
	outReq.Header.Del(HeaderCacheRevalidate)

	if (!cache && !force) || t.cacheDir == "" {
		return t.base().RoundTrip(outReq)
	}

	hash := sha256.Sum256([]byte(outReq.URL.String()))
	key := hex.EncodeToString(hash[:])

	// Sharding: cache/aa/bb/<key> to avoid too many files in a single directory.
	finalPath := filepath.Join(t.cacheDir, key[:2], key[2:4], key)

	if !force {
		if body, h, err := t.open(finalPath); err == nil {
			h.Set(HeaderLocalCache, CacheHit)
			return t.response(outReq, body, h), nil
		}
	}

	res, err := t.executeRequest(outReq, finalPath, force)
	if err != nil {
		return t.base().RoundTrip(outReq)
	}

	return res, nil
}

func (t *Transport) executeRequest(req *http.Request, finalPath string, force bool) (*http.Response, error) {
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return t.base().RoundTrip(req)
	}

	if force {
		if body, h, err := t.open(finalPath); err == nil {
			if lm := h.Get(HeaderLastModified); lm != "" {
				req.Header.Set(HeaderIfModifiedSince, lm)
			}
			if et := h.Get(HeaderEtag); et != "" {
				req.Header.Set(HeaderIfNoneMatch, et)
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
		if body, h, err := t.open(finalPath); err == nil {
			h.Set(HeaderLocalCache, CacheHit)
			return t.response(req, body, h), nil
		}

		// Cache file missing/corrupt between conditional request and read.
		// Retry without conditional headers.
		req.Header.Del(HeaderIfNoneMatch)
		req.Header.Del(HeaderIfModifiedSince)
		resp, err = t.base().RoundTrip(req)
		if err != nil {
			return nil, err
		}
	}

	if resp.StatusCode == http.StatusOK {
		resp.Body = t.write(resp.Body, finalPath, resp.Header)
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
	if metaLen > maxMetaLen {
		return fail(fmt.Errorf("meta length %d exceeds %d limit", metaLen, maxMetaLen))
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

func (t *Transport) write(src io.ReadCloser, finalPath string, h http.Header) io.ReadCloser {
	tmpDir := filepath.Join(t.cacheDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return src
	}

	f, err := os.CreateTemp(tmpDir, "cache-*.tmp")
	if err != nil {
		return src
	}

	_ = os.Chmod(f.Name(), 0o644)

	if err := t.writeMeta(f, h); err != nil {
		f.Close()
		os.Remove(f.Name())
		return src
	}

	return &writer{
		src:   src,
		dst:   f,
		tmp:   f.Name(),
		final: finalPath,
	}
}

func (t *Transport) writeMeta(w io.Writer, h http.Header) error {
	if err := binary.Write(w, binary.BigEndian, uint32(headerMagic)); err != nil {
		return err
	}
	if _, err := w.Write([]byte(cacheVersion)); err != nil {
		return err
	}

	cacheable := http.Header{}
	for _, key := range cacheableHeaders {
		if values, ok := h[key]; ok {
			for _, v := range values {
				cacheable.Add(key, v)
			}
		}
	}

	b, err := json.Marshal(meta{Headers: cacheable})
	if err != nil {
		return err
	}
	if len(b) > maxMetaLen {
		return fmt.Errorf("cacheable headers too large: %d bytes", len(b))
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
	hitEOF      bool
	cacheFailed bool
}

func (w *writer) Read(p []byte) (int, error) {
	n, err := w.src.Read(p)
	if n > 0 && !w.cacheFailed {
		if _, wErr := w.dst.Write(p[:n]); wErr != nil {
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
	srcErr := w.src.Close()
	if w.cacheFailed {
		return srcErr
	}

	success := srcErr == nil && w.hitEOF
	if success {
		if err := binary.Write(w.dst, binary.BigEndian, uint32(footerMagic)); err != nil {
			success = false
		}
	}

	closeErr := w.dst.Close()
	if !success || closeErr != nil {
		os.Remove(w.tmp)
		return srcErr
	}

	if err := renameFile(w.tmp, w.final); err != nil {
		os.Remove(w.tmp)
	}
	return srcErr
}

// renameFile retries on transient Windows errors (file locked by reader,
// AV scanner). Exponential backoff up to ~3.5s.
func renameFile(src, dst string) error {
	const maxAttempts = 8
	backoff := 25 * time.Millisecond

	var err error
	for i := range maxAttempts {
		if i > 0 {
			time.Sleep(backoff)
			if backoff < time.Second {
				backoff *= 2
			}
		}
		err = os.Rename(src, dst)
		if err == nil {
			return nil
		}
	}
	return err
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
