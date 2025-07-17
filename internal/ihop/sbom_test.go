package ihop_test

import (
	"testing"

	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
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
				Packages: pkg.NewCollection(
					pkg.Package{
						Name: "c-package",
						Metadata: pkg.DpkgDBEntry{
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
						Metadata: pkg.ApkDBEntry{
							Package:      "a-package",
							Version:      "1.2.3",
							Architecture: "all",
						},
					},
					pkg.Package{
						Name: "b-package",
						Metadata: pkg.RpmDBEntry{
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
					"id": "fe5a291d49d734ea",
					"name": "a-package",
					"version": "",
					"type": "",
					"foundBy": "",
					"locations": [],
					"licenses": [],
					"language": "",
					"cpes": [],
					"purl": "",
					"metadataType": "apk-db-entry",
					"metadata": {
						"package": "a-package",
						"originPackage": "",
						"maintainer": "",
						"version": "1.2.3",
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
					"id": "7b80a8dd06b00cb4",
					"name": "b-package",
					"version": "",
					"type": "",
					"foundBy": "",
					"locations": [],
					"licenses": [],
					"language": "",
					"cpes": [],
					"purl": "",
					"metadataType": "rpm-db-entry",
					"metadata": {
						"name": "b-package",
						"version": "2.3.1",
						"epoch": null,
						"architecture": "amd64",
						"release": "",
						"sourceRpm": "b-package-source",
						"size": 0,
						"vendor": "",
						"files": null
					}
					},
					{
					"id": "8c94a67c9b13a73f",
					"name": "c-package",
					"version": "",
					"type": "",
					"foundBy": "",
					"locations": [],
					"licenses": [],
					"language": "",
					"cpes": [],
					"purl": "",
					"metadataType": "dpkg-db-entry",
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
					"id": "",
					"name": "",
					"version": "",
					"type": "",
					"metadata": null
				},
				"distro": {
					"id": "some-distro-name",
					"idLike": [
					"some-distro-id-like"
					],
					"versionID": "some-distro-version"
				},
				"descriptor": {
					"name": "",
					"version": ""
				},
				"schema": {
					"version": "16.0.34",
					"url": "https://raw.githubusercontent.com/anchore/syft/main/schema/json/schema-16.0.34.json"
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
