package ihop_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testCataloger(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		cataloger ihop.Cataloger
		client    ihop.Client
		dir       string
	)

	context("Scan", func() {
		it.Before(func() {
			var err error
			dir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			client, err = ihop.NewClient(dir)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(dir)).To(Succeed())
		})

		it("returns a bill of materials for an image", func() {
			err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM ubuntu:jammy\nUSER some-user:some-group"), 0600)
			Expect(err).NotTo(HaveOccurred())

			image, err := client.Build(ihop.DefinitionImage{
				Dockerfile: filepath.Join(dir, "Dockerfile"),
			}, "linux/amd64")
			Expect(err).NotTo(HaveOccurred())

			bom, err := cataloger.Scan(image.Path)
			Expect(err).NotTo(HaveOccurred())
			Expect(bom.Packages()).To(ContainElements(
				"apt",
				"dpkg",
			))
		})

		context("failure cases", func() {
			context("when the oci layout cannot be scanned", func() {
				it("returns an error", func() {
					_, err := cataloger.Scan("not a valid path")
					Expect(err).To(MatchError(ContainSubstring("an error occurred attempting to resolve")))
				})
			})
		})
	})
}
