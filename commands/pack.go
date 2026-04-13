package commands

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
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
	cmd.MarkFlagsOneRequired("buildpack", "extension")

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

	buildpackOrExtensionTOMLPath := ""

	if flags.buildpackTOMLPath != "" {
		buildpackOrExtensionTOMLPath = flags.buildpackTOMLPath
	} else if flags.extensionTOMLPath != "" {
		buildpackOrExtensionTOMLPath = flags.extensionTOMLPath
	} else {
		return fmt.Errorf(`--buildpack or --extension path must not be empty`)
	}

	if flags.offline && buildpackOrExtensionTOMLPath == flags.extensionTOMLPath {
		return fmt.Errorf("offline mode is not supported for extensions")
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

	directoryDuplicator := cargo.NewDirectoryDuplicator()
	err = directoryDuplicator.Duplicate(filepath.Dir(buildpackOrExtensionTOMLPath), tmpDir)
	if err != nil {
		return fmt.Errorf("failed to duplicate directory: %s", err)
	}

	if flags.extensionTOMLPath != "" {
		err := packRunExtension(flags, tmpDir)
		if err != nil {
			return fmt.Errorf("failed to pack extension: %s", err)
		}
		return nil
	}

	buildpackTOMLPath := filepath.Join(tmpDir, filepath.Base(buildpackOrExtensionTOMLPath))

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

	var bundleFiles []string
	if len(config.Targets) > 1 {
		bundleFiles, err = fixIncludeFilesDirectoryStructure(config.Metadata.IncludeFiles, config.Targets, tmpDir)
		if err != nil {
			return fmt.Errorf("failed to fix include files directory structure: %s", err)
		}
	} else {
		bundleFiles = config.Metadata.IncludeFiles
	}

	if flags.offline {
		transport := cargo.NewTransport()
		dependencyCacher := internal.NewDependencyCacher(transport, logger)
		config.Metadata.Dependencies, err = dependencyCacher.Cache(tmpDir, config.Metadata.Dependencies)
		if err != nil {
			return fmt.Errorf("failed to cache dependencies: %s", err)
		}

		dependenciesDir := "dependencies"

		depsDir := filepath.Join(tmpDir, dependenciesDir)
		info, err := os.Stat(depsDir)
		if err != nil {
			return fmt.Errorf("expected dependencies directory: %s", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("expected dependencies path is not a directory: %s", depsDir)
		}

		// We want to ensure that in case at least one target is specified
		// each dependency has an OS and Arch attribute.
		// Also, we ensure that if there are multiple targets, the same dependency
		// will be used for each target.
		var metadataDeps []cargo.ConfigMetadataDependency
		if len(config.Targets) > 0 {
			for _, dependency := range config.Metadata.Dependencies {
				if dependency.OS == "" || dependency.Arch == "" {
					for _, target := range config.Targets {
						d := dependency
						d.OS = target.OS
						d.Arch = target.Arch
						metadataDeps = append(metadataDeps, d)
					}
				} else {
					metadataDeps = append(metadataDeps, dependency)
				}
			}
		} else {
			metadataDeps = config.Metadata.Dependencies
		}

		config.Metadata.Dependencies = metadataDeps

		isMultiArch := len(config.Targets) > 1

		// This is a multi-arch buildpack and dependencies need to be moved into the platform-specific directory because
		// `pack buildpack package` will be called with `--target <os>/<arch>` and files outside the path will not be included
		for _, dependency := range config.Metadata.Dependencies {
			if isMultiArch {
				dependencyPlatformDir := filepath.Join(dependency.OS, dependency.Arch, dependenciesDir)

				offlinePath := strings.TrimPrefix(dependency.URI, "file:///")
				offlineFilename := filepath.Base(offlinePath)

				err = os.MkdirAll(filepath.Join(tmpDir, dependencyPlatformDir), os.ModePerm)
				if err != nil {
					return fmt.Errorf("failed to create platform specific dependencies directory: %s", err)
				}

				_, err = copyFile(filepath.Join(tmpDir, offlinePath), filepath.Join(tmpDir, dependencyPlatformDir, offlineFilename))
				if err != nil {
					return fmt.Errorf("failed to copy offline dependency to platform specific directory: %s", err)
				}

				relativePath := path.Join(dependencyPlatformDir, offlineFilename)
				bundleFiles = append(bundleFiles, relativePath)
			} else {
				bundleFiles = append(bundleFiles, strings.TrimPrefix(dependency.URI, "file:///"))
			}
		}
	}

	fileBundler := internal.NewFileBundler()
	files, err := fileBundler.Bundle(tmpDir, bundleFiles, config)
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

	var bundleFiles []string
	if len(config.Targets) > 1 {
		bundleFiles, err = fixIncludeFilesDirectoryStructure(config.Metadata.IncludeFiles, config.Targets, tmpDir)
		if err != nil {
			return fmt.Errorf("failed to fix include files directory structure: %s", err)
		}
	} else {
		bundleFiles = config.Metadata.IncludeFiles
	}

	fileBundler := internal.NewFileBundler()
	files, err := fileBundler.BundleExtension(tmpDir, bundleFiles, config)
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

func copyFile(src string, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}
	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err2 := source.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	destination, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, sourceFileStat.Mode())
	if err != nil {
		return 0, err
	}
	defer func() {
		if err2 := destination.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func fixIncludeFilesDirectoryStructure(includeFiles []string, targets []cargo.ConfigTarget, tmpDir string) ([]string, error) {
	osArchDirs := []string{}
	for _, target := range targets {
		osArchDirs = append(osArchDirs, target.OS+"/"+target.Arch)
	}

	fixedIncludeFiles := []string{}
	for _, file := range includeFiles {
		if file == "buildpack.toml" {
			fixedIncludeFiles = append(fixedIncludeFiles, file)
			continue
		}

		if file == "extension.toml" {
			fixedIncludeFiles = append(fixedIncludeFiles, file)
			continue
		}

		hasOsArchPrefix := slices.ContainsFunc(osArchDirs, func(dir string) bool {
			return strings.HasPrefix(file, dir)
		})

		if hasOsArchPrefix {
			fixedIncludeFiles = append(fixedIncludeFiles, file)
		} else {
			for _, dir := range osArchDirs {

				destRelativePath := filepath.Join(dir, file)
				destAbsolutePath := filepath.Join(tmpDir, destRelativePath)

				err := os.MkdirAll(filepath.Dir(destAbsolutePath), os.ModePerm)
				if err != nil {
					return nil, fmt.Errorf("failed to create platform specific directory for include file or dependencies - attempted directory: %s", err)
				}

				_, err = copyFile(filepath.Join(tmpDir, file), destAbsolutePath)
				if err != nil {
					return nil, fmt.Errorf("failed to copy file %s to %s: %s", filepath.Join(tmpDir, file), destAbsolutePath, err)
				}
				fixedIncludeFiles = append(fixedIncludeFiles, destRelativePath)
			}
		}
	}

	return fixedIncludeFiles, nil
}
