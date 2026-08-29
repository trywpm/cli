package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// maxErrorBodySize caps how much of an error response we read into memory
// before parsing.
const maxErrorBodySize = 1 << 18

// HTTPError is a non 2xx response from the registry.
type HTTPError struct {
	Message    string
	StatusCode int
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if txt := http.StatusText(e.StatusCode); txt != "" {
		return strings.ToLower(txt)
	}
	return "unexpected registry response (HTTP " + strconv.Itoa(e.StatusCode) + ")"
}

// HandleHTTPError turns a non 2xx response into an *HTTPError. The caller
// still owns the response body.
func HandleHTTPError(resp *http.Response) error {
	httpError := &HTTPError{
		StatusCode: resp.StatusCode,
	}

	if !jsonTypeRE.MatchString(resp.Header.Get(HeaderContentType)) {
		return httpError
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
	if err != nil {
		return httpError
	}

	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		httpError.Message = strings.TrimSpace(parsed.Error)
	}
	return httpError
}
