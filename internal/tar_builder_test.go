package internal_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testTarBuilder(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		tempFile string
		tempDir  string
		output   *bytes.Buffer
		builder  internal.TarBuilder
	)

	it.Before(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "output")
		Expect(err).NotTo(HaveOccurred())

		tempFile = filepath.Join(tempDir, "buildpack.tgz")

		output = bytes.NewBuffer(nil)
		builder = internal.NewTarBuilder(scribe.NewLogger(output))
	})

	it.After(func() {
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	context("Build", func() {
		context("given a destination and a list of files", func() {
			it("constructs a tarball", func() {
				err := builder.Build(tempFile, []internal.File{
					{
						Name:       "buildpack.toml",
						Info:       internal.NewFileInfo("buildpack.toml", len("buildpack-toml-contents"), 0644, time.Now()),
						ReadCloser: io.NopCloser(strings.NewReader("buildpack-toml-contents")),
					},
					{
						Name:       "bin/build",
						Info:       internal.NewFileInfo("build", len("build-contents"), 0755, time.Now()),
						ReadCloser: io.NopCloser(strings.NewReader("build-contents")),
					},
					{
						Name:       "bin/detect",
						Info:       internal.NewFileInfo("detect", len("detect-contents"), 0755, time.Now()),
						ReadCloser: io.NopCloser(strings.NewReader("detect-contents")),
					},
					{
						Name: "bin/link",
						Info: internal.NewFileInfo("link", len("./build"), os.ModeSymlink|0755, time.Now()),
						Link: "./build",
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(output.String()).To(ContainSubstring(fmt.Sprintf("Building tarball: %s", tempFile)))
				Expect(output.String()).To(ContainSubstring("bin/build"))
				Expect(output.String()).To(ContainSubstring("bin/detect"))
				Expect(output.String()).To(ContainSubstring("bin/link"))
				Expect(output.String()).To(ContainSubstring("buildpack.toml"))

				file, err := os.Open(tempFile)
				Expect(err).NotTo(HaveOccurred())

				contents, hdr, err := ExtractFile(file, "buildpack.toml")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("buildpack-toml-contents"))
				Expect(hdr.Mode).To(Equal(int64(0644)))

				contents, hdr, err = ExtractFile(file, "bin")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(BeEmpty())
				Expect(hdr.Mode).To(Equal(int64(0777)))
				Expect(hdr.Typeflag).To(Equal(uint8(tar.TypeDir)))

				contents, hdr, err = ExtractFile(file, "bin/build")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("build-contents"))
				Expect(hdr.Mode).To(Equal(int64(0755)))

				contents, hdr, err = ExtractFile(file, "bin/detect")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("detect-contents"))
				Expect(hdr.Mode).To(Equal(int64(0755)))

				_, hdr, err = ExtractFile(file, "bin/link")
				Expect(err).NotTo(HaveOccurred())
				Expect(hdr.Typeflag).To(Equal(byte(tar.TypeSymlink)))
				Expect(hdr.Linkname).To(Equal("./build"))
				Expect(hdr.Mode).To(Equal(int64(0755)))
			})
		})

		context("failure cases", func() {
			context("when it is unable to create the destination file", func() {
				it.Before(func() {
					Expect(os.Chmod(tempDir, 0000)).To(Succeed())
				})

				it.Before(func() {
					Expect(os.Chmod(tempDir, 0644)).To(Succeed())
				})

				it("returns an error", func() {
					err := builder.Build(tempFile, []internal.File{
						{
							Name:       "bin/build",
							Info:       internal.NewFileInfo("build", len("build-contents"), 0755, time.Now()),
							ReadCloser: io.NopCloser(strings.NewReader("build-contents")),
						},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to create tarball")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when one of the files cannot be written", func() {
				it("returns an error", func() {
					err := builder.Build(tempFile, []internal.File{
						{
							Name:       "bin/build",
							Info:       internal.NewFileInfo("build", 1, 0755, time.Now()),
							ReadCloser: io.NopCloser(strings.NewReader("build-contents")),
						},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to write file to tarball")))
					Expect(err).To(MatchError(ContainSubstring("write too long")))
				})
			})

			context("when one of the files cannot have its header created", func() {
				it("returns an error", func() {
					err := builder.Build(tempFile, []internal.File{
						{
							Name:       "bin/build",
							ReadCloser: io.NopCloser(strings.NewReader("build-contents")),
						},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to create header for file \"bin/build\":")))
					Expect(err).To(MatchError(ContainSubstring("FileInfo is nil")))
				})
			})
		})
	})
}
