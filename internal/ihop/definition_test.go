package ihop_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/jam/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDefinition(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("NewDefinitionFromFile", func() {
		var dir string

		it.Before(func() {
			var err error
			dir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"
homepage = "some-stack-homepage"
maintainer = "some-stack-maintainer"

platforms = ["some-stack-platform"]

[build]
	dockerfile = "some-build-dockerfile"
	description = "some-build-description"

	uid = 1234
	gid = 4321
	shell = "/bin/bash"

	[build.args]
		some-build-arg-key = "some-build-arg-value"

[run]
	dockerfile = "some-run-dockerfile"
	description = "some-run-description"

	uid = 4321
	gid = 1234
	shell = "/bin/fish"
	
	[run.args]
		some-run-arg-key = "some-run-arg-value"

[deprecated]
	legacy-sbom = true
	mixins = true
`), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(dir)).To(Succeed())
		})

		it("creates a definition from a config file", func() {
			definition, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(definition).To(Equal(ihop.Definition{
				ID:         "some-stack-id",
				Homepage:   "some-stack-homepage",
				Maintainer: "some-stack-maintainer",
				Platforms:  []string{"some-stack-platform"},
				Deprecated: ihop.DefinitionDeprecated{
					LegacySBOM: true,
					Mixins:     true,
				},
				Build: ihop.DefinitionImage{
					Args: map[string]string{
						"some-build-arg-key": "some-build-arg-value",
					},
					Description: "some-build-description",
					Dockerfile:  filepath.Join(dir, "some-build-dockerfile"),
					GID:         4321,
					Shell:       "/bin/bash",
					UID:         1234,
				},
				Run: ihop.DefinitionImage{
					Args: map[string]string{
						"some-run-arg-key": "some-run-arg-value",
					},
					Description: "some-run-description",
					Dockerfile:  filepath.Join(dir, "some-run-dockerfile"),
					GID:         1234,
					Shell:       "/bin/fish",
					UID:         4321,
				},
			}))
		})

		context("fields with defaults", func() {
			context("it has defaults", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345

[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("sets the defaults", func() {
					definition, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(definition).To(Equal(ihop.Definition{
						ID:        "some-stack-id",
						Platforms: []string{"linux/amd64"},
						Build: ihop.DefinitionImage{
							Dockerfile: filepath.Join(dir, "some-build-dockerfile"),
							UID:        1234,
							GID:        2345,
							Shell:      "/sbin/nologin",
						},
						Run: ihop.DefinitionImage{
							Dockerfile: filepath.Join(dir, "some-run-dockerfile"),
							UID:        1234,
							GID:        2345,
							Shell:      "/sbin/nologin",
						},
					}))
				})
			})
		})

		context("required fields", func() {
			context("when id is missing", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345

[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).To(MatchError("failed to parse stack descriptor: 'id' is a required field"))
				})
			})

			context("when build.dockerfile is missing", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"

[build]
	uid = 1234
	gid = 2345

[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).To(MatchError("failed to parse stack descriptor: 'build.dockerfile' is a required field"))
				})
			})

			context("when build.uid is missing", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"

[build]
	dockerfile = "some-build-dockerfile"
	gid = 2345

[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).To(MatchError("failed to parse stack descriptor: 'build.uid' is a required field"))
				})
			})

			context("when build.gid is missing", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234

[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345

`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).To(MatchError("failed to parse stack descriptor: 'build.gid' is a required field"))
				})
			})

			context("when run.dockerfile is missing", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345

[run]
	uid = 1234
	gid = 2345
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).To(MatchError("failed to parse stack descriptor: 'run.dockerfile' is a required field"))
				})
			})

			context("when run.uid is missing", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345

[run]
	dockerfile = "some-run-dockerfile"
	gid = 2345
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).To(MatchError("failed to parse stack descriptor: 'run.uid' is a required field"))
				})
			})

			context("when run.gid is missing", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345

[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).To(MatchError("failed to parse stack descriptor: 'run.gid' is a required field"))
				})
			})
		})

		context("when secrets are given", func() {
			it("includes them in the build/run image definitions", func() {
				definition, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"), "first-secret=first-value", "second-secret=second-value")
				Expect(err).NotTo(HaveOccurred())
				Expect(definition.Build.Secrets).To(Equal(map[string]string{
					"first-secret":  "first-value",
					"second-secret": "second-value",
				}))
				Expect(definition.Run.Secrets).To(Equal(map[string]string{
					"first-secret":  "first-value",
					"second-secret": "second-value",
				}))
			})
		})

		context("failure cases", func() {
			context("when the file cannot be opened", func() {
				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile("this file does not exist")
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})

			context("when the file contents cannot be parsed", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte("%%%"), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).To(MatchError(ContainSubstring("but got '%' instead")))
				})
			})

			context("when a secret is malformed", func() {
				it("returns an error", func() {
					_, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"), "this secret is malformed")
					Expect(err).To(MatchError("malformed secret: \"this secret is malformed\" must be in the form \"key=value\""))
				})
			})
		})
	})
}
