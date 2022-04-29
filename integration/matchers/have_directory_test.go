package matchers_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/onsi/gomega/types"
	"github.com/paketo-buildpacks/jam/integration/matchers"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testHaveDirectory(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		matcher types.GomegaMatcher
		image   v1.Image
	)

	it.Before(func() {
		ref, err := name.ParseReference("alpine:latest")
		Expect(err).NotTo(HaveOccurred())

		image, err = daemon.Image(ref)
		Expect(err).NotTo(HaveOccurred())
	})

	context("when the directory exists", func() {
		it.Before(func() {
			matcher = matchers.HaveDirectory("/tmp")
		})

		it("matches", func() {
			match, err := matcher.Match(image)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(BeTrue())
		})
	})

	context("when the directory does not exist", func() {
		it.Before(func() {
			matcher = matchers.HaveDirectory("/no/such/directory")
		})

		it("does not match", func() {
			match, err := matcher.Match(image)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(BeFalse())
		})
	})
}
