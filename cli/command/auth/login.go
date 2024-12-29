package auth

import (
	"context"
	"fmt"
	"os"
	"wpm/cli"
	"wpm/cli/command"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	token string
}

func NewLoginCommand(wpmCli command.Cli) *cobra.Command {
	var opts loginOptions

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to the wpm registry",
		Args:  cli.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runLogin(cmd.Context(), wpmCli, opts) },
	}

	flags := cmd.Flags()

	flags.StringVar(&opts.token, "token", "", "Token to use for authentication")

	return cmd
}

func verifyLoginOptions(wpmCli command.Cli, opts loginOptions) error {
	isCi := os.Getenv("CI") == "true"

	if opts.token != "" && !isCi {
		_, _ = fmt.Fprintln(wpmCli.Err(), "WARNING! Using --token via the CLI is insecure.")
	}

	return nil
}

func tokenStdinPrompt(ctx context.Context, wpmCli command.Cli, opts *loginOptions) error {
	restoreInput, err := command.DisableInputEcho(wpmCli.In())
	if err != nil {
		return err
	}
	defer func() {
		if err := restoreInput(); err != nil {
			_, _ = fmt.Fprintln(wpmCli.Err(), "Error: failed to restore terminal state to echo input:", err)
		}
	}()

	token, err := command.PromptForInput(ctx, wpmCli.In(), wpmCli.Out(), "Token: ")
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(wpmCli.Out())
	if token == "" {
		return errors.Errorf("Error: token cannot be empty")
	}

	opts.token = token

	return nil
}

func verifyUser(token string) (string, error) {
	return "", nil
}

func runLogin(ctx context.Context, wpmCli command.Cli, opts loginOptions) error {
	if err := verifyLoginOptions(wpmCli, opts); err != nil {
		return err
	}

	if opts.token == "" {
		if err := tokenStdinPrompt(ctx, wpmCli, &opts); err != nil {
			return err
		}
	}

	return nil
}
