package output

import (
	"io"
)

type Writer interface {
	io.Writer
	WriteString(s string)
	IsColorEnabled() bool
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
		o.out.WriteString(t.Fancy + "\n")
	} else {
		o.out.WriteString(t.Plain + "\n")
	}
}

func (o *Output) PrettyErrorln(t Text) {
	if o.err.IsColorEnabled() {
		o.err.WriteString(t.Fancy + "\n")
	} else {
		o.err.WriteString(t.Plain + "\n")
	}
}

func (o *Output) Write(s string) {
	o.out.WriteString(s)
}

func (o *Output) ErrorWrite(s string) {
	o.err.WriteString(s)
}
