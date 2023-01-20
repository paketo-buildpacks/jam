package ihop_test

import (
	"testing"

	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/paketo-buildpacks/jam/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testSBOM(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		bom ihop.SBOM
	)

	it.Before(func() {
		bom = ihop.NewSBOM(sbom.SBOM{
			Artifacts: sbom.Artifacts{
				LinuxDistribution: &linux.Release{
					ID:        "some-distro-name",
					VersionID: "some-distro-version",
					IDLike:    []string{"some-distro-id-like"},
				},
				PackageCatalog: pkg.NewCatalog(
					pkg.Package{
						Name: "c-package",
						Metadata: pkg.DpkgMetadata{
							Package:       "c-package",
							Version:       "3.1.2",
							Architecture:  "arm64",
							Source:        "c-package-source",
							SourceVersion: "3.1.2-upstream-ubuntu3",
							Description:   "a package for c\n provides a bunch of c stuff",
						},
					},
					pkg.Package{
						Name: "a-package",
						Metadata: pkg.ApkMetadata{
							Package:      "a-package",
							Version:      "1.2.3",
							Architecture: "all",
						},
					},
					pkg.Package{
						Name: "b-package",
						Metadata: pkg.RpmMetadata{
							Name:      "b-package",
							Version:   "2.3.1",
							Arch:      "amd64",
							SourceRpm: "b-package-source",
						},
					},
				),
			},
		})
	})

	context("Distro", func() {
		it("specifies the linux distro details", func() {
			Expect(bom.Distro.Name).To(Equal("some-distro-name"))
			Expect(bom.Distro.Version).To(Equal("some-distro-version"))
		})
	})

	context("Packages", func() {
		it("returns a list of packages in the SBOM", func() {
			Expect(bom.Packages()).To(Equal([]string{
				"a-package",
				"b-package",
				"c-package",
			}))
		})
	})

	context("LegacyFormat", func() {
		it("returns a legacy formatted SBOM", func() {
			output, err := bom.LegacyFormat()
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchJSON(`[
				{
					"name": "a-package",
					"version": "1.2.3",
					"arch": "all"
				},
				{
					"name": "b-package",
					"version": "2.3.1",
					"arch": "amd64",
					"source": {
						"name": "b-package-source"
					}
				},
				{
					"name": "c-package",
					"version": "3.1.2",
					"arch": "arm64",
					"source": {
						"name": "c-package-source",
						"version": "3.1.2-upstream-ubuntu3",
						"upstreamVersion": "3.1.2-upstream"
					},
					"summary": "a package for c"
				}
			]`))
		})
	})

	context("SyftFormat", func() {
		it("returns a Syft formatted SBOM", func() {
			output, err := bom.SyftFormat()
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchJSON(`{
				"artifacts": [
					{
						"id": "af027636bc3daa4c",
						"name": "a-package",
						"version": "",
						"type": "",
						"foundBy": "",
						"locations": [],
						"licenses": [],
						"language": "",
						"cpes": [],
						"purl": "",
						"metadataType": "",
						"metadata": {
							"package": "a-package",
							"originPackage": "",
							"maintainer": "",
							"version": "1.2.3",
							"license": "",
							"architecture": "all",
							"url": "",
							"description": "",
							"size": 0,
							"installedSize": 0,
							"pullDependencies": null,
							"provides": null,
							"pullChecksum": "",
							"gitCommitOfApkPort": "",
							"files": null
						}
					},
					{
						"id": "9e1d9a70039b8e49",
						"name": "b-package",
						"version": "",
						"type": "",
						"foundBy": "",
						"locations": [],
						"licenses": [],
						"language": "",
						"cpes": [],
						"purl": "",
						"metadataType": "",
						"metadata": {
							"name": "b-package",
							"version": "2.3.1",
							"epoch": null,
							"architecture": "amd64",
							"release": "",
							"sourceRpm": "b-package-source",
							"size": 0,
							"license": "",
							"vendor": "",
							"modularityLabel": "",
							"files": null
						}
					},
					{
						"id": "1d4dc26dfec3e100",
						"name": "c-package",
						"version": "",
						"type": "",
						"foundBy": "",
						"locations": [],
						"licenses": [],
						"language": "",
						"cpes": [],
						"purl": "",
						"metadataType": "",
						"metadata": {
							"package": "c-package",
							"source": "c-package-source",
							"version": "3.1.2",
							"sourceVersion": "3.1.2-upstream-ubuntu3",
							"architecture": "arm64",
							"maintainer": "",
							"installedSize": 0,
							"files": null
						}
					}
				],
				"artifactRelationships": [],
				"source": {
					"type": "",
					"target": null
				},
				"distro": {
					"name": "some-distro-name",
					"version": "",
					"idLike": "some-distro-id-like"
				},
				"descriptor": {
					"name": "",
					"version": ""
				},
				"schema": {
					"version": "2.0.2",
					"url": "https://raw.githubusercontent.com/anchore/syft/main/schema/json/schema-2.0.2.json"
				}
			}`))
		})
	})

	context("CycloneDXFormat", func() {
		it("returns a CycloneDX formatted SBOM", func() {
			output, err := bom.CycloneDXFormat()
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`"bomFormat": "CycloneDX"`))
			Expect(output).To(ContainSubstring(`"specVersion": "1.3"`))
		})
	})
}
