package ihop_test

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/paketo-buildpacks/packit/v2/vacation"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/jam/v2/integration/matchers"
)

func testClient(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		client ihop.Client
		dir    string
		images []ihop.Image
	)

	it.Before(func() {
		var err error
		dir, err = os.MkdirTemp("", "dockerfile-test")
		Expect(err).NotTo(HaveOccurred())

		client, err = ihop.NewClient(dir)
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(dir)).To(Succeed())
	})

	context("NewClient", func() {
		context("failure cases", func() {
			context("when the environment is malformed", func() {
				var dockerHost string

				it.Before(func() {
					dockerHost = os.Getenv("DOCKER_HOST")
					Expect(os.Setenv("DOCKER_HOST", "not a valid host")).To(Succeed())
				})

				it.After(func() {
					Expect(os.Unsetenv("DOCKER_HOST")).To(Succeed())

					if dockerHost != "" {
						Expect(os.Setenv("DOCKER_HOST", dockerHost)).To(Succeed())
					}
				})

				it("returns an error", func() {
					_, err := ihop.NewClient(os.TempDir())
					Expect(err).To(MatchError(ContainSubstring("unable to parse docker host")))
				})
			}, spec.Sequential())
		})
	})

	context("Build", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(`FROM scratch
COPY Dockerfile .
ARG test_build_arg
ARG test_build_slice_arg
LABEL testing.key=some-value
LABEL testing.build.arg.key=$test_build_arg
LABEL testing.build.arg.slice.key=$test_build_slice_arg`), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(dir)).To(Succeed())
		})

		it("can build images", func() {
			image, err := client.Build(ihop.DefinitionImage{
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				Args: map[string]any{
					"test_build_arg":       "1",
					"test_build_slice_arg": []string{"1", "2"},
				},
			}, "linux/arm64")
			Expect(err).NotTo(HaveOccurred())

			images = append(images, image)

			Expect(image.Labels).To(HaveKeyWithValue("testing.key", "some-value"))
			Expect(image.Labels).To(HaveKeyWithValue("testing.build.arg.key", "1"))
			Expect(image.Labels).To(HaveKeyWithValue("testing.build.arg.slice.key", "1 2"))
			Expect(image.Layers).To(HaveLen(1))
			Expect(image.OS).To(Equal("linux"))
			Expect(image.Architecture).To(Equal("arm64"))

			digest, err := image.Actual.Digest()
			Expect(err).NotTo(HaveOccurred())

			Expect(image.Digest).To(Equal(digest.String()))
		})

		context("when the build requires secrets", func() {
			it.Before(func() {
				err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(`# syntax=docker/dockerfile:experimental
FROM ubuntu:bionic
RUN --mount=type=secret,id=test-secret,dst=/temp cat /temp > /secret`), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			it("can pass secrets to docker build command", func() {
				image, err := client.Build(ihop.DefinitionImage{
					Dockerfile: filepath.Join(dir, "Dockerfile"),
					Secrets:    map[string]string{"test-secret": "some-secret"},
				}, "linux/amd64")
				Expect(err).NotTo(HaveOccurred())

				images = append(images, image)

				Expect(image.Actual).To(HaveFileWithContent("/secret", ContainSubstring("some-secret")))
			})
		})

		context("failure cases", func() {
			context("when there is an unreadable .dockerignore file", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(dir, ".dockerignore"), nil, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := client.Build(ihop.DefinitionImage{
						Dockerfile: filepath.Join(dir, "Dockerfile"),
					}, "linux/amd64")
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the platform is invalid", func() {
				it("returns an error", func() {
					_, err := client.Build(ihop.DefinitionImage{
						Dockerfile: filepath.Join(dir, "Dockerfile"),
						Args:       map[string]any{"test_build_arg": "1"},
					}, "not a valid platform")
					Expect(err).To(MatchError(ContainSubstring("failed to initiate image build")))
				})
			})

			context("when the image build fails", func() {
				it.Before(func() {
					err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\nRUN \"no such command\""), 0600)
					Expect(err).NotTo(HaveOccurred())

					client, err = ihop.NewClient(dir)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := client.Build(ihop.DefinitionImage{
						Dockerfile: filepath.Join(dir, "Dockerfile"),
					}, "linux/amd64")
					Expect(err).To(MatchError(ContainSubstring("load remote build context")))
					Expect(err).To(MatchError(ContainSubstring("RUN \"no such command\"")))
				})
			})

			context("when the layout cannot be written", func() {
				var tmp string

				it.Before(func() {
					var err error
					tmp, err = os.MkdirTemp("", "")
					Expect(err).NotTo(HaveOccurred())

					Expect(os.Chmod(tmp, 0000)).To(Succeed())

					client, err = ihop.NewClient(tmp)
					Expect(err).NotTo(HaveOccurred())
				})

				it.After(func() {
					Expect(os.RemoveAll(tmp)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := client.Build(ihop.DefinitionImage{
						Dockerfile: filepath.Join(dir, "Dockerfile"),
					}, "linux/amd64")
					Expect(err).To(MatchError(ContainSubstring("failed to write image layout")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})
		})
	})

	context("Update", func() {
		var image ihop.Image

		it.Before(func() {
			err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\nCOPY Dockerfile .\nUSER some-user:some-group"), 0600)
			Expect(err).NotTo(HaveOccurred())

			image, err = client.Build(ihop.DefinitionImage{
				Dockerfile: filepath.Join(dir, "Dockerfile"),
			}, "linux/amd64")
			Expect(err).NotTo(HaveOccurred())

			images = append(images, image)
		})

		it("can set labels", func() {
			Expect(image.Labels).To(BeEmpty())

			image.Labels["some-key"] = "some-value"

			image, err := client.Update(image)
			Expect(err).NotTo(HaveOccurred())

			images = append(images, image)

			Expect(image.Labels).To(HaveKeyWithValue("some-key", "some-value"))
		})

		it("can set the user", func() {
			Expect(image.User).To(Equal("some-user:some-group"))

			image.User = "other-user:other-group"

			image, err := client.Update(image)
			Expect(err).NotTo(HaveOccurred())

			images = append(images, image)

			Expect(image.User).To(Equal("other-user:other-group"))
		})

		it("can set env vars", func() {
			Expect(image.Env).To(Equal([]string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			}))

			image.Env = append(image.Env, "SOME_KEY=some-value")

			image, err := client.Update(image)
			Expect(err).NotTo(HaveOccurred())

			images = append(images, image)

			Expect(image.Env).To(ConsistOf([]string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"SOME_KEY=some-value",
			}))
		})

		context("when there are extra layers", func() {
			var path string

			it.Before(func() {
				tmp, err := os.CreateTemp("", "")
				Expect(err).NotTo(HaveOccurred())
				defer func() {
					Expect(tmp.Close()).To(Succeed())
				}()

				tw := tar.NewWriter(tmp)
				content := []byte("some-layer-content")
				err = tw.WriteHeader(&tar.Header{
					Name: "some/file",
					Mode: 0600,
					Size: int64(len(content)),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tw.Write(content)
				Expect(err).NotTo(HaveOccurred())

				Expect(tw.Close()).To(Succeed())

				path = tmp.Name()
			})

			it.After(func() {
				Expect(os.Remove(path)).To(Succeed())
			})

			it("can append layers", func() {
				Expect(image.Layers).To(HaveLen(1))

				layer, err := tarball.LayerFromFile(path)
				Expect(err).NotTo(HaveOccurred())

				diffID, err := layer.DiffID()
				Expect(err).NotTo(HaveOccurred())

				image.Layers = append(image.Layers, ihop.Layer{
					DiffID: diffID.String(),
					Layer:  layer,
				})

				image, err := client.Update(image)
				Expect(err).NotTo(HaveOccurred())

				images = append(images, image)

				Expect(image.Actual).To(HaveFileWithContent("/some/file", ContainSubstring("some-layer-content")))
			})
		})

		context("when there are two identical images to be updated", func() {
			var image2 ihop.Image

			it.Before(func() {
				image2 = image
				images = append(images, image2)
			})

			it("both are updated successfully", func() {
				_, err := client.Update(image)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Update(image2)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("failure cases", func() {
			context("when the image layer diff ID is not valid", func() {
				var img ihop.Image

				it.Before(func() {
					img = image
					img.Layers[0].DiffID = "this is not a diff id"
				})

				it("returns an error", func() {
					_, err := client.Update(img)
					Expect(err).To(MatchError(ContainSubstring("cannot parse hash")))
				})
			})

			context("when the image cannot be found on its path", func() {
				it.Before(func() {
					image.Path = "/this/is/a/made/up/path"
				})

				it("returns an error", func() {
					_, err := client.Update(image)
					Expect(err).To(MatchError(ContainSubstring(`could not load layout from path "/this/is/a/made/up/path"`)))
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})
		})
	})

	context("Export", func() {
		var tmpDir string

		it.Before(func() {
			err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\nCOPY Dockerfile .\nUSER some-user:some-group"), 0600)
			Expect(err).NotTo(HaveOccurred())

			for _, platform := range []string{"linux/amd64", "linux/arm64"} {
				image, err := client.Build(ihop.DefinitionImage{Dockerfile: filepath.Join(dir, "Dockerfile")}, platform)
				Expect(err).NotTo(HaveOccurred())

				images = append(images, image)
			}

			tmpDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		it("produces an OCI-archive file", func() {
			err := client.Export(filepath.Join(dir, "archive.oci"), images...)
			Expect(err).NotTo(HaveOccurred())

			file, err := os.Open(filepath.Join(dir, "archive.oci"))
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if err2 := file.Close(); err2 != nil && err == nil {
					err = err2
				}
			}()

			err = vacation.NewArchive(file).Decompress(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			path, err := layout.FromPath(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			index, err := path.ImageIndex()
			Expect(err).NotTo(HaveOccurred())

			indexManifest, err := index.IndexManifest()
			Expect(err).NotTo(HaveOccurred())

			Expect(indexManifest.Manifests).To(HaveLen(2))

			image, err := path.Image(indexManifest.Manifests[0].Digest)
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(HaveFileWithContent("/Dockerfile", ContainSubstring("USER some-user:some-group")))

			Expect(indexManifest.Manifests[0].Platform).To(Equal(&v1.Platform{
				OS:           "linux",
				Architecture: "amd64",
			}))

			image, err = path.Image(indexManifest.Manifests[1].Digest)
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(HaveFileWithContent("/Dockerfile", ContainSubstring("USER some-user:some-group")))

			Expect(indexManifest.Manifests[1].Platform).To(Equal(&v1.Platform{
				OS:           "linux",
				Architecture: "arm64",
			}))
		})

		context("failure cases", func() {
			context("when the output file cannot be created", func() {
				it("returns an error", func() {
					err := client.Export("/no/such/directory/archive.oci", images...)
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})
		})
	})

	context("Image", func() {
		context("FromImage", func() {
			var partial ihop.Image

			it.Before(func() {
				err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\nCOPY Dockerfile .\nUSER some-user:some-group"), 0600)
				Expect(err).NotTo(HaveOccurred())

				partial, err = client.Build(ihop.DefinitionImage{Dockerfile: filepath.Join(dir, "Dockerfile")}, "linux/amd64")
				Expect(err).NotTo(HaveOccurred())
			})

			it("hydrates an Image from a partially populated one", func() {
				image, err := ihop.FromImage(partial.Path, partial.Actual)
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(partial))
			})
		})
	})
}
