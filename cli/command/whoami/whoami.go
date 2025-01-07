package whoami

import (
	"fmt"
	"wpm/cli"
	"wpm/cli/command"
	"wpm/pkg/api"
	"wpm/pkg/config"
	wpmTerm "wpm/pkg/term"

	"github.com/moby/term"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	token string
}

func NewWhoamiCommand(wpmCli command.Cli) *cobra.Command {
	var opts loginOptions

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the current user",
		Args:  cli.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runWhoami(wpmCli) },
	}

	flags := cmd.Flags()

	flags.StringVar(&opts.token, "token", "", "Token to use for authentication")

	return cmd
}

func validateToken(wpmCli command.Cli, token string) (string, error) {
	client, err := api.NewRESTClient(api.ClientOptions{
		Log:         wpmCli.Err(),
		AuthToken:   token,
		EnableCache: true,
		CacheTTL:    300,
		CacheDir:    config.UserAuthCacheDir(),
		Host:        wpmCli.Registry(),
		Headers:     map[string]string{"User-Agent": command.UserAgent()},
		LogColorize: !wpmTerm.IsColorDisabled() && term.IsTerminal(wpmCli.Err().FD()),
	})
	if err != nil {
		return "", err
	}

	response := struct {
		Username string `json:"username"`
	}{}

	err = client.Get("/-/whoami", &response)
	if err != nil {
		return "", err
	}

	return response.Username, nil
}

func runWhoami(wpmCli command.Cli) error {
	cfg := wpmCli.ConfigFile()
	if cfg.AuthToken == "" {
		return errors.New("user must be logged in to perform this action")
	}

	username, err := validateToken(wpmCli, cfg.AuthToken)
	if err != nil {
		return err
	}
	if username == "" {
		return errors.New(aec.RedF.Apply("unable to resolve identity from token"))
	}

	_, _ = fmt.Fprintln(wpmCli.Out(), username)

	return nil
}
