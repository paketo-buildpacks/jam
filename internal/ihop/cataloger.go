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
	ctx := context.Background()

	src, err := syft.GetSource(ctx, path, nil)
	if err != nil {
		return SBOM{}, nil
	}

	cfg := syft.DefaultCreateSBOMConfig()
	cfg.Search.Scope = source.SquashedScope

	bom, err := syft.CreateSBOM(ctx, src, cfg)
	if err != nil {
		return SBOM{}, err
	}

	return NewSBOM(*bom), nil
}
