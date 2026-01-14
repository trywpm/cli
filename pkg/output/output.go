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

func (o *Output) Prettyln(t Text) {
	if o.out.IsColorEnabled() {
		_, _ = o.out.WriteString(t.Fancy + "\n")
	} else {
		_, _ = o.out.WriteString(t.Plain + "\n")
	}
}

func (o *Output) PrettyErrorln(t Text) {
	if o.err.IsColorEnabled() {
		_, _ = o.err.WriteString(t.Fancy + "\n")
	} else {
		_, _ = o.err.WriteString(t.Plain + "\n")
	}
}

func (o *Output) Write(s string) {
	_, _ = o.out.WriteString(s)
}

func (o *Output) ErrorWrite(s string) {
	_, _ = o.err.WriteString(s)
}
