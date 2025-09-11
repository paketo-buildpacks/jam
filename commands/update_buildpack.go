package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
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
			f := cmd.Flags().Lookup("no-cnb-registry")
			if f != nil && f.Changed {
				fmt.Fprintln(os.Stderr, "WARNING: The --no-cnb-registry flag is deprecated and ignored. You can safely remove it.")
			}
			return updateBuildpackRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.buildpackFile, "buildpack-file", "", "path to the buildpack.toml file (required)")
	cmd.Flags().StringVar(&flags.packageFile, "package-file", "", "path to the package.toml file (required)")
	cmd.Flags().StringVar(&flags.api, "api", "https://registry.buildpacks.io/api/", "api for cnb registry (default: https://registry.buildpacks.io/api/)")
	cmd.Flags().BoolVar(&flags.noCNBRegistry, "no-cnb-registry", false, "when false updates dependencies to use cnb-registry uris (DEPRECATED and ignored)")
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

	highestFoundSemverBump := "<none>"
	for i, dependency := range pkg.Dependencies {
		var (
			buildpackageID string
			image          internal.Image
			err            error
			oldVersion     string
		)

		if strings.HasPrefix(dependency.URI, "urn:cnb:registry") {
			if flags.patchOnly {
				oldVersion = strings.Split(dependency.URI, "@")[1]
			}

			image, err = internal.FindLatestImageOnCNBRegistry(dependency.URI, flags.api, oldVersion)
			if err != nil {
				return err
			}

			pkg.Dependencies[i].URI = fmt.Sprintf("%s@%s", image.Name, image.Version)
			buildpackageID = image.Path

		} else {
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
		}

		for j, order := range bp.Order {
			for k, group := range order.Group {
				if group.ID == buildpackageID {
					bump, err := semverBump(group.Version, image.Version)
					if err != nil {
						return err
					}
					highestFoundSemverBump = highestSemverBump(highestFoundSemverBump, bump)

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

	fmt.Printf("Highest semver bump: %s\n", highestFoundSemverBump)

	return nil
}

func semverBump(oldVersion, newVersion string) (string, error) {
	oldSemver, err := semver.StrictNewVersion(oldVersion)
	if err != nil {
		return "", err
	}

	newSemver, err := semver.StrictNewVersion(newVersion)
	if err != nil {
		return "", err
	}

	if newSemver.Major() > oldSemver.Major() {
		return "major", nil
	}

	if newSemver.Minor() > oldSemver.Minor() {
		return "minor", nil
	}

	if newSemver.Patch() > oldSemver.Patch() {
		return "patch", nil
	}

	return "<none>", nil
}

func highestSemverBump(highest, current string) string {
	if highest == "major" {
		return highest
	}

	if highest == "minor" {
		if current == "major" {
			return current
		} else {
			return highest
		}
	}

	if highest == "patch" {
		if current == "major" || current == "minor" {
			return current
		} else {
			return highest
		}
	}

	if highest == "<none>" {
		return current
	}

	return highest
}
