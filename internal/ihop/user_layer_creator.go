package ihop

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// A UserLayerCreator can be used to construct a layer that includes user and
// group metadata defining the cnb user for the container.
type UserLayerCreator struct{}

// Create returns a Layer that can be attached to an existing image.
func (c UserLayerCreator) Create(image Image, def DefinitionImage, _ SBOM) (Layer, error) {
	files := make(map[*tar.Header]io.Reader)

	img, err := image.ToDaemonImage()
	if err != nil {
		return Layer{}, err
	}

	tarBuffer := bytes.NewBuffer(nil)
	tw := tar.NewWriter(tarBuffer)

	// find any existing /etc/ folder and copy the header
	hdr, _, err := c.find(img, "etc/")
	if err != nil {
		return Layer{}, err
	}
	err = tw.WriteHeader(hdr)
	if err != nil {
		return Layer{}, err
	}

	// find any existing /etc/group file in the given image so the group can be
	// appended to its contents
	hdr, content, err := c.find(img, "etc/group")
	if err != nil {
		return Layer{}, err
	}

	buffer := bytes.NewBuffer(nil)
	_, err = buffer.ReadFrom(content)
	if err != nil {
		return Layer{}, err
	}
	buffer.WriteString(fmt.Sprintf("cnb:x:%d:\n", def.GID))
	hdr.Size = int64(buffer.Len())
	files[hdr] = buffer

	// find any existing /etc/passed file in the given image so the user can be
	// appended to its contents
	hdr, content, err = c.find(img, "etc/passwd")
	if err != nil {
		return Layer{}, err
	}

	buffer = bytes.NewBuffer(nil)
	_, err = buffer.ReadFrom(content)
	if err != nil {
		return Layer{}, err
	}
	buffer.WriteString(fmt.Sprintf("cnb:x:%d:%d::/home/cnb:%s\n", def.UID, def.GID, def.Shell))
	hdr.Size = int64(buffer.Len())
	files[hdr] = buffer

	for hdr, content := range files {
		err := tw.WriteHeader(&tar.Header{
			Name: hdr.Name,
			Mode: hdr.Mode,
			Size: hdr.Size,
		})
		if err != nil {
			return Layer{}, err
		}

		_, err = io.Copy(tw, content)
		if err != nil {
			return Layer{}, err
		}
	}

	// create a $HOME directory for the cnb user
	err = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "home/cnb",
		Mode:     int64(os.ModePerm),
		Uid:      def.UID,
		Gid:      def.GID,
	})
	if err != nil {
		return Layer{}, err
	}

	err = tw.Close()
	if err != nil {
		return Layer{}, err
	}

	layer, err := tarball.LayerFromReader(tarBuffer)
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

func (c UserLayerCreator) find(image v1.Image, path string) (*tar.Header, io.Reader, error) {
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

			if strings.TrimPrefix(hdr.Name, "/") == strings.TrimPrefix(path, "/") {
				buffer := bytes.NewBuffer(nil)
				_, err = io.CopyN(buffer, tr, hdr.Size)
				if err != nil {
					return nil, nil, err
				}

				header = hdr
				reader = buffer
				found = true
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
