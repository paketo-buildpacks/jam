package integration_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/packit/v2/matchers"
)

func mustConfigName(t *testing.T, img v1.Image) v1.Hash {
	h, err := img.ConfigName()
	if err != nil {
		t.Fatalf("ConfigName() = %v", err)
	}
	return h
}

func mustRawManifest(t *testing.T, img remote.Taggable) []byte {
	m, err := img.RawManifest()
	if err != nil {
		t.Fatalf("RawManifest() = %v", err)
	}
	return m
}

func mustRawConfigFile(t *testing.T, img v1.Image) []byte {
	c, err := img.RawConfigFile()
	if err != nil {
		t.Fatalf("RawConfigFile() = %v", err)
	}
	return c
}

func testUpdateBuilder(t *testing.T, context spec.G, it spec.S) {
	var (
		withT      = NewWithT(t)
		Expect     = withT.Expect
		Eventually = withT.Eventually

		server     *httptest.Server
		builderDir string
	)

	it.Before(func() {
		goRef, err := name.ParseReference("index.docker.io/paketobuildpacks/go")
		Expect(err).ToNot(HaveOccurred())
		goImg, err := remote.Image(goRef)
		Expect(err).ToNot(HaveOccurred())

		nodeRef, err := name.ParseReference("paketobuildpacks/nodejs")
		Expect(err).ToNot(HaveOccurred())
		nodeImg, err := remote.Image(nodeRef)
		Expect(err).ToNot(HaveOccurred())

		extensionRef, err := name.ParseReference("paketocommunity/ubi-nodejs-extension")
		Expect(err).ToNot(HaveOccurred())
		extensionImg, err := remote.Image(extensionRef)
		Expect(err).ToNot(HaveOccurred())

		goManifestPath := "/v2/paketo-buildpacks/go/manifests/0.0.10"
		goConfigPath := fmt.Sprintf("/v2/paketo-buildpacks/go/blobs/%s", mustConfigName(t, goImg))
		goManifestReqCount := 0
		nodeManifestPath := "/v2/paketobuildpacks/nodejs/manifests/0.20.22"
		nodeConfigPath := fmt.Sprintf("/v2/paketobuildpacks/nodejs/blobs/%s", mustConfigName(t, nodeImg))
		nodeManifestReqCount := 0
		extensionManifestPath := "/v2/paketocommunity/ubi-nodejs-extension/manifests/0.0.3"
		extensionConfigPath := fmt.Sprintf("/v2/paketocommunity/ubi-nodejs-extension/blobs/%s", mustConfigName(t, extensionImg))
		extensionManifestReqCount := 0
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			if req.Method == http.MethodHead {
				http.Error(w, "NotFound", http.StatusNotFound)

				return
			}

			switch req.URL.Path {
			case "/v2/":
				w.WriteHeader(http.StatusOK)

			case "/v2/paketo-buildpacks/go/tags/list":
				w.WriteHeader(http.StatusOK)
				_, err = fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10",
								"0.20.1",
								"0.20.12",
								"latest"
							]
					}`)
				Expect(err).NotTo(HaveOccurred())

			case "/v2/paketobuildpacks/nodejs/tags/list":
				w.WriteHeader(http.StatusOK)
				_, err = fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10",
								"0.1.0",
								"0.20.2",
								"0.20.22"
							]
					}`)
				Expect(err).NotTo(HaveOccurred())

			case "/v2/paketocommunity/ubi-nodejs-extension/tags/list":
				w.WriteHeader(http.StatusOK)
				_, err = fmt.Fprintln(w, `{
						  "tags": [
								"0.0.3",
								"0.0.4"
							]
					}`)
				Expect(err).NotTo(HaveOccurred())

			case "/v2/some-repository/lifecycle/tags/list":
				w.WriteHeader(http.StatusOK)
				_, err = fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10",
								"0.20.1",
								"0.21.1",
								"latest"
							]
					}`)
				Expect(err).NotTo(HaveOccurred())

			case "/v2/somerepository/build/tags/list":
				w.WriteHeader(http.StatusOK)
				_, err = fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10-some-cnb",
								"0.20.1",
								"0.20.12-some-cnb",
								"0.20.12-other-cnb",
								"latest"
							]
					}`)
				Expect(err).NotTo(HaveOccurred())

			case goConfigPath:
				if req.Method != http.MethodGet {
					t.Errorf("Method; got %v, want %v", req.Method, http.MethodGet)
				}
				_, _ = w.Write(mustRawConfigFile(t, goImg))

			case goManifestPath:
				goManifestReqCount++
				if req.Method != http.MethodGet {
					t.Errorf("Method; got %v, want %v", req.Method, http.MethodGet)
				}
				_, _ = w.Write(mustRawManifest(t, goImg))

			case nodeConfigPath:
				if req.Method != http.MethodGet {
					t.Errorf("Method; got %v, want %v", req.Method, http.MethodGet)
				}
				_, _ = w.Write(mustRawConfigFile(t, nodeImg))

			case nodeManifestPath:
				nodeManifestReqCount++
				if req.Method != http.MethodGet {
					t.Errorf("Method; got %v, want %v", req.Method, http.MethodGet)
				}
				_, _ = w.Write(mustRawManifest(t, nodeImg))

			case extensionConfigPath:
				if req.Method != http.MethodGet {
					t.Errorf("Method; got %v, want %v", req.Method, http.MethodGet)
				}
				_, _ = w.Write(mustRawConfigFile(t, extensionImg))

			case extensionManifestPath:
				extensionManifestReqCount++
				if req.Method != http.MethodGet {
					t.Errorf("Method; got %v, want %v", req.Method, http.MethodGet)
				}
				_, _ = w.Write(mustRawManifest(t, extensionImg))

			case "/v2/some-repository/error-buildpack-id/tags/list":
				w.WriteHeader(http.StatusTeapot)

			case "/v2/some-repository/error-lifecycle/tags/list":
				w.WriteHeader(http.StatusTeapot)

			case "/v2/somerepository/error-build/tags/list":
				w.WriteHeader(http.StatusTeapot)

			case "/v2/some-repository/nonexistent-labels-id/tags/list":
				w.WriteHeader(http.StatusOK)
				_, err = fmt.Fprintln(w, `{
						  "tags": [
								"0.1.0",
								"0.2.0",
								"latest"
							]
					}`)
				Expect(err).NotTo(HaveOccurred())

			case "/v2/some-repository/nonexistent-labels-id/manifests/0.2.0":
				w.WriteHeader(http.StatusBadRequest)

			default:
				t.Fatalf("unknown path: %s", req.URL.Path)
			}
		}))

		builderDir, err = os.MkdirTemp("", "")
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
description = "Some description"

