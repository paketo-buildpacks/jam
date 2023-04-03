package ihop

import (
	"fmt"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/pkg/cataloger"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
)

// A Cataloger can be used to generate a software bill-of-materials result
// using Syft.
type Cataloger struct{}

// Scan generates an SBOM for an image tagged in the Docker daemon.
func (c Cataloger) Scan(path string) (SBOM, error) {
	input, err := source.ParseInput(fmt.Sprintf("oci-dir:%s", path), "")
	if err != nil {
		return SBOM{}, err
	}

	src, cleanup, err := source.New(*input, nil, nil)
	if err != nil {
		return SBOM{}, err
	}
	defer cleanup()

	catalog, _, release, err := syft.CatalogPackages(src, cataloger.Config{
		Search: cataloger.SearchConfig{
			Scope: source.SquashedScope,
		},
	})
	if err != nil {
		return SBOM{}, err
	}

	return NewSBOM(sbom.SBOM{
		Artifacts: sbom.Artifacts{
			PackageCatalog:    catalog,
			LinuxDistribution: release,
		},
		Source: src.Metadata,
	}), nil
}
