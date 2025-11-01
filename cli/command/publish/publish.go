package publish

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/registry/client"
	"wpm/cli/version"
	"wpm/pkg/archive"
	"wpm/pkg/pm/wpmignore"
	"wpm/pkg/pm/wpmjson"

	"github.com/docker/go-units"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type publishOptions struct {
	dryRun  bool
	verbose bool
	tag     string
	access  string
}

func NewPublishCommand(wpmCli command.Cli) *cobra.Command {
	var opts publishOptions

	cmd := &cobra.Command{
		Use:   "publish [OPTIONS]",
		Short: "Publish a package to the wpm registry",
		Args:  cli.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runPublish(wpmCli, opts) },
	}

	flags := cmd.Flags()

	flags.StringVar(&opts.tag, "tag", "latest", "Set the package tag")
	flags.BoolVar(&opts.verbose, "verbose", false, "Enable verbose output")
	flags.StringVarP(&opts.access, "access", "a", "private", "Set the package access level to either public or private")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Perform a publish operation without actually publishing the package")

	return cmd
}

func pack(stdOut io.Writer, path string, opts publishOptions) (*archive.Tarballer, error) {
	ignorePatterns, err := wpmignore.ReadWpmIgnore(path)
	if err != nil {
		return nil, err
	}

	tar, err := archive.Tar(path, &archive.TarOptions{
		ShowInfo:        opts.verbose,
		ExcludePatterns: ignorePatterns,
	}, stdOut)
	if err != nil {
		return nil, err
	}

	return tar, nil
}

func readme(path string) (string, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.ToLower(file.Name()) == "readme.md" {
			readme, err := os.ReadFile(path + "/" + file.Name())
			if err != nil {
				return "", err
			}

			return string(readme), nil
		}
	}

	return "", nil
}

// tarballSizeCounter is an io.Writer that counts bytes.
type tarballSizeCounter struct {
	total int64
}

func (c *tarballSizeCounter) Write(p []byte) (n int, err error) {
	c.total += int64(len(p))
	return len(p), nil
}

