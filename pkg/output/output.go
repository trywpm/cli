package output

import (
	"io"
)

type Writer interface {
	io.Writer
	IsColorEnabled() bool
	WriteString(s string) (int, error)
}

type Output struct {
	out Writer
	err Writer
}

func New(out, err Writer) *Output {
	return &Output{
		out: out,
		err: err,
	}
}

type Text struct {
	Plain string
	Fancy string
}

func (o *Output) Prettyln(t Text) (int, error) {
	if o.out.IsColorEnabled() {
		return o.out.WriteString(t.Fancy + "\n")
	}
	return o.out.WriteString(t.Plain + "\n")
}

func (o *Output) PrettyErrorln(t Text) (int, error) {
	if o.err.IsColorEnabled() {
		return o.err.WriteString(t.Fancy + "\n")
	}
	return o.err.WriteString(t.Plain + "\n")
}
