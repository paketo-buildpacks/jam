package ihop_test

import (
	"testing"

	"github.com/paketo-buildpacks/jam/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testCataloger(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		cataloger ihop.Cataloger
	)

	context("Scan", func() {
		it("returns a bill of materials for an image", func() {
			bom, err := cataloger.Scan("ubuntu:jammy")
			Expect(err).NotTo(HaveOccurred())
			Expect(bom.Packages()).To(ContainElements(
				"apt",
				"dpkg",
			))
		})

		context("failure cases", func() {
			context("when the tag cannot be parsed", func() {
				it("returns an error", func() {
					_, err := cataloger.Scan("not a valid tag")
					Expect(err).To(MatchError(ContainSubstring("could not fetch image")))
				})
			})
		})
	})
}
