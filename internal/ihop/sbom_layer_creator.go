package ihop

import (
	"archive/tar"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// A SBOMLayerCreator can be used to construct a layer that includes the
// contents of an SBOM in Syft and CycloneDX formats.
type SBOMLayerCreator struct{}

// Create returns a Layer that can be attached to an existing image.
func (c SBOMLayerCreator) Create(image Image, def DefinitionImage, sbom SBOM) (Layer, error) {
	digest := strings.TrimPrefix(image.Digest, "sha256:")

	buffer, err := os.CreateTemp("", "")
	if err != nil {
		return Layer{}, err
	}
	defer buffer.Close()

	tw := tar.NewWriter(buffer)
	syftSBOM, err := sbom.SyftFormat()
	if err != nil {
		return Layer{}, err
	}

	err = tw.WriteHeader(&tar.Header{
		Name: fmt.Sprintf("cnb/sbom/%s.syft.json", digest[:8]),
		Mode: 0600,
		Size: int64(len(syftSBOM)),
	})
	if err != nil {
		return Layer{}, err
	}

	_, err = tw.Write([]byte(syftSBOM))
	if err != nil {
		return Layer{}, err
	}

	cdxSBOM, err := sbom.CycloneDXFormat()
	if err != nil {
		return Layer{}, err
	}

	err = tw.WriteHeader(&tar.Header{
		Name: fmt.Sprintf("cnb/sbom/%s.cdx.json", digest[:8]),
		Mode: 0600,
		Size: int64(len(cdxSBOM)),
	})
	if err != nil {
		return Layer{}, err
	}

	_, err = tw.Write([]byte(cdxSBOM))
	if err != nil {
		return Layer{}, err
	}

	err = tw.Close()
	if err != nil {
		return Layer{}, err
	}

	layer, err := tarball.LayerFromFile(buffer.Name())
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