func runPublish(wpmCli command.Cli, opts publishOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	if opts.access != "public" && opts.access != "private" {
		return errors.New("access must be either public or private")
	}

	cfg := wpmCli.ConfigFile()
	if cfg.AuthToken == "" || cfg.DefaultTId == "" {
		return errors.New("user must be logged in to perform this action")
	}

	wpmJson, err := wpmjson.ReadAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	if wpmJson.Private {
		return errors.New("package marked as private cannot be published")
	}

	fmt.Fprintf(wpmCli.Err(), aec.CyanF.Apply("📦 preparing %s@%s for publishing 📦\n\n"), wpmJson.Name, wpmJson.Version)

	tempFile, err := os.CreateTemp("", "wpm-tarball-*.tar.zst")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary tarball")
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	tarballer, err := pack(wpmCli.Err(), cwd, opts)
	if err != nil {
		return errors.Wrap(err, "failed to pack the package into a tarball")
	}

	tarReader := tarballer.Reader()

	if _, err := io.Copy(tempFile, tarReader); err != nil {
		return errors.Wrap(err, "failed to write temporary tarball")
	}

	if err := tempFile.Close(); err != nil {
		return errors.Wrap(err, "failed to close temporary tarball")
	}

	tempTarball, err := os.Open(tempFile.Name())
	if err != nil {
		return errors.Wrap(err, "failed to open tarball for reading")
	}
	defer tempTarball.Close()

	hasher := sha256.New()
	counter := &tarballSizeCounter{}
	teeReader := io.TeeReader(tempTarball, io.MultiWriter(hasher, counter))

	if _, err := io.Copy(io.Discard, teeReader); err != nil {
		return errors.Wrap(err, "failed to process tarball")
	}

	digest := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	if opts.verbose {
		fmt.Fprint(wpmCli.Err(), "\n")
	}

	// bail if tarball size is zero or greater than 128mb
	switch {
	case counter.total == 0:
		return errors.New("tarball size is zero, cannot publish empty package")
	case counter.total > 128*1024*1024:
		return errors.New("tarball size exceeds 128mb, cannot publish package")
	}

	fmt.Fprintf(wpmCli.Err(), "%s: %d\n", aec.LightBlueF.Apply("total files"), tarballer.FileCount())
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("digest"), digest)
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("unpacked size"), units.HumanSize(float64(tarballer.UnpackedSize())))
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("packed size"), units.HumanSize(float64(counter.total)))
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("tag"), opts.tag)
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("access"), opts.access)
	fmt.Fprint(wpmCli.Err(), "\n")

	if opts.dryRun {
		fmt.Fprintf(wpmCli.Err(), "dry run complete, %s@%s is ready to be published\n", wpmJson.Name, wpmJson.Version)
		return nil
	}

	if _, err := tempTarball.Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(err, "failed to seek to the start of the tarball")
	}

	registryClient, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	rawIdempotencyKey := sha256.Sum256([]byte(wpmJson.Name + ":" + wpmJson.Version + ":" + cfg.DefaultTId))
	idempotencyKey := base64.StdEncoding.EncodeToString(rawIdempotencyKey[:])

	var reqId string
	err = wpmCli.Progress().RunWithProgress(
		"uploading tarball to registry",
		func() error {
			resp, err := registryClient.GetUploadTarballUrl(context.TODO(), client.UploadTarballOptions{
				Acl:            opts.access,
				Name:           wpmJson.Name,
				Version:        wpmJson.Version,
				Digest:         digest,
				Type:           wpmJson.Type,
				ContentLength:  counter.total,
				IdempotencyKey: idempotencyKey,
			})
			if err != nil {
				return err
			}
			if resp.Id == "" {
				return errors.New("failed to get request id from upload response")
			}
			reqId = resp.Id

			if resp.Url != "" {
				req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, resp.Url, tempTarball)
				if err != nil {
					return err
				}

				req.ContentLength = counter.total
				req.Header.Set("x-amz-checksum-sha256", digest)
				req.Header.Set("x-amz-meta-request-id", reqId)
				req.Header.Set("x-amz-sdk-checksum-algorithm", "SHA256")
				req.Header.Set("Content-Type", "application/octet-stream")
				req.Header.Set("x-amz-meta-idempotency-key", idempotencyKey)

				client := &http.Client{}
				uploadResp, err := client.Do(req)
				if err != nil {
					return err
				}
				defer uploadResp.Body.Close()

				if uploadResp.StatusCode != http.StatusOK {
					return fmt.Errorf("failed to upload tarball, status code: %d", uploadResp.StatusCode)
				}
			}

			return nil
		},
		wpmCli.Err(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to upload tarball to registry")
	}

	readmeText, err := readme(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to read readme file")
	}

	// trim readme to 100kb with warning
	if len(readmeText) > 100*1024 {
		fmt.Fprint(wpmCli.Err(), aec.YellowF.Apply("⚠️  readme file is larger than 100kb, trimming to 100kb ⚠️ \n"))
		readmeText = readmeText[:100*1024]
	}

	newPackageData := &wpmjson.Package{
		Config: wpmjson.Config{
			Name:            wpmJson.Name,
			Description:     wpmJson.Description,
			Type:            wpmJson.Type,
			Version:         wpmJson.Version,
			Platform:        wpmJson.Platform,
			License:         wpmJson.License,
			Homepage:        wpmJson.Homepage,
			Tags:            wpmJson.Tags,
			Team:            wpmJson.Team,
			Dependencies:    wpmJson.Dependencies,
			DevDependencies: wpmJson.DevDependencies,
		},
		Meta: wpmjson.Meta{
			Dist: wpmjson.Dist{
				Digest:       digest,
				PackedSize:   int64(counter.total),
				TotalFiles:   tarballer.FileCount(),
				UnpackedSize: int64(tarballer.UnpackedSize()),
			},
			Tag:        opts.tag,
			Visibility: opts.access,
			Readme:     readmeText,
			Wpm:        version.Version,
		},
	}

	var message string
	err = wpmCli.Progress().RunWithProgress(
		"publishing package to registry",
		func() error {
			message, err = registryClient.PublishPackage(context.TODO(), newPackageData, client.PublishPackageOptions{
				Name:           wpmJson.Name,
				Version:        wpmJson.Version,
				RequestId:      reqId,
				IdempotencyKey: idempotencyKey,
			})
			return err
		},
		wpmCli.Err(),
	)
	if err != nil {
		return err
	}

	fmt.Fprintf(wpmCli.Err(), "🚀 %s 🚀\n", aec.GreenF.Apply(message))

	return nil
}
