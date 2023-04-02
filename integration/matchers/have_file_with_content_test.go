package matchers_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/onsi/gomega/types"
	"github.com/paketo-buildpacks/jam/v2/integration/matchers"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testHaveFileWithContent(t *testing.T, context spec.G, it spec.S) {
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
			matcher = matchers.HaveFileWithContent("/etc/os-release", ContainSubstring("VERSION"))
		})

		it("matches", func() {
			match, err := matcher.Match(image)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(BeTrue())
		})

		context("when the content doesn't match", func() {
			it.Before(func() {
				matcher = matchers.HaveFileWithContent("/etc/os-release", ContainSubstring("no such content"))
			})

			it("does not match", func() {
				match, err := matcher.Match(image)
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(BeFalse())
			})
		})
	})

	context("when the file does not exist", func() {
		it.Before(func() {
			matcher = matchers.HaveFileWithContent("/no/such/directory", "no such content")
		})

		it("does not match", func() {
			match, err := matcher.Match(image)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(BeFalse())
		})
	})
}
