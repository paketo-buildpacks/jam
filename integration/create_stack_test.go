package integration_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/onsi/gomega/gexec"
	"github.com/paketo-buildpacks/packit/v2/vacation"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/jam/v2/integration/matchers"
	. "github.com/paketo-buildpacks/packit/v2/matchers"
)

func testCreateStack(t *testing.T, _ spec.G, it spec.S) {
	var (
		withT      = NewWithT(t)
		Expect     = withT.Expect
		Eventually = withT.Eventually

		tmpDir string
	)

	it.Before(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	it("builds an example stack", func() {
		buffer := bytes.NewBuffer(nil)
		command := exec.Command(
			path, "create-stack",
			"--config", filepath.Join("testdata", "example-stack", "stack.toml"),
			"--build-output", filepath.Join(tmpDir, "build.oci"),
			"--run-output", filepath.Join(tmpDir, "run.oci"),
			"--secret", "some-secret=my-secret-value",
			"--label", "additional.label=label-value",
		)
		command.Env = append(os.Environ(), "EXPERIMENTAL_ATTACH_RUN_IMAGE_SBOM=true")
		session, err := gexec.Start(command, buffer, buffer)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0), buffer.String)

		var buildReleaseDate, runReleaseDate time.Time

		by("confirming that the build image is correct", func() {
			dir := filepath.Join(tmpDir, "build-index")
			err := os.Mkdir(dir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			archive, err := os.Open(filepath.Join(tmpDir, "build.oci"))
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				Expect(archive.Close()).To(Succeed())
			}()

			err = vacation.NewArchive(archive).Decompress(dir)
			Expect(err).NotTo(HaveOccurred())

			path, err := layout.FromPath(dir)
			Expect(err).NotTo(HaveOccurred())

			index, err := path.ImageIndex()
			Expect(err).NotTo(HaveOccurred())

			indexManifest, err := index.IndexManifest()
			Expect(err).NotTo(HaveOccurred())

			Expect(indexManifest.Manifests).To(HaveLen(2))
			Expect(indexManifest.Manifests[0].Platform).To(Equal(&v1.Platform{
				OS:           "linux",
				Architecture: "amd64",
			}))
			Expect(indexManifest.Manifests[1].Platform).To(Equal(&v1.Platform{
				OS:           "linux",
				Architecture: "arm64",
			}))

			imageArm, err := index.Image(indexManifest.Manifests[1].Digest)
			Expect(err).NotTo(HaveOccurred())

			fileArm, err := imageArm.ConfigFile()
			Expect(err).NotTo(HaveOccurred())

			Expect(fileArm.Config.Labels).To(SatisfyAll(
				// just check that the platform is different
				HaveKeyWithValue("platform", "arm64"),
			))

			image, err := index.Image(indexManifest.Manifests[0].Digest)
			Expect(err).NotTo(HaveOccurred())

			file, err := image.ConfigFile()
			Expect(err).NotTo(HaveOccurred())

			Expect(file.Config.Labels).To(SatisfyAll(
				HaveKeyWithValue("io.buildpacks.stack.id", "io.paketo.stacks.example"),
				HaveKeyWithValue("io.buildpacks.stack.description", "this build stack is for example purposes only"),
				HaveKeyWithValue("io.buildpacks.stack.distro.name", "alpine"),
				HaveKeyWithValue("io.buildpacks.stack.distro.version", "3.15.4"),
				HaveKeyWithValue("io.buildpacks.stack.homepage", "https://github.com/paketo-buildpacks/stacks"),
				HaveKeyWithValue("io.buildpacks.stack.maintainer", "Paketo Buildpacks"),
				HaveKeyWithValue("io.buildpacks.stack.metadata", MatchJSON("{}")),
				HaveKeyWithValue("io.buildpacks.stack.mixins", ContainSubstring(`"openssl"`)),
				HaveKeyWithValue("io.buildpacks.stack.mixins", ContainSubstring(`"build:git"`)),
				HaveKeyWithValue("io.paketo.stack.packages", ContainSubstring(`"openssl"`)),
				HaveKeyWithValue("platform", "amd64"),
				HaveKeyWithValue("additional.label", "label-value"),
			))

			Expect(file.Config.Labels).NotTo(HaveKeyWithValue("io.buildpacks.stack.mixins", ContainSubstring("run:")))

			buildReleaseDate, err = time.Parse(time.RFC3339, file.Config.Labels["io.buildpacks.stack.released"])
			Expect(err).NotTo(HaveOccurred())
			Expect(buildReleaseDate).To(BeTemporally("~", time.Now(), 10*time.Minute))

			Expect(file.Config.User).To(Equal("1000:1000"))

			Expect(file.Config.Env).To(ContainElements(
				"CNB_USER_ID=1000",
				"CNB_GROUP_ID=1000",
				"CNB_STACK_ID=io.paketo.stacks.example",
			))

			Expect(image).To(SatisfyAll(
				HaveFileWithContent("/etc/group", ContainSubstring("cnb:x:1000:")),
				HaveFileWithContent("/etc/passwd", ContainSubstring("cnb:x:1000:1000::/home/cnb:/bin/bash")),
				HaveDirectory("/home/cnb"),
			))

			Expect(image).To(HaveFileWithContent("/my-secret", "my-secret-value"))
		})

		by("confirming that the run image is correct", func() {
			dir := filepath.Join(tmpDir, "run-index")
			err := os.Mkdir(dir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			archive, err := os.Open(filepath.Join(tmpDir, "run.oci"))
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				Expect(archive.Close()).NotTo(HaveOccurred())
			}()

			err = vacation.NewArchive(archive).Decompress(dir)
			Expect(err).NotTo(HaveOccurred())

			path, err := layout.FromPath(dir)
			Expect(err).NotTo(HaveOccurred())

			index, err := path.ImageIndex()
			Expect(err).NotTo(HaveOccurred())

			indexManifest, err := index.IndexManifest()
			Expect(err).NotTo(HaveOccurred())

			Expect(indexManifest.Manifests).To(HaveLen(2))
			Expect(indexManifest.Manifests[0].Platform).To(Equal(&v1.Platform{
				OS:           "linux",
				Architecture: "amd64",
			}))
			Expect(indexManifest.Manifests[1].Platform).To(Equal(&v1.Platform{
				OS:           "linux",
				Architecture: "arm64",
			}))

			image, err := index.Image(indexManifest.Manifests[0].Digest)
			Expect(err).NotTo(HaveOccurred())

			file, err := image.ConfigFile()
			Expect(err).NotTo(HaveOccurred())

			Expect(file.Config.Labels).To(SatisfyAll(
				HaveKeyWithValue("io.buildpacks.stack.id", "io.paketo.stacks.example"),
				HaveKeyWithValue("io.buildpacks.stack.description", "this run stack is for example purposes only"),
				HaveKeyWithValue("io.buildpacks.stack.distro.name", "alpine"),
				HaveKeyWithValue("io.buildpacks.stack.distro.version", "3.15.4"),
				HaveKeyWithValue("io.buildpacks.stack.homepage", "https://github.com/paketo-buildpacks/stacks"),
				HaveKeyWithValue("io.buildpacks.stack.maintainer", "Paketo Buildpacks"),
				HaveKeyWithValue("io.buildpacks.stack.metadata", MatchJSON("{}")),
				HaveKeyWithValue("io.buildpacks.stack.mixins", ContainSubstring(`"openssl"`)),
				HaveKeyWithValue("io.paketo.stack.packages", ContainSubstring(`"openssl"`)),
				HaveKeyWithValue("io.buildpacks.base.sbom", file.RootFS.DiffIDs[len(file.RootFS.DiffIDs)-1].String()),
				HaveKeyWithValue("additional.label", "label-value"),
			))

			Expect(file.Config.Labels).NotTo(HaveKeyWithValue("io.buildpacks.stack.mixins", ContainSubstring("build:")))

			runReleaseDate, err = time.Parse(time.RFC3339, file.Config.Labels["io.buildpacks.stack.released"])
			Expect(err).NotTo(HaveOccurred())
			Expect(runReleaseDate).To(BeTemporally("~", time.Now(), 10*time.Minute))

			Expect(file.Config.User).To(Equal("1001:1000"))

			Expect(file.Config.Env).NotTo(ContainElements(
				"CNB_USER_ID=1001",
				"CNB_GROUP_ID=1000",
				"CNB_STACK_ID=io.buildpacks.stacks.bionic",
			))

			Expect(image).To(SatisfyAll(
				HaveFileWithContent("/etc/group", ContainSubstring("cnb:x:1000:")),
				HaveFileWithContent("/etc/passwd", ContainSubstring("cnb:x:1001:1000::/home/cnb:/bin/bash")),
				HaveFileWithContent("/etc/os-release", ContainSubstring(`PRETTY_NAME="Example Stack"`)),
				HaveFileWithContent("/etc/os-release", ContainSubstring(`HOME_URL="https://github.com/paketo-buildpacks/stacks"`)),
				HaveFileWithContent("/etc/os-release", ContainSubstring(`SUPPORT_URL="https://github.com/paketo-buildpacks/stacks/blob/main/README.md"`)),
				HaveFileWithContent("/etc/os-release", ContainSubstring(`BUG_REPORT_URL="https://github.com/paketo-buildpacks/stacks/issues/new"`)),
				HaveDirectory("/home/cnb"),
			))

			diffID, err := v1.NewHash(file.Config.Labels["io.buildpacks.base.sbom"])
			Expect(err).NotTo(HaveOccurred())

			layer, err := image.LayerByDiffID(diffID)
			Expect(err).NotTo(HaveOccurred())

			Expect(layer).To(SatisfyAll(
				HaveFileWithContent(`/cnb/sbom/([a-f0-9]{8}).syft.json`, ContainSubstring("https://raw.githubusercontent.com/anchore/syft/main/schema/json/schema-16.0.34.json")),
				HaveFileWithContent(`/cnb/sbom/([a-f0-9]{8}).cdx.json`, ContainSubstring(`"bomFormat": "CycloneDX"`)),
				HaveFileWithContent(`/cnb/sbom/([a-f0-9]{8}).cdx.json`, ContainSubstring(`"specVersion": "1.3"`)),
			))
		})

		Expect(buildReleaseDate).To(Equal(runReleaseDate))

		by("confirming that the logging output is correct", func() {
			Expect(buffer.String()).To(ContainLines(
				"Building io.paketo.stacks.example",
				"  Building on linux/amd64",
				"    Building base images",
				"      Build complete for base images",
				"    build: Decorating base image",
				"      Adding CNB_* environment variables",
				"      Adding io.buildpacks.stack.* labels",
				"      Adding io.buildpacks.stack.mixins label",
				"      Adding io.paketo.stack.packages label",
				"      Adding additional.label label",
				"      Creating cnb user",
				"    run: Decorating base image",
				"      Adding io.buildpacks.stack.* labels",
				"      Adding io.buildpacks.stack.mixins label",
				"      Adding io.paketo.stack.packages label",
				"      Adding additional.label label",
				"      Creating cnb user",
				"      Updating /etc/os-release",
				"      Attaching experimental SBOM",
				"    build: Updating image",
				"    run: Updating image",
				"",
				"  Building on linux/arm64",
				"    Building base images",
				"      Build complete for base images",
				"    build: Decorating base image",
				"      Adding CNB_* environment variables",
				"      Adding io.buildpacks.stack.* labels",
				"      Adding io.buildpacks.stack.mixins label",
				"      Adding io.paketo.stack.packages label",
				"      Adding additional.label label",
				"      Creating cnb user",
				"    run: Decorating base image",
				"      Adding io.buildpacks.stack.* labels",
				"      Adding io.buildpacks.stack.mixins label",
				"      Adding io.paketo.stack.packages label",
				"      Adding additional.label label",
				"      Creating cnb user",
				"      Updating /etc/os-release",
				"      Attaching experimental SBOM",
				"    build: Updating image",
				"    run: Updating image",
				"",
				fmt.Sprintf("  Exporting build image to %s", filepath.Join(tmpDir, "build.oci")),
				fmt.Sprintf("  Exporting run image to %s", filepath.Join(tmpDir, "run.oci")),
			))
		})
	})
}
