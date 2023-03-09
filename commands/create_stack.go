package commands

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(createStack())
}

type createStackFlags struct {
	config      string
	buildOutput string
	runOutput   string
	secrets     []string
	unbuffered  bool
}

func createStack() *cobra.Command {
	flags := &createStackFlags{}
	cmd := &cobra.Command{
		Use:   "create-stack",
		Short: "create-stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createStackRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.config, "config", "", "path to a stack descriptor file (required)")
	cmd.Flags().StringVar(&flags.buildOutput, "build-output", "", "path to output the build image OCI archive (required)")
	cmd.Flags().StringVar(&flags.runOutput, "run-output", "", "path to output the run image OCI archive (required)")
	cmd.Flags().StringSliceVar(&flags.secrets, "secret", nil, "secret to be passed to your Dockerfile")
	cmd.Flags().BoolVar(&flags.unbuffered, "unbuffered", false, "do not buffer image contents into memory for fast access")

	err := cmd.MarkFlagRequired("config")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark config flag as required")
	}

	err = cmd.MarkFlagRequired("build-output")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark build-output flag as required")
	}

	err = cmd.MarkFlagRequired("run-output")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark run-output flag as required")
	}

	return cmd
}

func createStackRun(flags createStackFlags) error {
	logger := scribe.NewLogger(os.Stdout)

	if flags.unbuffered {
		logger.Process("WARNING: The --unbuffered flag is deprecated. You can safely remove it.")
	}

	definition, err := ihop.NewDefinitionFromFile(flags.config, flags.secrets...)
	if err != nil {
		return err
	}

	_, definition.IncludeExperimentalSBOM = os.LookupEnv("EXPERIMENTAL_ATTACH_RUN_IMAGE_SBOM")

	scratch, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)

	client, err := ihop.NewClient(scratch)
	if err != nil {
		return err
	}

	builder := ihop.NewBuilder(client, ihop.Cataloger{}, runtime.NumCPU())
	creator := ihop.NewCreator(client, builder, ihop.UserLayerCreator{}, ihop.SBOMLayerCreator{}, ihop.OsReleaseLayerCreator{Def: definition}, time.Now, logger)

	stack, err := creator.Execute(definition)
	if err != nil {
		return err
	}

	logger.Process("Exporting build image to %s", flags.buildOutput)
	err = client.Export(flags.buildOutput, stack.Build...)
	if err != nil {
		return err
	}

	logger.Process("Exporting run image to %s", flags.runOutput)
	err = client.Export(flags.runOutput, stack.Run...)
	if err != nil {
		return err
	}

	return nil
}
