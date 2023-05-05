package integration_test

import (
	"fmt"

	"os"
	"os/exec"
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

	context("when packaging a language family extension", func() {
		it.Before(func() {
			err := cargo.NewDirectoryDuplicator().Duplicate(filepath.Join("testdata", "extension-example-language-family-cnb"), extensionDir)
			Expect(err).NotTo(HaveOccurred())
		})

		it("creates a language family archive", func() {
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
			Expect(session.Out).To(gbytes.Say(fmt.Sprintf("  Building tarball: %s", filepath.Join(tmpDir, "output.tgz"))))
			Expect(session.Out).To(gbytes.Say("    extension.toml"))

			file, err := os.Open(filepath.Join(tmpDir, "output.tgz"))
			Expect(err).NotTo(HaveOccurred())

			contents, _, err := ExtractFile(file, "extension.toml")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(MatchTOML(`api = "0.7"

[extension]
  description = "some-extensin-description"
  homepage = "some-extension-homepage"
  id = "some-extension-id"
  keywords = [ "some-extension-keyword" ]
  name = "some-extension-name"
  version = "some-version"

  [[extension.licenses]]
    type = "some-extension-license-type"
    uri = "some-extension-license-uri"

[metadata]
  include-files = ["extension.toml"]

  [[metadata.configurations]]
    build = true
    default = "16"
    description = "the Node.js version"
    name = "BP_NODE_VERSION"

  [metadata.default-versions]
    node = "18.*.*"

  [[metadata.dependencies]]
    id = "some-dependency"
    name = "Some Dependency"
    sha256 = "shasum"
    source = "http://some-source-url"
    stacks = ["io.buildpacks.stacks.bionic", "org.cloudfoundry.stacks.tiny", "*"]
    uri = "http://some-url"
    version = "1.2.3"

  [[metadata.dependencies]]
    id = "other-dependency"
    name = "Other Dependency"
    sha256 = "shasum"
    source = "http://some-source-url"
    stacks = ["org.cloudfoundry.stacks.tiny"]
    uri = "http://other-url"
    version = "4.5.6"`))
		})
	})

}
