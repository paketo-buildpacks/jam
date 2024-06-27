package internal_test

import (
	"bytes"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testExtensionFormatter(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		buffer    *bytes.Buffer
		formatter internal.ExtensionFormatter
	)

	it.Before(func() {
		buffer = bytes.NewBuffer(nil)
		formatter = internal.NewExtensionFormatter(buffer)
	})

	context("Markdown", func() {
		it("returns a list of dependencies", func() {
			formatter.Markdown([]internal.ExtensionMetadata{
				{
					Config: cargo.ExtensionConfig{
						Extension: cargo.ConfigExtension{
							ID:      "paketo-community/ubi-nodejs-extension",
							Name:    "Ubi Node.js Extension",
							Version: "0.0.2",
						},
						Metadata: cargo.ConfigExtensionMetadata{
							DefaultVersions: map[string]string{
								"node": "20.*.*",
							},
							Dependencies: []cargo.ConfigExtensionMetadataDependency{
								{
									ID:      "node",
									Name:    "Ubi Node Extension",
									Source:  "paketocommunity/run-nodejs-20-ubi-base",
									Stacks:  []string{"io.buildpacks.stacks.ubi8"},
									Version: "20.1000",
								},
								{
									ID:      "node",
									Name:    "Ubi Node Extension",
									Source:  "paketocommunity/run-nodejs-18-ubi-base",
									Stacks:  []string{"io.buildpacks.stacks.ubi8"},
									Version: "18.1000",
								},
								{
									ID:      "node",
									Name:    "Ubi Node Extension",
									Source:  "paketocommunity/run-nodejs-16-ubi-base",
									Stacks:  []string{"io.buildpacks.stacks.ubi8"},
									Version: "16.1000",
								},
							},
						},
					},
					SHA256: "sha256:manifest-sha",
				},
			})
			Expect(buffer).To(ContainLines(
				"## Ubi Node.js Extension 0.0.2",
				"",
				"**ID:** `paketo-community/ubi-nodejs-extension`",
				"",
				"**Digest:** `sha256:manifest-sha`",
				"",
				"#### Default Dependency Versions:",
				"| ID | Version |",
				"|---|---|",
				"| node | 20.*.* |",
				"",
				"#### Dependencies:",
				"| Name | Version | Stacks | Source |",
				"|---|---|---|---|",
				"| node | 20.1000 | io.buildpacks.stacks.ubi8 | paketocommunity/run-nodejs-20-ubi-base |",
				"| node | 18.1000 | io.buildpacks.stacks.ubi8 | paketocommunity/run-nodejs-18-ubi-base |",
				"| node | 16.1000 | io.buildpacks.stacks.ubi8 | paketocommunity/run-nodejs-16-ubi-base |",
			))
		})

		context("when dependencies and default-versions are empty", func() {
			it("returns a list of dependencies", func() {
				formatter.Markdown([]internal.ExtensionMetadata{
					{
						Config: cargo.ExtensionConfig{
							Extension: cargo.ConfigExtension{
								ID:      "paketo-community/ubi-nodejs-extension",
								Name:    "Ubi Node.js Extension",
								Version: "0.0.2",
							},
						},
						SHA256: "sha256:manifest-sha",
					},
				})
				Expect(buffer.String()).To(Equal(`## Ubi Node.js Extension 0.0.2` +

					"\n\n**ID:** `paketo-community/ubi-nodejs-extension`\n\n" +

					"**Digest:** `sha256:manifest-sha`\n\n",
				))
			})
		})

	})

	context("JSON", func() {
		it("returns a list of dependencies", func() {
			formatter.JSON([]internal.ExtensionMetadata{
				{
					Config: cargo.ExtensionConfig{
						Extension: cargo.ConfigExtension{
							ID:      "paketo-community/ubi-nodejs-extension",
							Name:    "Ubi Node.js Extension",
							Version: "0.0.2",
						},
						Metadata: cargo.ConfigExtensionMetadata{
							DefaultVersions: map[string]string{
								"node": "20.*.*",
							},
							Dependencies: []cargo.ConfigExtensionMetadataDependency{
								{
									ID:      "node",
									Name:    "Ubi Node Extension",
									Source:  "paketocommunity/run-nodejs-20-ubi-base",
									Stacks:  []string{"io.buildpacks.stacks.ubi8"},
									Version: "20.1000",
								},
								{
									ID:      "node",
									Name:    "Ubi Node Extension",
									Source:  "paketocommunity/run-nodejs-18-ubi-base",
									Stacks:  []string{"io.buildpacks.stacks.ubi8"},
									Version: "18.1000",
								},
								{
									ID:      "node",
									Name:    "Ubi Node Extension",
									Source:  "paketocommunity/run-nodejs-16-ubi-base",
									Stacks:  []string{"io.buildpacks.stacks.ubi8"},
									Version: "16.1000",
								},
							},
						},
					},
					SHA256: "sha256:manifest-sha",
				},
			})
			Expect(buffer.String()).To(MatchJSON(`{
				"buildpackage": {
				  "extension": {
					"id": "paketo-community/ubi-nodejs-extension",
					"name": "Ubi Node.js Extension",
					"version": "0.0.2"
				  },
				  "metadata": {
					"default-versions": {
					  "node": "20.*.*"
					},
					"dependencies": [
					  {
						"id": "node",
						"name": "Ubi Node Extension",
						"source": "paketocommunity/run-nodejs-20-ubi-base",
						"stacks": [
						  "io.buildpacks.stacks.ubi8"
						],
						"version": "20.1000"
					  },
					  {
						"id": "node",
						"name": "Ubi Node Extension",
						"source": "paketocommunity/run-nodejs-18-ubi-base",
						"stacks": [
						  "io.buildpacks.stacks.ubi8"
						],
						"version": "18.1000"
					  },
					  {
						"id": "node",
						"name": "Ubi Node Extension",
						"source": "paketocommunity/run-nodejs-16-ubi-base",
						"stacks": [
						  "io.buildpacks.stacks.ubi8"
						],
						"version": "16.1000"
					  }
					]
				  }
				}
			  }`))
		})
	})
}
