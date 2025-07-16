package ihop_test

import (
	"archive/tar"
	"io"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testSBOMLayerCreator(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		creator ihop.SBOMLayerCreator
	)

	it("creates a layer containing SBOM documents", func() {
		layer, err := creator.Create(
			ihop.Image{Digest: "sha256:abcdef123456789"},
			ihop.DefinitionImage{},
			ihop.SBOM{},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(layer.DiffID).To(MatchRegexp(`^sha256:[a-f0-9]{64}$`))

		reader, err := layer.Uncompressed()
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(reader.Close()).To(Succeed())
		}()

		tr := tar.NewReader(reader)
		files := make(map[string]interface{})
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
		}

		Expect(files).To(SatisfyAll(
			HaveLen(2),
			HaveKeyWithValue("cnb/sbom/abcdef12.syft.json", ContainSubstring("https://raw.githubusercontent.com/anchore/syft/main/schema/json/schema-16.0.34.json")),
			HaveKeyWithValue("cnb/sbom/abcdef12.cdx.json", ContainSubstring(`"bomFormat": "CycloneDX"`)),
			HaveKeyWithValue("cnb/sbom/abcdef12.cdx.json", ContainSubstring(`"specVersion": "1.3"`)),
		))
	})
}
