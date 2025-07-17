package commands

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(publishStack())
}

type publishStackFlags struct {
	buildArchive   string
	buildReference string
	runArchive     string
	runReference   string
}

func publishStack() *cobra.Command {
	flags := &publishStackFlags{}
	cmd := &cobra.Command{
		Use:   "publish-stack",
		Short: "publish-stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return publishStackRun(*flags)
		},
	}

	cmd.Flags().StringVar(&flags.buildArchive, "build-archive", "", "path to the build image OCI archive (required)")
	cmd.Flags().StringVar(&flags.buildReference, "build-ref", "", "reference that specifies where to publish the build image (required)")
	cmd.Flags().StringVar(&flags.runArchive, "run-archive", "", "path to the run image OCI archive (required)")
	cmd.Flags().StringVar(&flags.runReference, "run-ref", "", "reference that specifies where to publish the run image (required)")

	err := cmd.MarkFlagRequired("build-archive")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark build-archive flag as required")
	}

	err = cmd.MarkFlagRequired("build-ref")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark build-ref flag as required")
	}

	err = cmd.MarkFlagRequired("run-archive")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark run-archive flag as required")
	}

	err = cmd.MarkFlagRequired("run-ref")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark run-ref flag as required")
	}

	return cmd
}

func publishStackRun(flags publishStackFlags) error {
	logger := scribe.NewLogger(os.Stdout)

	scratch, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer func() {
		if err2 := os.RemoveAll(scratch); err2 != nil && err == nil {
			err = err2
		}
	}()

	tmpBuild := filepath.Join(scratch, "build")
	err = extractTar(flags.buildArchive, tmpBuild)
	if err != nil {
		return err
	}

	tmpRun := filepath.Join(scratch, "run")
	err = extractTar(flags.runArchive, tmpRun)
	if err != nil {
		return err
	}

	client, err := ihop.NewClient(scratch)
	if err != nil {
		return err
	}

	logger.Process("Uploading build image to %s", flags.buildReference)
	err = client.Upload(flags.buildReference, tmpBuild)
	if err != nil {
		return err
	}

	logger.Process("Uploading run image to %s", flags.runReference)
	err = client.Upload(flags.runReference, tmpRun)
	if err != nil {
		return err
	}

	return err // err should be nil here, but return err to catch deferred error
}

func extractTar(input string, destination string) error {
	source, err := os.Open(input)
	if err != nil {
		return err
	}

	t := tar.NewReader(source)

	for {
		header, err := t.Next()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("entry contains unsafe relative link")
		}
		target := filepath.Join(destination, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeLink:
			symlinkPath := header.Linkname
			if !filepath.IsAbs(symlinkPath) {
				realSymlinkPath, err := filepath.EvalSymlinks(filepath.Join(target, symlinkPath))
				if err != nil {
					return err
				}

				symlinkPath, err := filepath.Rel(target, realSymlinkPath)
				if err != nil {
					return err
				}

				if strings.HasPrefix(filepath.Clean(symlinkPath), "..") {
					return fmt.Errorf("unsafe relative symlink")
				}
			}
			if err := os.Symlink(symlinkPath, target); err != nil {
				return err
			}
		case tar.TypeReg:
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(out, t); err != nil {
				return err
			}

			if err := out.Close(); err != nil {
				return err
			}
		}
	}
}
