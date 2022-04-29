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

func testHaveFile(t *testing.T, context spec.G, it spec.S) {
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

	context("when the file exists", func() {
		it.Before(func() {
			matcher = matchers.HaveFile("/etc/os-release")
		})

		it("matches", func() {
			match, err := matcher.Match(image)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(BeTrue())
		})
	})

	context("when the file does not exist", func() {
		it.Before(func() {
			matcher = matchers.HaveFile("/no/such/file")
		})

		it("does not match", func() {
			match, err := matcher.Match(image)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(BeFalse())
		})
	})
}
