package internal

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type BuildpackInspector struct{}

func NewBuildpackInspector() BuildpackInspector {
	return BuildpackInspector{}
}

type BuildpackMetadata struct {
	Config cargo.Config
	SHA256 string
}

func (i BuildpackInspector) Dependencies(path string) ([]BuildpackMetadata, error) {
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

	var metadataCollection []BuildpackMetadata
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

		// Generally, each layer corresponds to a buildpack.
		// But certain buildpacks are "flattened" and contain multiple buildpacks
		// in the same layer.
		buildpackTOMLs, err := fetchFromArchive(tar.NewReader(layerGR), "buildpack.toml", false)
		if err != nil {
			return nil, err
		}

		for _, buildpackTOML := range buildpackTOMLs {
			var config cargo.Config
			err = cargo.DecodeConfig(buildpackTOML, &config)
			if err != nil {
				return nil, err
			}

			metadata := BuildpackMetadata{
				Config: config,
			}
			if len(config.Order) > 0 {
				metadata.SHA256 = buildpackageDigest
			}
			metadataCollection = append(metadataCollection, metadata)
		}
	}

	if len(metadataCollection) == 1 {
		metadataCollection[0].SHA256 = buildpackageDigest
	}

	return metadataCollection, err // err should be nil here, but return err to catch deferred error
}

// This function takes a boolean to stop search after the first match because
// tar.Reader is a streaming reader, and once you move to the next entry via a
// Next() call, the previous file reader becomes invalid. This forces us to
// copy the file contents to memory if we want to fetch multiple matches, and
// we only want to do so for small text files, and not large files like layer
// blobs.
func fetchFromArchive(tr *tar.Reader, filename string, stopAtFirstMatch bool) ([]io.Reader, error) {
	var readers []io.Reader
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(hdr.Name, filename) {
			if stopAtFirstMatch {
				return []io.Reader{tr}, nil
			}
			buff := bytes.NewBuffer(nil)
			_, err = io.CopyN(buff, tr, hdr.Size)
			if err != nil {
				return nil, fmt.Errorf("failed to copy file %s: %w", hdr.Name, err)
			}
			readers = append(readers, buff)
		}
	}

	if len(readers) < 1 {
		return nil, fmt.Errorf("failed to fetch archived file %s", filename)
	}
	return readers, nil
}
