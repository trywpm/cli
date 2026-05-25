package jsonpretty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	colorDelim  = "\x1b[1;38m" // bright white
	colorKey    = "\x1b[1;34m" // bright blue
	colorNull   = "\x1b[36m"   // cyan
	colorString = "\x1b[32m"   // green
	colorBool   = "\x1b[33m"   // yellow
	colorReset  = "\x1b[m"
)

// Format reads JSON from r and writes a prettified version of it to w.
func Format(w io.Writer, r io.Reader, indent string, colorize bool) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()

	c := func(ansi string) string {
		if !colorize {
			return ""
		}
		return ansi
	}

	var idx int
	var stack []json.Delim

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		skipTrailing, isKey, err := processToken(w, dec, t, c, indent, &stack, &idx)
		if err != nil {
			return err
		}

		if skipTrailing || isKey {
			continue
		}

		if err := processTrailing(w, dec, c, indent, stack); err != nil {
			return err
		}
	}

	return nil
}

func processToken(w io.Writer, dec *json.Decoder, t json.Token, c func(string) string, indent string, stack *[]json.Delim, idx *int) (skipTrailing, isKey bool, err error) {
	switch tt := t.(type) {
	case json.Delim:
		err = processDelim(w, dec, tt, c, indent, stack, idx)
		return true, false, err
	default:
		isKey, err = processValue(w, tt, t, c, *stack, idx)
		return false, isKey, err
	}
}

func processDelim(w io.Writer, dec *json.Decoder, tt json.Delim, c func(string) string, indent string, stack *[]json.Delim, idx *int) error {
	switch tt {
	case '{', '[':
		*stack = append(*stack, tt)
		*idx = 0
		if _, err := fmt.Fprint(w, c(colorDelim), tt, c(colorReset)); err != nil {
			return err
		}
		if dec.More() {
			if _, err := fmt.Fprint(w, "\n", strings.Repeat(indent, len(*stack))); err != nil {
				return err
			}
		}
	case '}', ']':
		*stack = (*stack)[:len(*stack)-1]
		*idx = 0
		if _, err := fmt.Fprint(w, c(colorDelim), tt, c(colorReset)); err != nil {
			return err
		}
	}
	return nil
}

func processValue(w io.Writer, tt any, t json.Token, c func(string) string, stack []json.Delim, idx *int) (isKey bool, err error) {
	b, err := marshalJSON(tt)
	if err != nil {
		return false, err
	}

	isKey = len(stack) > 0 && stack[len(stack)-1] == '{' && *idx%2 == 0
	*idx++

	color := selectColor(isKey, tt, t)

	if color != "" {
		if _, err := fmt.Fprint(w, c(color)); err != nil {
			return false, err
		}
	}
	if _, err := w.Write(b); err != nil {
		return false, err
	}
	if color != "" {
		if _, err := fmt.Fprint(w, c(colorReset)); err != nil {
			return false, err
		}
	}

	if isKey {
		if _, err := fmt.Fprint(w, c(colorDelim), ":", c(colorReset), " "); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func selectColor(isKey bool, tt any, t json.Token) string {
	switch {
	case isKey:
		return colorKey
	case tt == nil:
		return colorNull
	default:
		switch t.(type) {
		case string:
			return colorString
		case bool:
			return colorBool
		}
	}
	return ""
}

func processTrailing(w io.Writer, dec *json.Decoder, c func(string) string, indent string, stack []json.Delim) error {
	switch {
	case dec.More():
		if _, err := fmt.Fprint(w, c(colorDelim), ",", c(colorReset), "\n", strings.Repeat(indent, len(stack))); err != nil {
			return err
		}
	case len(stack) > 0:
		if _, err := fmt.Fprint(w, "\n", strings.Repeat(indent, len(stack)-1)); err != nil {
			return err
		}
	default:
		if _, err := fmt.Fprint(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// marshalJSON works like json.Marshal, but with HTML-escaping disabled.
func marshalJSON(v any) ([]byte, error) {
	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	bb := buf.Bytes()
	// omit trailing newline added by json.Encoder
	if len(bb) > 0 && bb[len(bb)-1] == '\n' {
		return bb[:len(bb)-1], nil
	}
	return bb, nil
}
