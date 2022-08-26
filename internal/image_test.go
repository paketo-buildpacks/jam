package internal_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/jam/internal"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testImage(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		server       *httptest.Server
		dockerConfig string
	)

	context("FindLatestImageOnCNBRegistry", func() {
		it.Before(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

				switch req.URL.Path {
				case "/v1/buildpacks/some-ns/some-name":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
					  "latest": {
					    "version": "0.1.0",
					    "namespace": "some-ns",
					    "name": "some-name",
					    "description": "",
					    "homepage": "",
					    "licenses": null,
					    "stacks": [
								"some-stack"
					    ],
					    "id": "a52bd991-0e17-46c0-a413-229b35943765"
					  },
					  "versions": [
					    {
					      "version": "0.1.0",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/some-ns/some-name/0.1.0"
					    }
					  ]
					}`)

				case "/v1/buildpacks/paketo-buildpacks/go":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						"latest": {
							"version": "0.1.0"
						}
					}`)

				case "/v1/buildpacks/not/ok":
					w.WriteHeader(http.StatusTeapot)

				case "/v1/buildpacks/broken/response":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `%`)

				default:
					t.Fatalf("unknown path: %s", req.URL.Path)
				}
			}))
		})

		it.After(func() {
			server.Close()
		})

		it("returns the latest semver tag for the given image uri", func() {
			image, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:some-ns/some-name@some-version", server.URL)
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(Equal(internal.Image{
				Name:    "urn:cnb:registry:some-ns/some-name",
				Path:    "some-ns/some-name",
				Version: "0.1.0",
			}))
		})

		context("when the uri is an image refernce that does not conform to the CNB registry", func() {
			it("returns converts the uri", func() {
				image, err := internal.FindLatestImageOnCNBRegistry("paketo-buildpacks/go", server.URL)
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(internal.Image{
					Name:    "urn:cnb:registry:paketo-buildpacks/go",
					Path:    "paketo-buildpacks/go",
					Version: "0.1.0",
				}))
			})
		})

		context("failure cases", func() {
			context("when the url cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:some-ns/some-name@some-version", "not a valid URL")
					Expect(err).To(MatchError(ContainSubstring("unsupported protocol scheme")))
				})
			})

			context("when the get returns a not OK status", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:not/ok", server.URL)
					Expect(err).To(MatchError(ContainSubstring("unexpected response status: 418 I'm a teapot")))
				})
			})

			context("when the response body is broken json", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:broken/response", server.URL)
					Expect(err).To(MatchError(ContainSubstring("invalid character")))
				})
			})
		})
	})

	context("FindLatestImage", func() {
		it.Before(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Header.Get("Authorization") != "Basic c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk" {
					w.Header().Set("WWW-Authenticate", `Basic realm="localhost"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				switch req.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)

				case "/v2/some-org/some-repo/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10",
								"0.20.1",
								"0.20.12",
								"999999",
								"latest",
								"0.20.13-rc1"
							]
					}`)

				case "/v2/some-org/some-other-repo/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"v0.0.10",
								"some-weird-tag",
								"999999",
								"latest"
							]
					}`)

				case "/v2/some-org/error-repo/tags/list":
					w.WriteHeader(http.StatusTeapot)

				default:
					t.Fatalf("unknown path: %s", req.URL.Path)
				}
			}))

			var err error
			dockerConfig, err = os.MkdirTemp("", "docker-config")
			Expect(err).NotTo(HaveOccurred())

			contents := fmt.Sprintf(`{
				"auths": {
					%q: {
						"username": "some-username",
						"password": "some-password"
					}
				}
			}`, strings.TrimPrefix(server.URL, "http://"))

			err = os.WriteFile(filepath.Join(dockerConfig, "config.json"), []byte(contents), 0600)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Setenv("DOCKER_CONFIG", dockerConfig)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("DOCKER_CONFIG")).To(Succeed())
			Expect(os.RemoveAll(dockerConfig)).To(Succeed())
			server.Close()
		})

		it("returns the latest non-prerelease semver tag for the given image uri", func() {
			image, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/some-repo:latest", strings.TrimPrefix(server.URL, "http://")))
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo",
				Version: "0.20.12",
			}))
		})

		context("failure cases", func() {
			context("when the uri cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage("not a valid uri")
					Expect(err).To(MatchError("failed to parse image reference \"not a valid uri\": invalid reference format"))
				})
			})

			context("when the repo name cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage(fmt.Sprintf("%s/a:latest", strings.TrimPrefix(server.URL, "http://")))
					Expect(err).To(MatchError("failed to parse image repository: repository must be between 2 and 255 characters in length: a"))
				})
			})

			context("when the tags cannot be listed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/error-repo:latest", strings.TrimPrefix(server.URL, "http://")))
					Expect(err).To(MatchError(ContainSubstring("failed to list tags:")))
					Expect(err).To(MatchError(ContainSubstring("status code 418")))
				})
			})

			context("when no valid tag can be found", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/some-other-repo:latest", strings.TrimPrefix(server.URL, "http://")))
					Expect(err).To(MatchError(ContainSubstring("could not find any valid tag")))
				})
			})
		})
	}, spec.Sequential())

	context("FindLatestBuildImage", func() {
		it.Before(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Header.Get("Authorization") != "Basic c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk" {
					w.Header().Set("WWW-Authenticate", `Basic realm="localhost"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				switch req.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)

				case "/v2/some-org/some-repo-build/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10-some-cnb",
								"0.0.10",
								"0.20.1",
								"0.20.2",
								"0.20.12-some-cnb",
								"0.20.12-other-cnb",
								"999999-some-cnb",
								"latest"
							]
					}`)

				case "/v2/some-org/some-other-repo-build/tags/list":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
						  "tags": [
								"v0.0.10-some-cnb",
								"v0.20.2",
								"v0.20.12-some-cnb",
								"v0.20.12-other-cnb",
								"999999-some-cnb",
								"latest"
							]
					}`)

				case "/v2/some-org/error-repo/tags/list":
					w.WriteHeader(http.StatusTeapot)

				default:
					t.Fatalf("unknown path: %s", req.URL.Path)
				}
			}))

			var err error
			dockerConfig, err = os.MkdirTemp("", "docker-config")
			Expect(err).NotTo(HaveOccurred())

			contents := fmt.Sprintf(`{
				"auths": {
					%q: {
						"username": "some-username",
						"password": "some-password"
					}
				}
			}`, strings.TrimPrefix(server.URL, "http://"))

			err = os.WriteFile(filepath.Join(dockerConfig, "config.json"), []byte(contents), 0600)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Setenv("DOCKER_CONFIG", dockerConfig)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("DOCKER_CONFIG")).To(Succeed())
			Expect(os.RemoveAll(dockerConfig)).To(Succeed())
			server.Close()
		})

		it("for suffixed stack repos, returns the latest semver tag for the given image uri", func() {
			image, err := internal.FindLatestBuildImage(
				fmt.Sprintf("%s/some-org/some-repo-run:some-cnb", strings.TrimPrefix(server.URL, "http://")),
				fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo-build", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo-build",
				Version: "0.20.12-some-cnb",
			}))
		})

		it("for non-suffixed stack repos, returns the latest semver tag for the given image uri", func() {
			image, err := internal.FindLatestBuildImage(
				fmt.Sprintf("%s/some-org/some-repo-run:latest", strings.TrimPrefix(server.URL, "http://")),
				fmt.Sprintf("%s/some-org/some-repo-build:0.0.10", strings.TrimPrefix(server.URL, "http://")),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo-build", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo-build",
				Version: "0.20.2",
			}))
		})

		context("failure cases", func() {
			context("when the run uri cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						"not a valid uri",
						fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError("failed to parse run image reference \"not a valid uri\": invalid reference format"))
				})
			})

			context("when the run image is not tagged", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						fmt.Sprintf("%s/some-org/some-repo-run", strings.TrimPrefix(server.URL, "http://")),
						fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError("expected the run image to be tagged but it was not"))
				})
			})

			context("when the build uri cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						fmt.Sprintf("%s/some-org/some-repo-run:some-cnb", strings.TrimPrefix(server.URL, "http://")),
						"not a valid uri",
					)
					Expect(err).To(MatchError("failed to parse build image reference \"not a valid uri\": invalid reference format"))
				})
			})

			context("when the repo name cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						fmt.Sprintf("%s/some-org/some-repo-run:some-cnb", strings.TrimPrefix(server.URL, "http://")),
						fmt.Sprintf("%s/a:latest", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError("failed to parse build image repository: repository must be between 2 and 255 characters in length: a"))
				})
			})

			context("when the tags cannot be listed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						fmt.Sprintf("%s/some-org/some-repo-run:some-cnb", strings.TrimPrefix(server.URL, "http://")),
						fmt.Sprintf("%s/some-org/error-repo:latest", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError(ContainSubstring("failed to list tags:")))
					Expect(err).To(MatchError(ContainSubstring("status code 418")))
				})
			})

			context("when no valid tag can be found", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						fmt.Sprintf("%s/some-org/some-repo-run:some-cnb", strings.TrimPrefix(server.URL, "http://")),
						fmt.Sprintf("%s/some-org/some-other-repo-build:latest", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError(ContainSubstring("could not find any valid tag")))
				})
			})
		})
	}, spec.Sequential())

	context("GetBuildpackageID", func() {
		it("returns the buildpackage ID from the io.buildpacks.buildpackage.metadata image label", func() {
			id, err := internal.GetBuildpackageID("index.docker.io/paketobuildpacks/go")
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal("paketo-buildpacks/go"))
		})

		context("failure cases", func() {
			context("uri cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.GetBuildpackageID("some garbage uri")
					Expect(err).To(MatchError(ContainSubstring("could not parse reference")))
				})
			})

			context("image cannot be created from ref", func() {
				it("returns an error", func() {
					_, err := internal.GetBuildpackageID("index.docker.io/does-not-exist/go:0.5.0")
					fmt.Println(err)
					Expect(err).To(MatchError(ContainSubstring("UNAUTHORIZED: authentication required")))
				})
			})

			context("image has no buildpackage metadata label", func() {
				it("returns an error", func() {
					_, err := internal.GetBuildpackageID("index.docker.io/paketobuildpacks/builder:base")
					Expect(err).To(MatchError(ContainSubstring("could not get buildpackage id: image index.docker.io/paketobuildpacks/builder:base has no label 'io.buildpacks.buildpackage.metadata'")))
				})
			})
		})
	})
}
