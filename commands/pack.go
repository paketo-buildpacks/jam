package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/spf13/cobra"
)

type packFlags struct {
	buildpackTOMLPath string
	extensionTOMLPath string
	output            string
	version           string
	offline           bool
	stack             string
}

func pack() *cobra.Command {
	flags := &packFlags{}
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "package buildpack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return packRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.buildpackTOMLPath, "buildpack", "", "path to buildpack.toml")
	cmd.Flags().StringVar(&flags.extensionTOMLPath, "extension", "", "path to extension.toml")
	cmd.Flags().StringVar(&flags.output, "output", "", "path to location of output tarball")
	cmd.Flags().StringVar(&flags.version, "version", "", "version of the buildpack")
	cmd.Flags().BoolVar(&flags.offline, "offline", false, "enable offline caching of dependencies")
	cmd.Flags().StringVar(&flags.stack, "stack", "", "restricts dependencies to given stack")

	cmd.MarkFlagsMutuallyExclusive("buildpack", "extension")

	err := cmd.MarkFlagRequired("output")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark output flag as required")
	}
	err = cmd.MarkFlagRequired("version")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to mark version flag as required")
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(pack())
}

func packRun(flags packFlags) error {

	if flags.buildpackTOMLPath == "" && flags.extensionTOMLPath == "" {
		return fmt.Errorf(`"buildpack" or "extension" flag is required`)
	}

	tmpDir, err := os.MkdirTemp("", "dup-dest")
	if err != nil {
		return fmt.Errorf("unable to create temporary directory: %s", err)
	}
	defer func() {
		if err2 := os.RemoveAll(tmpDir); err2 != nil && err == nil {
			err = err2
		}
	}()

	if flags.extensionTOMLPath != "" {
		err := packRunExtension(flags, tmpDir)
		if err != nil {
			return fmt.Errorf("failed to pack extension: %s", err)
		}
		return nil
	}

	directoryDuplicator := cargo.NewDirectoryDuplicator()
	err = directoryDuplicator.Duplicate(filepath.Dir(flags.buildpackTOMLPath), tmpDir)
	if err != nil {
		return fmt.Errorf("failed to duplicate directory: %s", err)
	}

	buildpackTOMLPath := filepath.Join(tmpDir, filepath.Base(flags.buildpackTOMLPath))

	configParser := cargo.NewBuildpackParser()
	config, err := configParser.Parse(buildpackTOMLPath)
	if err != nil {
		return fmt.Errorf("failed to parse buildpack.toml: %s", err)
	}

	config.Buildpack.Version = flags.version

	_, _ = fmt.Fprintf(os.Stdout, "Packing %s %s...\n", config.Buildpack.Name, flags.version)

	if flags.stack != "" {
		var filteredDependencies []cargo.ConfigMetadataDependency
		for _, dep := range config.Metadata.Dependencies {
			if dep.HasStack(flags.stack) {
				filteredDependencies = append(filteredDependencies, dep)
			}
		}

		config.Metadata.Dependencies = filteredDependencies
	}

	logger := scribe.NewLogger(os.Stdout)
	bash := pexec.NewExecutable("bash")
	prePackager := internal.NewPrePackager(bash, logger, scribe.NewWriter(os.Stdout, scribe.WithIndent(2)))
	err = prePackager.Execute(config.Metadata.PrePackage, tmpDir)
	if err != nil {
		return fmt.Errorf("failed to execute pre-packaging script %q: %s", config.Metadata.PrePackage, err)
	}

	if flags.offline {
		transport := cargo.NewTransport()
		dependencyCacher := internal.NewDependencyCacher(transport, logger)
		config.Metadata.Dependencies, err = dependencyCacher.Cache(tmpDir, config.Metadata.Dependencies)
		if err != nil {
			return fmt.Errorf("failed to cache dependencies: %s", err)
		}

		for _, dependency := range config.Metadata.Dependencies {
			config.Metadata.IncludeFiles = append(config.Metadata.IncludeFiles, strings.TrimPrefix(dependency.URI, "file:///"))
		}
	}

	fileBundler := internal.NewFileBundler()
	files, err := fileBundler.Bundle(tmpDir, config.Metadata.IncludeFiles, config)
	if err != nil {
		return fmt.Errorf("failed to bundle files: %s", err)
	}

	tarBuilder := internal.NewTarBuilder(logger)
	err = tarBuilder.Build(flags.output, files)
	if err != nil {
		return fmt.Errorf("failed to create output: %s", err)
	}

	return err // err should be nil here, but return err to catch deferred error
}

func packRunExtension(flags packFlags, tmpDir string) error {

	directoryDuplicator := cargo.NewDirectoryDuplicator()
	err := directoryDuplicator.Duplicate(filepath.Dir(flags.extensionTOMLPath), tmpDir)
	if err != nil {
		return fmt.Errorf("failed to duplicate directory: %s", err)
	}

	extensionTOMLPath := filepath.Join(tmpDir, filepath.Base(flags.extensionTOMLPath))

	configParser := cargo.NewExtensionParser()
	config, err := configParser.Parse(extensionTOMLPath)
	if err != nil {
		return fmt.Errorf("failed to parse extension.toml: %s", err)
	}

	config.Extension.Version = flags.version

	_, _ = fmt.Fprintf(os.Stdout, "Packing %s %s...\n", config.Extension.Name, flags.version)

	if flags.stack != "" {
		var filteredDependencies []cargo.ConfigExtensionMetadataDependency
		for _, dep := range config.Metadata.Dependencies {
			if dep.HasStack(flags.stack) {
				filteredDependencies = append(filteredDependencies, dep)
			}
		}

		config.Metadata.Dependencies = filteredDependencies
	}

	logger := scribe.NewLogger(os.Stdout)
	bash := pexec.NewExecutable("bash")
	prePackager := internal.NewPrePackager(bash, logger, scribe.NewWriter(os.Stdout, scribe.WithIndent(2)))
	err = prePackager.Execute(config.Metadata.PrePackage, tmpDir)
	if err != nil {
		return fmt.Errorf("failed to execute pre-packaging script %q: %s", config.Metadata.PrePackage, err)
	}

	if flags.offline {
		transport := cargo.NewTransport()
		dependencyCacher := internal.NewDependencyCacher(transport, logger)
		config.Metadata.Dependencies, err = dependencyCacher.CacheExtension(tmpDir, config.Metadata.Dependencies)
		if err != nil {
			return fmt.Errorf("failed to cache dependencies: %s", err)
		}

		for _, dependency := range config.Metadata.Dependencies {
			config.Metadata.IncludeFiles = append(config.Metadata.IncludeFiles, strings.TrimPrefix(dependency.URI, "file:///"))
		}
	}

	fileBundler := internal.NewFileBundler()
	files, err := fileBundler.BundleExtension(tmpDir, config.Metadata.IncludeFiles, config)
	if err != nil {
		return fmt.Errorf("failed to bundle files: %s", err)
	}

	tarBuilder := internal.NewTarBuilder(logger)
	err = tarBuilder.Build(flags.output, files)
	if err != nil {
		return fmt.Errorf("failed to create output: %s", err)
	}

	return nil
}
