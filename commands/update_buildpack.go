package commands

import (
	"fmt"
	"os"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/spf13/cobra"
)

func updateBuildpack() *cobra.Command {
	flags := &internal.UpdateBuildpackFlags{}
	cmd := &cobra.Command{
		Use:   "update-buildpack",
		Short: "update buildpack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internal.UpdateBuildpackRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.BuildpackFile, "buildpack-file", "", "path to the buildpack.toml file (required)")
	cmd.Flags().StringVar(&flags.PackageFile, "package-file", "", "path to the package.toml file (required)")
	cmd.Flags().StringVar(&flags.API, "api", "https://registry.buildpacks.io/api/", "api for cnb registry (default: https://registry.buildpacks.io/api/)")
	cmd.Flags().BoolVar(&flags.NoCNBRegistry, "no-cnb-registry", false, "buildpacks not available on the CNB registry, so will revert to previous image references behavior")
	cmd.Flags().BoolVar(&flags.PatchOnly, "patch-only", false, "allow patch changes ONLY to buildpack version bumps")

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
