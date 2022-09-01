package ihop

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
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

	tarBuffer, err := os.CreateTemp("", "")
	if err != nil {
		return Layer{}, err
	}
	defer tarBuffer.Close()
	tw := tar.NewWriter(tarBuffer)

	// find any existing /etc/ folder and copy the header
	hdr, _, err := findFile(img, "etc/")
	if err != nil {
		return Layer{}, err
	}
	err = tw.WriteHeader(hdr)
	if err != nil {
		return Layer{}, err
	}

	// find any existing /etc/group file in the given image so the group can be
	// appended to its contents
	hdr, content, err := findFile(img, "etc/group")
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
	hdr, content, err = findFile(img, "etc/passwd")
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

	return tarToLayer(tarBuffer)
}
