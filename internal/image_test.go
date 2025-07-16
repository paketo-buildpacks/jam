package internal_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testImage(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		server       *httptest.Server
		dockerConfig string
		count        int
	)

	context("FindLatestImageOnCNBRegistry", func() {
		it.Before(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

				switch req.URL.Path {
				case "/v1/buildpacks/some-ns/some-name":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `{
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
					    },
					    {
					      "version": "0.0.3",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/some-ns/some-name/0.0.3"
					    },
					    {
					      "version": "0.0.2",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/some-ns/some-name/0.0.2"
					    },
					    {
					      "version": "0.0.1",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/some-ns/some-name/0.0.1"
					    }
					  ]
					}`)
					Expect(err).NotTo(HaveOccurred())

				case "/v1/buildpacks/no-new-patch":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `{
					  "latest": {
					    "version": "0.1.0"
					  },
					  "versions": [
					    {
					      "version": "0.1.0",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/no-new-patch/0.1.0"
					    },
					    {
					      "version": "0.0.3-rc",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/no-new-patch/0.0.3-beta.1"
					    },
					    {
					      "version": "random-version",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/no-new-patch/0.0.3-random-version"
					    },
					    {
					      "version": "0.0.2-rc",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/no-new-patch/0.0.2-rc"
					    },
					    {
					      "version": "0.0.1",
								"_link": "https://registry.buildpacks.io//api/v1/buildpacks/no-new-patch/0.0.1"
					    }
					  ]
					}`)
					Expect(err).NotTo(HaveOccurred())

				case "/v1/buildpacks/paketo-buildpacks/go":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `{
						"latest": {
							"version": "0.1.0"
						}
					}`)
					Expect(err).NotTo(HaveOccurred())

				case "/v1/buildpacks/retry-endpoint":
					if count < 1 {
						w.WriteHeader(http.StatusTooManyRequests)
						count++
					} else {
						w.WriteHeader(http.StatusOK)
						_, err := fmt.Fprintln(w, `{
						"latest": {
							"version": "0.1.0"
							}
						}`)
						Expect(err).NotTo(HaveOccurred())
					}

				case "/v1/buildpacks/not/ok":
					w.WriteHeader(http.StatusTeapot)

				case "/v1/buildpacks/broken/response":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `%`)
					Expect(err).NotTo(HaveOccurred())

				default:
					t.Fatalf("unknown path: %s", req.URL.Path)
				}
			}))
		})

		it.After(func() {
			server.Close()
		})

		it("returns the latest semver tag for the given image uri", func() {
			image, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:some-ns/some-name@some-version", server.URL, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(Equal(internal.Image{
				Name:    "urn:cnb:registry:some-ns/some-name",
				Path:    "some-ns/some-name",
				Version: "0.1.0",
			}))
		})

		context("when the uri is an image refernce that does not conform to the CNB registry", func() {
			it("returns converts the uri", func() {
				image, err := internal.FindLatestImageOnCNBRegistry("paketo-buildpacks/go", server.URL, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(internal.Image{
					Name:    "urn:cnb:registry:paketo-buildpacks/go",
					Path:    "paketo-buildpacks/go",
					Version: "0.1.0",
				}))
			})
		})

		context("when the request fails the first time", func() {
			it("retries the request", func() {
				image, err := internal.FindLatestImageOnCNBRegistry("retry-endpoint", server.URL, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(internal.Image{
					Name:    "urn:cnb:registry:retry-endpoint",
					Path:    "retry-endpoint",
					Version: "0.1.0",
				}))
			})
		})

		context("when a patch version to base the lookup off of is given", func() {
			it("returns a latest semver patch for the given image uri", func() {
				image, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:some-ns/some-name@some-version", server.URL, "0.0.1")
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(internal.Image{
					Name:    "urn:cnb:registry:some-ns/some-name",
					Path:    "some-ns/some-name",
					Version: "0.0.3",
				}))
			})

			context("there are newer patches available, but they are pre-releases or not semantically versioned", func() {
				it("returns the highest semantically versioned regular patch", func() {
					image, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:no-new-patch@0.0.1", server.URL, "0.0.1")
					Expect(err).NotTo(HaveOccurred())
					Expect(image).To(Equal(internal.Image{
						Name:    "urn:cnb:registry:no-new-patch",
						Path:    "no-new-patch",
						Version: "0.0.1",
					}))
				})
			})
		})

		context("failure cases", func() {
			context("when the url cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:some-ns/some-name@some-version", "not a valid URL", "")
					Expect(err).To(MatchError(ContainSubstring("unsupported protocol scheme")))
				})
			})

			context("when the get returns a not OK status", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:not/ok", server.URL, "")
					Expect(err).To(MatchError(ContainSubstring("unexpected response status: 418 I'm a teapot")))
				})
			})

			context("when the response body is broken json", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:broken/response", server.URL, "")
					Expect(err).To(MatchError(ContainSubstring("invalid character")))
				})
			})

			context("when a non-semantic versioned patch version to base the lookup off of if is given", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImageOnCNBRegistry("urn:cnb:registry:some-ns/some-name@some-version", server.URL, "bad-version")
					Expect(err).To(MatchError(ContainSubstring("could not get the highest patch in the bad-version line")))
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
					_, err := fmt.Fprintln(w, `{
						  "tags": [
								"0.0.9",
								"0.0.10",
								"0.20.1",
								"0.20.12",
								"999999",
								"latest",
								"0.20.13-rc1"
							]
					}`)
					Expect(err).NotTo(HaveOccurred())

				case "/v2/some-org/no-new-patch/tags/list":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `{
						  "tags": [
								"0.0.1",
								"0.0.2-bad-version",
								"0.0.3-rc",
								"0.1.0",
								"latest"
							]
					}`)
					Expect(err).NotTo(HaveOccurred())

				case "/v2/some-org/some-other-repo/tags/list":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `{
						  "tags": [
								"v0.0.10",
								"some-weird-tag",
								"999999",
								"latest"
							]
					}`)
					Expect(err).NotTo(HaveOccurred())

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
			image, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/some-repo:latest", strings.TrimPrefix(server.URL, "http://")), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo",
				Version: "0.20.12",
			}))
		})

		context("when a patch version to base the lookup off of is given", func() {
			it("returns a latest semver patch for the given image uri", func() {
				image, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/some-repo:latest", strings.TrimPrefix(server.URL, "http://")), "0.0.8")
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(internal.Image{
					Name:    fmt.Sprintf("%s/some-org/some-repo", strings.TrimPrefix(server.URL, "http://")),
					Path:    "some-org/some-repo",
					Version: "0.0.10",
				}))
			})

			context("there are newer patches available, but they are pre-releases or not semantically versioned", func() {
				it("returns the highest semantically versioned regular patch", func() {
					image, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/no-new-patch:0.0.1", strings.TrimPrefix(server.URL, "http://")), "0.0.1")
					Expect(err).NotTo(HaveOccurred())
					Expect(image).To(Equal(internal.Image{
						Name:    fmt.Sprintf("%s/some-org/no-new-patch", strings.TrimPrefix(server.URL, "http://")),
						Path:    "some-org/no-new-patch",
						Version: "0.0.1",
					}))
				})
			})
		})

		context("failure cases", func() {
			context("when the uri cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage("not a valid uri", "")
					Expect(err).To(MatchError("failed to parse image reference \"not a valid uri\": invalid reference format"))
				})
			})

			context("when the repo name cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage(fmt.Sprintf("%s/a:latest", strings.TrimPrefix(server.URL, "http://")), "")
					Expect(err).To(MatchError("failed to parse image repository: repository must be between 2 and 255 characters in length: a"))
				})
			})

			context("when the tags cannot be listed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/error-repo:latest", strings.TrimPrefix(server.URL, "http://")), "")
					Expect(err).To(MatchError(ContainSubstring("failed to list tags:")))
					Expect(err).To(MatchError(ContainSubstring("status code 418")))
				})
			})

			context("when no valid tag can be found", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/some-other-repo:latest", strings.TrimPrefix(server.URL, "http://")), "")

					Expect(err).To(MatchError(ContainSubstring("could not find any valid tag")))
				})
			})

			context("when a non-semantic versioned patch version to base the lookup off of if is given", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestImage(fmt.Sprintf("%s/some-org/some-repo:latest", strings.TrimPrefix(server.URL, "http://")), "bad-version")
					Expect(err).To(MatchError(ContainSubstring("could not get the highest patch in the bad-version line")))
				})
			})
		})
	}, spec.Sequential())

	context("FindLatestStackImages", func() {
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
					_, err := fmt.Fprintln(w, `{
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
					Expect(err).NotTo(HaveOccurred())

				case "/v2/some-org/some-other-repo-build/tags/list":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `{
						  "tags": [
								"v0.0.10-some-cnb",
								"v0.20.2",
								"v0.20.12-some-cnb",
								"v0.20.12-other-cnb",
								"999999-some-cnb",
								"latest"
							]
					}`)
					Expect(err).NotTo(HaveOccurred())

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

		it("for suffixed stack repos, it returns the latest semver tag for both build and run images", func() {
			runImage, buildImage, err := internal.FindLatestStackImages(
				fmt.Sprintf("%s/some-org/some-repo-run:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
				fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(runImage).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo-run", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo-run",
				Version: "0.20.12-some-cnb",
			}))
			Expect(buildImage).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo-build", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo-build",
				Version: "0.20.12-some-cnb",
			}))
		})

		it("for suffixed stack repos, when the run image is not semver, it returns the latest semver tag for the build image only", func() {
			runImage, buildImage, err := internal.FindLatestStackImages(
				fmt.Sprintf("%s/some-org/some-repo-run:some-cnb", strings.TrimPrefix(server.URL, "http://")),
				fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(runImage).To(Equal(internal.Image{}))
			Expect(buildImage).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo-build", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo-build",
				Version: "0.20.12-some-cnb",
			}))
		})

		it("for non-suffixed stack repos, when the run image is semver, it returns the latest semver tag for both build and run images", func() {
			runImage, buildImage, err := internal.FindLatestStackImages(
				fmt.Sprintf("%s/some-org/some-repo-run:0.0.10", strings.TrimPrefix(server.URL, "http://")),
				fmt.Sprintf("%s/some-org/some-repo-build:0.0.10", strings.TrimPrefix(server.URL, "http://")),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(buildImage).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo-build", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo-build",
				Version: "0.20.2",
			}))
			Expect(runImage).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo-run", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo-run",
				Version: "0.20.2",
			}))
		})

		it("for non-suffixed stack repos, when the run image is `latest`, it returns the latest semver tag for the build image only", func() {
			runImage, buildImage, err := internal.FindLatestStackImages(
				fmt.Sprintf("%s/some-org/some-repo-run:latest", strings.TrimPrefix(server.URL, "http://")),
				fmt.Sprintf("%s/some-org/some-repo-build:0.0.10", strings.TrimPrefix(server.URL, "http://")),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(runImage).To(Equal(internal.Image{}))
			Expect(buildImage).To(Equal(internal.Image{
				Name:    fmt.Sprintf("%s/some-org/some-repo-build", strings.TrimPrefix(server.URL, "http://")),
				Path:    "some-org/some-repo-build",
				Version: "0.20.2",
			}))
		})

		context("failure cases", func() {
			context("when the run uri cannot be parsed", func() {
				it("returns an error while finding the build image first", func() {
					_, _, err := internal.FindLatestStackImages(
						"not a valid uri",
						fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError(ContainSubstring("failed to find latest build image")))
				})
			})

			context("when the build uri cannot be parsed", func() {
				it("returns an error while finding the build image first", func() {
					_, _, err := internal.FindLatestStackImages(
						fmt.Sprintf("%s/some-org/some-repo-run:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
						"not a valid uri",
					)
					Expect(err).To(MatchError(ContainSubstring("failed to find latest build image")))
				})
			})

			context("when the run image is not tagged", func() {
				it("returns an error while finding the build image first", func() {
					_, _, err := internal.FindLatestStackImages(
						fmt.Sprintf("%s/some-org/some-repo-run", strings.TrimPrefix(server.URL, "http://")),
						fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError(ContainSubstring("failed to find latest build image")))
				})
			})
		})
	}, spec.Sequential())

	context("UpdateRunImageMirrors", func() {
		it("only updates run image mirror versions with semantic versions", func() {
			mirrors, err := internal.UpdateRunImageMirrors("1.2.3", []string{
				"docker.io/some-repo/some-mirror:0.0.1",
				"docker.io/some-repo/some-other-mirror:0.0.1-some-cnb",
				"docker.io/some-repo/some-mirror:some-cnb",
				"docker.io/some-repo/some-mirror:latest",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(mirrors).To(Equal([]string{
				"docker.io/some-repo/some-mirror:1.2.3",
				"docker.io/some-repo/some-other-mirror:1.2.3",
				"docker.io/some-repo/some-mirror:some-cnb",
				"docker.io/some-repo/some-mirror:latest",
			}))
		})

		context("failure cases", func() {
			context("when a mirror uri cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.UpdateRunImageMirrors("1.2.3", []string{"bad URI"})
					Expect(err).To(MatchError(ContainSubstring("failed to parse image 'bad URI'")))
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
					_, err := fmt.Fprintln(w, `{
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
					Expect(err).NotTo(HaveOccurred())

				case "/v2/some-org/tag-suffix-repo-build/tags/list":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `{
						  "tags": [
								"0.0.10-some-cnb",
								"0.20.12-some-cnb",
								"0.20.12-other-cnb",
								"999999-some-cnb"
							]
					}`)
					Expect(err).NotTo(HaveOccurred())

				case "/v2/some-org/some-other-repo-build/tags/list":
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintln(w, `{
						  "tags": [
								"v0.0.10-some-cnb",
								"v0.20.2",
								"v0.20.12-some-cnb",
								"v0.20.12-other-cnb",
								"999999-some-cnb",
								"latest"
							]
					}`)
					Expect(err).NotTo(HaveOccurred())

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
			context("the run tag has a suffix, but there's no matching build tags with the suffix", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						fmt.Sprintf("%s/some-org/tag-suffix-repo-run:0.0.10-novel-suffix", strings.TrimPrefix(server.URL, "http://")),
						fmt.Sprintf("%s/some-org/tag-suffix-repo-build:0.0.10", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError(ContainSubstring("could not find any valid tag")))
				})
			})
			context("when the run uri cannot be parsed", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						"not a valid uri",
						fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError("failed to parse run image: failed to parse image reference \"not a valid uri\": invalid reference format"))
				})
			})

			context("when the run image is not tagged", func() {
				it("returns an error", func() {
					_, err := internal.FindLatestBuildImage(
						fmt.Sprintf("%s/some-org/some-repo-run", strings.TrimPrefix(server.URL, "http://")),
						fmt.Sprintf("%s/some-org/some-repo-build:0.0.10-some-cnb", strings.TrimPrefix(server.URL, "http://")),
					)
					Expect(err).To(MatchError("failed to parse run image: expected the image to be tagged but it was not"))
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
