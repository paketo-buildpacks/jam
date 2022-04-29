package ihop_test

import (
	"testing"

	"github.com/paketo-buildpacks/jam/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testPackages(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		packages ihop.Packages
	)

	it.Before(func() {
		packages = ihop.NewPackages(
			[]string{"A", "B", "C", "D"},
			[]string{"C", "D", "E", "F"},
		)
	})

	it("returns packages filtered by their context", func() {
		Expect(packages.Intersection).To(ConsistOf("C", "D"))
		Expect(packages.BuildComplement).To(ConsistOf("build:A", "build:B"))
		Expect(packages.RunComplement).To(ConsistOf("run:E", "run:F"))
	})
}
