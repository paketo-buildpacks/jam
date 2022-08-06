package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface Downloader --output fakes/downloader.go
type Downloader interface {
	Drop(root, uri string) (io.ReadCloser, error)
}

type DependencyCacher struct {
	downloader Downloader
	logger     scribe.Logger
}

func NewDependencyCacher(downloader Downloader, logger scribe.Logger) DependencyCacher {
	return DependencyCacher{
		downloader: downloader,
		logger:     logger,
	}
}

func (dc DependencyCacher) Cache(root string, deps []cargo.ConfigMetadataDependency) ([]cargo.ConfigMetadataDependency, error) {
	dc.logger.Process("Downloading dependencies...")
	dir := filepath.Join(root, "dependencies")
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create dependencies directory: %s", err)
	}

	sem := make(chan error, len(deps)) // semaphore pattern

	// var dependencies []cargo.ConfigMetadataDependency
	dependencies := make([]cargo.ConfigMetadataDependency, len(deps))
	for i, dep := range deps {
		dc.logger.Subprocess("%s (%s) [%s]", dep.ID, dep.Version, strings.Join(dep.Stacks, ", "))
		dc.logger.Action("↳  dependencies/%s", dep.SHA256)

		go func(i int, dep cargo.ConfigMetadataDependency) {
			// dc.logger.Subprocess("%s (%s) [%s]\n↳  dependencies/%s", dep.ID, dep.Version, strings.Join(dep.Stacks, ", "),dep.SHA256)

			source, err := dc.downloader.Drop("", dep.URI)
			if err != nil {
				sem <- fmt.Errorf("failed to download dependency: %s", err)
				return
				// return nil, fmt.Errorf("failed to download dependency: %s", err)
			}

			validatedSource := cargo.NewValidatedReader(source, dep.SHA256)

			destination, err := os.Create(filepath.Join(dir, dep.SHA256))
			if err != nil {
				sem <- fmt.Errorf("failed to create destination file: %s", err)
				// return nil, fmt.Errorf("failed to create destination file: %s", err)
				return
			}

			_, err = io.Copy(destination, validatedSource)
			if err != nil {
				sem <- fmt.Errorf("failed to copy dependency: %s", err)
				return
				// return nil, fmt.Errorf("failed to copy dependency: %s", err)
			}

			err = destination.Close()
			if err != nil {
				sem <- fmt.Errorf("failed to close dependency destination: %s", err)
				return
				// return nil, fmt.Errorf("failed to close dependency destination: %s", err)
			}

			err = source.Close()
			if err != nil {
				sem <- fmt.Errorf("failed to close dependency source: %s", err)
				return
				// return nil, fmt.Errorf("failed to close dependency source: %s", err)
			}

			dep.URI = fmt.Sprintf("file:///dependencies/%s", dep.SHA256)
			dependencies[i] = dep
			sem <- nil
			// dependencies = append(dependencies, dep)
		}(i, dep)
	}

	// wait for goroutines to finish
	for i := 0; i < len(deps); i++ {
		err := <-sem
		if err != nil {
			return nil, err
		}
	}

	dc.logger.Break()

	dc.logger.Subprocess("Downloading complete")

	dc.logger.Break()

	return dependencies, nil
}
