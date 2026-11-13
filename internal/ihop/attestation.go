package ihop

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/anchore/syft/syft/format"
	"github.com/anchore/syft/syft/format/spdxjson"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	inTotoMediaType             = "application/vnd.in-toto+json"
	inTotoStatementType         = "https://in-toto.io/Statement/v0.1"
	inTotoPredicateAnnotation   = "in-toto.io/predicate-type"
	cycloneDXPredicateType      = "https://cyclonedx.org/bom"
	spdxPredicateType           = "https://spdx.dev/Document"
	attestationTypeAnnotation   = "vnd.docker.reference.type"
	attestationDigestAnnotation = "vnd.docker.reference.digest"
	attestationManifestType     = "attestation-manifest"
	unknownPlatform             = "unknown"
)

// An SBOMAttestation is the CycloneDX or SPDX JSON SBOM of one architecture of an image.
type SBOMAttestation struct {
	Architecture string
	Path         string
}

// AttachSBOMs adds each SBOM to the index as a BuildKit-style attestation, without touching the
// images. CycloneDX is also attached converted to SPDX, the format Docker's tooling reads.
func AttachSBOMs(index v1.ImageIndex, ref name.Reference, sboms []SBOMAttestation) (v1.ImageIndex, error) {
	if len(sboms) == 0 {
		return index, nil
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}

	byArchitecture := map[string][]SBOMAttestation{}
	for _, sbom := range sboms {
		byArchitecture[sbom.Architecture] = append(byArchitecture[sbom.Architecture], sbom)
	}

	var addenda []mutate.IndexAddendum
	for _, descriptor := range manifest.Manifests {
		if descriptor.Platform == nil {
			continue
		}

		sboms, ok := byArchitecture[descriptor.Platform.Architecture]
		if !ok {
			continue
		}
		delete(byArchitecture, descriptor.Platform.Architecture)

		documents, err := sbomDocuments(sboms)
		if err != nil {
			return nil, err
		}

		attestation := empty.Image
		for _, document := range documents {
			statement, err := sbomStatement(document, ref, descriptor)
			if err != nil {
				return nil, err
			}

			attestation, err = mutate.Append(attestation, mutate.Addendum{
				Layer:       static.NewLayer(statement, inTotoMediaType),
				Annotations: map[string]string{inTotoPredicateAnnotation: document.predicateType},
			})
			if err != nil {
				return nil, err
			}
		}

		attestation = mutate.MediaType(attestation, types.OCIManifestSchema1)
		attestation = mutate.ConfigMediaType(attestation, types.OCIConfigJSON)

		config, err := attestation.ConfigFile()
		if err != nil {
			return nil, err
		}
		config.Architecture, config.OS = unknownPlatform, unknownPlatform

		attestation, err = mutate.ConfigFile(attestation, config)
		if err != nil {
			return nil, err
		}

		addenda = append(addenda, mutate.IndexAddendum{
			Add: attestation,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{Architecture: unknownPlatform, OS: unknownPlatform},
				Annotations: map[string]string{
					attestationTypeAnnotation:   attestationManifestType,
					attestationDigestAnnotation: descriptor.Digest.String(),
				},
			},
		})
	}

	for architecture := range byArchitecture {
		return nil, fmt.Errorf("unable to attach SBOM: the image has no %q architecture", architecture)
	}

	return mutate.AppendManifests(index, addenda...), nil
}

type sbomDocument struct {
	predicateType string
	content       []byte
}

// sbomDocuments reads the SBOMs of an image, adding the SPDX conversion of the CycloneDX ones.
func sbomDocuments(sboms []SBOMAttestation) ([]sbomDocument, error) {
	var documents []sbomDocument
	var hasSPDX bool

	for _, sbom := range sboms {
		content, err := os.ReadFile(sbom.Path)
		if err != nil {
			return nil, err
		}

		var format struct {
			BOMFormat   string `json:"bomFormat"`
			SPDXVersion string `json:"spdxVersion"`
		}
		if err := json.Unmarshal(content, &format); err != nil {
			return nil, fmt.Errorf("unable to read SBOM %s: %w", sbom.Path, err)
		}

		switch {
		case format.BOMFormat == "CycloneDX":
			documents = append(documents, sbomDocument{predicateType: cycloneDXPredicateType, content: content})
		case format.SPDXVersion != "":
			documents = append(documents, sbomDocument{predicateType: spdxPredicateType, content: content})
			hasSPDX = true
		default:
			return nil, fmt.Errorf("unable to read SBOM %s: neither CycloneDX nor SPDX JSON", sbom.Path)
		}
	}

	if hasSPDX {
		return documents, nil
	}

	for _, document := range documents {
		spdx, err := cycloneDXToSPDX(document.content)
		if err != nil {
			return nil, fmt.Errorf("unable to convert SBOM to SPDX: %w", err)
		}
		documents = append(documents, sbomDocument{predicateType: spdxPredicateType, content: spdx})
	}

	return documents, nil
}

// cycloneDXToSPDX converts with Syft, then replaces the random namespace and the current time it
// sets, so that the same CycloneDX document always gives the same SPDX one.
func cycloneDXToSPDX(cycloneDX []byte) ([]byte, error) {
	sbom, _, _, err := format.Decode(bytes.NewReader(cycloneDX))
	if err != nil {
		return nil, err
	}

	encoder, err := spdxjson.NewFormatEncoderWithConfig(spdxjson.DefaultEncoderConfig())
	if err != nil {
		return nil, err
	}

	encoded, err := format.Encode(*sbom, encoder)
	if err != nil {
		return nil, err
	}

	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		return nil, err
	}

	var source struct {
		Metadata struct {
			Timestamp string `json:"timestamp"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(cycloneDX, &source); err != nil {
		return nil, err
	}
	created := source.Metadata.Timestamp
	if created == "" {
		created = "1970-01-01T00:00:00Z"
	}

	document["documentNamespace"] = fmt.Sprintf("https://paketo.io/spdx/%x", sha256.Sum256(cycloneDX))
	if creationInfo, ok := document["creationInfo"].(map[string]any); ok {
		creationInfo["created"] = created
	}

	return json.Marshal(document)
}

// sbomStatement wraps the SBOM in an in-toto statement about the image.
func sbomStatement(document sbomDocument, ref name.Reference, image v1.Descriptor) ([]byte, error) {
	// No tag in the name: the statement must be the same under every tag the image is published as.
	subject := fmt.Sprintf("pkg:docker/%s?platform=%s", ref.Context().Name(),
		url.QueryEscape(fmt.Sprintf("%s/%s", image.Platform.OS, image.Platform.Architecture)))

	return json.Marshal(map[string]any{
		"_type":         inTotoStatementType,
		"predicateType": document.predicateType,
		"subject": []any{map[string]any{
			"name":   subject,
			"digest": map[string]string{image.Digest.Algorithm: image.Digest.Hex},
		}},
		"predicate": json.RawMessage(document.content),
	})
}
