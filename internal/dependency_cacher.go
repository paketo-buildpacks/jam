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

	var dependencies []cargo.ConfigMetadataDependency
	for _, dep := range deps {
		source, err := dc.downloader.Drop("", dep.URI)
		if err != nil {
			return nil, fmt.Errorf("failed to download dependency: %s", err)
		}

		var checksum string
		if dep.SHA256 != "" {
			checksum = fmt.Sprintf("sha256:%s", dep.SHA256)
		} else {
			checksum = dep.Checksum
		}

		validatedSource := cargo.NewValidatedReader(source, checksum)

		checksumValue := strings.SplitN(checksum, ":", 2)[1]

		dc.logger.Subprocess("%s (%s) [%s]", dep.ID, dep.Version, strings.Join(dep.Stacks, ", "))
		dc.logger.Action("↳  dependencies/%s", checksumValue)

		destination, err := os.Create(filepath.Join(dir, checksumValue))
		if err != nil {
			return nil, fmt.Errorf("failed to create destination file: %s", err)
		}

		_, err = io.Copy(destination, validatedSource)
		if err != nil {
			return nil, fmt.Errorf("failed to copy dependency: %s", err)
		}

		err = destination.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to close dependency destination: %s", err)
		}

		err = source.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to close dependency source: %s", err)
		}

		dep.URI = fmt.Sprintf("file:///dependencies/%s", checksumValue)
		dependencies = append(dependencies, dep)
	}

	dc.logger.Break()

	return dependencies, nil
}
