package internal_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testExtensionInspector(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect            = NewWithT(t).Expect
		inspector         internal.ExtensionInspector
		buildpackage      string
		contentExtension1 []byte
	)

	it.Before(func() {
		file, err := os.CreateTemp("", "buildpackage")
		Expect(err).NotTo(HaveOccurred())

		tw := tar.NewWriter(file)

		firstExtension := bytes.NewBuffer(nil)
		firstExtensionGW := gzip.NewWriter(firstExtension)
		firstExtensionTW := tar.NewWriter(firstExtensionGW)

		contentExtension1 = []byte(`
		api = "1.2"

		[extension]
			description = "This extension installs the appropriate nodejs runtime via dnf"
			id = "paketo-community/ubi-nodejs-extension"
			name = "Ubi Node.js Extension"
			version = "0.0.2"
			homepage = "https://example.com/extension"
			keywords = ["keyword1", "keyword2"]
			licenses = [{type = "type-1.0", uri = "https://example.com/license"}]
			sbom-formats = ["spdx", "cyclonedx"]

		[metadata]
			include-files = ["bin/generate", "bin/detect", "bin/run", "extension.toml"]
			pre-package = "./scripts/build.sh"
			[metadata.default-versions]
			node = "20.*.*"

			[[metadata.dependencies]]
			checksum = "checksum-1"
			id = "node"
			licenses = ["license-1.0"]
			name = "Ubi Node Extension"
			SHA256 = "sha256:first-dependency-sha"
			source = "paketocommunity/run-nodejs-20-ubi-base"
			source-checksum = "source-checksum-1"
			source_sha256 = "source-sha-1"
			stacks = ["io.buildpacks.stacks.ubi8"]
			uri = "https://example.com/first-dependency"
			version = "20.1000"

			[[metadata.dependencies]]
			checksum = "checksum-2"
			id = "node"
			licenses = ["license-2.0"]
			name = "Ubi Node Extension"
			SHA256 = "sha256:second-dependency-sha"
			source = "paketocommunity/run-nodejs-18-ubi-base"
			source-checksum = "source-checksum-2"
			source_sha256 = "source-sha-2"
			stacks = ["io.buildpacks.stacks.ubi8"]
			uri = "https://example.com/second-dependency"
			version = "18.1000"

			[[metadata.dependencies]]
			checksum = "checksum-3"
			id = "node"
			licenses = ["license-3.0"]
			SHA256 = "sha256:third-dependency-sha"
			source = "paketocommunity/run-nodejs-16-ubi-base"
			source-checksum = "source-checksum-3"
			source_sha256 = "source-sha-3"
			name = "Ubi Node Extension"
			stacks = ["io.buildpacks.stacks.ubi8"]
			uri = "https://example.com/third-dependency"
			version = "16.1000"
	`)

		err = firstExtensionTW.WriteHeader(&tar.Header{
			Name: "./extension.toml",
			Mode: 0644,
			Size: int64(len(contentExtension1)),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = firstExtensionTW.Write(contentExtension1)
		Expect(err).NotTo(HaveOccurred())

		Expect(firstExtensionTW.Close()).To(Succeed())
		Expect(firstExtensionGW.Close()).To(Succeed())

		err = tw.WriteHeader(&tar.Header{
			Name: "blobs/sha256/first-extension-sha",
			Mode: 0644,
			Size: int64(firstExtension.Len()),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = tw.Write(firstExtension.Bytes())
		Expect(err).NotTo(HaveOccurred())

		manifest := bytes.NewBuffer(nil)
		err = json.NewEncoder(manifest).Encode(map[string]interface{}{
			"layers": []map[string]interface{}{
				{"digest": "sha256:first-extension-sha"},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = tw.WriteHeader(&tar.Header{
			Name: "blobs/sha256/manifest-sha",
			Mode: 0644,
			Size: int64(manifest.Len()),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = tw.Write(manifest.Bytes())
		Expect(err).NotTo(HaveOccurred())

		index := bytes.NewBuffer(nil)
		err = json.NewEncoder(index).Encode(map[string]interface{}{
			"manifests": []map[string]interface{}{
				{"digest": "sha256:manifest-sha"},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = tw.WriteHeader(&tar.Header{
			Name: "index.json",
			Mode: 0644,
			Size: int64(index.Len()),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = tw.Write(index.Bytes())
		Expect(err).NotTo(HaveOccurred())

		buildpackage = file.Name()

		Expect(tw.Close()).To(Succeed())
		Expect(file.Close()).To(Succeed())

		inspector = internal.NewExtensionInspector()
	})

	it.After(func() {
		Expect(os.Remove(buildpackage)).To(Succeed())
	})

	context("Dependencies", func() {
		var (
			expectedMetadata []internal.ExtensionMetadata
			buildpackageFlat string
		)

		it.Before(func() {
			expectedMetadata = []internal.ExtensionMetadata{
				{
					Config: cargo.ExtensionConfig{
						API: "1.2",
						Extension: cargo.ConfigExtension{
							ID:          "paketo-community/ubi-nodejs-extension",
							Name:        "Ubi Node.js Extension",
							Version:     "0.0.2",
							Homepage:    "https://example.com/extension",
							Description: "This extension installs the appropriate nodejs runtime via dnf",
							Keywords:    []string{"keyword1", "keyword2"},
							Licenses: []cargo.ConfigExtensionLicense{
								{
									Type: "type-1.0",
									URI:  "https://example.com/license",
								},
							},
							SBOMFormats: []string{"spdx", "cyclonedx"},
						},
						Metadata: cargo.ConfigExtensionMetadata{
							IncludeFiles: []string{"bin/generate", "bin/detect", "bin/run", "extension.toml"},
							PrePackage:   "./scripts/build.sh",
							DefaultVersions: map[string]string{
								"node": "20.*.*",
							},
							Dependencies: []cargo.ConfigExtensionMetadataDependency{
								{
									Checksum:       "checksum-1",
									ID:             "node",
									Licenses:       []interface{}{"license-1.0"},
									Name:           "Ubi Node Extension",
									SHA256:         "sha256:first-dependency-sha",
									Source:         "paketocommunity/run-nodejs-20-ubi-base",
									SourceChecksum: "source-checksum-1",
									SourceSHA256:   "source-sha-1",
									Stacks:         []string{"io.buildpacks.stacks.ubi8"},
									URI:            "https://example.com/first-dependency",
									Version:        "20.1000",
								},
								{
									Checksum:       "checksum-2",
									ID:             "node",
									Licenses:       []interface{}{"license-2.0"},
									Name:           "Ubi Node Extension",
									SHA256:         "sha256:second-dependency-sha",
									Source:         "paketocommunity/run-nodejs-18-ubi-base",
									SourceChecksum: "source-checksum-2",
									SourceSHA256:   "source-sha-2",
									Stacks:         []string{"io.buildpacks.stacks.ubi8"},
									URI:            "https://example.com/second-dependency",
									Version:        "18.1000",
								},
								{
									Checksum:       "checksum-3",
									ID:             "node",
									Licenses:       []interface{}{"license-3.0"},
									Name:           "Ubi Node Extension",
									SHA256:         "sha256:third-dependency-sha",
									Source:         "paketocommunity/run-nodejs-16-ubi-base",
									SourceChecksum: "source-checksum-3",
									SourceSHA256:   "source-sha-3",
									Stacks:         []string{"io.buildpacks.stacks.ubi8"},
									URI:            "https://example.com/third-dependency",
									Version:        "16.1000",
								},
							},
						},
					},
					SHA256: "sha256:manifest-sha",
				},
			}
		})

		context("Unflattened extension", func() {
			it("returns a list of dependencies", func() {
				configs, err := inspector.Dependencies(buildpackage)
				Expect(err).NotTo(HaveOccurred())
				Expect(configs).To(Equal(expectedMetadata))
			})
		})

		context("Flattened extension", func() {
			it.Before(func() {
				/* Flattened buildpackage, but with the same extension metadata as the unflattened */
				fileFlat, err := os.CreateTemp("", "buildpackage-flattened")
				Expect(err).NotTo(HaveOccurred())

				twf := tar.NewWriter(fileFlat)

				flatLayer := bytes.NewBuffer(nil)
				flatLayerGW := gzip.NewWriter(flatLayer)
				flatLayerTW := tar.NewWriter(flatLayerGW)

				err = flatLayerTW.WriteHeader(&tar.Header{
					Name: "./extension-one/extension.toml",
					Mode: 0644,
					Size: int64(len(contentExtension1)),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = flatLayerTW.Write(contentExtension1)
				Expect(err).NotTo(HaveOccurred())

				Expect(flatLayerTW.Close()).To(Succeed())
				Expect(flatLayerGW.Close()).To(Succeed())

				err = twf.WriteHeader(&tar.Header{
					Name: "blobs/sha256/all-extensions-flattened-layer-sha",
					Mode: 0644,
					Size: int64(flatLayer.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = twf.Write(flatLayer.Bytes())
				Expect(err).NotTo(HaveOccurred())

				manifestFlat := bytes.NewBuffer(nil)
				err = json.NewEncoder(manifestFlat).Encode(map[string]interface{}{
					"layers": []map[string]interface{}{
						{"digest": "sha256:all-extensions-flattened-layer-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = twf.WriteHeader(&tar.Header{
					Name: "blobs/sha256/manifest-sha",
					Mode: 0644,
					Size: int64(manifestFlat.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = twf.Write(manifestFlat.Bytes())
				Expect(err).NotTo(HaveOccurred())

				indexFlat := bytes.NewBuffer(nil)
				err = json.NewEncoder(indexFlat).Encode(map[string]interface{}{
					"manifests": []map[string]interface{}{
						{"digest": "sha256:manifest-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = twf.WriteHeader(&tar.Header{
					Name: "index.json",
					Mode: 0644,
					Size: int64(indexFlat.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = twf.Write(indexFlat.Bytes())
				Expect(err).NotTo(HaveOccurred())

				buildpackageFlat = fileFlat.Name()

				Expect(twf.Close()).To(Succeed())
				Expect(fileFlat.Close()).To(Succeed())

				inspector = internal.NewExtensionInspector()
			})

			it.After(func() {
				Expect(os.Remove(buildpackageFlat)).To(Succeed())
			})

			it("returns a list of dependencies", func() {
				configs, err := inspector.Dependencies(buildpackageFlat)
				Expect(err).NotTo(HaveOccurred())
				Expect(configs).To(Equal(expectedMetadata))
			})
		})

		context("failure cases", func() {
			context("when the file cannot be opened", func() {
				it("returns an error", func() {
					_, err := inspector.Dependencies("no-real-file")
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})

			context("when the index.json does not exist", func() {
				it.Before(func() {
					err := os.Truncate(buildpackage, 0)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := inspector.Dependencies(buildpackage)
					Expect(err).To(MatchError("failed to fetch archived file index.json"))
				})
			})

			context("when the index.json is malformed", func() {
				it.Before(func() {
					file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
					Expect(err).NotTo(HaveOccurred())

					tw := tar.NewWriter(file)

					err = tw.WriteHeader(&tar.Header{
						Name: "index.json",
						Mode: 0644,
						Size: 3,
					})
					Expect(err).NotTo(HaveOccurred())

					_, err = tw.Write([]byte(`%%%`))
					Expect(err).NotTo(HaveOccurred())

					Expect(tw.Close()).To(Succeed())
					Expect(file.Close()).To(Succeed())
				})

				it("returns an error", func() {
					_, err := inspector.Dependencies(buildpackage)
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})
		})

		context("when the manifest does not exist", func() {
			it.Before(func() {
				file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
				Expect(err).NotTo(HaveOccurred())

				tw := tar.NewWriter(file)

				index := bytes.NewBuffer(nil)
				err = json.NewEncoder(index).Encode(map[string]interface{}{
					"manifests": []map[string]interface{}{
						{"digest": "sha256:manifest-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "index.json",
					Mode: 0644,
					Size: int64(index.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(index.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(tw.Close()).To(Succeed())
				Expect(file.Close()).To(Succeed())
			})

			it("returns an error", func() {
				_, err := inspector.Dependencies(buildpackage)
				Expect(err).To(MatchError("failed to fetch archived file blobs/sha256/manifest-sha"))
			})
		})

		context("when the manifest is malformed", func() {
			it.Before(func() {
				file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
				Expect(err).NotTo(HaveOccurred())

				tw := tar.NewWriter(file)

				err = tw.WriteHeader(&tar.Header{
					Name: "blobs/sha256/manifest-sha",
					Mode: 0644,
					Size: 3,
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write([]byte(`%%%`))
				Expect(err).NotTo(HaveOccurred())

				index := bytes.NewBuffer(nil)
				err = json.NewEncoder(index).Encode(map[string]interface{}{
					"manifests": []map[string]interface{}{
						{"digest": "sha256:manifest-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "index.json",
					Mode: 0644,
					Size: int64(index.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(index.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(tw.Close()).To(Succeed())
				Expect(file.Close()).To(Succeed())
			})

			it("returns an error", func() {
				_, err := inspector.Dependencies(buildpackage)
				Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
			})
		})

		context("when the extension blob does not exist", func() {
			it.Before(func() {
				file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
				Expect(err).NotTo(HaveOccurred())

				tw := tar.NewWriter(file)

				manifest := bytes.NewBuffer(nil)
				err = json.NewEncoder(manifest).Encode(map[string]interface{}{
					"layers": []map[string]interface{}{
						{"digest": "sha256:extension-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "blobs/sha256/manifest-sha",
					Mode: 0644,
					Size: int64(manifest.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(manifest.Bytes())
				Expect(err).NotTo(HaveOccurred())

				index := bytes.NewBuffer(nil)
				err = json.NewEncoder(index).Encode(map[string]interface{}{
					"manifests": []map[string]interface{}{
						{"digest": "sha256:manifest-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "index.json",
					Mode: 0644,
					Size: int64(index.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(index.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(tw.Close()).To(Succeed())
				Expect(file.Close()).To(Succeed())
			})

			it("returns an error", func() {
				_, err := inspector.Dependencies(buildpackage)
				Expect(err).To(MatchError("failed to fetch archived file blobs/sha256/extension-sha"))
			})
		})

		context("when the buildpack blob is not a gziped tar", func() {
			it.Before(func() {
				file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
				Expect(err).NotTo(HaveOccurred())

				tw := tar.NewWriter(file)

				err = tw.WriteHeader(&tar.Header{
					Name: "blobs/sha256/extension-sha",
					Mode: 0644,
					Size: 3,
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write([]byte(`%%%`))
				Expect(err).NotTo(HaveOccurred())

				manifest := bytes.NewBuffer(nil)
				err = json.NewEncoder(manifest).Encode(map[string]interface{}{
					"layers": []map[string]interface{}{
						{"digest": "sha256:extension-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "blobs/sha256/manifest-sha",
					Mode: 0644,
					Size: int64(manifest.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(manifest.Bytes())
				Expect(err).NotTo(HaveOccurred())

				index := bytes.NewBuffer(nil)
				err = json.NewEncoder(index).Encode(map[string]interface{}{
					"manifests": []map[string]interface{}{
						{"digest": "sha256:manifest-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "index.json",
					Mode: 0644,
					Size: int64(index.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(index.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(tw.Close()).To(Succeed())
				Expect(file.Close()).To(Succeed())
			})

			it("returns an error", func() {
				_, err := inspector.Dependencies(buildpackage)
				Expect(err).To(MatchError("failed to read layer blob: unexpected EOF"))
			})
		})

		context("when the extension blob does not contain a extension.toml", func() {
			it.Before(func() {
				file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
				Expect(err).NotTo(HaveOccurred())

				tw := tar.NewWriter(file)

				extension := bytes.NewBuffer(nil)
				extensionGW := gzip.NewWriter(extension)
				extensionTW := tar.NewWriter(extensionGW)

				Expect(extensionTW.Close()).To(Succeed())
				Expect(extensionGW.Close()).To(Succeed())

				err = tw.WriteHeader(&tar.Header{
					Name: "blobs/sha256/extension-sha",
					Mode: 0644,
					Size: int64(extension.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(extension.Bytes())
				Expect(err).NotTo(HaveOccurred())

				manifest := bytes.NewBuffer(nil)
				err = json.NewEncoder(manifest).Encode(map[string]interface{}{
					"layers": []map[string]interface{}{
						{"digest": "sha256:extension-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "blobs/sha256/manifest-sha",
					Mode: 0644,
					Size: int64(manifest.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(manifest.Bytes())
				Expect(err).NotTo(HaveOccurred())

				index := bytes.NewBuffer(nil)
				err = json.NewEncoder(index).Encode(map[string]interface{}{
					"manifests": []map[string]interface{}{
						{"digest": "sha256:manifest-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "index.json",
					Mode: 0644,
					Size: int64(index.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(index.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(tw.Close()).To(Succeed())
				Expect(file.Close()).To(Succeed())
			})

			it("returns an error", func() {
				_, err := inspector.Dependencies(buildpackage)
				Expect(err).To(MatchError("failed to fetch archived file extension.toml"))
			})
		})

		context("when the extension.toml is malformed", func() {
			it.Before(func() {
				file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
				Expect(err).NotTo(HaveOccurred())

				tw := tar.NewWriter(file)

				extension := bytes.NewBuffer(nil)
				extensionGW := gzip.NewWriter(extension)
				extensionTW := tar.NewWriter(extensionGW)

				err = extensionTW.WriteHeader(&tar.Header{
					Name: "./extension.toml",
					Mode: 0644,
					Size: 3,
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = extensionTW.Write([]byte(`%%%`))
				Expect(err).NotTo(HaveOccurred())

				Expect(extensionTW.Close()).To(Succeed())
				Expect(extensionGW.Close()).To(Succeed())

				err = tw.WriteHeader(&tar.Header{
					Name: "blobs/sha256/extension-sha",
					Mode: 0644,
					Size: int64(extension.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(extension.Bytes())
				Expect(err).NotTo(HaveOccurred())

				manifest := bytes.NewBuffer(nil)
				err = json.NewEncoder(manifest).Encode(map[string]interface{}{
					"layers": []map[string]interface{}{
						{"digest": "sha256:extension-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "blobs/sha256/manifest-sha",
					Mode: 0644,
					Size: int64(manifest.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(manifest.Bytes())
				Expect(err).NotTo(HaveOccurred())

				index := bytes.NewBuffer(nil)
				err = json.NewEncoder(index).Encode(map[string]interface{}{
					"manifests": []map[string]interface{}{
						{"digest": "sha256:manifest-sha"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = tw.WriteHeader(&tar.Header{
					Name: "index.json",
					Mode: 0644,
					Size: int64(index.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(index.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(tw.Close()).To(Succeed())
				Expect(file.Close()).To(Succeed())
			})

			it("returns an error", func() {
				_, err := inspector.Dependencies(buildpackage)
				Expect(err).To(MatchError(ContainSubstring("got '%' instead")))
			})
		})
	})
}
