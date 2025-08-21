package integration_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/packit/v2/matchers"
)

func testPack(t *testing.T, context spec.G, it spec.S) {
	var (
		withT      = NewWithT(t)
		Expect     = withT.Expect
		Eventually = withT.Eventually

		buffer       *Buffer
		tmpDir       string
		buildpackDir string
	)

	it.Before(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "output")
		Expect(err).NotTo(HaveOccurred())

		buildpackDir, err = os.MkdirTemp("", "buildpack")
		Expect(err).NotTo(HaveOccurred())

		buffer = &Buffer{}
	})

	it.After(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
		Expect(os.RemoveAll(buildpackDir)).To(Succeed())
	})

	context("when packaging a language family buildpack", func() {
		it.Before(func() {
			err := cargo.NewDirectoryDuplicator().Duplicate(filepath.Join("testdata", "example-language-family-cnb"), buildpackDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it("creates a language family archive", func() {
			command := exec.Command(
				path, "pack",
				"--buildpack", filepath.Join(buildpackDir, "buildpack.toml"),
				"--output", filepath.Join(tmpDir, "output.tgz"),
				"--version", "some-version",
			)
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, "5s").Should(gexec.Exit(0), func() string { return buffer.String() })

			Expect(session.Out).To(gbytes.Say("Packing some-buildpack-name some-version..."))
			Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
			Expect(session.Out).To(gbytes.Say("    buildpack.toml"))

			file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
			Expect(err).NotTo(HaveOccurred())

			contents, _, err := ExtractFile(file, "buildpack.toml")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(MatchTOML(`api = "0.2"

[buildpack]
  id = "some-buildpack-id"
  name = "some-buildpack-name"
  version = "some-version"

[metadata]
  include-files = ["buildpack.toml"]

  [[metadata.dependencies]]
    arch = "some-arch"
    deprecation_date = "2019-04-01T00:00:00Z"
    id = "some-dependency"
    name = "Some Dependency"
    os = "some-os"
    sha256 = "shasum"
    stacks = ["io.buildpacks.stacks.bionic", "org.cloudfoundry.stacks.tiny"]
    uri = "http://some-url"
    version = "1.2.3"

  [[metadata.dependencies]]
    arch = "some-other-arch"
    deprecation_date = "2022-04-01T00:00:00Z"
    id = "other-dependency"
    name = "Other Dependency"
    os = "some-other-os"
    sha256 = "shasum"
    stacks = ["org.cloudfoundry.stacks.tiny"]
    uri = "http://other-url"
    version = "4.5.6"

[[order]]
  [[order.group]]
    id = "some-dependency"
    version = "1.2.3"

  [[order.group]]
    id = "other-dependency"
    version = "4.5.6"

[[targets]]
  arch = "some-arch"
  os = "some-os"

[[targets]]
  arch = "some-other-arch"
  os = "some-other-os"`))
		})
	})

	context("when packaging an implementation buildpack", func() {
		context("that is single architecture", func() {
			it.Before(func() {
				err := cargo.NewDirectoryDuplicator().Duplicate(filepath.Join("testdata", "example-cnb"), buildpackDir)
				Expect(err).NotTo(HaveOccurred())
			})

			expectedBuildpackToml := `api = "0.6"

[buildpack]
  description = "some-buildpack-description"
  homepage = "some-homepage-link"
  id = "some-buildpack-id"
  keywords = ["some-buildpack-keyword"]
  name = "some-buildpack-name"
  version = "some-version"

[[buildpack.licenses]]
  type = "some-buildpack-license-type"
  uri = "some-buildpack-license-uri"

[metadata]
  include-files = ["bin/build", "bin/detect", "bin/link", "buildpack.toml", "generated-file"]
  pre-package = "./scripts/build.sh"
  [metadata.default-versions]
    some-dependency = "some-default-version"

  [[metadata.dependencies]]
    arch = "some-arch"
    checksum = "sha256:shasum"
    deprecation_date = "2019-04-01T00:00:00Z"
    id = "some-dependency"
    name = "Some Dependency"
    os = "some-os"
    stacks = ["io.buildpacks.stacks.bionic", "org.cloudfoundry.stacks.tiny"]
    uri = "http://some-url"
    version = "1.2.3"

    [[metadata.dependencies.distros]]
      name = "some-distro-name"
      version = "some-distro-version"

  [[metadata.dependencies]]
    arch = "some-other-arch"
    checksum = "sha256:shasum"
    deprecation_date = "2022-04-01T00:00:00Z"
    id = "other-dependency"
    name = "Other Dependency"
    os = "some-other-os"
    stacks = ["org.cloudfoundry.stacks.tiny"]
    uri = "http://other-url"
    version = "4.5.6"

    [[metadata.dependencies.distros]]
      name = "some-other-distro-name"
      version = "some-other-distro-version"

[[stacks]]
  id = "some-stack-id"
  mixins = ["some-mixin-id"]

[[targets]]
  arch = "some-arch"
  os = "some-os"

[[targets]]
  arch = "some-other-arch"
  os = "some-other-os"`

			it("creates a packaged buildpack", func() {
				command := exec.Command(
					path, "pack",
					"--buildpack", filepath.Join(buildpackDir, "buildpack.toml"),
					"--output", filepath.Join(tmpDir, "output.tgz"),
					"--version", "some-version",
				)
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, "5s").Should(gexec.Exit(0), func() string { return buffer.String() })

				Expect(session.Out).To(gbytes.Say("Packing some-buildpack-name some-version..."))
				Expect(session.Out).To(gbytes.Say("  Executing pre-packaging script: ./scripts/build.sh"))
				Expect(session.Out).To(gbytes.Say("    hello from the pre-packaging script"))
				Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
				Expect(session.Out).To(gbytes.Say("    bin/build"))
				Expect(session.Out).To(gbytes.Say("    bin/detect"))
				Expect(session.Out).To(gbytes.Say("    bin/link"))
				Expect(session.Out).To(gbytes.Say("    buildpack.toml"))
				Expect(session.Out).To(gbytes.Say("    generated-file"))

				file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
				Expect(err).NotTo(HaveOccurred())

				u, err := user.Current()
				Expect(err).NotTo(HaveOccurred())
				userName := u.Username

				group, err := user.LookupGroupId(u.Gid)
				Expect(err).NotTo(HaveOccurred())
				groupName := group.Name

				contents, hdr, err := ExtractFile(file, "buildpack.toml")
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(MatchTOML(expectedBuildpackToml))
				Expect(hdr.Mode).To(Equal(int64(0644)))

				contents, hdr, err = ExtractFile(file, "bin/build")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("build-contents"))
				Expect(hdr.Mode).To(Equal(int64(0755)))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))

				contents, hdr, err = ExtractFile(file, "bin/detect")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("detect-contents"))
				Expect(hdr.Mode).To(Equal(int64(0755)))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))

				_, hdr, err = ExtractFile(file, "bin/link")
				Expect(err).NotTo(HaveOccurred())
				Expect(hdr.Linkname).To(Equal("build"))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))

				contents, hdr, err = ExtractFile(file, "generated-file")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("hello\n"))
				Expect(hdr.Mode).To(Equal(int64(0644)))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))

				Expect(filepath.Join(buildpackDir, "generated-file")).NotTo(BeARegularFile())
			})

			context("when the buildpack is built to run offline", func() {
				var server *httptest.Server
				var config cargo.Config
				it.Before(func() {
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						if req.URL.Path != "/some-dependency.tgz" {
							http.NotFound(w, req)
						}

						_, _ = fmt.Fprint(w, "dependency-contents")
					}))

					var err error
					config, err = cargo.NewBuildpackParser().Parse(filepath.Join(buildpackDir, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Metadata.Dependencies).To(HaveLen(2))

					config.Metadata.Dependencies[0].URI = fmt.Sprintf("%s/some-dependency.tgz", server.URL)
					config.Metadata.Dependencies[0].Checksum = "sha256:f058c8bf6b65b829e200ef5c2d22fde0ee65b96c1fbd1b88869be133aafab64a"

					bpTomlWriter, err := os.Create(filepath.Join(buildpackDir, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())

					Expect(cargo.EncodeConfig(bpTomlWriter, config)).To(Succeed())
				})

				it.After(func() {
					server.Close()
				})

				it("creates an offline packaged buildpack", func() {
					command := exec.Command(
						path, "pack",
						"--buildpack", filepath.Join(buildpackDir, "buildpack.toml"),
						"--output", filepath.Join(tmpDir, "output.tgz"),
						"--version", "some-version",
						"--offline",
						"--stack", "io.buildpacks.stacks.bionic",
					)
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0), func() string { return buffer.String() })

					relativeDependencyPath0 := "dependencies/f058c8bf6b65b829e200ef5c2d22fde0ee65b96c1fbd1b88869be133aafab64a"

					Expect(session.Out).To(gbytes.Say("Packing some-buildpack-name some-version..."))
					Expect(session.Out).To(gbytes.Say("  Executing pre-packaging script: ./scripts/build.sh"))
					Expect(session.Out).To(gbytes.Say("    hello from the pre-packaging script"))
					Expect(session.Out).To(gbytes.Say("  Downloading dependencies..."))
					Expect(session.Out).To(gbytes.Say(`    some-dependency \(1.2.3\) \[io.buildpacks.stacks.bionic, org.cloudfoundry.stacks.tiny\]`))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("      ↳  %s", relativeDependencyPath0)))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
					Expect(session.Out).To(gbytes.Say("    bin"))
					Expect(session.Out).To(gbytes.Say("    bin/build"))
					Expect(session.Out).To(gbytes.Say("    bin/detect"))
					Expect(session.Out).To(gbytes.Say("    buildpack.toml"))
					Expect(session.Out).To(gbytes.Say("    dependencies"))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("    %s", relativeDependencyPath0)))
					Expect(session.Out).To(gbytes.Say("    generated-file"))

					Expect(string(session.Out.Contents())).NotTo(ContainSubstring("other-dependency"))

					file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
					Expect(err).NotTo(HaveOccurred())

					var extractedBuildpackConfig cargo.Config
					contents, hdr, err := ExtractFile(file, "buildpack.toml")
					Expect(err).NotTo(HaveOccurred())
					Expect(hdr.Mode).To(Equal(int64(0644)))
					buff := bytes.NewBuffer(contents)
					err = cargo.DecodeConfig(buff, &extractedBuildpackConfig)
					Expect(err).NotTo(HaveOccurred())

					updatedIncludeFiles := config.Metadata.IncludeFiles
					updatedIncludeFiles = append(updatedIncludeFiles, relativeDependencyPath0)

					Expect(extractedBuildpackConfig.Metadata.IncludeFiles).To(Equal(updatedIncludeFiles))
					Expect(extractedBuildpackConfig.Metadata.Dependencies[0].URI).To(Equal(fmt.Sprintf(`file:///%s`, relativeDependencyPath0)))
					Expect(extractedBuildpackConfig.Metadata.Dependencies[0].Checksum).To(Equal(config.Metadata.Dependencies[0].Checksum))

					contents, hdr, err = ExtractFile(file, "bin/build")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("build-contents"))
					Expect(hdr.Mode).To(Equal(int64(0755)))

					contents, hdr, err = ExtractFile(file, "bin/detect")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("detect-contents"))
					Expect(hdr.Mode).To(Equal(int64(0755)))

					contents, hdr, err = ExtractFile(file, "generated-file")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("hello\n"))
					Expect(hdr.Mode).To(Equal(int64(0644)))

					contents, hdr, err = ExtractFile(file, "dependencies/f058c8bf6b65b829e200ef5c2d22fde0ee65b96c1fbd1b88869be133aafab64a")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("dependency-contents"))
					Expect(hdr.Mode).To(Equal(int64(0644)))
				})
			})
		})

		context("that is multi architecture with os and arch and matching directory layout", func() {
			it.Before(func() {
				err := cargo.NewDirectoryDuplicator().Duplicate(filepath.Join("testdata", "example-cnb-multi-arch-with-os-arch"), buildpackDir)
				Expect(err).NotTo(HaveOccurred())
			})

			expectedBuildpackToml := `api = "0.6"

[buildpack]
  description = "some-buildpack-description"
  homepage = "some-homepage-link"
  id = "some-buildpack-id"
  keywords = ["some-buildpack-keyword"]
  name = "some-buildpack-name"
  version = "some-version"

[[buildpack.licenses]]
  type = "some-buildpack-license-type"
  uri = "some-buildpack-license-uri"

[metadata]
  include-files = [ "some-os/some-arch/bin/build", "some-os/some-arch/bin/detect", "some-os/some-arch/bin/link", "some-os/some-arch/generated-file", "some-other-os/some-other-arch/bin/build", "some-other-os/some-other-arch/bin/detect", "some-other-os/some-other-arch/bin/link", "some-other-os/some-other-arch/generated-file", "buildpack.toml" ]
  pre-package = "./scripts/build.sh"
  [metadata.default-versions]
    some-dependency = "some-default-version"

  [[metadata.dependencies]]
    arch = "some-arch"
    checksum = "sha256:shasum"
    deprecation_date = "2019-04-01T00:00:00Z"
    id = "some-dependency"
    name = "Some Dependency"
    os = "some-os"
    stacks = ["io.buildpacks.stacks.bionic", "org.cloudfoundry.stacks.tiny"]
    uri = "http://some-url"
    version = "1.2.3"

    [[metadata.dependencies.distros]]
      name = "some-distro-name"
      version = "some-distro-version"

  [[metadata.dependencies]]
    arch = "some-other-arch"
    checksum = "sha256:shasum"
    deprecation_date = "2022-04-01T00:00:00Z"
    id = "other-dependency"
    name = "Other Dependency"
    os = "some-other-os"
    stacks = ["org.cloudfoundry.stacks.tiny"]
    uri = "http://other-url"
    version = "4.5.6"

    [[metadata.dependencies.distros]]
      name = "some-other-distro-name"
      version = "some-other-distro-version"

[[stacks]]
  id = "some-stack-id"

[[targets]]
  os = "some-os"
  arch = "some-arch"

[[targets]]
  os = "some-other-os"
  arch = "some-other-arch"`

			platforms := []struct {
				os   string
				arch string
			}{
				{"some-os", "some-arch"},
				{"some-other-os", "some-other-arch"},
			}

			it("creates a packaged buildpack", func() {
				command := exec.Command(
					path, "pack",
					"--buildpack", filepath.Join(buildpackDir, "buildpack.toml"),
					"--output", filepath.Join(tmpDir, "output.tgz"),
					"--version", "some-version",
				)
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, "5s").Should(gexec.Exit(0), func() string { return buffer.String() })

				Expect(session.Out).To(gbytes.Say("Packing some-buildpack-name some-version..."))
				Expect(session.Out).To(gbytes.Say("  Executing pre-packaging script: ./scripts/build.sh"))
				Expect(session.Out).To(gbytes.Say("    hello from the pre-packaging script"))
				Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
				Expect(session.Out).To(gbytes.Say("    buildpack.toml"))
				Expect(session.Out).To(gbytes.Say("    some-os/some-arch/bin/build"))
				Expect(session.Out).To(gbytes.Say("    some-os/some-arch/bin/detect"))
				Expect(session.Out).To(gbytes.Say("    some-os/some-arch/bin/link"))
				Expect(session.Out).To(gbytes.Say("    some-os/some-arch/generated-file"))
				Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/bin/build"))
				Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/bin/detect"))
				Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/bin/link"))
				Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/generated-file"))

				file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
				Expect(err).NotTo(HaveOccurred())

				u, err := user.Current()
				Expect(err).NotTo(HaveOccurred())
				userName := u.Username

				group, err := user.LookupGroupId(u.Gid)
				Expect(err).NotTo(HaveOccurred())
				groupName := group.Name

				contents, hdr, err := ExtractFile(file, "buildpack.toml")
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(MatchTOML(expectedBuildpackToml))
				Expect(hdr.Mode).To(Equal(int64(0644)))

				for _, platform := range platforms {
					targetOs := platform.os
					targetArch := platform.arch
					contents, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/bin/build", targetOs, targetArch))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(fmt.Sprintf("%s/%s/build-contents", targetOs, targetArch)))
					Expect(hdr.Mode).To(Equal(int64(0755)))
					Expect(hdr.Uname).To(Equal(userName))
					Expect(hdr.Gname).To(Equal(groupName))

					contents, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/bin/detect", targetOs, targetArch))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(fmt.Sprintf("%s/%s/detect-contents", targetOs, targetArch)))
					Expect(hdr.Mode).To(Equal(int64(0755)))
					Expect(hdr.Uname).To(Equal(userName))
					Expect(hdr.Gname).To(Equal(groupName))

					_, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/bin/link", targetOs, targetArch))
					Expect(err).NotTo(HaveOccurred())
					Expect(hdr.Linkname).To(Equal("build"))
					Expect(hdr.Uname).To(Equal(userName))
					Expect(hdr.Gname).To(Equal(groupName))

					contents, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/generated-file", targetOs, targetArch))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(fmt.Sprintf("%s/%s/hello\n", targetOs, targetArch)))
					Expect(hdr.Mode).To(Equal(int64(0644)))
					Expect(hdr.Uname).To(Equal(userName))
					Expect(hdr.Gname).To(Equal(groupName))

					Expect(filepath.Join(buildpackDir, fmt.Sprintf("%s/%s/generated-file", targetOs, targetArch))).NotTo(BeARegularFile())
				}
			})

			context("when the buildpack is built to run offline", func() {
				var server *httptest.Server
				var config cargo.Config
				it.Before(func() {
					var err error
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						switch req.URL.Path {
						case "/some-dependency.tgz":
							_, _ = fmt.Fprint(w, "some-dependency-contents")
						case "/other-dependency.tgz":
							_, _ = fmt.Fprint(w, "other-dependency-contents")
						default:
							http.NotFound(w, req)
						}
					}))

					config, err = cargo.NewBuildpackParser().Parse(filepath.Join(buildpackDir, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Metadata.Dependencies).To(HaveLen(2))

					config.Metadata.Dependencies[0].URI = fmt.Sprintf("%s/some-dependency.tgz", server.URL)
					config.Metadata.Dependencies[0].Checksum = "sha256:1e49f27b5eaafc6c50ebe66d7a2b1d17ced63c2862b5d54ccbcdac816260ecbb"
					config.Metadata.Dependencies[1].URI = fmt.Sprintf("%s/other-dependency.tgz", server.URL)
					config.Metadata.Dependencies[1].Checksum = "sha256:be06b2ecb65c937562db18c106ff4c15f32a914aa3c6bf7002b6d82390a9bb13"

					bpTomlWriter, err := os.Create(filepath.Join(buildpackDir, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())

					Expect(cargo.EncodeConfig(bpTomlWriter, config)).To(Succeed())
				})

				it.After(func() {
					server.Close()
				})

				it("creates an offline packaged buildpack", func() {
					command := exec.Command(
						path, "pack",
						"--buildpack", filepath.Join(buildpackDir, "buildpack.toml"),
						"--output", filepath.Join(tmpDir, "output.tgz"),
						"--version", "some-version",
						"--offline",
					)
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0), func() string { return buffer.String() })

					relativeDependencyPath0 := "dependencies/1e49f27b5eaafc6c50ebe66d7a2b1d17ced63c2862b5d54ccbcdac816260ecbb"
					platformSpecificDependencyPath0 := fmt.Sprintf("some-os/some-arch/%s", relativeDependencyPath0)
					relativeDependencyPath1 := "dependencies/be06b2ecb65c937562db18c106ff4c15f32a914aa3c6bf7002b6d82390a9bb13"
					platformSpecificDependencyPath1 := fmt.Sprintf("some-other-os/some-other-arch/%s", relativeDependencyPath1)

					Expect(session.Out).To(gbytes.Say("Packing some-buildpack-name some-version..."))
					Expect(session.Out).To(gbytes.Say("  Executing pre-packaging script: ./scripts/build.sh"))
					Expect(session.Out).To(gbytes.Say("    hello from the pre-packaging script"))
					Expect(session.Out).To(gbytes.Say("  Downloading dependencies..."))
					Expect(session.Out).To(gbytes.Say(`    some-dependency \(1.2.3\) \[io.buildpacks.stacks.bionic, org.cloudfoundry.stacks.tiny\]`))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("      ↳  %s", relativeDependencyPath0)))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("      ↳  %s", relativeDependencyPath1)))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
					Expect(session.Out).To(gbytes.Say("    some-os/some-arch/dependencies"))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("    %s", platformSpecificDependencyPath0)))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("    %s", platformSpecificDependencyPath1)))

					file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
					Expect(err).NotTo(HaveOccurred())

					var extractedBuildpackConfig cargo.Config
					contents, hdr, err := ExtractFile(file, "buildpack.toml")
					Expect(err).NotTo(HaveOccurred())
					Expect(hdr.Mode).To(Equal(int64(0644)))
					buff := bytes.NewBuffer(contents)
					err = cargo.DecodeConfig(buff, &extractedBuildpackConfig)
					Expect(err).NotTo(HaveOccurred())

					Expect(config.Metadata.Dependencies).To(HaveLen(2))
					Expect(extractedBuildpackConfig.Metadata.Dependencies).To(HaveLen(2))

					updatedIncludeFiles := config.Metadata.IncludeFiles
					updatedIncludeFiles = append(updatedIncludeFiles, platformSpecificDependencyPath0, platformSpecificDependencyPath1)

					Expect(extractedBuildpackConfig.Metadata.IncludeFiles).To(Equal(updatedIncludeFiles))

					Expect(extractedBuildpackConfig.Metadata.Dependencies[0].URI).To(Equal(fmt.Sprintf(`file:///%s`, relativeDependencyPath0)))
					Expect(extractedBuildpackConfig.Metadata.Dependencies[0].Checksum).To(Equal(config.Metadata.Dependencies[0].Checksum))
					Expect(extractedBuildpackConfig.Metadata.Dependencies[1].URI).To(Equal(fmt.Sprintf(`file:///%s`, relativeDependencyPath1)))
					Expect(extractedBuildpackConfig.Metadata.Dependencies[1].Checksum).To(Equal(config.Metadata.Dependencies[1].Checksum))

					contents, hdr, err = ExtractFile(file, platformSpecificDependencyPath0)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("some-dependency-contents"))
					Expect(hdr.Mode).To(Equal(int64(0644)))

					contents, hdr, err = ExtractFile(file, platformSpecificDependencyPath1)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("other-dependency-contents"))
					Expect(hdr.Mode).To(Equal(int64(0644)))
				})
			})
		})

		context("that is multi architecture without os and arch but with default linux/amd64 directory layout", func() {
			it.Before(func() {
				err := cargo.NewDirectoryDuplicator().Duplicate(filepath.Join("testdata", "example-cnb-multi-arch-without-os-arch"), buildpackDir)
				Expect(err).NotTo(HaveOccurred())
			})

			context("when the buildpack is built to run offline", func() {
				var server *httptest.Server
				var config cargo.Config
				it.Before(func() {
					var err error
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						switch req.URL.Path {
						case "/no-platform-dependency.tgz":
							_, _ = fmt.Fprint(w, "no-platform-dependency-contents")
						default:
							http.NotFound(w, req)
						}
					}))

					config, err = cargo.NewBuildpackParser().Parse(filepath.Join(buildpackDir, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Metadata.Dependencies).To(HaveLen(1))

					config.Metadata.Dependencies[0].URI = fmt.Sprintf("%s/no-platform-dependency.tgz", server.URL)
					config.Metadata.Dependencies[0].Checksum = "sha256:1f384c990b7aba4b80f2b12d85dc4a2d8ffaf9097ca542818a504a592d041642"

					bpTomlWriter, err := os.Create(filepath.Join(buildpackDir, "buildpack.toml"))
					Expect(err).NotTo(HaveOccurred())

					Expect(cargo.EncodeConfig(bpTomlWriter, config)).To(Succeed())
				})

				it.After(func() {
					server.Close()
				})

				it("creates an offline packaged buildpack", func() {
					command := exec.Command(
						path, "pack",
						"--buildpack", filepath.Join(buildpackDir, "buildpack.toml"),
						"--output", filepath.Join(tmpDir, "output.tgz"),
						"--version", "some-version",
						"--offline",
					)
					session, err := gexec.Start(command, buffer, buffer)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0), func() string { return buffer.String() })

					relativeDependencyPath0 := "dependencies/1f384c990b7aba4b80f2b12d85dc4a2d8ffaf9097ca542818a504a592d041642"
					platformSpecificDependencyPath0 := fmt.Sprintf("linux/amd64/%s", relativeDependencyPath0)

					Expect(session.Out).To(gbytes.Say("Packing some-buildpack-name some-version..."))
					Expect(session.Out).To(gbytes.Say("  Executing pre-packaging script: ./scripts/build.sh"))
					Expect(session.Out).To(gbytes.Say("    hello from the pre-packaging script"))
					Expect(session.Out).To(gbytes.Say("  Downloading dependencies..."))
					Expect(session.Out).To(gbytes.Say(`    no-platform-dependency \(7.8.9\) \[some-stack-id\]`))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("      ↳  %s", relativeDependencyPath0)))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
					Expect(session.Out).To(gbytes.Say("    linux/amd64/dependencies"))
					Expect(session.Out).To(gbytes.Say(fmt.Sprintf("    %s", platformSpecificDependencyPath0)))

					file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
					Expect(err).NotTo(HaveOccurred())

					var extractedBuildpackConfig cargo.Config
					contents, hdr, err := ExtractFile(file, "buildpack.toml")
					Expect(err).NotTo(HaveOccurred())
					Expect(hdr.Mode).To(Equal(int64(0644)))
					buff := bytes.NewBuffer(contents)
					err = cargo.DecodeConfig(buff, &extractedBuildpackConfig)
					Expect(err).NotTo(HaveOccurred())

					Expect(config.Metadata.Dependencies).To(HaveLen(1))
					Expect(extractedBuildpackConfig.Metadata.Dependencies).To(HaveLen(1))

					updatedIncludeFiles := config.Metadata.IncludeFiles
					updatedIncludeFiles = append(updatedIncludeFiles, platformSpecificDependencyPath0)

					Expect(extractedBuildpackConfig.Metadata.IncludeFiles).To(Equal(updatedIncludeFiles))
					Expect(extractedBuildpackConfig.Metadata.Dependencies[0].URI).To(Equal(fmt.Sprintf(`file:///%s`, relativeDependencyPath0)))
					Expect(extractedBuildpackConfig.Metadata.Dependencies[0].Checksum).To(Equal(config.Metadata.Dependencies[0].Checksum))

					contents, hdr, err = ExtractFile(file, platformSpecificDependencyPath0)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("no-platform-dependency-contents"))
					Expect(hdr.Mode).To(Equal(int64(0644)))
				})
			})
		})
	})

	context("failure cases", func() {
		context("when the all the required flags are not set", func() {
			it("prints an error message", func() {
				command := exec.Command(path, "pack")
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return buffer.String() })

				Expect(session.Err.Contents()).To(ContainSubstring(`Error: required flag(s) "output", "version" not set`))
			})
		})

		context("when the required buildpack or extension flag is not set", func() {
			it("prints an error message", func() {
				command := exec.Command(
					path, "pack",
					"--output", filepath.Join(tmpDir, "output.tgz"),
					"--version", "some-version",
					"--offline",
					"--stack", "io.buildpacks.stacks.bionic",
				)
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return buffer.String() })

				Expect(session.Err.Contents()).To(ContainSubstring(`Error: "buildpack" or "extension" flag is required`))
			})
		})

		context("when the required output flag is not set", func() {
			it("prints an error message", func() {
				command := exec.Command(
					path, "pack",
					"--buildpack", filepath.Join(buildpackDir, "buildpack.toml"),
					"--version", "some-version",
					"--offline",
					"--stack", "io.buildpacks.stacks.bionic",
				)
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return buffer.String() })

				Expect(session.Err.Contents()).To(ContainSubstring(`Error: required flag(s) "output" not set`))
			})
		})

		context("when the required version flag is not set", func() {
			it("prints an error message", func() {
				command := exec.Command(
					path, "pack",
					"--buildpack", filepath.Join(buildpackDir, "buildpack.toml"),
					"--output", filepath.Join(tmpDir, "output.tgz"),
					"--offline",
					"--stack", "io.buildpacks.stacks.bionic",
				)
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return buffer.String() })

				Expect(session.Err.Contents()).To(ContainSubstring(`Error: required flag(s) "version" not set`))
			})
		})
	})
}
