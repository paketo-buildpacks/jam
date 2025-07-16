package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/spf13/cobra"
)

type updateDependenciesFlags struct {
	buildpackFile string
	metadataFile  string
}

func updateDependencies() *cobra.Command {
	flags := &updateDependenciesFlags{}
	cmd := &cobra.Command{
		Use:   "update-dependencies",
		Short: "updates all depdendencies in a buildpack.toml from a metadata JSON file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateDependenciesRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.buildpackFile, "buildpack-file", "", "path to the buildpack.toml file (required)")
	cmd.Flags().StringVar(&flags.metadataFile, "metadata-file", "", "metadata.json file with all entries to be added to the buildpack.toml (required)")

	err := cmd.MarkFlagRequired("buildpack-file")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark buildpack-file flag as required")
	}
	err = cmd.MarkFlagRequired("metadata-file")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark metadata-file flag as required")
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(updateDependencies())
}

func updateDependenciesRun(flags updateDependenciesFlags) error {
	configParser := cargo.NewBuildpackParser()
	config, err := configParser.Parse(flags.buildpackFile)
	if err != nil {
		return fmt.Errorf("failed to parse buildpack.toml: %s", err)
	}

	originalVersions := map[string]string{}
	for _, d := range config.Metadata.Dependencies {
		originalVersions[d.Version] = d.Version
	}

	var matchingDependencies []cargo.ConfigMetadataDependency

	metadataFile, err := os.Open(flags.metadataFile)
	if err != nil {
		return fmt.Errorf("failed to open metadata.json file: %w", err)
	}

	newVersions := []cargo.ConfigMetadataDependency{}
	err = json.NewDecoder(metadataFile).Decode(&newVersions)
	if err != nil {
		return fmt.Errorf("failed decode metadata.json: %w", err)
	}
	err = metadataFile.Close()
	if err != nil {
		//untested
		return fmt.Errorf("failed close metadata.json: %w", err)
	}

	// combine buildpack.toml versions and new versions
	allDependencies := append(config.Metadata.Dependencies, newVersions...)

	for _, constraint := range config.Metadata.DependencyConstraints {
		// Filter allDependencies for only those that match the constraint
		// mds is a just the right number of deps for the constraint
		mds, err := internal.GetCargoDependenciesWithinConstraint(allDependencies, constraint)
		if err != nil {
			return err
		}

		matchingDependencies = append(matchingDependencies, mds...)
		if len(matchingDependencies) > 0 {
			config.Metadata.Dependencies = matchingDependencies
		}
	}

	newVersionsFound := map[string]string{}
	for _, d := range config.Metadata.Dependencies {
		if _, ok := originalVersions[d.Version]; !ok {
			newVersionsFound[d.Version] = ""
		}
	}

	file, err := os.OpenFile(flags.buildpackFile, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open buildpack config file: %w", err)
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	err = cargo.EncodeConfig(file, config)
	if err != nil {
		return fmt.Errorf("failed to write buildpack config: %w", err)
	}

	fmt.Println("Updating buildpack.toml with new versions: ", reflect.ValueOf(newVersionsFound).MapKeys())

	return err // err should be nil here, but return err to catch deferred error
}
