package internal_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDeprecations(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		path   string
		buffer *bytes.Buffer
	)

	it.Before(func() {
		path = filepath.Join(t.TempDir(), "buildpack.toml")
		buffer = bytes.NewBuffer(nil)
	})

	writeFile := func(content string) {
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
	}

	context("WarnDeprecatedDependencyKeys", func() {
		it("writes nothing when the document uses the standard format", func() {
			writeFile(`
[[metadata.dependencies]]
	id = "some-dependency"
	version = "1.2.3"
	cpes = ["some-cpe"]
	purls = ["some-purl"]
	eol-date = "2020-06-01T00:00:00Z"
`)

			Expect(internal.WarnDeprecatedDependencyKeys(path, buffer)).To(Succeed())
			Expect(buffer.String()).To(BeEmpty())
		})

		it("warns about each deprecated key with the replacement", func() {
			writeFile(`
[[metadata.dependencies]]
	id = "some-dependency"
	version = "1.2.3"
	cpe = "some-cpe"
	purl = "some-purl"
	deprecation_date = "2020-06-01T00:00:00Z"
	source-sha256 = "some-source-sha"
	licenses = ["MIT"]
`)

			Expect(internal.WarnDeprecatedDependencyKeys(path, buffer)).To(Succeed())
			Expect(buffer.String()).To(Equal(`WARNING: buildpack.toml: dependency "some-dependency" (metadata.dependencies[0]) uses deprecated key "cpe", use "cpes" instead
WARNING: buildpack.toml: dependency "some-dependency" (metadata.dependencies[0]) uses deprecated key "deprecation_date", use "eol-date" instead
WARNING: buildpack.toml: dependency "some-dependency" (metadata.dependencies[0]) uses deprecated key "purl", use "purls" instead
WARNING: buildpack.toml: dependency "some-dependency" (metadata.dependencies[0]) uses deprecated key "source-sha256", use "source-checksum" instead
`))
		})

		it("does not warn about keys that are not safe to migrate yet", func() {
			writeFile(`
[[metadata.dependencies]]
	id = "some-dependency"
	version = "1.2.3"
	sha256 = "some-sha"
	stacks = ["some-stack"]
	licenses = ["MIT"]
`)

			Expect(internal.WarnDeprecatedDependencyKeys(path, buffer)).To(Succeed())
			Expect(buffer.String()).To(BeEmpty())
		})

		it("names the extension.toml file when warning", func() {
			path = filepath.Join(t.TempDir(), "extension.toml")
			writeFile(`
[[metadata.dependencies]]
	id = "some-dependency"
	cpe = "some-cpe"
`)

			Expect(internal.WarnDeprecatedDependencyKeys(path, buffer)).To(Succeed())
			Expect(buffer.String()).To(Equal(`WARNING: extension.toml: dependency "some-dependency" (metadata.dependencies[0]) uses deprecated key "cpe", use "cpes" instead
`))
		})

		context("failure cases", func() {
			context("when the file cannot be opened", func() {
				it("returns an error", func() {
					err := internal.WarnDeprecatedDependencyKeys(filepath.Join(t.TempDir(), "missing.toml"), buffer)
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})

			context("when the file is not valid TOML", func() {
				it("returns an error", func() {
					writeFile(`not = valid = toml`)
					Expect(internal.WarnDeprecatedDependencyKeys(path, buffer)).To(MatchError(ContainSubstring("expected value but found")))
				})
			})
		})
	})
}
