package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxErrorBodySize caps how much of an error response we'll read into memory
// before parsing.
const maxErrorBodySize = 1 << 18 // 256 KiB

// HTTPError represents an error response from the wpm API.
type HTTPError struct {
	Message    string
	Headers    http.Header
	RequestURL *url.URL
	StatusCode int
}

func (err *HTTPError) Error() string {
	if err.Message == "" {
		return "wpm registry error: " + strings.ToLower(http.StatusText(err.StatusCode))
	}
	return "wpm registry error: " + err.Message
}

// HandleHTTPError parses a http.Response into a HTTPError. The response body
// is not closed here and the caller owns the response and must close it. We do,
// however, fully consume the body so the connection stays reusable.
func HandleHTTPError(resp *http.Response) error {
	httpError := &HTTPError{
		Headers:    resp.Header,
		RequestURL: resp.Request.URL,
		StatusCode: resp.StatusCode,
	}

	if !jsonTypeRE.MatchString(resp.Header.Get(HeaderContentType)) {
		httpError.Message = resp.Status
		return httpError
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
	if err != nil {
		httpError.Message = err.Error()
		return httpError
	}

	var parsedBody struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		return httpError
	}

	httpError.Message = parsedBody.Error

	return httpError
}
