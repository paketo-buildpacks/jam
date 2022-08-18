package ihop_test

import (
	"archive/tar"
	"io"
	"testing"

	"github.com/paketo-buildpacks/jam/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testOsReleaseLayerCreator(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		creator ihop.OsReleaseLayerCreator
	)

	it("creates a layer with adjusted /etc/os-release", func() {
		creator.Def = ihop.Definition{
			ID:           "some-stack-id",
			Name:         "some-stack-name",
			Homepage:     "some-stack-homepage",
			SupportURL:   "some-stack-support-url",
			BugReportURL: "some-stack-bug-report-url",
		}
		layer, err := creator.Create(
			ihop.Image{Tag: "ubuntu:latest"},
			ihop.DefinitionImage{},
			ihop.SBOM{},
		)
		Expect(err).NotTo(HaveOccurred())

		reader, err := layer.Uncompressed()
		Expect(err).NotTo(HaveOccurred())
		defer reader.Close()

		tr := tar.NewReader(reader)
		files := make(map[string]interface{})
		headers := make(map[string]*tar.Header)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			Expect(err).NotTo(HaveOccurred())

			var content interface{}
			if hdr.Typeflag != tar.TypeDir {
				b, err := io.ReadAll(tr)
				Expect(err).NotTo(HaveOccurred())

				content = string(b)
			}

			files[hdr.Name] = content
			headers[hdr.Name] = hdr
		}

		Expect(files).To(SatisfyAll(
			HaveLen(2),
			HaveKeyWithValue("etc/", BeNil()),
			HaveKeyWithValue("etc/os-release", ContainSubstring(`NAME="Ubuntu"`)),
			HaveKeyWithValue("etc/os-release", ContainSubstring(`PRETTY_NAME="some-stack-name"`)),
			HaveKeyWithValue("etc/os-release", ContainSubstring(`HOME_URL="some-stack-homepage"`)),
			HaveKeyWithValue("etc/os-release", ContainSubstring(`SUPPORT_URL="some-stack-support-url"`)),
			HaveKeyWithValue("etc/os-release", ContainSubstring(`BUG_REPORT_URL="some-stack-bug-report-url"`)),
		))
	})
}
