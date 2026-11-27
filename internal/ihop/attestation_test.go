package ihop_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testAttestation(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		dir   string
		ref   name.Reference
		index v1.ImageIndex
		amd64 v1.Hash
		arm64 v1.Hash
	)

	it.Before(func() {
		var err error
		dir = t.TempDir()

		ref, err = name.ParseReference("registry.example.com/some-org/some-image:1.2.3")
		Expect(err).NotTo(HaveOccurred())

		index = empty.Index
		for _, architecture := range []string{"amd64", "arm64"} {
			image, err := random.Image(64, 1)
			Expect(err).NotTo(HaveOccurred())

			index = mutate.AppendManifests(index, mutate.IndexAddendum{
				Add:        image,
				Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: architecture}},
			})
		}

		manifest, err := index.IndexManifest()
		Expect(err).NotTo(HaveOccurred())
		amd64, arm64 = manifest.Manifests[0].Digest, manifest.Manifests[1].Digest

		Expect(os.WriteFile(filepath.Join(dir, "amd64.cdx.json"), []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","metadata":{"timestamp":"2026-01-02T03:04:05Z"},"components":[{"bom-ref":"amd64-package","type":"library","name":"amd64-package","version":"1.0.0","purl":"pkg:deb/ubuntu/amd64-package@1.0.0"}]}`), 0600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "amd64.spdx.json"), []byte(`{"spdxVersion":"SPDX-2.3","packages":[{"name":"amd64-package"}]}`), 0600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "arm64.cdx.json"), []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6","metadata":{"timestamp":"2026-01-02T03:04:05Z"},"components":[{"bom-ref":"arm64-package","type":"library","name":"arm64-package","version":"1.0.0","purl":"pkg:deb/ubuntu/arm64-package@1.0.0"}]}`), 0600)).To(Succeed())
	})

	it("leaves the index alone when there is no SBOM", func() {
		attached, err := ihop.AttachSBOMs(index, ref, nil)
		Expect(err).NotTo(HaveOccurred())

		expected, err := index.Digest()
		Expect(err).NotTo(HaveOccurred())
		Expect(attached.Digest()).To(Equal(expected))
	})

	it("adds an attestation manifest per image, without touching the images", func() {
		attached, err := ihop.AttachSBOMs(index, ref, []ihop.SBOMAttestation{
			{Architecture: "amd64", Path: filepath.Join(dir, "amd64.cdx.json")},
			{Architecture: "amd64", Path: filepath.Join(dir, "amd64.spdx.json")},
			{Architecture: "arm64", Path: filepath.Join(dir, "arm64.cdx.json")},
		})
		Expect(err).NotTo(HaveOccurred())

		manifest, err := attached.IndexManifest()
		Expect(err).NotTo(HaveOccurred())
		Expect(manifest.Manifests).To(HaveLen(4))

		Expect(manifest.Manifests[0].Digest).To(Equal(amd64))
		Expect(manifest.Manifests[1].Digest).To(Equal(arm64))

		for i, image := range []v1.Hash{amd64, arm64} {
			descriptor := manifest.Manifests[2+i]
			Expect(descriptor.MediaType).To(Equal(types.OCIManifestSchema1))
			Expect(descriptor.Platform).To(Equal(&v1.Platform{OS: "unknown", Architecture: "unknown"}))
			Expect(descriptor.Annotations).To(Equal(map[string]string{
				"vnd.docker.reference.type":   "attestation-manifest",
				"vnd.docker.reference.digest": image.String(),
			}))
		}
	})

	it("stores each SBOM as an in-toto statement about its image", func() {
		attached, err := ihop.AttachSBOMs(index, ref, []ihop.SBOMAttestation{
			{Architecture: "amd64", Path: filepath.Join(dir, "amd64.cdx.json")},
			{Architecture: "amd64", Path: filepath.Join(dir, "amd64.spdx.json")},
		})
		Expect(err).NotTo(HaveOccurred())

		manifest, err := attached.IndexManifest()
		Expect(err).NotTo(HaveOccurred())
		Expect(manifest.Manifests).To(HaveLen(3))

		attestation, err := attached.Image(manifest.Manifests[2].Digest)
		Expect(err).NotTo(HaveOccurred())

		attestationManifest, err := attestation.Manifest()
		Expect(err).NotTo(HaveOccurred())
		Expect(attestationManifest.Layers).To(HaveLen(2))

		layers, err := attestation.Layers()
		Expect(err).NotTo(HaveOccurred())

		for i, expected := range []struct{ predicateType, sbom string }{
			{"https://cyclonedx.org/bom", "amd64.cdx.json"},
			{"https://spdx.dev/Document", "amd64.spdx.json"},
		} {
			Expect(attestationManifest.Layers[i].MediaType).To(Equal(types.MediaType("application/vnd.in-toto+json")))
			Expect(attestationManifest.Layers[i].Annotations).To(Equal(map[string]string{"in-toto.io/predicate-type": expected.predicateType}))

			reader, err := layers[i].Uncompressed()
			Expect(err).NotTo(HaveOccurred())
			content, err := io.ReadAll(reader)
			Expect(err).NotTo(HaveOccurred())
			Expect(reader.Close()).To(Succeed())

			var statement struct {
				Type          string `json:"_type"`
				PredicateType string `json:"predicateType"`
				Subject       []struct {
					Name   string            `json:"name"`
					Digest map[string]string `json:"digest"`
				} `json:"subject"`
				Predicate json.RawMessage `json:"predicate"`
			}
			Expect(json.Unmarshal(content, &statement)).To(Succeed())

			Expect(statement.Type).To(Equal("https://in-toto.io/Statement/v0.1"))
			Expect(statement.PredicateType).To(Equal(expected.predicateType))
			Expect(statement.Subject).To(HaveLen(1))
			Expect(statement.Subject[0].Name).To(Equal("pkg:docker/registry.example.com/some-org/some-image?platform=linux%2Famd64"))
			Expect(statement.Subject[0].Digest).To(Equal(map[string]string{"sha256": amd64.Hex}))

			sbom, err := os.ReadFile(filepath.Join(dir, expected.sbom))
			Expect(err).NotTo(HaveOccurred())
			Expect(statement.Predicate).To(MatchJSON(sbom))
		}
	})

	context("when only a CycloneDX SBOM is given", func() {
		it("also attaches its conversion to SPDX, which is what Docker's tooling reads", func() {
			attached, err := ihop.AttachSBOMs(index, ref, []ihop.SBOMAttestation{
				{Architecture: "arm64", Path: filepath.Join(dir, "arm64.cdx.json")},
			})
			Expect(err).NotTo(HaveOccurred())

			manifest, err := attached.IndexManifest()
			Expect(err).NotTo(HaveOccurred())
			Expect(manifest.Manifests).To(HaveLen(3))
			Expect(manifest.Manifests[2].Annotations).To(HaveKeyWithValue("vnd.docker.reference.digest", arm64.String()))

			attestation, err := attached.Image(manifest.Manifests[2].Digest)
			Expect(err).NotTo(HaveOccurred())

			attestationManifest, err := attestation.Manifest()
			Expect(err).NotTo(HaveOccurred())
			Expect(attestationManifest.Layers).To(HaveLen(2))
			Expect(attestationManifest.Layers[0].Annotations).To(Equal(map[string]string{"in-toto.io/predicate-type": "https://cyclonedx.org/bom"}))
			Expect(attestationManifest.Layers[1].Annotations).To(Equal(map[string]string{"in-toto.io/predicate-type": "https://spdx.dev/Document"}))

			layers, err := attestation.Layers()
			Expect(err).NotTo(HaveOccurred())

			reader, err := layers[1].Uncompressed()
			Expect(err).NotTo(HaveOccurred())
			content, err := io.ReadAll(reader)
			Expect(err).NotTo(HaveOccurred())
			Expect(reader.Close()).To(Succeed())

			var statement struct {
				Predicate struct {
					SPDXVersion       string `json:"spdxVersion"`
					DocumentNamespace string `json:"documentNamespace"`
					CreationInfo      struct {
						Created string `json:"created"`
					} `json:"creationInfo"`
					Packages []struct {
						Name string `json:"name"`
					} `json:"packages"`
				} `json:"predicate"`
			}
			Expect(json.Unmarshal(content, &statement)).To(Succeed())

			Expect(statement.Predicate.SPDXVersion).To(HavePrefix("SPDX-"))
			Expect(statement.Predicate.DocumentNamespace).To(HavePrefix("https://paketo.io/spdx/"))
			Expect(statement.Predicate.CreationInfo.Created).To(Equal("2026-01-02T03:04:05Z"))

			var names []string
			for _, p := range statement.Predicate.Packages {
				names = append(names, p.Name)
			}
			Expect(names).To(ContainElement("arm64-package"))
		})

		it("gives the same index whenever it is computed, and whichever tag is published", func() {
			sboms := []ihop.SBOMAttestation{
				{Architecture: "amd64", Path: filepath.Join(dir, "amd64.cdx.json")},
				{Architecture: "arm64", Path: filepath.Join(dir, "arm64.cdx.json")},
			}

			first, err := ihop.AttachSBOMs(index, ref, sboms)
			Expect(err).NotTo(HaveOccurred())
			firstDigest, err := first.Digest()
			Expect(err).NotTo(HaveOccurred())

			latest, err := name.ParseReference("registry.example.com/some-org/some-image:latest")
			Expect(err).NotTo(HaveOccurred())

			second, err := ihop.AttachSBOMs(index, latest, sboms)
			Expect(err).NotTo(HaveOccurred())
			Expect(second.Digest()).To(Equal(firstDigest))
		})
	})

	context("failure cases", func() {
		it("fails when the image has no such architecture", func() {
			_, err := ihop.AttachSBOMs(index, ref, []ihop.SBOMAttestation{
				{Architecture: "s390x", Path: filepath.Join(dir, "amd64.cdx.json")},
			})
			Expect(err).To(MatchError(`unable to attach SBOM: the image has no "s390x" architecture`))
		})

		it("fails when the SBOM cannot be read", func() {
			_, err := ihop.AttachSBOMs(index, ref, []ihop.SBOMAttestation{
				{Architecture: "amd64", Path: filepath.Join(dir, "no-such-file.json")},
			})
			Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
		})

		it("fails when the SBOM is neither CycloneDX nor SPDX", func() {
			path := filepath.Join(dir, "other.json")
			Expect(os.WriteFile(path, []byte(`{"artifacts":[]}`), 0600)).To(Succeed())

			_, err := ihop.AttachSBOMs(index, ref, []ihop.SBOMAttestation{{Architecture: "amd64", Path: path}})
			Expect(err).To(MatchError(ContainSubstring("neither CycloneDX nor SPDX JSON")))
		})
	})
}
