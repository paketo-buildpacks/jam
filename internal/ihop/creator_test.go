package ihop_test

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/paketo-buildpacks/jam/v2/internal/ihop/fakes"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

type imageBuildInvocation struct {
	Def      ihop.DefinitionImage
	Platform string
}

type imageUpdateInvocation struct {
	Image ihop.Image
}

type layerCreateInvocation struct {
	Image ihop.Image
	Def   ihop.DefinitionImage
	SBOM  ihop.SBOM
}

func testCreator(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		imageBuildInvocations      []imageBuildInvocation
		imageUpdateInvocations     []imageUpdateInvocation
		userLayerCreateInvocations []layerCreateInvocation
		sbomLayerCreateInvocations []layerCreateInvocation

		buildSBOM, runSBOM ihop.SBOM

		imageClient           *fakes.ImageClient
		imageBuilder          *fakes.ImageBuilder
		userLayerCreator      *fakes.LayerCreator
		sbomLayerCreator      *fakes.LayerCreator
		osReleaseLayerCreator *fakes.LayerCreator

		creator ihop.Creator
	)

	it.Before(func() {
		imageBuildInvocations = []imageBuildInvocation{}
		imageUpdateInvocations = []imageUpdateInvocation{}
		userLayerCreateInvocations = []layerCreateInvocation{}
		sbomLayerCreateInvocations = []layerCreateInvocation{}

		var _imageDigestCount int
		imageDigest := func() string {
			_imageDigestCount++
			return fmt.Sprintf("image-digest-%d", _imageDigestCount)
		}

		imageClient = &fakes.ImageClient{}
		imageClient.UpdateCall.Stub = func(image ihop.Image) (ihop.Image, error) {
			labels := make(map[string]string)
			for key, value := range image.Labels {
				labels[key] = value
			}

			imageUpdateInvocations = append(imageUpdateInvocations, imageUpdateInvocation{
				Image: ihop.Image{
					Digest: image.Digest,
					Layers: image.Layers,
					User:   image.User,
					Env:    append([]string{}, image.Env...),
					Labels: labels,
				},
			})

			image.Digest = imageDigest()

			return image, nil
		}

		buildSBOM = ihop.NewSBOM(sbom.SBOM{
			Artifacts: sbom.Artifacts{
				LinuxDistribution: &linux.Release{
					ID:        "some-distro-name",
					VersionID: "some-distro-version",
					IDLike:    []string{"some-distro-id-like"},
				},
				Packages: pkg.NewCollection(
					pkg.Package{
						Name: "some-build-package",
						Metadata: pkg.DpkgDBEntry{
							Package:       "some-build-package",
							Version:       "1.2.3",
							Architecture:  "all",
							Source:        "some-build-package-source",
							SourceVersion: "2.3.4",
						},
					},
					pkg.Package{
						Name: "some-common-package",
						Metadata: pkg.DpkgDBEntry{
							Package:       "some-common-package",
							Version:       "2.2.2",
							Architecture:  "amd64",
							Source:        "some-common-package-source",
							SourceVersion: "2.2.2-source-ubuntu1",
						},
					},
				),
			},
		})

		runSBOM = ihop.NewSBOM(sbom.SBOM{
			Artifacts: sbom.Artifacts{
				LinuxDistribution: &linux.Release{
					ID:        "some-distro-name",
					VersionID: "some-distro-version",
					IDLike:    []string{"some-distro-id-like"},
				},
				Packages: pkg.NewCollection(
					pkg.Package{
						Name: "some-common-package",
						Metadata: pkg.DpkgDBEntry{
							Package:       "some-common-package",
							Version:       "2.2.2",
							Architecture:  "amd64",
							Source:        "some-common-package-source",
							SourceVersion: "2.2.2-source-ubuntu1",
						},
					},
					pkg.Package{
						Name: "some-run-package",
						Metadata: pkg.DpkgDBEntry{
							Package:       "some-run-package",
							Version:       "4.5.6",
							Architecture:  "all",
							Source:        "some-run-package-source",
							SourceVersion: "2:4.5.6",
						},
					},
				),
			},
		})

		imageBuilder = &fakes.ImageBuilder{}
		imageBuilder.ExecuteCall.Stub = func(def ihop.DefinitionImage, platform string) ihop.ImageBuildPromise {
			imageBuildInvocations = append(imageBuildInvocations, imageBuildInvocation{Def: def, Platform: platform})

			sboms := []ihop.SBOM{buildSBOM, runSBOM, buildSBOM, runSBOM}

			promise := &fakes.ImageBuildPromise{}
			promise.ResolveCall.Returns.Image = ihop.Image{
				Digest: imageDigest(),
				Labels: map[string]string{},
			}
			promise.ResolveCall.Returns.SBOM = sboms[imageBuilder.ExecuteCall.CallCount-1]

			return promise
		}

		userLayerCreator = &fakes.LayerCreator{}
		userLayerCreator.CreateCall.Stub = func(image ihop.Image, def ihop.DefinitionImage, sbom ihop.SBOM) (ihop.Layer, error) {
			labels := make(map[string]string)
			for key, value := range image.Labels {
				labels[key] = value
			}

			userLayerCreateInvocations = append(userLayerCreateInvocations, layerCreateInvocation{
				Image: ihop.Image{
					Digest: image.Digest,
					Layers: image.Layers,
					User:   image.User,
					Env:    append([]string{}, image.Env...),
					Labels: labels,
				},
				Def:  def,
				SBOM: sbom,
			})

			layers := []ihop.Layer{
				{DiffID: "build-user-layer-id"},
				{DiffID: "run-user-layer-id"},
				{DiffID: "build-user-layer-id"},
				{DiffID: "run-user-layer-id"},
			}

			return layers[userLayerCreator.CreateCall.CallCount-1], nil
		}

		sbomLayerCreator = &fakes.LayerCreator{}
		sbomLayerCreator.CreateCall.Stub = func(image ihop.Image, def ihop.DefinitionImage, sbom ihop.SBOM) (ihop.Layer, error) {
			labels := make(map[string]string)
			for key, value := range image.Labels {
				labels[key] = value
			}

			sbomLayerCreateInvocations = append(sbomLayerCreateInvocations, layerCreateInvocation{
				Image: ihop.Image{
					Digest: image.Digest,
					Layers: image.Layers,
					User:   image.User,
					Env:    append([]string{}, image.Env...),
					Labels: labels,
				},
				Def:  def,
				SBOM: sbom,
			})

			layers := []ihop.Layer{
				{DiffID: "sbom-layer-id"},
			}

			return layers[sbomLayerCreator.CreateCall.CallCount-1], nil
		}

		osReleaseLayerCreator = &fakes.LayerCreator{}
		osReleaseLayerCreator.CreateCall.Stub = func(image ihop.Image, def ihop.DefinitionImage, sbom ihop.SBOM) (ihop.Layer, error) {
			return ihop.Layer{
				DiffID: "os-release-layer-id",
			}, nil
		}

		clock := func() time.Time {
			return time.Date(2006, time.January, 2, 15, 4, 5, 0, time.FixedZone("UTC-7", -7*60*60))
		}

		creator = ihop.NewCreator(imageClient, imageBuilder, userLayerCreator, sbomLayerCreator, osReleaseLayerCreator, clock, scribe.NewLogger(io.Discard))
	})

	it("creates a stack", func() {
		stack, err := creator.Execute(ihop.Definition{
			ID:         "some-stack-id",
			Homepage:   "some-stack-homepage",
			Maintainer: "some-stack-maintainer",
			Platforms:  []string{"some-platform"},
			Build: ihop.DefinitionImage{
				Description: "some-stack-build-description",
				Dockerfile:  "test-base-build-dockerfile-path",
				Args: map[string]any{
					"sources":  "test-sources",
					"packages": "test-build-packages",
				},
				UID: 1234,
				GID: 2345,
			},
			Run: ihop.DefinitionImage{
				Description: "some-stack-run-description",
				Dockerfile:  "test-base-run-dockerfile-path",
				Args: map[string]any{
					"sources":  "test-sources",
					"packages": "test-run-packages",
				},
				UID: 3456,
				GID: 4567,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(stack).To(Equal(ihop.Stack{
			Build: []ihop.Image{
				{
					Digest: "image-digest-3",
					User:   "1234:2345",
					Env: []string{
						"CNB_USER_ID=1234",
						"CNB_GROUP_ID=2345",
						"CNB_STACK_ID=some-stack-id",
					},
					Labels: map[string]string{
						"io.buildpacks.stack.description":    "some-stack-build-description",
						"io.buildpacks.stack.distro.name":    "some-distro-name",
						"io.buildpacks.stack.distro.version": "some-distro-version",
						"io.buildpacks.stack.homepage":       "some-stack-homepage",
						"io.buildpacks.stack.id":             "some-stack-id",
						"io.buildpacks.stack.maintainer":     "some-stack-maintainer",
						"io.buildpacks.stack.metadata":       "{}",
						"io.buildpacks.stack.released":       "2006-01-02T15:04:05-07:00",
					},
					Layers: []ihop.Layer{
						{
							DiffID: "build-user-layer-id",
							Layer:  nil,
						},
					},
				},
			},
			Run: []ihop.Image{
				{
					Digest: "image-digest-4",
					User:   "3456:4567",
					Labels: map[string]string{
						"io.buildpacks.stack.description":    "some-stack-run-description",
						"io.buildpacks.stack.distro.name":    "some-distro-name",
						"io.buildpacks.stack.distro.version": "some-distro-version",
						"io.buildpacks.stack.homepage":       "some-stack-homepage",
						"io.buildpacks.stack.id":             "some-stack-id",
						"io.buildpacks.stack.maintainer":     "some-stack-maintainer",
						"io.buildpacks.stack.metadata":       "{}",
						"io.buildpacks.stack.released":       "2006-01-02T15:04:05-07:00",
					},
					Layers: []ihop.Layer{
						{
							DiffID: "run-user-layer-id",
							Layer:  nil,
						},
						{
							DiffID: "os-release-layer-id",
							Layer:  nil,
						},
					},
				},
			},
		}))

		Expect(imageBuilder.ExecuteCall.CallCount).To(Equal(2))
		Expect(imageBuildInvocations[0].Def).To(Equal(ihop.DefinitionImage{
			Description: "some-stack-build-description",
			Dockerfile:  "test-base-build-dockerfile-path",
			Args: map[string]any{
				"sources":  "test-sources",
				"packages": "test-build-packages",
			},
			UID: 1234,
			GID: 2345,
		}))
		Expect(imageBuildInvocations[0].Platform).To(Equal("some-platform"))
		Expect(imageBuildInvocations[1].Def).To(Equal(ihop.DefinitionImage{
			Description: "some-stack-run-description",
			Dockerfile:  "test-base-run-dockerfile-path",
			Args: map[string]any{
				"sources":  "test-sources",
				"packages": "test-run-packages",
			},
			UID: 3456,
			GID: 4567,
		}))
		Expect(imageBuildInvocations[1].Platform).To(Equal("some-platform"))

		Expect(userLayerCreator.CreateCall.CallCount).To(Equal(2))
		Expect(userLayerCreateInvocations[0].Image.Digest).To(Equal("image-digest-1"))
		Expect(userLayerCreateInvocations[0].Def).To(Equal(ihop.DefinitionImage{
			Description: "some-stack-build-description",
			Dockerfile:  "test-base-build-dockerfile-path",
			Args: map[string]any{
				"sources":  "test-sources",
				"packages": "test-build-packages",
			},
			UID: 1234,
			GID: 2345,
		}))
		Expect(userLayerCreateInvocations[0].SBOM).To(Equal(buildSBOM))
		Expect(userLayerCreateInvocations[1].Image.Digest).To(Equal("image-digest-2"))
		Expect(userLayerCreateInvocations[1].Def).To(Equal(ihop.DefinitionImage{
			Description: "some-stack-run-description",
			Dockerfile:  "test-base-run-dockerfile-path",
			Args: map[string]any{
				"sources":  "test-sources",
				"packages": "test-run-packages",
			},
			UID: 3456,
			GID: 4567,
		}))
		Expect(userLayerCreateInvocations[1].SBOM).To(Equal(runSBOM))

		Expect(imageClient.UpdateCall.CallCount).To(Equal(2))
		Expect(imageUpdateInvocations[0].Image.Digest).To(Equal("image-digest-1"))
		Expect(imageUpdateInvocations[0].Image.Labels).To(SatisfyAll(
			HaveKeyWithValue("io.buildpacks.stack.id", "some-stack-id"),
			HaveKeyWithValue("io.buildpacks.stack.description", "some-stack-build-description"),
			HaveKeyWithValue("io.buildpacks.stack.distro.name", "some-distro-name"),
			HaveKeyWithValue("io.buildpacks.stack.distro.version", "some-distro-version"),
			HaveKeyWithValue("io.buildpacks.stack.homepage", "some-stack-homepage"),
			HaveKeyWithValue("io.buildpacks.stack.maintainer", "some-stack-maintainer"),
			HaveKeyWithValue("io.buildpacks.stack.metadata", MatchJSON("{}")),
			HaveKeyWithValue("io.buildpacks.stack.released", "2006-01-02T15:04:05-07:00"),
		))
		Expect(imageUpdateInvocations[0].Image.Labels).NotTo(SatisfyAny(
			HaveKey("io.paketo.stack.packages"),
			HaveKey("io.buildpacks.stack.mixins"),
		))
		Expect(imageUpdateInvocations[0].Image.Layers).To(Equal([]ihop.Layer{
			{DiffID: "build-user-layer-id"},
		}))
		Expect(imageUpdateInvocations[0].Image.User).To(Equal("1234:2345"))
		Expect(imageUpdateInvocations[0].Image.Env).To(ContainElements(
			"CNB_USER_ID=1234",
			"CNB_GROUP_ID=2345",
			"CNB_STACK_ID=some-stack-id",
		))
		Expect(imageUpdateInvocations[1].Image.Digest).To(Equal("image-digest-2"))
		Expect(imageUpdateInvocations[1].Image.Labels).To(SatisfyAll(
			HaveKeyWithValue("io.buildpacks.stack.id", "some-stack-id"),
			HaveKeyWithValue("io.buildpacks.stack.description", "some-stack-run-description"),
			HaveKeyWithValue("io.buildpacks.stack.distro.name", "some-distro-name"),
			HaveKeyWithValue("io.buildpacks.stack.distro.version", "some-distro-version"),
			HaveKeyWithValue("io.buildpacks.stack.homepage", "some-stack-homepage"),
			HaveKeyWithValue("io.buildpacks.stack.maintainer", "some-stack-maintainer"),
			HaveKeyWithValue("io.buildpacks.stack.metadata", MatchJSON("{}")),
			HaveKeyWithValue("io.buildpacks.stack.released", "2006-01-02T15:04:05-07:00"),
		))
		Expect(imageUpdateInvocations[1].Image.Labels).NotTo(SatisfyAny(
			HaveKey("io.paketo.stack.packages"),
			HaveKey("io.buildpacks.stack.mixins"),
		))
		Expect(imageUpdateInvocations[1].Image.Layers).To(Equal([]ihop.Layer{
			{DiffID: "run-user-layer-id"},
			{DiffID: "os-release-layer-id"},
		}))
		Expect(imageUpdateInvocations[1].Image.User).To(Equal("3456:4567"))
		Expect(imageUpdateInvocations[1].Image.Env).To(BeEmpty())
	})

	context("when there are multiple platforms", func() {
		it("creates a multi-arch stack", func() {
			stack, err := creator.Execute(ihop.Definition{
				ID:        "some-stack-id",
				Name:      "Some Name",
				Platforms: []string{"some-platform", "other-platform"},
				Build: ihop.DefinitionImage{
					Dockerfile: "test-base-build-dockerfile-path",
					UID:        1234,
					GID:        2345,
				},
				Run: ihop.DefinitionImage{
					Dockerfile: "test-base-run-dockerfile-path",
					UID:        3456,
					GID:        4567,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(stack).To(Equal(ihop.Stack{
				Build: []ihop.Image{
					{
						Digest: "image-digest-3",
						User:   "1234:2345",
						Env: []string{
							"CNB_USER_ID=1234",
							"CNB_GROUP_ID=2345",
							"CNB_STACK_ID=some-stack-id",
						},
						Labels: map[string]string{
							"io.buildpacks.stack.description":    "",
							"io.buildpacks.stack.distro.name":    "some-distro-name",
							"io.buildpacks.stack.distro.version": "some-distro-version",
							"io.buildpacks.stack.homepage":       "",
							"io.buildpacks.stack.id":             "some-stack-id",
							"io.buildpacks.stack.maintainer":     "",
							"io.buildpacks.stack.metadata":       "{}",
							"io.buildpacks.stack.released":       "2006-01-02T15:04:05-07:00",
						},
						Layers: []ihop.Layer{
							{
								DiffID: "build-user-layer-id",
								Layer:  nil,
							},
						},
					},
					{
						Digest: "image-digest-7",
						User:   "1234:2345",
						Env: []string{
							"CNB_USER_ID=1234",
							"CNB_GROUP_ID=2345",
							"CNB_STACK_ID=some-stack-id",
						},
						Labels: map[string]string{
							"io.buildpacks.stack.description":    "",
							"io.buildpacks.stack.distro.name":    "some-distro-name",
							"io.buildpacks.stack.distro.version": "some-distro-version",
							"io.buildpacks.stack.homepage":       "",
							"io.buildpacks.stack.id":             "some-stack-id",
							"io.buildpacks.stack.maintainer":     "",
							"io.buildpacks.stack.metadata":       "{}",
							"io.buildpacks.stack.released":       "2006-01-02T15:04:05-07:00",
						},
						Layers: []ihop.Layer{
							{
								DiffID: "build-user-layer-id",
								Layer:  nil,
							},
						},
					},
				},
				Run: []ihop.Image{
					{
						Digest: "image-digest-4",
						User:   "3456:4567",
						Labels: map[string]string{
							"io.buildpacks.stack.description":    "",
							"io.buildpacks.stack.distro.name":    "some-distro-name",
							"io.buildpacks.stack.distro.version": "some-distro-version",
							"io.buildpacks.stack.homepage":       "",
							"io.buildpacks.stack.id":             "some-stack-id",
							"io.buildpacks.stack.maintainer":     "",
							"io.buildpacks.stack.metadata":       "{}",
							"io.buildpacks.stack.released":       "2006-01-02T15:04:05-07:00",
						},
						Layers: []ihop.Layer{
							{
								DiffID: "run-user-layer-id",
								Layer:  nil,
							},
							{
								DiffID: "os-release-layer-id",
								Layer:  nil,
							},
						},
					},
					{
						Digest: "image-digest-8",
						User:   "3456:4567",
						Labels: map[string]string{
							"io.buildpacks.stack.description":    "",
							"io.buildpacks.stack.distro.name":    "some-distro-name",
							"io.buildpacks.stack.distro.version": "some-distro-version",
							"io.buildpacks.stack.homepage":       "",
							"io.buildpacks.stack.id":             "some-stack-id",
							"io.buildpacks.stack.maintainer":     "",
							"io.buildpacks.stack.metadata":       "{}",
							"io.buildpacks.stack.released":       "2006-01-02T15:04:05-07:00",
						},
						Layers: []ihop.Layer{
							{
								DiffID: "run-user-layer-id",
								Layer:  nil,
							},
							{
								DiffID: "os-release-layer-id",
								Layer:  nil,
							},
						},
					},
				},
			}))

			Expect(imageBuilder.ExecuteCall.CallCount).To(Equal(4))
			Expect(imageBuildInvocations[0].Platform).To(Equal("some-platform"))
			Expect(imageBuildInvocations[1].Platform).To(Equal("some-platform"))
			Expect(imageBuildInvocations[2].Platform).To(Equal("other-platform"))
			Expect(imageBuildInvocations[3].Platform).To(Equal("other-platform"))

			Expect(userLayerCreator.CreateCall.CallCount).To(Equal(4))
			Expect(userLayerCreateInvocations[0].Image.Digest).To(Equal("image-digest-1"))
			Expect(userLayerCreateInvocations[1].Image.Digest).To(Equal("image-digest-2"))
			Expect(userLayerCreateInvocations[2].Image.Digest).To(Equal("image-digest-5"))
			Expect(userLayerCreateInvocations[3].Image.Digest).To(Equal("image-digest-6"))

			Expect(imageClient.UpdateCall.CallCount).To(Equal(4))
			Expect(imageUpdateInvocations[0].Image.Digest).To(Equal("image-digest-1"))
			Expect(imageUpdateInvocations[1].Image.Digest).To(Equal("image-digest-2"))
			Expect(imageUpdateInvocations[2].Image.Digest).To(Equal("image-digest-5"))
			Expect(imageUpdateInvocations[3].Image.Digest).To(Equal("image-digest-6"))
		})
	})

	context("when a legacy SBOM is requested", func() {
		it("includes it in the image labels", func() {
			_, err := creator.Execute(ihop.Definition{
				ID:         "some-stack-id",
				Homepage:   "some-stack-homepage",
				Maintainer: "some-stack-maintainer",
				Platforms:  []string{"some-platform"},
				Deprecated: ihop.DefinitionDeprecated{
					LegacySBOM: true,
				},
				Build: ihop.DefinitionImage{
					Description: "some-stack-build-description",
					Dockerfile:  "test-base-build-dockerfile-path",
					Args: map[string]any{
						"sources":  "test-sources",
						"packages": "test-build-packages",
					},
				},
				Run: ihop.DefinitionImage{
					Description: "some-stack-run-description",
					Dockerfile:  "test-base-run-dockerfile-path",
					Args: map[string]any{
						"sources":  "test-sources",
						"packages": "test-run-packages",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(imageClient.UpdateCall.CallCount).To(Equal(2))
			Expect(imageUpdateInvocations[0].Image.Digest).To(Equal("image-digest-1"))
			Expect(imageUpdateInvocations[0].Image.Labels).To(HaveKeyWithValue("io.paketo.stack.packages", MatchJSON(`[
 				{
					"name": "some-build-package",
					"version": "1.2.3",
					"arch": "all",
					"source": {
						"name": "some-build-package-source",
						"version": "2.3.4",
						"upstreamVersion": "2.3.4"
					}
				},
				{
					"name": "some-common-package",
					"version": "2.2.2",
					"arch": "amd64",
					"source": {
						"name": "some-common-package-source",
						"version": "2.2.2-source-ubuntu1",
						"upstreamVersion": "2.2.2-source"
					}
				}
			]`)))
			Expect(imageUpdateInvocations[1].Image.Digest).To(Equal("image-digest-2"))
			Expect(imageUpdateInvocations[1].Image.Labels).To(HaveKeyWithValue("io.paketo.stack.packages", MatchJSON(`[
				{
					"name": "some-common-package",
					"version": "2.2.2",
					"arch": "amd64",
					"source": {
						"name": "some-common-package-source",
						"version": "2.2.2-source-ubuntu1",
						"upstreamVersion": "2.2.2-source"
					}
				},
				{
					"name": "some-run-package",
					"version": "4.5.6",
					"arch": "all",
					"source": {
						"name": "some-run-package-source",
						"version": "2:4.5.6",
						"upstreamVersion": "4.5.6"
					}
				}
			]`)))
		})
	})

	context("when mixins is requested", func() {
		it("includes them in the image labels", func() {
			_, err := creator.Execute(ihop.Definition{
				ID:         "some-stack-id",
				Homepage:   "some-stack-homepage",
				Maintainer: "some-stack-maintainer",
				Platforms:  []string{"some-platform"},
				Deprecated: ihop.DefinitionDeprecated{
					Mixins: true,
				},
				Build: ihop.DefinitionImage{
					Description: "some-stack-build-description",
					Dockerfile:  "test-base-build-dockerfile-path",
					Args: map[string]any{
						"sources":  "test-sources",
						"packages": "test-build-packages",
					},
				},
				Run: ihop.DefinitionImage{
					Description: "some-stack-run-description",
					Dockerfile:  "test-base-run-dockerfile-path",
					Args: map[string]any{
						"sources":  "test-sources",
						"packages": "test-run-packages",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(imageClient.UpdateCall.CallCount).To(Equal(2))

			Expect(imageUpdateInvocations[0].Image.Digest).To(Equal("image-digest-1"))
			Expect(imageUpdateInvocations[0].Image.Labels).To(HaveKeyWithValue("io.buildpacks.stack.mixins", MatchJSON(`["some-common-package","build:some-build-package"]`)))

			Expect(imageUpdateInvocations[1].Image.Digest).To(Equal("image-digest-2"))
			Expect(imageUpdateInvocations[1].Image.Labels).To(HaveKeyWithValue("io.buildpacks.stack.mixins", MatchJSON(`["some-common-package","run:some-run-package"]`)))
		})
	})

	context("when an experimental SBOM layer is requested", func() {
		it.Before(func() {
			sbomLayerCreator.CreateCall.Returns.Layer = ihop.Layer{DiffID: "sbom-layer-id"}
		})

		it("attaches that layer to the run image", func() {
			_, err := creator.Execute(ihop.Definition{
				ID:                      "some-stack-id",
				Homepage:                "some-stack-homepage",
				Maintainer:              "some-stack-maintainer",
				Platforms:               []string{"some-platform"},
				IncludeExperimentalSBOM: true,
				Build: ihop.DefinitionImage{
					Description: "some-stack-build-description",
					Dockerfile:  "test-base-build-dockerfile-path",
					Args: map[string]any{
						"sources":  "test-sources",
						"packages": "test-build-packages",
					},
					UID: 1234,
					GID: 2345,
				},
				Run: ihop.DefinitionImage{
					Description: "some-stack-run-description",
					Dockerfile:  "test-base-run-dockerfile-path",
					Args: map[string]any{
						"sources":  "test-sources",
						"packages": "test-run-packages",
					},
					UID: 3456,
					GID: 4567,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(imageBuilder.ExecuteCall.CallCount).To(Equal(2))
			Expect(userLayerCreator.CreateCall.CallCount).To(Equal(2))

			Expect(imageClient.UpdateCall.CallCount).To(Equal(2))
			Expect(imageUpdateInvocations[0].Image.Digest).To(Equal("image-digest-1"))
			Expect(imageUpdateInvocations[0].Image.Labels).NotTo(HaveKey("io.buildpacks.base.sbom"))
			Expect(imageUpdateInvocations[0].Image.Layers).To(Equal([]ihop.Layer{
				{DiffID: "build-user-layer-id"},
			}))
			Expect(imageUpdateInvocations[1].Image.Digest).To(Equal("image-digest-2"))
			Expect(imageUpdateInvocations[1].Image.Labels).To(HaveKeyWithValue("io.buildpacks.base.sbom", "sbom-layer-id"))
			Expect(imageUpdateInvocations[1].Image.Layers).To(Equal([]ihop.Layer{
				{DiffID: "run-user-layer-id"},
				{DiffID: "os-release-layer-id"},
				{DiffID: "sbom-layer-id"},
			}))

			Expect(sbomLayerCreator.CreateCall.CallCount).To(Equal(1))
			Expect(sbomLayerCreateInvocations[0].Image.Digest).To(Equal("image-digest-2"))
			Expect(sbomLayerCreateInvocations[0].Image.Labels).NotTo(HaveKey("io.buildpacks.base.sbom"))
			Expect(sbomLayerCreateInvocations[0].Image.Layers).To(Equal([]ihop.Layer{
				{DiffID: "run-user-layer-id"},
				{DiffID: "os-release-layer-id"},
			}))
			Expect(sbomLayerCreateInvocations[0].Def).To(Equal(ihop.DefinitionImage{
				Description: "some-stack-run-description",
				Dockerfile:  "test-base-run-dockerfile-path",
				Args: map[string]any{
					"sources":  "test-sources",
					"packages": "test-run-packages",
				},
				UID: 3456,
				GID: 4567,
			}))
			Expect(sbomLayerCreateInvocations[0].SBOM).To(Equal(runSBOM))
		})
	})

	context("when additional labels are given", func() {
		it("adds those labels to the build and run image", func() {
			_, err := creator.Execute(ihop.Definition{
				ID:                      "some-stack-id",
				Homepage:                "some-stack-homepage",
				Maintainer:              "some-stack-maintainer",
				Platforms:               []string{"some-platform"},
				IncludeExperimentalSBOM: true,
				Build: ihop.DefinitionImage{
					Description: "some-stack-build-description",
					Dockerfile:  "test-base-build-dockerfile-path",
					Args: map[string]any{
						"sources":  "test-sources",
						"packages": "test-build-packages",
					},
					UID: 1234,
					GID: 2345,
				},
				Run: ihop.DefinitionImage{
					Description: "some-stack-run-description",
					Dockerfile:  "test-base-run-dockerfile-path",
					Args: map[string]any{
						"sources":  "test-sources",
						"packages": "test-run-packages",
					},
					UID: 3456,
					GID: 4567,
				},
				Labels: []string{"additional.label=label-value"},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(imageClient.UpdateCall.CallCount).To(Equal(2))

			Expect(imageUpdateInvocations[0].Image.Digest).To(Equal("image-digest-1"))
			Expect(imageUpdateInvocations[0].Image.Labels).To(HaveKeyWithValue("additional.label", "label-value"))

			Expect(imageUpdateInvocations[1].Image.Digest).To(Equal("image-digest-2"))
			Expect(imageUpdateInvocations[1].Image.Labels).To(HaveKeyWithValue("additional.label", "label-value"))
		})
	})

	context("failure cases", func() {
		context("when the build image promise errors", func() {
			it.Before(func() {
				promise := &fakes.ImageBuildPromise{}
				promise.ResolveCall.Returns.Error = errors.New("failed to build image: build")

				imageBuilder.ExecuteCall.Stub = nil
				imageBuilder.ExecuteCall.Returns.ImageBuildPromise = promise
			})

			it("returns an error", func() {
				_, err := creator.Execute(ihop.Definition{Platforms: []string{"some-platform"}})
				Expect(err).To(MatchError("failed to build image: build"))
			})
		})

		context("when the run image promise errors", func() {
			it.Before(func() {
				imageBuilder.ExecuteCall.Stub = func(def ihop.DefinitionImage, platform string) ihop.ImageBuildPromise {
					promise := &fakes.ImageBuildPromise{}
					if imageBuilder.ExecuteCall.CallCount == 2 {
						promise.ResolveCall.Returns.Error = errors.New("failed to build image: run")
					}
					return promise
				}
			})

			it("returns an error", func() {
				_, err := creator.Execute(ihop.Definition{Platforms: []string{"some-platform"}})
				Expect(err).To(MatchError("failed to build image: run"))
			})
		})

		context("when the build user layer creator errors", func() {
			it.Before(func() {
				userLayerCreator.CreateCall.Stub = nil
				userLayerCreator.CreateCall.Returns.Error = errors.New("failed to create build user layer")
			})

			it("returns an error", func() {
				_, err := creator.Execute(ihop.Definition{Platforms: []string{"some-platform"}})
				Expect(err).To(MatchError("failed to create build user layer"))
			})
		})

		context("when the run user layer creator errors", func() {
			it.Before(func() {
				userLayerCreator.CreateCall.Stub = func(ihop.Image, ihop.DefinitionImage, ihop.SBOM) (ihop.Layer, error) {
					if userLayerCreator.CreateCall.CallCount == 2 {
						return ihop.Layer{}, errors.New("failed to create run user layer")
					}
					return ihop.Layer{}, nil
				}
			})

			it("returns an error", func() {
				_, err := creator.Execute(ihop.Definition{Platforms: []string{"some-platform"}})
				Expect(err).To(MatchError("failed to create run user layer"))
			})
		})

		context("when the build image update errors", func() {
			it.Before(func() {
				imageClient.UpdateCall.Stub = nil
				imageClient.UpdateCall.Returns.Error = errors.New("failed to update build image")
			})

			it("returns an error", func() {
				_, err := creator.Execute(ihop.Definition{Platforms: []string{"some-platform"}})
				Expect(err).To(MatchError("failed to update build image"))
			})
		})

		context("when the run image update errors", func() {
			it.Before(func() {
				imageClient.UpdateCall.Stub = func(ihop.Image) (ihop.Image, error) {
					if imageClient.UpdateCall.CallCount == 2 {
						return ihop.Image{}, errors.New("failed to update run image")
					}
					return ihop.Image{}, nil
				}
			})

			it("returns an error", func() {
				_, err := creator.Execute(ihop.Definition{Platforms: []string{"some-platform"}})
				Expect(err).To(MatchError("failed to update run image"))
			})
		})

		context("when the sbom layer creator errors", func() {
			it.Before(func() {
				sbomLayerCreator.CreateCall.Stub = nil
				sbomLayerCreator.CreateCall.Returns.Error = errors.New("failed to create sbom layer")
			})

			it("returns an error", func() {
				_, err := creator.Execute(ihop.Definition{
					Platforms:               []string{"some-platform"},
					IncludeExperimentalSBOM: true,
				})
				Expect(err).To(MatchError("failed to create sbom layer"))
			})
		})
	})
}
