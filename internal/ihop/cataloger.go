package ihop

import (
	"context"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/source"
)

// A Cataloger can be used to generate a software bill-of-materials result
// using Syft.
type Cataloger struct{}

// Scan generates an SBOM for an image tagged in the Docker daemon.
func (c Cataloger) Scan(path string) (SBOM, error) {
	src, err := syft.GetSource(context.Background(), path, syft.DefaultGetSourceConfig().WithSources("oci-dir"))
	if err != nil {
		return SBOM{}, err
	}

	cfg := syft.DefaultCreateSBOMConfig()
	cfg.Search.Scope = source.SquashedScope // this is the default, but we set it explicitly for clarity

	// build the SBOM
	s, err := syft.CreateSBOM(context.Background(), src, cfg)
	if err != nil {
		return SBOM{}, err
	}

	return NewSBOM(*s), nil
}
