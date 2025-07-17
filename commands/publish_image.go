package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(publishImage())
}

type publishImageFlags struct {
	imageArchive   string
	imageReference string
}

func publishImage() *cobra.Command {
	flags := &publishImageFlags{}
	cmd := &cobra.Command{
		Use:   "publish-image",
		Short: "publish-image",
		RunE: func(cmd *cobra.Command, args []string) error {
			return publishImageRun(*flags)
		},
	}

	cmd.Flags().StringVar(&flags.imageArchive, "image-archive", "", "path to the OCI image archive (required)")
	cmd.Flags().StringVar(&flags.imageReference, "image-ref", "", "reference that specifies where to publish the image (required)")

	err := cmd.MarkFlagRequired("image-archive")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark image-archive flag as required")
	}

	err = cmd.MarkFlagRequired("image-ref")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark image-ref flag as required")
	}

	return cmd
}

func publishImageRun(flags publishImageFlags) error {
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

	tmpExtractedImage := filepath.Join(scratch, "extracted_image")
	err = extractTar(flags.imageArchive, tmpExtractedImage)
	if err != nil {
		return err
	}

	client, err := ihop.NewClient(scratch)
	if err != nil {
		return err
	}

	logger.Process("Uploading image to %s", flags.imageReference)
	err = client.Upload(flags.imageReference, tmpExtractedImage)
	if err != nil {
		return err
	}

	return err // err should be nil here, but return err to catch deferred error
}
