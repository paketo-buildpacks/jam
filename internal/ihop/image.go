package ihop

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func findFile(image v1.Image, filepath string) (*tar.Header, io.Reader, error) {
	layers, err := image.Layers()
	if err != nil {
		return nil, nil, err
	}

	for i := len(layers) - 1; i >= 0; i-- {
		layer, err := layers[i].Uncompressed()
		if err != nil {
			return nil, nil, err
		}

		var (
			found  bool
			header *tar.Header
			reader io.Reader
		)

		tr := tar.NewReader(layer)
		for {
			hdr, err := tr.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return nil, nil, err
			}

			// Some images have filepaths with a preceding '.'
			// e.g. './etc/...' instead of '/etc/...'
			// Strip it off if it exists
			headerName := strings.TrimPrefix(hdr.Name, ".")

			if strings.TrimPrefix(headerName, "/") == strings.TrimPrefix(filepath, "/") {
				found = true
				if hdr.Typeflag == tar.TypeSymlink {
					header, reader, err = findFile(image, path.Join(path.Dir(filepath), hdr.Linkname))
					if err != nil {
						return nil, nil, err
					}

					break
				}

				buffer := bytes.NewBuffer(nil)
				_, err = io.CopyN(buffer, tr, hdr.Size)
				if err != nil {
					return nil, nil, err
				}

				header = hdr
				reader = buffer
				break
			}
		}

		err = layer.Close()
		if err != nil {
			return nil, nil, err
		}

		if found {
			return header, reader, nil
		}
	}

	return nil, nil, nil
}

func tarToLayer(reader *os.File) (Layer, error) {
	layer, err := tarball.LayerFromFile(reader.Name())
	if err != nil {
		return Layer{}, err
	}

	diffID, err := layer.DiffID()
	if err != nil {
		return Layer{}, err
	}

	return Layer{
		DiffID: diffID.String(),
		Layer:  layer,
	}, nil
}
