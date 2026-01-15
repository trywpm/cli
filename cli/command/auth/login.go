package auth

import (
	"context"
	"fmt"
	"wpm/cli"
	"wpm/cli/command"
	"wpm/pkg/output"

	"github.com/morikuni/aec"
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

func tokenStdinPrompt(ctx context.Context, wpmCli command.Cli, opts *loginOptions) error {
	restoreInput, err := command.DisableInputEcho(wpmCli.In())
	if err != nil {
		return err
	}
	defer func() {
		if err := restoreInput(); err != nil {
			_, _ = fmt.Fprintln(wpmCli.Err(), "failed to restore terminal state to echo input:", err)
		}
	}()

	token, err := command.PromptForInput(ctx, wpmCli.In(), wpmCli.Err(), "Token: ")
	if err != nil {
		return err
	}

	wpmCli.Err().WriteString("\n")

	if token == "" {
		return errors.Errorf("token cannot be empty")
	}

	opts.token = token

	return nil
}

func runLogin(ctx context.Context, wpmCli command.Cli, opts loginOptions) error {
	if opts.token != "" {
		wpmCli.Output().PrettyErrorln(output.Text{
			Plain: "WARNING! Using --token via the CLI is insecure.",
			Fancy: aec.RedF.Apply("WARNING! Using --token via the CLI is insecure."),
		})
	}

	if opts.token == "" && wpmCli.In().IsTerminal() {
		if err := tokenStdinPrompt(ctx, wpmCli, &opts); err != nil {
			return err
		}
	}

	client, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	var username string
	err = wpmCli.Progress().RunWithProgress(
		"validating token",
		func() error {
			var err error
			username, err = client.Whoami(ctx, opts.token)
			return err
		},
		wpmCli.Err(),
	)
	if err != nil {
		return err
	}

	if username == "" {
		return errors.New("failed to retrieve username")
	}

	cfg := wpmCli.ConfigFile()
	cfg.AuthToken = opts.token
	cfg.DefaultUser = username

	if err := cfg.Save(); err != nil {
		return err
	}

	wpmCli.Output().Prettyln(output.Text{
		Plain: fmt.Sprintf("welcome %s!", username),
		Fancy: aec.GreenF.Apply(fmt.Sprintf("welcome %s!", username)),
	})

	return nil
}
