package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/jam/v2/integration/matchers"
	"github.com/paketo-buildpacks/occam"
)

func testUpdateDependenciesFromMetadata(t *testing.T, context spec.G, it spec.S) {
	var (
		withT      = NewWithT(t)
		Expect     = withT.Expect
		Eventually = withT.Eventually

		source string
		err    error
	)

	it.Before(func() {
		source, err = occam.Source(filepath.Join("testdata", "update-dependency-from-metadata"))
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(source)).To(Succeed())
	})

	context("when there is one new dependency to add", func() {
		it("updates the buildpack.toml dependencies from a metadata file", func() {
			command := exec.Command(
				path,
				"update-dependencies",
				"--buildpack-file", filepath.Join(source, "basic-buildpack.toml"),
				"--metadata-file", filepath.Join(source, "one-dependency-example", "metadata.json"),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			Expect(filepath.Join(source, "basic-buildpack.toml")).To(MatchTomlContent(filepath.Join(source, "one-dependency-example", "expected.toml")))
		})
	})

	context("when there is one new version with two variants for different stacks", func() {
		it("updates the buildpack.toml dependencies from a metadata file", func() {
			command := exec.Command(
				path,
				"update-dependencies",
				"--buildpack-file", filepath.Join(source, "basic-buildpack.toml"),
				"--metadata-file", filepath.Join(source, "stack-variant-example", "metadata.json"),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			Expect(filepath.Join(source, "basic-buildpack.toml")).To(MatchTomlContent(filepath.Join(source, "stack-variant-example", "expected.toml")))
		})
	})

	context("there are multiple new versions across constraints", func() {
		it("updates the buildpack.toml dependencies from a metadata file", func() {
			command := exec.Command(
				path,
				"update-dependencies",
				"--buildpack-file", filepath.Join(source, "multiple-constraints-example", "buildpack.toml"),
				"--metadata-file", filepath.Join(source, "multiple-constraints-example", "metadata.json"),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			Expect(filepath.Join(source, "multiple-constraints-example", "buildpack.toml")).To(MatchTomlContent(filepath.Join(source, "multiple-constraints-example", "expected.toml")))
		})
	})

	context("there are less new versions available than requested in constraint", func() {
		it("updates the buildpack.toml dependencies with as many as are available", func() {
			command := exec.Command(
				path,
				"update-dependencies",
				"--buildpack-file", filepath.Join(source, "too-few-available-example", "buildpack.toml"),
				"--metadata-file", filepath.Join(source, "too-few-available-example", "metadata.json"),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			Expect(filepath.Join(source, "too-few-available-example", "buildpack.toml")).To(MatchTomlContent(filepath.Join(source, "too-few-available-example", "expected.toml")))
		})
	})

	context("there are more new versions available than requested in constraint", func() {
		it("updates the buildpack.toml dependencies with the right number of patches", func() {
			command := exec.Command(
				path,
				"update-dependencies",
				"--buildpack-file", filepath.Join(source, "more-available-example", "buildpack.toml"),
				"--metadata-file", filepath.Join(source, "more-available-example", "metadata.json"),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			Expect(filepath.Join(source, "more-available-example", "buildpack.toml")).To(MatchTomlContent(filepath.Join(source, "more-available-example", "expected.toml")))
		})
	})

	context("the metadata contains a dependency with a different ID", func() {
		it("the buildpack.toml is not updated", func() {
			command := exec.Command(
				path,
				"update-dependencies",
				"--buildpack-file", filepath.Join(source, "basic-buildpack.toml"),
				"--metadata-file", filepath.Join(source, "different-dependency", "metadata.json"),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			Expect(filepath.Join(source, "basic-buildpack.toml")).To(MatchTomlContent(filepath.Join(source, "different-dependency", "expected.toml")))
		})
	})

	context("the new dependencies are out of constraints", func() {
		it("the buildpack.toml is not updated", func() {
			command := exec.Command(
				path,
				"update-dependencies",
				"--buildpack-file", filepath.Join(source, "basic-buildpack.toml"),
				"--metadata-file", filepath.Join(source, "out-of-constraint-example", "metadata.json"),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			Expect(filepath.Join(source, "basic-buildpack.toml")).To(MatchTomlContent(filepath.Join(source, "out-of-constraint-example", "expected.toml")))
		})
	})

	context("failure cases", func() {
		context("the --buildpack-file flag is missing", func() {
			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-dependencies",
					"--metadata-file", filepath.Join(source, "one-dependency-example", "metadata.json"),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("Error: required flag(s) \"buildpack-file\" not set"))
			})
		})

		context("the buildpack file does not exist", func() {
			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-dependencies",
					"--buildpack-file", "/no/such/file",
					"--metadata-file", filepath.Join(source, "one-dependency-example", "metadata.json"),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to parse buildpack.toml"))
			})
		})

		context("the metadata file does not exist", func() {
			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-dependencies",
					"--buildpack-file", filepath.Join(source, "basic-buildpack.toml"),
					"--metadata-file", "nonexistent-file",
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to open metadata.json file"))
			})
		})

		context("the metadata file cannot be JSON decoded", func() {
			var tmpDir string
			var err error

			it.Before(func() {
				tmpDir = t.TempDir()
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte("bad JSON"), 0644)).To(Succeed())
			})

			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-dependencies",
					"--buildpack-file", filepath.Join(source, "basic-buildpack.toml"),
					"--metadata-file", filepath.Join(tmpDir, "metadata.json"),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed decode metadata.json"))
			})
		})

		context("when the buildpack file cannot be opened", func() {
			it.Before(func() {
				Expect(os.Chmod(filepath.Join(source, "basic-buildpack.toml"), 0400)).To(Succeed())
			})
			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-dependencies",
					"--buildpack-file", filepath.Join(source, "basic-buildpack.toml"),
					"--metadata-file", filepath.Join(source, "one-dependency-example", "metadata.json"),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to open buildpack config"))
			})
		})
	})
}
