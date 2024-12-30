package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"wpm/cli/streams"

	"github.com/docker/docker/errdefs"
	"github.com/moby/term"
)

var ErrPromptTerminated = errdefs.Cancelled(errors.New("prompt terminated"))

// DisableInputEcho disables input echo on the provided streams.In.
// This is useful when the user provides sensitive information like passwords.
// The function returns a restore function that should be called to restore the
// terminal state.
func DisableInputEcho(ins *streams.In) (restore func() error, err error) {
	oldState, err := term.SaveState(ins.FD())
	if err != nil {
		return nil, err
	}
	restore = func() error {
		return term.RestoreTerminal(ins.FD(), oldState)
	}
	return restore, term.DisableEcho(ins.FD(), oldState)
}

// PromptForInput requests input from the user.
//
// If the user terminates the CLI with SIGINT or SIGTERM while the prompt is
// active, the prompt will return an empty string ("") with an ErrPromptTerminated error.
// When the prompt returns an error, the caller should propagate the error up
// the stack and close the io.Reader used for the prompt which will prevent the
// background goroutine from blocking indefinitely.
func PromptForInput(ctx context.Context, in io.Reader, out io.Writer, message string) (string, error) {
	_, _ = fmt.Fprint(out, message)

	result := make(chan string)
	go func() {
		scanner := bufio.NewScanner(in)
		if scanner.Scan() {
			result <- strings.TrimSpace(scanner.Text())
		}
	}()

	select {
	case <-ctx.Done():
		_, _ = fmt.Fprintln(out, "")
		return "", ErrPromptTerminated
	case r := <-result:
		return r, nil
	}
}

// PromptForConfirmation requests and checks confirmation from the user.
// This will display the provided message followed by ' [y/N] '. If the user
// input 'y' or 'Y' it returns true otherwise false. If no message is provided,
// "Are you sure you want to proceed? [y/N] " will be used instead.
//
// If the user terminates the CLI with SIGINT or SIGTERM while the prompt is
// active, the prompt will return false with an ErrPromptTerminated error.
// When the prompt returns an error, the caller should propagate the error up
// the stack and close the io.Reader used for the prompt which will prevent the
// background goroutine from blocking indefinitely.
func PromptForConfirmation(ctx context.Context, ins io.Reader, outs io.Writer, message string) (bool, error) {
	if message == "" {
		message = "Are you sure you want to proceed?"
	}
	message += " [y/N] "

	_, _ = fmt.Fprint(outs, message)

	// On Windows, force the use of the regular OS stdin stream.
	if runtime.GOOS == "windows" {
		ins = streams.NewIn(os.Stdin)
	}

	result := make(chan bool)

	go func() {
		var res bool
		scanner := bufio.NewScanner(ins)
		if scanner.Scan() {
			answer := strings.TrimSpace(scanner.Text())
			if strings.EqualFold(answer, "y") {
				res = true
			}
		}
		result <- res
	}()

	select {
	case <-ctx.Done():
		_, _ = fmt.Fprintln(outs, "")
		return false, ErrPromptTerminated
	case r := <-result:
		return r, nil
	}
}
