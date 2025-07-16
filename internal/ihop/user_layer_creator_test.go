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

func testUserLayerCreator(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		creator ihop.UserLayerCreator
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

		err = os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM busybox\nUSER some-user:some-group"), 0600)
		Expect(err).NotTo(HaveOccurred())

		image, err = client.Build(ihop.DefinitionImage{
			Dockerfile: filepath.Join(dir, "Dockerfile"),
		}, "linux/amd64")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(dir)).To(Succeed())
	})

	it("creates a layer with user details", func() {
		layer, err := creator.Create(
			image,
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
			HaveLen(4),
			HaveKeyWithValue("etc/", BeNil()),
			HaveKeyWithValue("etc/group", ContainSubstring("cnb:x:1234:")),
			HaveKeyWithValue("etc/passwd", ContainSubstring("cnb:x:4567:1234::/home/cnb:/bin/sh")),
			HaveKeyWithValue("home/cnb", BeNil()),
		))

		Expect(headers["home/cnb"].Uid).To(Equal(4567))
		Expect(headers["home/cnb"].Gid).To(Equal(1234))
	})
}
