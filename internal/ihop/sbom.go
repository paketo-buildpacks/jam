package ihop

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	psbom "github.com/paketo-buildpacks/packit/v2/sbom"
)

// LegacySBOMPackage represents a package as defined in the legacy SBOM format.
type LegacySBOMPackage struct {
	Name    string                   `json:"name"`
	Version string                   `json:"version"`
	Arch    string                   `json:"arch"`
	Source  *LegacySBOMPackageSource `json:"source,omitempty"`
	Summary string                   `json:"summary,omitempty"`
}

// LegacySBOMPackageSource represents a package source as defined in the legacy
// SBOM format.
type LegacySBOMPackageSource struct {
	Name            string `json:"name"`
	Version         string `json:"version,omitempty"`
	UpstreamVersion string `json:"upstreamVersion,omitempty"`
}

// SBOMDistroy contains the name and version of the underlying image
// distribution.
type SBOMDistro struct {
	Name    string
	Version string
}

// SBOM represents the software bill-of-materials results from an image scan.
type SBOM struct {
	sbom   sbom.SBOM
	Distro SBOMDistro
}

// NewSBOM returns an SBOM given the results from a Syft image scan.
func NewSBOM(sbom sbom.SBOM) SBOM {
	var release linux.Release
	if sbom.Artifacts.LinuxDistribution != nil {
		release = *sbom.Artifacts.LinuxDistribution
	}

	return SBOM{
		sbom: sbom,
		Distro: SBOMDistro{
			Name:    release.ID,
			Version: release.VersionID,
		},
	}
}

// Packages returns the list of packages included in the SBOM.
func (s SBOM) Packages() []string {
	var packages []string
	for p := range s.sbom.Artifacts.Packages.Enumerate() {
		packages = append(packages, p.Name)
	}

	sort.Strings(packages)

	return packages
}

// LegacyFormat returns a JSON-encoded string representation of the legacy SBOM
// format.
func (s SBOM) LegacyFormat() (string, error) {
	var packages []LegacySBOMPackage
	for p := range s.sbom.Artifacts.Packages.Enumerate() {
		switch metadata := p.Metadata.(type) {
		case pkg.DpkgDBEntry:
			upstreamVersion := metadata.SourceVersion

			parts := strings.Split(upstreamVersion, ":")
			if len(parts) > 1 {
				upstreamVersion = strings.Join(parts[1:], ":")
			}

			parts = strings.Split(upstreamVersion, "-")
			if len(parts) > 1 {
				upstreamVersion = strings.Join(parts[:len(parts)-1], "-")
			}

			packages = append(packages, LegacySBOMPackage{
				Name:    metadata.Package,
				Version: metadata.Version,
				Arch:    metadata.Architecture,
				Source: &LegacySBOMPackageSource{
					Name:            metadata.Source,
					Version:         metadata.SourceVersion,
					UpstreamVersion: upstreamVersion,
				},
				Summary: strings.SplitN(metadata.Description, "\n", 2)[0],
			})

		case pkg.ApkDBEntry:
			// case pkg.ApkMetadata:
			packages = append(packages, LegacySBOMPackage{
				Name:    metadata.Package,
				Version: metadata.Version,
				Arch:    metadata.Architecture,
			})

		case pkg.RpmDBEntry:
			// case pkg.RpmMetadata:
			packages = append(packages, LegacySBOMPackage{
				Name:    metadata.Name,
				Version: metadata.Version,
				Arch:    metadata.Arch,
				Source: &LegacySBOMPackageSource{
					Name: metadata.SourceRpm,
				},
			})
		}
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	output, err := json.Marshal(packages)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// SyftFormat returns a Syft JSON-encoded string representation of the SBOM contents
// using schema version 2.0.2.
func (s SBOM) SyftFormat() (string, error) {
	return s.inFormat(psbom.Format("application/vnd.syft+json;version=2.0.2"))
}

// CycloneDXFormat returns a CycloneDX JSON-encoded string representation of
// the SBOM contents using schema version 1.3.
func (s SBOM) CycloneDXFormat() (string, error) {
	return s.inFormat(psbom.Format("application/vnd.cyclonedx+json;version=1.3"))
}

func (s SBOM) inFormat(format psbom.Format) (string, error) {
	reader := psbom.NewFormattedReader(psbom.NewSBOM(s.sbom), format)
	buffer := bytes.NewBuffer(nil)

	_, err := io.Copy(buffer, reader)
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
