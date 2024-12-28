package init

import "github.com/spf13/cobra"

func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new WordPress project",
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := runInit(cmd, args)
			if err != nil {
				return err
			}
			cmd.Println(output)
			return nil
		},
	}

	return cmd
}

func runInit(cmd *cobra.Command, args []string) (output string, err error) {
	return "Initializing WordPress project", nil
}
