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
		image, err := internal.FindLatestImage(buildpack.URI, "")
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
		image, err := internal.FindLatestImage(extension.URI, "")
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

	if builder.Stack.RunImage == "" && len(builder.Run.Images) == 0 {
		return fmt.Errorf("run image is not specified in the builder")
	}

	if builder.Stack.BuildImage == "" && builder.Build.Image == "" {
		return fmt.Errorf("build image is not specified in the builder")
	}

	if builder.Stack.RunImage != "" && len(builder.Run.Images) > 1 {
		return fmt.Errorf("run image mismatch: builder stack run image is set to '%s' but multiple run images are specified in the builder", builder.Stack.RunImage)
	}

	if builder.Stack.RunImage != "" && (len(builder.Run.Images) > 0 && builder.Run.Images[0].Image != "") && builder.Stack.RunImage != builder.Run.Images[0].Image {
		return fmt.Errorf("run image mismatch: builder stack run image is set to '%s' but run image is set to '%s'", builder.Stack.RunImage, builder.Run.Images[0].Image)
	}

	if builder.Stack.BuildImage != "" && builder.Build.Image != "" && builder.Stack.BuildImage != builder.Build.Image {
		return fmt.Errorf("build image mismatch: builder stack build image is set to '%s' but build image is set to '%s'", builder.Stack.BuildImage, builder.Build.Image)
	}

	if (builder.Stack.BuildImage == "" && builder.Stack.RunImage != "") || (builder.Stack.BuildImage != "" && builder.Stack.RunImage == "") {
		return fmt.Errorf("both build and run images must be specified in the builder")
	}

	if (builder.Build.Image != "" && (len(builder.Run.Images) > 0 && builder.Run.Images[0].Image == "")) || (builder.Build.Image == "" && (len(builder.Run.Images) > 0 && builder.Run.Images[0].Image != "")) {
		return fmt.Errorf("both build and run images must be specified in the builder")
	}

	currentBuildImage := builder.Stack.BuildImage
	if currentBuildImage == "" {
		currentBuildImage = builder.Build.Image
	}

	currentRunImages := []string{}
	if builder.Stack.RunImage == "" {
		for _, img := range builder.Run.Images {
			currentRunImages = append(currentRunImages, img.Image)
		}
	} else {
		currentRunImages = append(currentRunImages, builder.Stack.RunImage)

	}

	updatedRunImages := []internal.ImageRegistry{}
	for _, currentRunImg := range currentRunImages {
		runImage, buildImage, err := internal.FindLatestStackImages(currentRunImg, currentBuildImage)
		if err != nil {
			return err
		}

		if builder.Stack.BuildImage != "" {
			builder.Stack.BuildImage = fmt.Sprintf("%s:%s", buildImage.Name, buildImage.Version)
		}

		if builder.Build.Image != "" {
			builder.Build.Image = fmt.Sprintf("%s:%s", buildImage.Name, buildImage.Version)
		}

		if runImage != (internal.Image{}) {
			if builder.Stack.RunImage != "" {
				builder.Stack.RunImage = fmt.Sprintf("%s:%s", runImage.Name, runImage.Version)
			}

			if len(builder.Run.Images) > 0 {
				updatedRunImages = append(updatedRunImages, internal.ImageRegistry{
					Image: fmt.Sprintf("%s:%s", runImage.Name, runImage.Version),
				})
			}

			updatedMirrors, err := internal.UpdateRunImageMirrors(runImage.Version, builder.Stack.RunImageMirrors)
			if err != nil {
				return err
			}
			builder.Stack.RunImageMirrors = updatedMirrors
		} else {
			if len(builder.Run.Images) > 0 {
				updatedRunImages = append(updatedRunImages, internal.ImageRegistry{
					Image: currentRunImg,
				})
			}
		}
	}

	if len(builder.Run.Images) > 0 {
		builder.Run.Images = updatedRunImages
	}

	lifecycleImage, err := internal.FindLatestImage(flags.lifecycleURI, "")
	if err != nil {
		return err
	}

	builder.Lifecycle.Version = lifecycleImage.Version

	err = internal.OverwriteBuilderConfig(flags.builderFile, builder)
	if err != nil {
		return err
	}

	return nil
}
