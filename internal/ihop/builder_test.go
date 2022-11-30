package ihop_test

import (
	"errors"
	"testing"

	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/sbom"
	"github.com/paketo-buildpacks/jam/internal/ihop"
	"github.com/paketo-buildpacks/jam/internal/ihop/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuilder(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		client  *fakes.ImageClient
		scanner *fakes.ImageScanner
		builder ihop.Builder
	)

	it.Before(func() {
		client = &fakes.ImageClient{}
		client.BuildCall.Returns.Image = ihop.Image{Path: "some-image-path"}

		scanner = &fakes.ImageScanner{}
		scanner.ScanCall.Returns.SBOM = ihop.NewSBOM(sbom.SBOM{
			Artifacts: sbom.Artifacts{
				LinuxDistribution: &linux.Release{ID: "some-distro-name"},
			},
		})

		builder = ihop.NewBuilder(client, scanner, 1)
	})

	it("produces an image with SBOM", func() {
		promise := builder.Execute(ihop.DefinitionImage{
			Dockerfile: "some-dockerfile",
		}, "some-platform")

		image, bom, err := promise.Resolve()
		Expect(err).NotTo(HaveOccurred())
		Expect(image).To(Equal(ihop.Image{
			Path: "some-image-path",
		}))
		Expect(bom).To(Equal(ihop.NewSBOM(sbom.SBOM{
			Artifacts: sbom.Artifacts{
				LinuxDistribution: &linux.Release{ID: "some-distro-name"},
			},
		})))

		Expect(client.BuildCall.CallCount).To(Equal(1))
		Expect(client.BuildCall.Receives.DefinitionImage).To(Equal(ihop.DefinitionImage{
			Dockerfile: "some-dockerfile",
		}))
		Expect(client.BuildCall.Receives.Platform).To(Equal("some-platform"))

		Expect(scanner.ScanCall.CallCount).To(Equal(1))
		Expect(scanner.ScanCall.Receives.Path).To(Equal("some-image-path"))
	})

	context("failure cases", func() {
		context("when the image build fails", func() {
			it.Before(func() {
				client.BuildCall.Returns.Error = errors.New("failed to build image")
			})

			it("returns an error", func() {
				promise := builder.Execute(ihop.DefinitionImage{
					Dockerfile: "some-dockerfile",
				}, "some-platform")

				_, _, err := promise.Resolve()
				Expect(err).To(MatchError("failed to build image"))
			})
		})

		context("when the image scan fails", func() {
			it.Before(func() {
				scanner.ScanCall.Returns.Error = errors.New("failed to scan image")
			})

			it("returns an error", func() {
				promise := builder.Execute(ihop.DefinitionImage{
					Dockerfile: "some-dockerfile",
				}, "some-platform")

				_, _, err := promise.Resolve()
				Expect(err).To(MatchError("failed to scan image"))
			})
		})
	})
}