[[buildpacks]]
	uri = "docker://REGISTRY-URI/paketo-buildpacks/go:0.0.10"
  version = "0.0.10"

[[buildpacks]]
	uri = "docker://REGISTRY-URI/paketobuildpacks/nodejs:0.20.22"
  version = "0.20.22"

[[extensions]]
  id = "paketo-community/ubi-nodejs-extension"
  version = "0.0.3"
  uri = "docker://REGISTRY-URI/paketocommunity/ubi-nodejs-extension:0.0.3"

[lifecycle]
  version = "0.10.2"

[[order]]

  [[order.group]]
    id = "paketo-buildpacks/nodejs"

[[order]]

  [[order.group]]
		id = "paketo-buildpacks/go"
    version = "0.0.10"
		optional = true

[[order-extensions]]

  [[order-extensions.group]]
    id = "paketo-community/ubi-nodejs-extension"
    version = "0.0.3"

[stack]
  id = "io.paketo.stacks.some-stack"
  build-image = "REGISTRY-URI/somerepository/build:0.0.10-some-cnb"
  run-image = "REGISTRY-URI/somerepository/run:some-cnb"
  run-image-mirrors = ["REGISTRY-URI/some-repository/run:some-cnb"]

[[targets]]
	os = "linux"
	arch = "amd64"
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		server.Close()
		Expect(os.RemoveAll(builderDir)).To(Succeed())
	})

	it("updates the builder files", func() {
		command := exec.Command(
			path,
			"update-builder",
			"--builder-file", filepath.Join(builderDir, "builder.toml"),
			"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
		)

		buffer := gbytes.NewBuffer()
		session, err := gexec.Start(command, buffer, buffer)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

		builderContents, err := os.ReadFile(filepath.Join(builderDir, "builder.toml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(builderContents)).To(MatchTOML(strings.ReplaceAll(`
description = "Some description"

[[buildpacks]]
  uri = "docker://REGISTRY-URI/paketo-buildpacks/go:0.20.12"
  version = "0.20.12"

[[buildpacks]]
  uri = "docker://REGISTRY-URI/paketobuildpacks/nodejs:0.20.22"
  version = "0.20.22"

[[extensions]]
  id = "paketo-community/ubi-nodejs-extension"
  version = "0.0.4"
  uri = "docker://REGISTRY-URI/paketocommunity/ubi-nodejs-extension:0.0.4"

[lifecycle]
  version = "0.21.1"

[[order]]

  [[order.group]]
    id = "paketo-buildpacks/nodejs"

[[order]]

  [[order.group]]
    id = "paketo-buildpacks/go"
    optional = true
    version = "0.20.12"

[[order-extensions]]
  [[order-extensions.group]]
    id = "paketo-community/ubi-nodejs-extension"
    version = "0.0.4"

[stack]
  build-image = "REGISTRY-URI/somerepository/build:0.20.12-some-cnb"
  id = "io.paketo.stacks.some-stack"
  run-image = "REGISTRY-URI/somerepository/run:some-cnb"
  run-image-mirrors = ["REGISTRY-URI/some-repository/run:some-cnb"]

[[targets]]
	os = "linux"
	arch = "amd64"
			`, "REGISTRY-URI", strings.TrimPrefix(server.URL, "http://"))))
	})

	context("when the run image is set to latest", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
description = "Some description"

[lifecycle]
  version = "0.10.2"

[stack]
  id = "io.paketo.stacks.some-stack"
  build-image = "REGISTRY-URI/somerepository/build:0.0.10"
  run-image = "REGISTRY-URI/somerepository/run:latest"
  run-image-mirrors = ["another-registry-uri/somerepository/run:latest"]
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it("updates the build image as expected", func() {
			command := exec.Command(
				path,
				"update-builder",
				"--builder-file", filepath.Join(builderDir, "builder.toml"),
				"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			builderContents, err := os.ReadFile(filepath.Join(builderDir, "builder.toml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(builderContents)).To(MatchTOML(strings.ReplaceAll(`
description = "Some description"

[lifecycle]
  version = "0.21.1"

[stack]
  build-image = "REGISTRY-URI/somerepository/build:0.20.1"
  id = "io.paketo.stacks.some-stack"
  run-image = "REGISTRY-URI/somerepository/run:latest"
  run-image-mirrors = ["another-registry-uri/somerepository/run:latest"]
			`, "REGISTRY-URI", strings.TrimPrefix(server.URL, "http://"))))
		})
	})

	context("when the run image is set to a semantic version, without a tag suffix", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
description = "Some description"

[lifecycle]
  version = "0.10.2"

[stack]
  id = "io.paketo.stacks.some-stack"
  build-image = "REGISTRY-URI/somerepository/build:0.0.10"
  run-image = "REGISTRY-URI/somerepository/run:0.0.10"
  run-image-mirrors = ["SOME-OTHER-REGISTRY-URI/somerepository/run:0.0.10"]
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it("updates the build, run, and mirror images", func() {
			command := exec.Command(
				path,
				"update-builder",
				"--builder-file", filepath.Join(builderDir, "builder.toml"),
				"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			builderContents, err := os.ReadFile(filepath.Join(builderDir, "builder.toml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(builderContents)).To(MatchTOML(strings.ReplaceAll(`
description = "Some description"

[lifecycle]
  version = "0.21.1"

[stack]
  build-image = "REGISTRY-URI/somerepository/build:0.20.1"
  id = "io.paketo.stacks.some-stack"
  run-image = "REGISTRY-URI/somerepository/run:0.20.1"
  run-image-mirrors = ["SOME-OTHER-REGISTRY-URI/somerepository/run:0.20.1"]
			`, "REGISTRY-URI", strings.TrimPrefix(server.URL, "http://"))))
		})
	})

	context("when the run image is set to a semantic version, with tag suffix", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
description = "Some description"

[lifecycle]
  version = "0.10.2"

[stack]
  id = "io.paketo.stacks.some-stack"
  build-image = "REGISTRY-URI/somerepository/build:0.0.10"
  run-image = "REGISTRY-URI/somerepository/run:0.0.10-some-cnb"
  run-image-mirrors = ["SOME-OTHER-REGISTRY-URI/somerepository/run:0.0.10"]
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it("updates the build, run, and mirror images to the highest version with a tag suffix", func() {
			command := exec.Command(
				path,
				"update-builder",
				"--builder-file", filepath.Join(builderDir, "builder.toml"),
				"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
			)

			buffer := gbytes.NewBuffer()
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

			builderContents, err := os.ReadFile(filepath.Join(builderDir, "builder.toml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(builderContents)).To(MatchTOML(strings.ReplaceAll(`
description = "Some description"

[lifecycle]
  version = "0.21.1"

[stack]
  build-image = "REGISTRY-URI/somerepository/build:0.20.12-some-cnb"
  id = "io.paketo.stacks.some-stack"
  run-image = "REGISTRY-URI/somerepository/run:0.20.12-some-cnb"
	run-image-mirrors = ["SOME-OTHER-REGISTRY-URI/somerepository/run:0.20.12-some-cnb"]
			`, "REGISTRY-URI", strings.TrimPrefix(server.URL, "http://"))))
		})
	})

	context("failure cases", func() {
		context("when the --builder-file flag is missing", func() {
			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("Error: required flag(s) \"builder-file\" not set"))
			})
		})

		context("when the builder file cannot be found", func() {
			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--builder-file", "/no/such/file",
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to execute: failed to open builder config file: open /no/such/file: no such file or directory"))
			})
		})

		context("when the latest buildpack image cannot be found", func() {
			it.Before(func() {
				err := os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
[[buildpacks]]
	uri = "docker://REGISTRY-URI/some-repository/error-buildpack-id:0.0.10"
  version = "0.0.10"
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--builder-file", filepath.Join(builderDir, "builder.toml"),
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to list tags"))
			})
		})

		context("when the latest lifecycle image cannot be found", func() {
			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--builder-file", filepath.Join(builderDir, "builder.toml"),
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/error-lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to list tags"))
			})
		})

		context("when the buildpackage ID cannot be retrieved", func() {
			it.Before(func() {
				err := os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
description = "Some description"

[[buildpacks]]
	uri = "docker://REGISTRY-URI/some-repository/nonexistent-labels-id:0.2.0"
  version = "0.2.0"

[lifecycle]
  version = "0.10.2"

[[order]]

  [[order.group]]
		id = "some-repository/nonexistent-labels-id"
  	version = "0.2.0"

[stack]
  id = "io.paketo.stacks.some-stack"
	build-image = "REGISTRY-URI/somerepository/error-build:0.0.10-some-cnb"
  run-image = "REGISTRY-URI/somerepository/run:some-cnb"
  run-image-mirrors = ["REGISTRY-URI/some-repository/run:some-cnb"]
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
				Expect(err).NotTo(HaveOccurred())

			})

			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--builder-file", filepath.Join(builderDir, "builder.toml"),
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(MatchRegexp(`failed to get buildpackage ID for \d+\.\d+\.\d+\.\d+\:\d+\/some\-repository\/nonexistent\-labels\-id\:0\.2\.0\:`))
				Expect(string(buffer.Contents())).To(ContainSubstring("unexpected status code 400 Bad Request"))
			})
		})

		context("when the latest build image cannot be found", func() {
			it.Before(func() {
				err := os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
description = "Some description"

[lifecycle]
  version = "0.10.2"

[stack]
  id = "io.paketo.stacks.some-stack"
	build-image = "REGISTRY-URI/somerepository/error-build:0.0.10-some-cnb"
  run-image = "REGISTRY-URI/somerepository/run:some-cnb"
  run-image-mirrors = ["REGISTRY-URI/some-repository/run:some-cnb"]
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--builder-file", filepath.Join(builderDir, "builder.toml"),
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to list tags"))
			})
		})

		context("when no build images with a matching run image tag suffix exist", func() {
			it.Before(func() {
				err := os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
description = "Some description"

[lifecycle]
  version = "0.10.2"

[stack]
  id = "io.paketo.stacks.some-stack"
	build-image = "REGISTRY-URI/somerepository/error-build:0.0.10-some-cnb"
  run-image = "REGISTRY-URI/somerepository/run:some-random-suffix"
  run-image-mirrors = []
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--builder-file", filepath.Join(builderDir, "builder.toml"),
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to find latest build image: failed to list tags"))
			})
		})

		context("when the run image URI cannot be parsed", func() {
			it.Before(func() {
				err := os.WriteFile(filepath.Join(builderDir, "builder.toml"), bytes.ReplaceAll([]byte(`
description = "Some description"

[lifecycle]
  version = "0.10.2"

[stack]
  id = "io.paketo.stacks.some-stack"
	build-image = "REGISTRY-URI/somerepository/error-build:0.0.10-some-cnb"
	run-image = "bad uri"
  run-image-mirrors = []
			`), []byte(`REGISTRY-URI`), []byte(strings.TrimPrefix(server.URL, "http://"))), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--builder-file", filepath.Join(builderDir, "builder.toml"),
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to parse run image"))
			})
		})

		context("when the builder file cannot be overwritten", func() {
			it.Before(func() {
				err := os.Chmod(filepath.Join(builderDir, "builder.toml"), 0400)
				Expect(err).NotTo(HaveOccurred())
			})

			it("prints an error and exits non-zero", func() {
				command := exec.Command(
					path,
					"update-builder",
					"--builder-file", filepath.Join(builderDir, "builder.toml"),
					"--lifecycle-uri", fmt.Sprintf("%s/some-repository/lifecycle", strings.TrimPrefix(server.URL, "http://")),
				)

				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })
				Expect(string(buffer.Contents())).To(ContainSubstring("failed to open builder config"))
			})
		})
	})
}
