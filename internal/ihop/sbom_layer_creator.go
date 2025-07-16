package ihop

import (
	"archive/tar"
	"fmt"
	"os"
	"strings"
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
	defer func() {
		if err2 := buffer.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

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

	layer, err := tarToLayer(buffer)
	return layer, err // err should be nil here, but return err to catch deferred error
}
