package internal_test

import (
	"os"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/pelletier/go-toml"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuildpackUpdate(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
	)

	testCases := []struct {
		Desc          string
		BuildpackFile internal.BuildpackConfig
		PackageFile   internal.PackageConfig
	}{
		{
			Desc: "returns no updates: cnb",
			BuildpackFile: internal.BuildpackConfig{
				API: "0.8",
			},
			PackageFile: internal.PackageConfig{
				Dependencies: []internal.PackageConfigDependency{
					{
						URI: "urn:cnb:registry:paketo-buildpacks/dotnet-execute@1.0.17",
					},
					{
						URI: "./buildpack-2.cnb",
					},
				},
			},
		},
		{
			Desc: "returns no updates: empty",
			BuildpackFile: internal.BuildpackConfig{
				API: "0.8",
			},
			PackageFile: internal.PackageConfig{
				Dependencies: []internal.PackageConfigDependency{
					{
						URI: "http://example.org/my.cnb",
					},
				},
			},
		},
	}

	context("archives", func() {
		for _, tc := range testCases {
			var (
				buildpackToml string
				packageToml   string
			)

			it.Before(func() {
				pkg, err := os.CreateTemp("", "package.toml")
				Expect(err).NotTo(HaveOccurred())
				err = toml.NewEncoder(pkg).Encode(tc.PackageFile)
				Expect(err).NotTo(HaveOccurred())

				packageToml = pkg.Name()

				bp, err := os.CreateTemp("", "buildpack.toml")
				Expect(err).NotTo(HaveOccurred())
				err = toml.NewEncoder(bp).Encode(tc.PackageFile)
				Expect(err).NotTo(HaveOccurred())

				buildpackToml = bp.Name()
			})

			it.After(func() {
				Expect(os.RemoveAll(packageToml)).To(Succeed())
				Expect(os.RemoveAll(buildpackToml)).To(Succeed())
			})

			it(tc.Desc, func() {
				flags := internal.UpdateBuildpackFlags{
					API:           "https://registry.buildpacks.io/api/",
					BuildpackFile: buildpackToml,
					PackageFile:   packageToml,
					NoCNBRegistry: false,
					PatchOnly:     false,
				}
				err := internal.UpdateBuildpackRun(flags)
				Expect(err).NotTo(HaveOccurred())

				pkg, err := os.Open(packageToml)
				Expect(err).NotTo(HaveOccurred())

				var pkgAfter internal.PackageConfig
				err = toml.NewDecoder(pkg).Decode(&pkgAfter)
				Expect(err).NotTo(HaveOccurred())

				Expect(pkgAfter.Dependencies).To(Equal(tc.PackageFile.Dependencies))
			})
		}
	})
}
