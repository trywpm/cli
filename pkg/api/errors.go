package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HTTPError represents an error response from the wpm API.
type HTTPError struct {
	Message    string
	Headers    http.Header
	RequestURL *url.URL
	StatusCode int
}

// Allow HTTPError to satisfy error interface.
func (err *HTTPError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("wpm registry error: %s", strings.ToLower(http.StatusText(err.StatusCode)))
	}
	return fmt.Sprintf("wpm registry error: %s", strings.ToLower(err.Message))
}

// HandleHTTPError parses a http.Response into a HTTPError.
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

	body, err := io.ReadAll(resp.Body)
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
