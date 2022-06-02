package ihop_test

import (
	"archive/tar"
	"bytes"
	ctx "context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/paketo-buildpacks/jam/internal/ihop"
	"github.com/paketo-buildpacks/packit/v2/vacation"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/jam/integration/matchers"
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
		dir, err = ioutil.TempDir("", "dockerfile-test")
		Expect(err).NotTo(HaveOccurred())

		client, err = ihop.NewClient(dir)
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		cli, err := docker.NewClientWithOpts(docker.FromEnv)
		Expect(err).NotTo(HaveOccurred())

		for _, image := range images {
			_, err = cli.ImageRemove(ctx.Background(), image.Tag, types.ImageRemoveOptions{})
			if !docker.IsErrNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}

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
LABEL testing.key=some-value
LABEL testing.build.arg.key=$test_build_arg`), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(os.RemoveAll(dir)).To(Succeed())
		})

		it("can build images", func() {
			image, err := client.Build(ihop.DefinitionImage{
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				Args:       map[string]string{"test_build_arg": "1"},
			}, "linux/arm64")
			Expect(err).NotTo(HaveOccurred())

			images = append(images, image)

			Expect(image.Tag).To(MatchRegexp(`^paketo\.io/stack/[a-z0-9]{10}$`))
			Expect(image.Labels).To(HaveKeyWithValue("testing.key", "some-value"))
			Expect(image.Labels).To(HaveKeyWithValue("testing.build.arg.key", "1"))
			Expect(image.Layers).To(HaveLen(1))
			Expect(image.OS).To(Equal("linux"))
			Expect(image.Architecture).To(Equal("arm64"))

			ref, err := name.ParseReference(image.Tag)
			Expect(err).NotTo(HaveOccurred())

			img, err := daemon.Image(ref)
			Expect(err).NotTo(HaveOccurred())

			digest, err := img.Digest()
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

				contents, err := exec.Command("docker", "run", "--rm", image.Tag, "cat", "/secret").CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(contents))

				Expect(string(contents)).To(Equal("some-secret"))
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
						Args:       map[string]string{"test_build_arg": "1"},
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
					Expect(err).To(MatchError(ContainSubstring("executor failed running")))
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

		it("can append layers", func() {
			Expect(image.Layers).To(HaveLen(1))

			buffer := bytes.NewBuffer(nil)
			tw := tar.NewWriter(buffer)
			content := []byte("some-layer-content")
			err := tw.WriteHeader(&tar.Header{
				Name: "some/file",
				Mode: 0600,
				Size: int64(len(content)),
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = tw.Write(content)
			Expect(err).NotTo(HaveOccurred())

			Expect(tw.Close()).To(Succeed())

			layer, err := tarball.LayerFromReader(buffer)
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

			ref, err := name.ParseReference(image.Tag)
			Expect(err).NotTo(HaveOccurred())

			img, err := daemon.Image(ref)
			Expect(err).NotTo(HaveOccurred())

			Expect(img).To(HaveFileWithContent("/some/file", ContainSubstring("some-layer-content")))
		})

		context("when there are two identical images to be updated", func() {
			var image2 ihop.Image

			it.Before(func() {
				contents, err := exec.Command("docker", "tag", fmt.Sprintf("%s:latest", image.Tag), "image2:latest").CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(contents))

				image2 = image
				image2.Tag = "image2"
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

			context("when the image tag cannot be parsed", func() {
				it("returns an error", func() {
					_, err := client.Update(ihop.Image{Tag: "not a valid tag"})
					Expect(err).To(MatchError(ContainSubstring("invalid reference format")))
				})
			})

			context("when the image layer diff ID is not valid", func() {
				var img ihop.Image
				it.Before(func() {
					img = image
					img.Layers[0].DiffID = "this is not a diff id"
				})
				it.After(func() {
					daemonImage, err := img.ToDaemonImage()
					Expect(err).NotTo(HaveOccurred())

					configName, _ := daemonImage.ConfigName()
					images = append(images, ihop.Image{Tag: configName.String()})
				})

				it("returns an error", func() {
					_, err := client.Update(img)
					Expect(err).To(MatchError(ContainSubstring("cannot parse hash")))
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
			defer file.Close()

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

	context("Cleanup", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\nCOPY Dockerfile .\nUSER some-user:some-group"), 0600)
			Expect(err).NotTo(HaveOccurred())

			for _, platform := range []string{"linux/amd64", "linux/arm64"} {
				image, err := client.Build(ihop.DefinitionImage{Dockerfile: filepath.Join(dir, "Dockerfile")}, platform)
				Expect(err).NotTo(HaveOccurred())

				images = append(images, image)
			}
		})

		it.After(func() {
			images = nil
		})

		it("removes the given images", func() {
			Expect(client.Cleanup(images...)).To(Succeed())

			cli, err := docker.NewClientWithOpts(docker.FromEnv)
			Expect(err).NotTo(HaveOccurred())

			for _, image := range images {
				_, _, err := cli.ImageInspectWithRaw(ctx.Background(), image.Tag)
				Expect(docker.IsErrNotFound(err)).To(BeTrue())
			}
		})
	})

	context("Image", func() {
		context("ToDaemonImage", func() {
			var image ihop.Image

			it.Before(func() {
				err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\nCOPY Dockerfile .\nUSER some-user:some-group"), 0600)
				Expect(err).NotTo(HaveOccurred())

				image, err = client.Build(ihop.DefinitionImage{Dockerfile: filepath.Join(dir, "Dockerfile")}, "linux/amd64")
				Expect(err).NotTo(HaveOccurred())

				images = append(images, image)
			})

			it("returns a v1.Image from the docker daemon", func() {
				img, err := image.ToDaemonImage()
				Expect(err).NotTo(HaveOccurred())

				digest, err := img.Digest()
				Expect(err).NotTo(HaveOccurred())
				Expect(digest.String()).To(Equal(image.Digest))
			})

			context("failure cases", func() {
				context("when the image tag cannot be parsed", func() {
					it.Before(func() {
						image.Tag = "not a valid tag"
					})

					it("returns an error", func() {
						_, err := image.ToDaemonImage()
						Expect(err).To(MatchError(ContainSubstring("could not parse reference")))
					})
				})

				context("when the image does not exist in the daemon", func() {
					it.Before(func() {
						image.Tag = "no-such-image"
					})

					it("returns an error", func() {
						_, err := image.ToDaemonImage()
						Expect(err).To(MatchError(ContainSubstring("No such image")))
					})
				})
			})
		})
	})
}
