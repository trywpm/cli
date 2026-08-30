package cache

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docker/go-units"
	"github.com/morikuni/aec"
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/pkg/config"
	"go.wpm.so/cli/pkg/pm/cas"
)

func newLsCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the cached content",
		Args:  cli.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runLs(wpmCli) },
	}

	return cmd
}

func runLs(wpmCli command.Cli) error {
	blobs := cas.Blobs(config.ContentCacheDir())

	slices.SortFunc(blobs, func(a, b cas.BlobInfo) int {
		if c := b.ModTime.Compare(a.ModTime); c != 0 {
			return c
		}
		return cmp.Compare(a.Digest, b.Digest)
	})

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 12, 1, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tDIGEST\tSIZE\tCREATED")
	for _, b := range blobs {
		names := cas.Refs(config.ContentCacheDir(), b.Digest)
		if len(names) == 0 {
			names = []string{"<none>"}
		}
		for _, name := range names {
			_, _ = fmt.Fprintf(w, "%s\t%.12s\t%s\t%s\n",
				name, b.Digest.Encoded(), units.HumanSize(float64(b.Size)), units.HumanDuration(time.Since(b.ModTime))+" ago")
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}

	table := buf.String()
	if wpmCli.Out().IsColorEnabled() {
		header, rows, _ := strings.Cut(table, "\n")
		table = aec.Bold.Apply(header) + "\n" + rows
	}
	wpmCli.Out().WriteString(table)

	return nil
}
