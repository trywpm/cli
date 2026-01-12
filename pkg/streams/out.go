package streams

import (
	"fmt"
	"io"
	"os"

	"github.com/moby/term"
	"github.com/morikuni/aec"
	"github.com/sirupsen/logrus"
)

// Out is an output stream to write normal program output. It implements
// an [io.Writer], with additional utilities for detecting whether a terminal
// is connected, getting the TTY size, and putting the terminal in raw mode.
type Out struct {
	commonStream
	out         io.Writer
	enableColor bool
}

func (o *Out) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

func (o *Out) IsColorEnabled() bool {
	return o.enableColor
}

// SetRawTerminal puts the output of the terminal connected to the stream
// into raw mode.
//
// On UNIX, this does nothing. On Windows, it disables LF -> CRLF/ translation.
// It is a no-op if Out is not a TTY, or if the "NORAW" environment variable is
// set to a non-empty value.
func (o *Out) SetRawTerminal() (err error) {
	if !o.isTerminal || os.Getenv("NORAW") != "" {
		return nil
	}
	o.state, err = term.SetRawTerminalOutput(o.fd)
	return err
}

// GetTtySize returns the height and width in characters of the TTY, or
// zero for both if no TTY is connected.
func (o *Out) GetTtySize() (height uint, width uint) {
	if !o.isTerminal {
		return 0, 0
	}
	ws, err := term.GetWinsize(o.fd)
	if err != nil {
		logrus.WithError(err).Debug("Error getting TTY size")
		if ws == nil {
			return 0, 0
		}
	}
	return uint(ws.Height), uint(ws.Width)
}

// NewOut returns a new [Out] from an [io.Writer].
func NewOut(out io.Writer) *Out {
	o := &Out{out: out}
	o.fd, o.isTerminal = term.GetFdInfo(out)
	o.enableColor = hasColors(o.isTerminal)
	return o
}

func hasColors(isTerminal bool) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	force := os.Getenv("CLICOLOR_FORCE")
	if force != "" && force != "0" {
		return true
	}

	if os.Getenv("CLICOLOR") == "0" {
		return false
	}

	return isTerminal
}

func (o *Out) With(styles ...aec.ANSI) *StyledOut {
	return &StyledOut{
		parent: o,
		styles: styles,
	}
}

type StyledOut struct {
	parent *Out
	styles []aec.ANSI
}

func (s *StyledOut) apply(msg string) string {
	if len(s.styles) == 0 {
		return msg
	}

	combined := s.styles[0]
	for _, next := range s.styles[1:] {
		combined = combined.With(next)
	}

	return combined.Apply(msg)
}

func (s *StyledOut) Println(a ...any) {
	msg := fmt.Sprint(a...)

	if s.parent.enableColor {
		msg = s.apply(msg)
	}

	fmt.Fprintln(s.parent.out, msg)
}

func (s *StyledOut) Print(a ...any) {
	msg := fmt.Sprint(a...)

	if s.parent.enableColor {
		msg = s.apply(msg)
	}

	fmt.Fprint(s.parent.out, msg)
}

func (s *StyledOut) Printf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)

	if s.parent.enableColor {
		msg = s.apply(msg)
	}

	fmt.Fprint(s.parent.out, msg)
}
