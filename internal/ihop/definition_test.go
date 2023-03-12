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
name = "some-stack-name"
homepage = "some-stack-homepage"
support-url = "some-stack-support-url"
bug-report-url = "some-stack-bug-report-url"

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

	[build.platforms."linux/arm64".args]
	    some-build-platform-arg-key = "some-build-platform-arg-value"

[run]
	dockerfile = "some-run-dockerfile"
	description = "some-run-description"

	uid = 4321
	gid = 1234
	shell = "/bin/fish"
	
	[run.args]
		some-run-arg-key = "some-run-arg-value"

	[run.platforms."linux/arm64".args]
	    some-run-platform-arg-key = "some-run-platform-arg-value"

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
				ID:           "some-stack-id",
				Name:         "some-stack-name",
				Homepage:     "some-stack-homepage",
				SupportURL:   "some-stack-support-url",
				BugReportURL: "some-stack-bug-report-url",
				Maintainer:   "some-stack-maintainer",
				Platforms:    []string{"some-stack-platform"},
				Deprecated: ihop.DefinitionDeprecated{
					LegacySBOM: true,
					Mixins:     true,
				},
				Build: ihop.DefinitionImage{
					Args: map[string]any{
						"some-build-arg-key": "some-build-arg-value",
					},
					Platforms: map[string]ihop.DefinitionImagePlatforms{
						"linux/arm64": {
							Args: map[string]any{
								"some-build-platform-arg-key": "some-build-platform-arg-value",
							},
						},
					},
					Description: "some-build-description",
					Dockerfile:  filepath.Join(dir, "some-build-dockerfile"),
					GID:         4321,
					Shell:       "/bin/bash",
					UID:         1234,
				},
				Run: ihop.DefinitionImage{
					Args: map[string]any{
						"some-run-arg-key": "some-run-arg-value",
					},
					Platforms: map[string]ihop.DefinitionImagePlatforms{
						"linux/arm64": {
							Args: map[string]any{
								"some-run-platform-arg-key": "some-run-platform-arg-value",
							},
						},
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
name = "some-stack-name"
homepage = "https://github.com/some-stack"

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
						ID:           "some-stack-id",
						Name:         "some-stack-name",
						Platforms:    []string{"linux/amd64"},
						Homepage:     "https://github.com/some-stack",
						SupportURL:   "https://github.com/some-stack/blob/main/README.md",
						BugReportURL: "https://github.com/some-stack/issues/new",
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
name = "some-stack-name"
homepage = "some-stack-homepage"

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
name = "some-stack-name"
homepage = "some-stack-homepage"

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
name = "some-stack-name"
homepage = "some-stack-homepage"

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
name = "some-stack-name"
homepage = "some-stack-homepage"

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
name = "some-stack-name"
homepage = "some-stack-homepage"

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
name = "some-stack-name"
homepage = "some-stack-homepage"

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
name = "some-stack-name"
homepage = "some-stack-homepage"

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

		context("when args are of non string types", func() {
			context("when args are slices and integers", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"
name = "some-stack-name"
homepage = "https://github.com/some-stack"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345
	[build.args]
		build-arg-slice = [
			"value1",
			"value2",
			3
		]
[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345

	[run.args]
		run-arg-int = 1
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("transforms args to string", func() {
					definition, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).NotTo(HaveOccurred())
					buildArgs, err := definition.Build.Arguments("linux/amd64")
					Expect(err).NotTo(HaveOccurred())
					Expect(buildArgs).To(Equal([]string{"build-arg-slice=value1 value2 3"}))

					runArgs, err := definition.Run.Arguments("linux/amd64")
					Expect(err).NotTo(HaveOccurred())
					Expect(runArgs).To(Equal([]string{"run-arg-int=1"}))
				})
			})

			context("when args have platform-specific overrides", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"
name = "some-stack-name"
homepage = "https://github.com/some-stack"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345

	[build.args]
		build-arg-slice = [
			"value1",
			"value2",
			3
		]

	[build.platforms."linux/arm64".args]
	    build-arg-slice = [
			"valueA",
			"valueB",
			42
		]

[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345

	[run.args]
		run-arg-int = 1

	[run.platforms."linux/arm64".args]
	    run-arg-int = 2
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns platform-specific variant", func() {
					definition, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).NotTo(HaveOccurred())

					buildArgs, err := definition.Build.Arguments("linux/amd64")
					Expect(err).NotTo(HaveOccurred())
					Expect(buildArgs).To(Equal([]string{"build-arg-slice=value1 value2 3"}))
					buildArgsArm64, err := definition.Build.Arguments("linux/arm64")
					Expect(err).NotTo(HaveOccurred())
					Expect(buildArgsArm64).To(Equal([]string{"build-arg-slice=valueA valueB 42"}))

					runArgs, err := definition.Run.Arguments("linux/amd64")
					Expect(err).NotTo(HaveOccurred())
					Expect(runArgs).To(Equal([]string{"run-arg-int=1"}))
					runArgsArm64, err := definition.Run.Arguments("linux/arm64")
					Expect(err).NotTo(HaveOccurred())
					Expect(runArgsArm64).To(Equal([]string{"run-arg-int=2"}))
				})
			})

			context("when slice contains unsupported types", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"
name = "some-stack-name"
homepage = "https://github.com/some-stack"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345
	[build.args]
		key = [false]
[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					definition, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).ToNot(HaveOccurred())

					_, err = definition.Build.Arguments("linux/amd64")
					Expect(err).To(MatchError("unsupported type bool for the argument element \"key\".0"))
				})
			})

			context("when arg is a map", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "stack.toml"), []byte(`
id = "some-stack-id"
name = "some-stack-name"
homepage = "https://github.com/some-stack"

[build]
	dockerfile = "some-build-dockerfile"
	uid = 1234
	gid = 2345
	[build.args]
		build-arg-slice = [
			"value1",
			"value2"
		]
[run]
	dockerfile = "some-run-dockerfile"
	uid = 1234
	gid = 2345

	[run.args]
		[run.args.map]
			key = "value"
`), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					definition, err := ihop.NewDefinitionFromFile(filepath.Join(dir, "stack.toml"))
					Expect(err).ToNot(HaveOccurred())

					buildArgs, err := definition.Build.Arguments("linux/amd64")
					Expect(err).NotTo(HaveOccurred())
					Expect(buildArgs).To(Equal([]string{"build-arg-slice=value1 value2"}))

					_, err = definition.Run.Arguments("linux/amd64")
					Expect(err).To(MatchError("unsupported type map[string]interface {} for the argument \"map\""))
				})
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
