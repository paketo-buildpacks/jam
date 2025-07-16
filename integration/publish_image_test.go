package integration_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testPublishImage(t *testing.T, context spec.G, it spec.S) {
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

		buffer := bytes.NewBuffer(nil)
		command := exec.Command(
			path, "create-stack",
			"--config", filepath.Join("testdata", "example-stack", "stack.toml"),
			"--build-output", filepath.Join(tmpDir, "build.oci"),
			"--run-output", filepath.Join(tmpDir, "run.oci"),
			"--secret", "some-secret=my-secret-value",
		)
		session, err := gexec.Start(command, buffer, buffer)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0), buffer.String)

		archive, err := os.Open(filepath.Join(tmpDir, "run.oci"))
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(archive.Close()).To(Succeed())
		}()
	})

	it.After(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	context("When it tries to push an image to a non existence registry", func() {
		it("It fails with an error message", func() {

			buffer := bytes.NewBuffer(nil)
			command := exec.Command(
				path, "publish-image",
				"--image-ref", "registry-does-not-exist/image-name:latest",
				"--image-archive", filepath.Join(tmpDir, "run.oci"),
			)
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1), buffer.String)

			fmt.Println(buffer.String())

			Expect(buffer.String()).To(ContainSubstring(
				"Uploading image to registry-does-not-exist/image-name:latest",
			))

			Expect(buffer.String()).To(ContainSubstring(
				"unexpected status code 401 Unauthorized",
			))
		})
	})
}
