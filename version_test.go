package main_test

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testVersion(t *testing.T, context spec.G, it spec.S) {
	var (
		withT      = NewWithT(t)
		Expect     = withT.Expect
		Eventually = withT.Eventually

		buffer *bytes.Buffer
	)

	it.Before(func() {
		buffer = bytes.NewBuffer(nil)
	})

	context("when the format is set to markdown", func() {
		it("prints out the summary of a buildpack tarball", func() {
			command := exec.Command(
				path, "version",
			)
			session, err := gexec.Start(command, buffer, buffer)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0), func() string { return buffer.String() })

			Expect(string(session.Out.Contents())).To(Equal(`jam 1.2.3
`))
		})
	})
}
