package commands

import (
	"fmt"
	"os"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/spf13/cobra"
)

type updateBuilderFlags struct {
	builderFile  string
	lifecycleURI string
}

func updateBuilder() *cobra.Command {
	flags := &updateBuilderFlags{}
	cmd := &cobra.Command{
		Use:   "update-builder",
		Short: "update builder",
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateBuilderRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.builderFile, "builder-file", "", "path to the builder.toml file (required)")
	cmd.Flags().StringVar(&flags.lifecycleURI, "lifecycle-uri", "index.docker.io/buildpacksio/lifecycle", "URI for lifecycle image (optional: default=index.docker.io/buildpacksio/lifecycle)")

	err := cmd.MarkFlagRequired("builder-file")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark builder-file flag as required")
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(updateBuilder())
}

func updateBuilderRun(flags updateBuilderFlags) error {
	builder, err := internal.ParseBuilderConfig(flags.builderFile)
	if err != nil {
		return err
	}

	for i, buildpack := range builder.Buildpacks {
		image, err := internal.FindLatestImage(buildpack.URI)
		if err != nil {
			return err
		}

		builder.Buildpacks[i].Version = image.Version
		builder.Buildpacks[i].URI = fmt.Sprintf("%s:%s", image.Name, image.Version)

		buildpackageID, err := internal.GetBuildpackageID(buildpack.URI)
		if err != nil {
			return fmt.Errorf("failed to get buildpackage ID for %s: %w", buildpack.URI, err)
		}

		for j, order := range builder.Order {
			for k, group := range order.Group {
				if group.ID == buildpackageID {
					if builder.Order[j].Group[k].Version != "" {
						builder.Order[j].Group[k].Version = image.Version
					}
				}
			}
		}
	}

	for i, extension := range builder.Extensions {
		image, err := internal.FindLatestImage(extension.URI)
		if err != nil {
			return err
		}

		builder.Extensions[i].Version = image.Version
		builder.Extensions[i].URI = fmt.Sprintf("%s:%s", image.Name, image.Version)

		extensionID, err := internal.GetBuildpackageID(extension.URI)
		if err != nil {
			return fmt.Errorf("failed to get extension ID for %s: %w", extension.URI, err)
		}

		for j, orderextensions := range builder.OrderExtension {
			for k, group := range orderextensions.Group {
				if group.ID == extensionID {
					if builder.OrderExtension[j].Group[k].Version != "" {
						builder.OrderExtension[j].Group[k].Version = image.Version
					}
				}
			}
		}
	}

	lifecycleImage, err := internal.FindLatestImage(flags.lifecycleURI)
	if err != nil {
		return err
	}

	builder.Lifecycle.Version = lifecycleImage.Version

	buildImage, err := internal.FindLatestBuildImage(builder.Stack.RunImage, builder.Stack.BuildImage)
	if err != nil {
		return err
	}

	builder.Stack.BuildImage = fmt.Sprintf("%s:%s", buildImage.Name, buildImage.Version)

	err = internal.OverwriteBuilderConfig(flags.builderFile, builder)
	if err != nil {
		return err
	}

	return nil
}
