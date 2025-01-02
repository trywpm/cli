package client

import (
	"bytes"
	"io"

	"wpm/pkg/jsonpretty"
)

// jsonFormatter is a httpretty.Formatter that prettifies JSON payloads and GraphQL queries.
type jsonFormatter struct {
	colorize bool
}

func (f *jsonFormatter) Format(w io.Writer, src []byte) error {
	return jsonpretty.Format(w, bytes.NewReader(src), "  ", f.colorize)
}

func (f *jsonFormatter) Match(t string) bool {
	return jsonTypeRE.MatchString(t)
}
