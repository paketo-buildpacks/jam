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

func testBuildpackInspector(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		inspector                             internal.BuildpackInspector
		buildpackage                          string
		contentBp1, contentBp2, contentMetaBp []byte
	)

	it.Before(func() {
		file, err := os.CreateTemp("", "buildpackage")
		Expect(err).NotTo(HaveOccurred())

		tw := tar.NewWriter(file)

		firstBuildpack := bytes.NewBuffer(nil)
		firstBuildpackGW := gzip.NewWriter(firstBuildpack)
		firstBuildpackTW := tar.NewWriter(firstBuildpackGW)

		contentBp1 = []byte(`[buildpack]
id = "some-buildpack"
version = "1.2.3"

[metadata.default-versions]
some-dependency = "1.2.x"
other-dependency = "2.3.x"

[[metadata.dependencies]]
	id = "some-dependency"
	stacks = ["some-stack"]
	version = "1.2.3"

[[metadata.dependencies]]
	id = "other-dependency"
	stacks = ["other-stack"]
	version = "2.3.4"

[[stacks]]
	id = "some-stack"

[[stacks]]
	id = "other-stack"
`)

		err = firstBuildpackTW.WriteHeader(&tar.Header{
			Name: "./buildpack.toml",
			Mode: 0644,
			Size: int64(len(contentBp1)),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = firstBuildpackTW.Write(contentBp1)
		Expect(err).NotTo(HaveOccurred())

		Expect(firstBuildpackTW.Close()).To(Succeed())
		Expect(firstBuildpackGW.Close()).To(Succeed())

		err = tw.WriteHeader(&tar.Header{
			Name: "blobs/sha256/first-buildpack-sha",
			Mode: 0644,
			Size: int64(firstBuildpack.Len()),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = tw.Write(firstBuildpack.Bytes())
		Expect(err).NotTo(HaveOccurred())

		secondBuildpack := bytes.NewBuffer(nil)
		secondBuildpackGW := gzip.NewWriter(secondBuildpack)
		secondBuildpackTW := tar.NewWriter(secondBuildpackGW)

		contentBp2 = []byte(`[buildpack]
id = "other-buildpack"
version = "2.3.4"

[metadata.default-versions]
first-dependency = "4.5.x"
second-dependency = "5.6.x"

[[metadata.dependencies]]
	id = "first-dependency"
	stacks = ["first-stack"]
	version = "4.5.6"

[[metadata.dependencies]]
	id = "second-dependency"
	stacks = ["second-stack"]
	version = "5.6.7"

[[stacks]]
	id = "first-stack"

[[stacks]]
	id = "second-stack"
`)

		err = secondBuildpackTW.WriteHeader(&tar.Header{
			Name: "./buildpack.toml",
			Mode: 0644,
			Size: int64(len(contentBp2)),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = secondBuildpackTW.Write(contentBp2)
		Expect(err).NotTo(HaveOccurred())

		Expect(secondBuildpackTW.Close()).To(Succeed())
		Expect(secondBuildpackGW.Close()).To(Succeed())

		err = tw.WriteHeader(&tar.Header{
			Name: "blobs/sha256/second-buildpack-sha",
			Mode: 0644,
			Size: int64(secondBuildpack.Len()),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = tw.Write(secondBuildpack.Bytes())
		Expect(err).NotTo(HaveOccurred())

		thirdBuildpack := bytes.NewBuffer(nil)
		thirdBuildpackGW := gzip.NewWriter(thirdBuildpack)
		thirdBuildpackTW := tar.NewWriter(thirdBuildpackGW)

		contentMetaBp = []byte(`[buildpack]
id = "meta-buildpack"
version = "3.4.5"

[[order]]
[[order.group]]
id = "some-buildpack"
version = "1.2.3"

[[order]]
[[order.group]]
id = "other-buildpack"
version = "2.3.4"
`)

		err = thirdBuildpackTW.WriteHeader(&tar.Header{
			Name: "./buildpack.toml",
			Mode: 0644,
			Size: int64(len(contentMetaBp)),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = thirdBuildpackTW.Write(contentMetaBp)
		Expect(err).NotTo(HaveOccurred())

		Expect(thirdBuildpackTW.Close()).To(Succeed())
		Expect(thirdBuildpackGW.Close()).To(Succeed())

		err = tw.WriteHeader(&tar.Header{
			Name: "blobs/sha256/third-buildpack-sha",
			Mode: 0644,
			Size: int64(thirdBuildpack.Len()),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = tw.Write(thirdBuildpack.Bytes())
		Expect(err).NotTo(HaveOccurred())

		manifest := bytes.NewBuffer(nil)
		err = json.NewEncoder(manifest).Encode(map[string]interface{}{
			"layers": []map[string]interface{}{
				{"digest": "sha256:first-buildpack-sha"},
				{"digest": "sha256:second-buildpack-sha"},
				{"digest": "sha256:third-buildpack-sha"},
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

		inspector = internal.NewBuildpackInspector()
	})

	it.After(func() {
		Expect(os.Remove(buildpackage)).To(Succeed())
	})

	context("Dependencies", func() {
		var (
			expectedMetadata []internal.BuildpackMetadata
			buildpackageFlat string
		)

		it.Before(func() {
			expectedMetadata = []internal.BuildpackMetadata{
				{
					Config: cargo.Config{
						Buildpack: cargo.ConfigBuildpack{
							ID:      "some-buildpack",
							Version: "1.2.3",
						},
						Metadata: cargo.ConfigMetadata{
							Dependencies: []cargo.ConfigMetadataDependency{
								{
									ID:      "some-dependency",
									Stacks:  []string{"some-stack"},
									Version: "1.2.3",
								},
								{
									ID:      "other-dependency",
									Stacks:  []string{"other-stack"},
									Version: "2.3.4",
								},
							},
							DefaultVersions: map[string]string{
								"some-dependency":  "1.2.x",
								"other-dependency": "2.3.x",
							},
						},
						Stacks: []cargo.ConfigStack{
							{ID: "some-stack"},
							{ID: "other-stack"},
						},
					},
				},
				{
					Config: cargo.Config{
						Buildpack: cargo.ConfigBuildpack{
							ID:      "other-buildpack",
							Version: "2.3.4",
						},
						Metadata: cargo.ConfigMetadata{
							Dependencies: []cargo.ConfigMetadataDependency{
								{
									ID:      "first-dependency",
									Stacks:  []string{"first-stack"},
									Version: "4.5.6",
								},
								{
									ID:      "second-dependency",
									Stacks:  []string{"second-stack"},
									Version: "5.6.7",
								},
							},
							DefaultVersions: map[string]string{
								"first-dependency":  "4.5.x",
								"second-dependency": "5.6.x",
							},
						},
						Stacks: []cargo.ConfigStack{
							{ID: "first-stack"},
							{ID: "second-stack"},
						},
					},
				},
				{
					Config: cargo.Config{
						Buildpack: cargo.ConfigBuildpack{
							ID:      "meta-buildpack",
							Version: "3.4.5",
						},
						Order: []cargo.ConfigOrder{
							{
								Group: []cargo.ConfigOrderGroup{
									{
										ID:      "some-buildpack",
										Version: "1.2.3",
									},
								},
							},
							{
								Group: []cargo.ConfigOrderGroup{
									{
										ID:      "other-buildpack",
										Version: "2.3.4",
									},
								},
							},
						},
					},
					SHA256: "sha256:manifest-sha",
				},
			}
		})

		context("Unflattened buildpack", func() {
			it("returns a list of dependencies", func() {
				configs, err := inspector.Dependencies(buildpackage)
				Expect(err).NotTo(HaveOccurred())
				Expect(configs).To(Equal(expectedMetadata))
			})
		})

		context("Flattened buildpack", func() {
			it.Before(func() {
				/* Flattened buildpackage, but with the same buildpack metadata as the unflattened */
				fileFlat, err := os.CreateTemp("", "buildpackage-flattened")
				Expect(err).NotTo(HaveOccurred())

				twf := tar.NewWriter(fileFlat)

				flatLayer := bytes.NewBuffer(nil)
				flatLayerGW := gzip.NewWriter(flatLayer)
				flatLayerTW := tar.NewWriter(flatLayerGW)

				err = flatLayerTW.WriteHeader(&tar.Header{
					Name: "./buildpack-one/buildpack.toml",
					Mode: 0644,
					Size: int64(len(contentBp1)),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = flatLayerTW.Write(contentBp1)
				Expect(err).NotTo(HaveOccurred())

				err = flatLayerTW.WriteHeader(&tar.Header{
					Name: "./buildpack-two/buildpack.toml",
					Mode: 0644,
					Size: int64(len(contentBp2)),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = flatLayerTW.Write(contentBp2)
				Expect(err).NotTo(HaveOccurred())

				err = flatLayerTW.WriteHeader(&tar.Header{
					Name: "./buildpack-three-meta/buildpack.toml",
					Mode: 0644,
					Size: int64(len(contentMetaBp)),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = flatLayerTW.Write(contentMetaBp)
				Expect(err).NotTo(HaveOccurred())

				Expect(flatLayerTW.Close()).To(Succeed())
				Expect(flatLayerGW.Close()).To(Succeed())

				err = twf.WriteHeader(&tar.Header{
					Name: "blobs/sha256/all-buildpacks-flattened-layer-sha",
					Mode: 0644,
					Size: int64(flatLayer.Len()),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = twf.Write(flatLayer.Bytes())
				Expect(err).NotTo(HaveOccurred())

				manifestFlat := bytes.NewBuffer(nil)
				err = json.NewEncoder(manifestFlat).Encode(map[string]interface{}{
					"layers": []map[string]interface{}{
						{"digest": "sha256:all-buildpacks-flattened-layer-sha"},
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

				inspector = internal.NewBuildpackInspector()
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

			context("when the buildpack blob does not exist", func() {
				it.Before(func() {
					file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
					Expect(err).NotTo(HaveOccurred())

					tw := tar.NewWriter(file)

					manifest := bytes.NewBuffer(nil)
					err = json.NewEncoder(manifest).Encode(map[string]interface{}{
						"layers": []map[string]interface{}{
							{"digest": "sha256:buildpack-sha"},
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
					Expect(err).To(MatchError("failed to fetch archived file blobs/sha256/buildpack-sha"))
				})
			})

			context("when the buildpack blob is not a gziped tar", func() {
				it.Before(func() {
					file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
					Expect(err).NotTo(HaveOccurred())

					tw := tar.NewWriter(file)

					err = tw.WriteHeader(&tar.Header{
						Name: "blobs/sha256/buildpack-sha",
						Mode: 0644,
						Size: 3,
					})
					Expect(err).NotTo(HaveOccurred())

					_, err = tw.Write([]byte(`%%%`))
					Expect(err).NotTo(HaveOccurred())

					manifest := bytes.NewBuffer(nil)
					err = json.NewEncoder(manifest).Encode(map[string]interface{}{
						"layers": []map[string]interface{}{
							{"digest": "sha256:buildpack-sha"},
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

			context("when the buildpack blob does not contain a buildpack.toml", func() {
				it.Before(func() {
					file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
					Expect(err).NotTo(HaveOccurred())

					tw := tar.NewWriter(file)

					buildpack := bytes.NewBuffer(nil)
					buildpackGW := gzip.NewWriter(buildpack)
					buildpackTW := tar.NewWriter(buildpackGW)

					Expect(buildpackTW.Close()).To(Succeed())
					Expect(buildpackGW.Close()).To(Succeed())

					err = tw.WriteHeader(&tar.Header{
						Name: "blobs/sha256/buildpack-sha",
						Mode: 0644,
						Size: int64(buildpack.Len()),
					})
					Expect(err).NotTo(HaveOccurred())

					_, err = tw.Write(buildpack.Bytes())
					Expect(err).NotTo(HaveOccurred())

					manifest := bytes.NewBuffer(nil)
					err = json.NewEncoder(manifest).Encode(map[string]interface{}{
						"layers": []map[string]interface{}{
							{"digest": "sha256:buildpack-sha"},
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
					Expect(err).To(MatchError("failed to fetch archived file buildpack.toml"))
				})
			})

			context("when the buildpack.toml is malformed", func() {
				it.Before(func() {
					file, err := os.OpenFile(buildpackage, os.O_TRUNC|os.O_RDWR, 0644)
					Expect(err).NotTo(HaveOccurred())

					tw := tar.NewWriter(file)

					buildpack := bytes.NewBuffer(nil)
					buildpackGW := gzip.NewWriter(buildpack)
					buildpackTW := tar.NewWriter(buildpackGW)

					err = buildpackTW.WriteHeader(&tar.Header{
						Name: "./buildpack.toml",
						Mode: 0644,
						Size: 3,
					})
					Expect(err).NotTo(HaveOccurred())

					_, err = buildpackTW.Write([]byte(`%%%`))
					Expect(err).NotTo(HaveOccurred())

					Expect(buildpackTW.Close()).To(Succeed())
					Expect(buildpackGW.Close()).To(Succeed())

					err = tw.WriteHeader(&tar.Header{
						Name: "blobs/sha256/buildpack-sha",
						Mode: 0644,
						Size: int64(buildpack.Len()),
					})
					Expect(err).NotTo(HaveOccurred())

					_, err = tw.Write(buildpack.Bytes())
					Expect(err).NotTo(HaveOccurred())

					manifest := bytes.NewBuffer(nil)
					err = json.NewEncoder(manifest).Encode(map[string]interface{}{
						"layers": []map[string]interface{}{
							{"digest": "sha256:buildpack-sha"},
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
	})
}
