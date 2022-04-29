package ihop_test

import (
	"archive/tar"
	"io"
	"testing"

	"github.com/paketo-buildpacks/jam/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testUserLayerCreator(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		creator ihop.UserLayerCreator
	)

	it("creates a layer with user details", func() {
		layer, err := creator.Create(
			ihop.Image{Tag: "busybox:latest"},
			ihop.DefinitionImage{
				UID:   4567,
				GID:   1234,
				Shell: "/bin/sh",
			},
			ihop.SBOM{},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(layer.DiffID).To(MatchRegexp(`^sha256:[a-f0-9]{64}$`))

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
			HaveLen(3),
			HaveKeyWithValue("etc/group", ContainSubstring("cnb:x:1234:")),
			HaveKeyWithValue("etc/passwd", ContainSubstring("cnb:x:4567:1234::/home/cnb:/bin/sh")),
			HaveKeyWithValue("home/cnb", BeNil()),
		))

		Expect(headers["home/cnb"].Uid).To(Equal(4567))
		Expect(headers["home/cnb"].Gid).To(Equal(1234))
	})

	context("failure cases", func() {
		context("when the image does not exist on the daemon", func() {
			it("returns an error", func() {
				_, err := creator.Create(ihop.Image{Tag: "not an image"}, ihop.DefinitionImage{}, ihop.SBOM{})
				Expect(err).To(MatchError(ContainSubstring("could not parse reference")))
			})
		})
	})
}
