package client

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
	Errors     []HTTPErrorItem
	Headers    http.Header
	Message    string
	RequestURL *url.URL
	StatusCode int
}

// HTTPErrorItem stores additional information about an error response
// returned from the wpm API.
type HTTPErrorItem struct {
	Code    string
	Message string
}

// Allow HTTPError to satisfy error interface.
func (err *HTTPError) Error() string {
	if msgs := strings.SplitN(err.Message, "\n", 2); len(msgs) > 1 {
		return fmt.Sprintf("HTTP %d: %s (%s)\n%s", err.StatusCode, msgs[0], err.RequestURL, msgs[1])
	} else if err.Message != "" {
		return fmt.Sprintf("HTTP %d: %s (%s)", err.StatusCode, err.Message, err.RequestURL)
	}
	return fmt.Sprintf("HTTP %d (%s)", err.StatusCode, err.RequestURL)
}

func matchPath(p, expect string) bool {
	if strings.HasSuffix(expect, ".") {
		return strings.HasPrefix(p, expect) || p == strings.TrimSuffix(expect, ".")
	}
	return p == expect
}

// HandleHTTPError parses a http.Response into a HTTPError.
func HandleHTTPError(resp *http.Response) error {
	httpError := &HTTPError{
		Headers:    resp.Header,
		RequestURL: resp.Request.URL,
		StatusCode: resp.StatusCode,
	}

	if !jsonTypeRE.MatchString(resp.Header.Get(contentType)) {
		httpError.Message = resp.Status
		return httpError
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		httpError.Message = err.Error()
		return httpError
	}

	var parsedBody struct {
		Message string `json:"message"`
		Errors  []json.RawMessage
	}
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		return httpError
	}

	var messages []string
	if parsedBody.Message != "" {
		messages = append(messages, parsedBody.Message)
	}
	for _, raw := range parsedBody.Errors {
		switch raw[0] {
		case '"':
			var errString string
			_ = json.Unmarshal(raw, &errString)
			messages = append(messages, errString)
			httpError.Errors = append(httpError.Errors, HTTPErrorItem{Message: errString})
		case '{':
			var errInfo HTTPErrorItem
			_ = json.Unmarshal(raw, &errInfo)
			msg := errInfo.Message
			if msg != "" {
				messages = append(messages, msg)
			}
			httpError.Errors = append(httpError.Errors, errInfo)
		}
	}
	httpError.Message = strings.Join(messages, "\n")

	return httpError
}
