package internal

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type ExtensionInspector struct{}

func NewExtensionInspector() ExtensionInspector {
	return ExtensionInspector{}
}

type ExtensionMetadata struct {
	Config cargo.ExtensionConfig
	SHA256 string
}

func (i ExtensionInspector) Dependencies(path string) ([]ExtensionMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	indicesJSON, err := fetchFromArchive(tar.NewReader(file), "index.json", true)
	if err != nil {
		return nil, err
	}

	var index struct {
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}

	// There can only be 1 image index
	err = json.NewDecoder(indicesJSON[0]).Decode(&index)
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	manifests, err := fetchFromArchive(tar.NewReader(file), filepath.Join("blobs", "sha256", strings.TrimPrefix(index.Manifests[0].Digest, "sha256:")), true)
	if err != nil {
		return nil, err
	}

	buildpackageDigest := index.Manifests[0].Digest

	var m struct {
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}

	// We only support single manifest images
	err = json.NewDecoder(manifests[0]).Decode(&m)
	if err != nil {
		return nil, err
	}

	var metadataCollection []ExtensionMetadata
	for _, layer := range m.Layers {
		_, err = file.Seek(0, 0)
		if err != nil {
			return nil, err
		}

		layerBlobs, err := fetchFromArchive(tar.NewReader(file), filepath.Join("blobs", "sha256", strings.TrimPrefix(layer.Digest, "sha256:")), true)
		if err != nil {
			return nil, err
		}

		layerGR, err := gzip.NewReader(layerBlobs[0])
		if err != nil {
			return nil, fmt.Errorf("failed to read layer blob: %w", err)
		}
		defer func() {
			if err2 := layerGR.Close(); err2 != nil && err == nil {
				err = err2
			}
		}()

		// Generally, each layer corresponds to an extension.
		// But certain extension are "flattened" and contain multiple extension
		// in the same layer.
		extensionTOMLs, err := fetchFromArchive(tar.NewReader(layerGR), "extension.toml", false)
		if err != nil {
			return nil, err
		}

		for _, extensionTOML := range extensionTOMLs {
			var config cargo.ExtensionConfig
			err = cargo.DecodeExtensionConfig(extensionTOML, &config)
			if err != nil {
				return nil, err
			}

			metadata := ExtensionMetadata{
				Config: config,
			}
			metadataCollection = append(metadataCollection, metadata)
		}
	}

	if len(metadataCollection) == 1 {
		metadataCollection[0].SHA256 = buildpackageDigest
	}

	return metadataCollection, err // err should be nil here, but return err to catch deferred error
}
