package integration_test

import (
	"fmt"
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

func testPackExtension(t *testing.T, context spec.G, it spec.S) {
	var (
		withT      = NewWithT(t)
		Expect     = withT.Expect
		Eventually = withT.Eventually

		buffer       *Buffer
		tmpDir       string
		extensionDir string
	)

	it.Before(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "output")
		Expect(err).NotTo(HaveOccurred())

		extensionDir, err = os.MkdirTemp("", "extension")
		Expect(err).NotTo(HaveOccurred())

		buffer = &Buffer{}
	})

	it.After(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
		Expect(os.RemoveAll(extensionDir)).To(Succeed())
	})

	context("when packaging an implementation extension", func() {
		it.Before(func() {
			err := cargo.NewDirectoryDuplicator().Duplicate(filepath.Join("testdata", "extension-example-cnb"), extensionDir)
			Expect(err).NotTo(HaveOccurred())
		})
		it("creates a packaged extension", func() {

			command := exec.Command(
				path, "pack",
				"--extension", filepath.Join(extensionDir, "extension.toml"),
				"--output", filepath.Join(tmpDir, "output.tgz"),
				"--version", "some-version",
			)
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, "5s").Should(gexec.Exit(0), func() string { return buffer.String() })

			Expect(session.Out).To(gbytes.Say("Packing some-extension-name some-version..."))
			Expect(session.Out).To(gbytes.Say("  Executing pre-packaging script: ./scripts/build.sh"))
			Expect(session.Out).To(gbytes.Say("    hello from the pre-packaging script"))
			Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
			Expect(session.Out).To(gbytes.Say("    bin/detect"))
			Expect(session.Out).To(gbytes.Say("    bin/generate"))
			Expect(session.Out).To(gbytes.Say("    bin/run"))
			Expect(session.Out).To(gbytes.Say("    extension.toml"))
			Expect(session.Out).To(gbytes.Say("    generated-file"))

			file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
			Expect(err).NotTo(HaveOccurred())

			u, err := user.Current()
			Expect(err).NotTo(HaveOccurred())
			userName := u.Username

			group, err := user.LookupGroupId(u.Gid)
			Expect(err).NotTo(HaveOccurred())
			groupName := group.Name

			contents, hdr, err := ExtractFile(file, "extension.toml")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(MatchTOML(`api = "0.7"

[extension]
  description = "some-extension-description"
  homepage = "some-extension-homepage"
  id = "some-extension-id"
  keywords = ["some-extension-keyword"]
  name = "some-extension-name"
  version = "some-version"

  [[extension.licenses]]
    type = "some-extension-license-type"
    uri = "some-extension-license-uri"

[metadata]
  include-files = ["README.md", "bin/generate", "bin/detect", "bin/run", "extension.toml", "generated-file"]
  pre-package = "./scripts/build.sh"

  [[metadata.configurations]]
    build = true
    default = "16"
    description = "the Node.js version"
    name = "BP_NODE_VERSION"
  [metadata.default-versions]
    node = "18.*.*"`))
			Expect(hdr.Mode).To(Equal(int64(0644)))

			contents, hdr, err = ExtractFile(file, "bin/detect")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("detect-contents"))
			Expect(hdr.Mode).To(Equal(int64(0755)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))

			contents, hdr, err = ExtractFile(file, "bin/generate")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("generate-contents"))
			Expect(hdr.Mode).To(Equal(int64(0755)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))

			contents, hdr, err = ExtractFile(file, "bin/run")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("run-contents"))
			Expect(hdr.Mode).To(Equal(int64(0755)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))

			contents, hdr, err = ExtractFile(file, "generated-file")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello\n"))
			Expect(hdr.Mode).To(Equal(int64(0644)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))

			Expect(filepath.Join(extensionDir, "generated-file")).NotTo(BeARegularFile())
		})

	})

	context("when packaging a signle architecture implementation extension with a target specified", func() {
		it.Before(func() {
			err := cargo.NewDirectoryDuplicator().Duplicate(filepath.Join("testdata", "extension-example-cnb-with-target"), extensionDir)
			Expect(err).NotTo(HaveOccurred())
		})
		it("creates a packaged extension", func() {
			command := exec.Command(
				path, "pack",
				"--extension", filepath.Join(extensionDir, "extension.toml"),
				"--output", filepath.Join(tmpDir, "output.tgz"),
				"--version", "some-version",
			)
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, "5s").Should(gexec.Exit(0), func() string { return buffer.String() })

			Expect(session.Out).To(gbytes.Say("Packing some-extension-name some-version..."))
			Expect(session.Out).To(gbytes.Say("  Executing pre-packaging script: ./scripts/build.sh"))
			Expect(session.Out).To(gbytes.Say("    hello from the pre-packaging script"))
			Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
			Expect(session.Out).To(gbytes.Say("    bin/detect"))
			Expect(session.Out).To(gbytes.Say("    bin/generate"))
			Expect(session.Out).To(gbytes.Say("    bin/run"))
			Expect(session.Out).To(gbytes.Say("    extension.toml"))
			Expect(session.Out).To(gbytes.Say("    generated-file"))

			file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
			Expect(err).NotTo(HaveOccurred())

			u, err := user.Current()
			Expect(err).NotTo(HaveOccurred())
			userName := u.Username

			group, err := user.LookupGroupId(u.Gid)
			Expect(err).NotTo(HaveOccurred())
			groupName := group.Name

			contents, hdr, err := ExtractFile(file, "extension.toml")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(MatchTOML(`api = "0.7"

[extension]
  description = "some-extension-description"
  homepage = "some-extension-homepage"
  id = "some-extension-id"
  keywords = ["some-extension-keyword"]
  name = "some-extension-name"
  version = "some-version"

  [[extension.licenses]]
    type = "some-extension-license-type"
    uri = "some-extension-license-uri"

[metadata]
  include-files = ["README.md", "bin/generate", "bin/detect", "bin/run", "extension.toml", "generated-file"]
  pre-package = "./scripts/build.sh"

  [[metadata.configurations]]
    build = true
    default = "16"
    description = "the Node.js version"
    name = "BP_NODE_VERSION"
  [metadata.default-versions]
    node = "18.*.*"

[[targets]]
  arch = "amd64"
  os = "linux"`))
			Expect(hdr.Mode).To(Equal(int64(0644)))

			contents, hdr, err = ExtractFile(file, "bin/detect")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("detect-contents"))
			Expect(hdr.Mode).To(Equal(int64(0755)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))

			contents, hdr, err = ExtractFile(file, "bin/generate")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("generate-contents"))
			Expect(hdr.Mode).To(Equal(int64(0755)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))

			contents, hdr, err = ExtractFile(file, "bin/run")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("run-contents"))
			Expect(hdr.Mode).To(Equal(int64(0755)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))

			contents, hdr, err = ExtractFile(file, "generated-file")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello\n"))
			Expect(hdr.Mode).To(Equal(int64(0644)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))

			Expect(filepath.Join(extensionDir, "generated-file")).NotTo(BeARegularFile())

			expectedReadme, readErr := os.ReadFile(filepath.Join(extensionDir, "README.md"))
			Expect(readErr).NotTo(HaveOccurred())

			contents, hdr, err = ExtractFile(file, "README.md")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal(string(expectedReadme)))
			Expect(hdr.Mode).To(Equal(int64(0644)))
			Expect(hdr.Uname).To(Equal(userName))
			Expect(hdr.Gname).To(Equal(groupName))
		})
	})

	context("when packaging a multi-architecture implementation extension", func() {
		it.Before(func() {
			err := cargo.NewDirectoryDuplicator().Duplicate(filepath.Join("testdata", "extension-example-cnb-multi-arch"), extensionDir)
			Expect(err).NotTo(HaveOccurred())
		})
		it("creates a packaged extension with the correct files for each target", func() {
			command := exec.Command(
				path, "pack",
				"--extension", filepath.Join(extensionDir, "extension.toml"),
				"--output", filepath.Join(tmpDir, "output.tgz"),
				"--version", "some-version",
			)
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, "5s").Should(gexec.Exit(0), func() string { return buffer.String() })

			Expect(session.Out).To(gbytes.Say("Packing some-extension-name some-version..."))
			Expect(session.Out).To(gbytes.Say("  Executing pre-packaging script: ./scripts/build.sh"))
			Expect(session.Out).To(gbytes.Say("    hello from the pre-packaging script"))
			Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
			Expect(session.Out).To(gbytes.Say("    extension.toml"))
			Expect(session.Out).To(gbytes.Say("    some-os"))
			Expect(session.Out).To(gbytes.Say("    some-os/some-arch"))
			Expect(session.Out).To(gbytes.Say("    some-os/some-arch/README.md"))
			Expect(session.Out).To(gbytes.Say("    some-os/some-arch/bin"))
			Expect(session.Out).To(gbytes.Say("    some-os/some-arch/bin/detect"))
			Expect(session.Out).To(gbytes.Say("    some-os/some-arch/bin/generate"))
			Expect(session.Out).To(gbytes.Say("    some-os/some-arch/bin/run"))
			Expect(session.Out).To(gbytes.Say("    some-os/some-arch/generated-file"))
			Expect(session.Out).To(gbytes.Say("    some-other-os"))
			Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch"))
			Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/README.md"))
			Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/bin"))
			Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/bin/detect"))
			Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/bin/generate"))
			Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/bin/run"))
			Expect(session.Out).To(gbytes.Say("    some-other-os/some-other-arch/generated-file"))

			file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
			Expect(err).NotTo(HaveOccurred())

			u, err := user.Current()
			Expect(err).NotTo(HaveOccurred())
			userName := u.Username

			group, err := user.LookupGroupId(u.Gid)
			Expect(err).NotTo(HaveOccurred())
			groupName := group.Name

			contents, hdr, err := ExtractFile(file, "extension.toml")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(MatchTOML(`api = "0.7"

[extension]
	description = "some-extension-description"
	homepage = "some-extension-homepage"
	id = "some-extension-id"
	keywords = ["some-extension-keyword"]
	name = "some-extension-name"
	version = "some-version"

	[[extension.licenses]]
	type = "some-extension-license-type"
	uri = "some-extension-license-uri"

[metadata]
	include-files = ["README.md", "extension.toml", "some-os/some-arch/bin/generate", "some-os/some-arch/bin/detect", "some-os/some-arch/bin/run", "some-os/some-arch/generated-file", "some-other-os/some-other-arch/bin/generate", "some-other-os/some-other-arch/bin/detect", "some-other-os/some-other-arch/bin/run", "some-other-os/some-other-arch/generated-file"]
	pre-package = "./scripts/build.sh"

	[[metadata.configurations]]
	build = true
	default = "16"
	description = "the Node.js version"
	name = "BP_NODE_VERSION"
	[metadata.default-versions]
	node = "18.*.*"

[[targets]]
	arch = "some-arch"
	os = "some-os"

[[targets]]
	arch = "some-other-arch"
	os = "some-other-os"`))

			Expect(hdr.Mode).To(Equal(int64(0644)))

			platforms := []struct {
				os   string
				arch string
			}{
				{"some-os", "some-arch"},
				{"some-other-os", "some-other-arch"},
			}
			for _, platform := range platforms {
				targetOs := platform.os
				targetArch := platform.arch

				contents, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/bin/detect", targetOs, targetArch))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(fmt.Sprintf("%s/%s/detect-contents", targetOs, targetArch)))
				Expect(hdr.Mode).To(Equal(int64(0755)))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))

				contents, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/bin/generate", targetOs, targetArch))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(fmt.Sprintf("%s/%s/generate-contents", targetOs, targetArch)))
				Expect(hdr.Mode).To(Equal(int64(0755)))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))

				contents, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/bin/run", targetOs, targetArch))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(fmt.Sprintf("%s/%s/run-contents", targetOs, targetArch)))
				Expect(hdr.Mode).To(Equal(int64(0755)))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))

				contents, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/generated-file", targetOs, targetArch))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(fmt.Sprintf("%s/%s/hello\n", targetOs, targetArch)))
				Expect(hdr.Mode).To(Equal(int64(0644)))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))

				Expect(filepath.Join(extensionDir, fmt.Sprintf("%s/%s/generated-file", targetOs, targetArch))).NotTo(BeARegularFile())

				expectedReadme, readErr := os.ReadFile(filepath.Join(extensionDir, "README.md"))
				Expect(readErr).NotTo(HaveOccurred())

				contents, hdr, err = ExtractFile(file, fmt.Sprintf("%s/%s/README.md", targetOs, targetArch))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(string(expectedReadme)))
				Expect(hdr.Mode).To(Equal(int64(0644)))
				Expect(hdr.Uname).To(Equal(userName))
				Expect(hdr.Gname).To(Equal(groupName))
			}
		})
	})

	context("failure cases", func() {
		context("when the all the required flags are not set", func() {
			it("prints an error message", func() {
				command := exec.Command(path, "pack")
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return buffer.String() })

				Expect(session.Err.Contents()).To(ContainSubstring("Error: required flag(s) \"output\", \"version\" not set"))
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

				Expect(session.Err.Contents()).To(ContainSubstring("Error: at least one of the flags in the group [buildpack extension] is required"))
			})
		})

		context("when the extension is built to run offline", func() {
			it("prints an error message", func() {
				command := exec.Command(
					path, "pack",
					"--extension", filepath.Join(extensionDir, "extension.toml"),
					"--output", filepath.Join(tmpDir, "output.tgz"),
					"--version", "some-version",
					"--offline",
					"--stack", "io.buildpacks.stacks.bionic",
				)
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return buffer.String() })

				Expect(session.Err.Contents()).To(ContainSubstring("Error: offline mode is not supported for extensions"))
			})
		})

		context("when the required output flag is not set", func() {
			it("prints an error message", func() {
				command := exec.Command(
					path, "pack",
					"--buildpack", filepath.Join(extensionDir, "buildpack.toml"),
					"--version", "some-version",
					"--offline",
					"--stack", "io.buildpacks.stacks.bionic",
				)
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return buffer.String() })

				Expect(session.Err.Contents()).To(ContainSubstring("Error: required flag(s) \"output\" not set"))
			})
		})

		context("when the required version flag is not set", func() {
			it("prints an error message", func() {
				command := exec.Command(
					path, "pack",
					"--buildpack", filepath.Join(extensionDir, "buildpack.toml"),
					"--output", filepath.Join(tmpDir, "output.tgz"),
					"--offline",
					"--stack", "io.buildpacks.stacks.bionic",
				)
				session, err := gexec.Start(command, buffer, buffer)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return buffer.String() })

				Expect(session.Err.Contents()).To(ContainSubstring("Error: required flag(s) \"version\" not set"))
			})
		})

	})

}
