package commands

import (
	"fmt"
	"os"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/spf13/cobra"
)

type summarizeFlags struct {
	buildpackTarballPath string
	format               string
}

func summarize() *cobra.Command {
	flags := &summarizeFlags{}
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "summarize buildpackage",
		RunE: func(cmd *cobra.Command, args []string) error {
			return summarizeRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.buildpackTarballPath, "buildpack", "", "path to a buildpackage tarball (required)")
	cmd.Flags().StringVar(&flags.format, "format", "markdown", "format of output options are (markdown, json)")

	err := cmd.MarkFlagRequired("buildpack")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark buildpack flag as required")
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(summarize())
}

func summarizeRun(flags summarizeFlags) error {
	buildpackInspector := internal.NewBuildpackInspector()
	formatter := internal.NewFormatter(os.Stdout)
	configs, err := buildpackInspector.Dependencies(flags.buildpackTarballPath)
	if err != nil {
		return fmt.Errorf("failed to inspect buildpack dependencies: %w", err)
	}

	switch flags.format {
	case "markdown":
		formatter.Markdown(configs)
	case "json":
		formatter.JSON(configs)
	default:
		return fmt.Errorf("unknown format %q, please choose from the following formats: markdown, json)", flags.format)
	}

	return nil
}
