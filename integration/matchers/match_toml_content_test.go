package matchers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/onsi/gomega/types"
	"github.com/paketo-buildpacks/jam/integration/matchers"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testMatchTomlContent(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect  = NewWithT(t).Expect
		matcher types.GomegaMatcher
		tmpDir  string
	)

	it.Before(func() {
		tmpDir = t.TempDir()
		Expect(os.WriteFile(filepath.Join(tmpDir, "example.toml"), []byte(`
				api = "0.2"

				[buildpack]
					id = "some-buildpack"
					name = "Some Buildpack"
					version = "some-buildpack-version"
		`), 0644)).To(Succeed())
	})

	context("when the toml files have the same content", func() {
		var matchingTomlPath string

		it.Before(func() {
			matcher = matchers.MatchTomlContent(filepath.Join(tmpDir, "example.toml"))

			matchingTomlPath = filepath.Join(tmpDir, "matching.toml")
			Expect(os.WriteFile(matchingTomlPath, []byte(`
				api = "0.2"

				[buildpack]
					id = "some-buildpack"
					name = "Some Buildpack"
					version = "some-buildpack-version"
		`), 0644)).To(Succeed())
		})

		it("matches", func() {
			match, err := matcher.Match(matchingTomlPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(BeTrue())
		})
	})

	context("when the toml files do not have the same content", func() {
		var matchingTomlPath string

		it.Before(func() {
			matcher = matchers.MatchTomlContent(filepath.Join(tmpDir, "example.toml"))

			matchingTomlPath = filepath.Join(tmpDir, "matching.toml")
			Expect(os.WriteFile(matchingTomlPath, []byte(``), 0644)).To(Succeed())
		})

		it("does not match", func() {
			match, err := matcher.Match(matchingTomlPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(BeFalse())
		})
	})

	context("failure cases", func() {
		context("when the filepath to match against is not a real filepath", func() {
			it.Before(func() {
				matcher = matchers.MatchTomlContent(filepath.Join(tmpDir, "example.toml"))
			})

			it("returns an error", func() {
				_, err := matcher.Match(123)
				Expect(err).To(MatchError(ContainSubstring("MatchTomlContent matcher expects a file path")))
			})
		})

		context("when the file to match against cannot be opened", func() {
			var matchingTomlPath string

			it.Before(func() {
				matcher = matchers.MatchTomlContent(filepath.Join(tmpDir, "example.toml"))
				matchingTomlPath = filepath.Join(tmpDir, "matching.toml")
				Expect(os.WriteFile(matchingTomlPath, []byte(``), 0000)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := matcher.Match(matchingTomlPath)
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when the expected file cannot beopened", func() {
			var matchingTomlPath string

			it.Before(func() {
				Expect(os.Chmod(filepath.Join(tmpDir, "example.toml"), 0000)).To(Succeed())
				matcher = matchers.MatchTomlContent(filepath.Join(tmpDir, "example.toml"))
				matchingTomlPath = filepath.Join(tmpDir, "matching.toml")
				Expect(os.WriteFile(matchingTomlPath, []byte(``), 0644)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := matcher.Match(matchingTomlPath)
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})
	})
}
