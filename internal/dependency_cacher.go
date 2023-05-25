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

type ConfigBuilpackOrExtensionMetadataDependency interface {
	GetChecksum() string
	GetID() string
	GetSHA256() string
	GetStacks() []string
	GetURI() string
	GetVersion() string
}

type ExtensionConfigMetadataDependency struct {
	cargo.ConfigExtensionMetadataDependency
}

func (cd ExtensionConfigMetadataDependency) GetChecksum() string {
	return cd.Checksum
}

func (cd ExtensionConfigMetadataDependency) GetID() string {
	return cd.ID
}

func (cd ExtensionConfigMetadataDependency) GetSHA256() string {
	return cd.SHA256
}

func (cd ExtensionConfigMetadataDependency) GetStacks() []string {
	return cd.Stacks
}

func (cd ExtensionConfigMetadataDependency) GetURI() string {
	return cd.URI
}

func (cd ExtensionConfigMetadataDependency) GetVersion() string {
	return cd.Version
}

type BuildpackConfigMetadataDependency struct {
	cargo.ConfigMetadataDependency
}

func (cd BuildpackConfigMetadataDependency) GetChecksum() string {
	return cd.Checksum
}

func (cd BuildpackConfigMetadataDependency) GetID() string {
	return cd.ID
}

func (cd BuildpackConfigMetadataDependency) GetSHA256() string {
	return cd.SHA256
}

func (cd BuildpackConfigMetadataDependency) GetStacks() []string {
	return cd.Stacks
}

func (cd BuildpackConfigMetadataDependency) GetURI() string {
	return cd.URI
}

func (cd BuildpackConfigMetadataDependency) GetVersion() string {
	return cd.Version
}

func (dc DependencyCacher) caching(root string, deps []ConfigBuilpackOrExtensionMetadataDependency) ([]string, error) {
	dc.logger.Process("Downloading dependencies...")
	dir := filepath.Join(root, "dependencies")
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create dependencies directory: %s", err)
	}

	var uris []string
	for _, dep := range deps {
		dc.logger.Subprocess("%s (%s) [%s]", dep.GetID(), dep.GetVersion(), strings.Join(dep.GetStacks(), ", "))

		source, err := dc.downloader.Drop("", dep.GetURI())
		if err != nil {
			return nil, fmt.Errorf("failed to download dependency: %s", err)
		}

		checksum := dep.GetChecksum()
		_, hash, _ := strings.Cut(dep.GetChecksum(), ":")

		if checksum == "" {
			checksum = fmt.Sprintf("sha256:%s", dep.GetSHA256())
			hash = dep.GetSHA256()
		}

		if checksum == "sha256:" {
			return nil, fmt.Errorf("failed to create file for %s: no sha256 or checksum provided", dep.GetID())
		}

		dc.logger.Action("↳  dependencies/%s", hash)

		validatedSource := cargo.NewValidatedReader(source, checksum)

		destination, err := os.Create(filepath.Join(dir, hash))
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

		uris = append(uris, fmt.Sprintf("file:///dependencies/%s", hash))
	}

	dc.logger.Break()

	return uris, nil
}

func (dc DependencyCacher) Cache(root string, deps []cargo.ConfigMetadataDependency) ([]cargo.ConfigMetadataDependency, error) {

	dependencies := []ConfigBuilpackOrExtensionMetadataDependency{}
	for _, dep := range deps {
		dependencies = append(dependencies, BuildpackConfigMetadataDependency{dep})
	}

	uris, err := dc.caching(root, dependencies)
	if err != nil {
		return nil, fmt.Errorf("%s", err)
	}

	for index := range deps {
		deps[index].URI = uris[index]
	}

	return deps, nil
}

func (dc DependencyCacher) CacheExtension(root string, deps []cargo.ConfigExtensionMetadataDependency) ([]cargo.ConfigExtensionMetadataDependency, error) {

	dependencies := []ConfigBuilpackOrExtensionMetadataDependency{}
	for _, dep := range deps {
		dependencies = append(dependencies, ExtensionConfigMetadataDependency{dep})
	}

	uris, err := dc.caching(root, dependencies)
	if err != nil {
		return nil, fmt.Errorf("%s", err)
	}

	for index := range deps {
		deps[index].URI = uris[index]
	}

	return deps, nil
}
