package ihop_test

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testOsReleaseLayerCreator(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		creator ihop.OsReleaseLayerCreator
		client  ihop.Client
		image   ihop.Image
		dir     string
	)

	it.Before(func() {
		var err error
		dir, err = os.MkdirTemp("", "dockerfile-test")
		Expect(err).NotTo(HaveOccurred())

		client, err = ihop.NewClient(dir)
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM ubuntu:jammy\nUSER some-user:some-group"), 0600)
		Expect(err).NotTo(HaveOccurred())

		image, err = client.Build(ihop.DefinitionImage{
			Dockerfile: filepath.Join(dir, "Dockerfile"),
		}, "linux/amd64")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(dir)).To(Succeed())
	})

	it("creates a layer with adjusted /etc/os-release", func() {
		creator.Def = ihop.Definition{
			ID:           "some-stack-id",
			Name:         "some-stack-name",
			Homepage:     "some-stack-homepage",
			SupportURL:   "some-stack-support-url",
			BugReportURL: "some-stack-bug-report-url",
		}
		layer, err := creator.Create(
			image,
			ihop.DefinitionImage{},
			ihop.SBOM{},
		)
		Expect(err).NotTo(HaveOccurred())

		reader, err := layer.Uncompressed()
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(reader.Close()).To(Succeed())
		}()

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
