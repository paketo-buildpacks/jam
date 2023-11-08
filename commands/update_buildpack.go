package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/spf13/cobra"
)

type updateBuildpackFlags struct {
	buildpackFile string
	packageFile   string
	api           string

	noCNBRegistry bool
	patchOnly     bool
}

func updateBuildpack() *cobra.Command {
	flags := &updateBuildpackFlags{}
	cmd := &cobra.Command{
		Use:   "update-buildpack",
		Short: "update buildpack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateBuildpackRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.buildpackFile, "buildpack-file", "", "path to the buildpack.toml file (required)")
	cmd.Flags().StringVar(&flags.packageFile, "package-file", "", "path to the package.toml file (required)")
	cmd.Flags().StringVar(&flags.api, "api", "https://registry.buildpacks.io/api/", "api for cnb registry (default: https://registry.buildpacks.io/api/)")
	cmd.Flags().BoolVar(&flags.noCNBRegistry, "no-cnb-registry", false, "buildpacks not available on the CNB registry, so will revert to previous image references behavior")
	cmd.Flags().BoolVar(&flags.patchOnly, "patch-only", false, "allow patch changes ONLY to buildpack version bumps")

	err := cmd.MarkFlagRequired("buildpack-file")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark buildpack-file flag as required")
	}
	err = cmd.MarkFlagRequired("package-file")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark package-file flag as required")
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(updateBuildpack())
}

func updateBuildpackRun(flags updateBuildpackFlags) error {
	bp, err := internal.ParseBuildpackConfig(flags.buildpackFile)
	if err != nil {
		return err
	}

	pkg, err := internal.ParsePackageConfig(flags.packageFile)
	if err != nil {
		return err
	}

	for i, dependency := range pkg.Dependencies {
		var (
			buildpackageID string
			image          internal.Image
			err            error
		)

		if flags.noCNBRegistry {
			// If --patch-only is set, retrieve new version in the same version line as previous version, if it exists
			oldVersion := ""
			if flags.patchOnly {
				oldVersionSlice := strings.Split(dependency.URI, ":")
				oldVersion = oldVersionSlice[len(oldVersionSlice)-1]
			}

			image, err = internal.FindLatestImage(dependency.URI, oldVersion)
			if err != nil {
				return err
			}
			pkg.Dependencies[i].URI = fmt.Sprintf("%s:%s", image.Name, image.Version)

			buildpackageID, err = internal.GetBuildpackageID(dependency.URI)
			if err != nil {
				return fmt.Errorf("failed to get buildpackage ID for %s: %w", dependency.URI, err)
			}
		} else {
			uri := dependency.URI
			if !strings.HasPrefix(dependency.URI, "urn:cnb:registry") {
				uri, err = internal.GetBuildpackageID(dependency.URI)
				if err != nil {
					return fmt.Errorf("failed to get buildpackage ID for %s: %w", dependency.URI, err)
				}
			}

			// If --patch-only is set, retrieve new version in the same version line as previous version, if it exists
			oldVersion := ""
			if flags.patchOnly {
				if strings.Contains(dependency.URI, "@") {
					oldVersion = strings.Split(dependency.URI, "@")[1]
				} else {
					oldVersionSlice := strings.Split(dependency.URI, ":")
					oldVersion = oldVersionSlice[len(oldVersionSlice)-1]
				}
			}
			image, err = internal.FindLatestImageOnCNBRegistry(uri, flags.api, oldVersion)
			if err != nil {
				return err
			}

			pkg.Dependencies[i].URI = fmt.Sprintf("%s@%s", image.Name, image.Version)
			buildpackageID = image.Path
		}
		for j, order := range bp.Order {
			for k, group := range order.Group {
				if group.ID == buildpackageID {
					bp.Order[j].Group[k].Version = image.Version
				}
			}
		}
	}

	err = internal.OverwriteBuildpackConfig(flags.buildpackFile, bp)
	if err != nil {
		return err
	}

	err = internal.OverwritePackageConfig(flags.packageFile, pkg)
	if err != nil {
		return err
	}

	return nil
}
