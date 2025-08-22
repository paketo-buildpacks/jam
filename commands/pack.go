package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

		systemOS := osFromSystem()
		systemArch := archFromSystem()

		// linux/amd64 will be the default target dir when dependencies don't specify os and arch
		defaultTargetDir := "linux/amd64"
		systemTargetDir := filepath.Join(systemOS, systemArch)

		for _, dependency := range config.Metadata.Dependencies {
			var targetPlatformDir string
			shouldMoveToPlatformDir := false
			checkTargetDirPaths := []string{}
			if dependency.OS != "" && dependency.Arch != "" {
				checkTargetDirPaths = append(checkTargetDirPaths, filepath.Join(dependency.OS, dependency.Arch))
			}
			checkTargetDirPaths = append(checkTargetDirPaths, systemTargetDir, defaultTargetDir)

			for _, dir := range checkTargetDirPaths {
				hasTargetDir := false
				info, err := os.Stat(filepath.Join(tmpDir, dir))
				if err == nil && info.IsDir() && !os.IsNotExist(err) {
					hasTargetDir = true
				}

				// Don't move to platform-specific directory unless include-files has required executables in the platform-specific bin directory.
				hasTargetExecutableIncludeFiles := false
				requiredFileNames := []string{"build", "detect"}
				requiredFilesFound := 0
				for _, file := range config.Metadata.IncludeFiles {
					if hasTargetExecutableIncludeFiles {
						break
					}
					for _, name := range requiredFileNames {
						if file == fmt.Sprintf("%s/bin/%s", dir, name) {
							requiredFilesFound++
						}
						if requiredFilesFound == len(requiredFileNames) {
							hasTargetExecutableIncludeFiles = true
							break
						}
					}
				}

				if hasTargetDir && hasTargetExecutableIncludeFiles {
					shouldMoveToPlatformDir = true
					targetPlatformDir = dir
					break
				}
			}

			if shouldMoveToPlatformDir {
				// This is a multi-arch buildpack and dependencies need to be moved into the platform-specific directory because
				// `pack buildpack package` will be called with `--target <os>/<arch>` and files outside the path will not be included
				offlinePath := strings.TrimPrefix(dependency.URI, "file:///")
				dependenciesDir := filepath.Dir(offlinePath)
				offlineFilename := filepath.Base(offlinePath)

				info, err := os.Stat(filepath.Join(tmpDir, dependenciesDir))
				if err != nil || os.IsNotExist(err) || !info.IsDir() {
					return fmt.Errorf("expected dependencies directory does not exist: %s", err)
				}

				err = os.MkdirAll(filepath.Join(tmpDir, targetPlatformDir, dependenciesDir), os.ModePerm)
				if err != nil {
					return fmt.Errorf("failed to create platform specific dependencies directory: %s", err)
				}

				err = os.Rename(filepath.Join(tmpDir, offlinePath), filepath.Join(tmpDir, targetPlatformDir, dependenciesDir, offlineFilename))
				if err != nil {
					return fmt.Errorf("failed to move offline dependency to platform specific directory: %s", err)
				}

				config.Metadata.IncludeFiles = append(config.Metadata.IncludeFiles, filepath.Join(targetPlatformDir, dependenciesDir, offlineFilename))
			} else {
				config.Metadata.IncludeFiles = append(config.Metadata.IncludeFiles, strings.TrimPrefix(dependency.URI, "file:///"))
			}
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

func osFromSystem() string {
	osFromEnv, ok := os.LookupEnv("BP_OS")
	if ok {
		return osFromEnv
	}

	return runtime.GOOS
}

func archFromSystem() string {
	archFromEnv, ok := os.LookupEnv("BP_ARCH")
	if ok {
		return archFromEnv
	}

	return runtime.GOARCH
}
