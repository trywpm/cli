package streams

import (
	"io"
	"os"

	"github.com/moby/term"
)

// In is an input stream to read user input. It implements [io.ReadCloser]
// with additional utilities, such as putting the terminal in raw mode.
type In struct {
	commonStream
	in io.ReadCloser
}

// Read implements the [io.Reader] interface.
func (i *In) Read(p []byte) (int, error) {
	return i.in.Read(p)
}

// Close implements the [io.Closer] interface.
func (i *In) Close() error {
	return i.in.Close()
}

// SetRawTerminal sets raw mode on the input terminal. It is a no-op if In
// is not a TTY, or if the "NORAW" environment variable is set to a non-empty
// value.
func (i *In) SetRawTerminal() (err error) {
	if !i.isTerminal || os.Getenv("NORAW") != "" {
		return nil
	}
	i.state, err = term.SetRawTerminal(i.fd)
	return err
}

// NewIn returns a new [In] from an [io.ReadCloser].
func NewIn(in io.ReadCloser) *In {
	i := &In{in: in}
	i.fd, i.isTerminal = term.GetFdInfo(in)
	return i
}
