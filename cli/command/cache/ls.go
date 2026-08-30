package cache

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
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

type blobInfo struct {
	digest  string
	size    int64
	modTime time.Time
}

func runLs(wpmCli command.Cli) error {
	var blobs []blobInfo
	blobRoot := filepath.Join(config.ContentCacheDir(), "sha256")
	_ = filepath.WalkDir(blobRoot, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		blobs = append(blobs, blobInfo{digest: d.Name(), size: info.Size(), modTime: info.ModTime()})
		return nil
	})

	slices.SortFunc(blobs, func(a, b blobInfo) int {
		return b.modTime.Compare(a.modTime)
	})

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 12, 1, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tDIGEST\tSIZE\tCREATED")
	for _, b := range blobs {
		names := cas.Refs(config.ContentCacheDir(), b.digest)
		if len(names) == 0 {
			names = []string{"<none>"}
		}
		for _, name := range names {
			_, _ = fmt.Fprintf(w, "%s\t%.12s\t%s\t%s\n",
				name, b.digest, units.HumanSize(float64(b.size)), units.HumanDuration(time.Since(b.modTime))+" ago")
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
