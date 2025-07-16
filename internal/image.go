package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/buildpacks/pack/pkg/buildpack"
	backoff "github.com/cenkalti/backoff/v4"
	"github.com/distribution/reference"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Image struct {
	Name    string
	Path    string
	Version string
}

func FindLatestImageOnCNBRegistry(uri, api, patchVersion string) (Image, error) {
	id, _ := buildpack.ParseIDLocator(uri)
	var resp *http.Response
	var err error

	retryTimeLimit, err := time.ParseDuration("3m")
	if err != nil {
		return Image{}, err
	}
	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.MaxElapsedTime = retryTimeLimit

	// Use exponential backoff to retry failed requests when they fail with http.StatusTooManyRequests
	err = backoff.RetryNotify(func() error {
		resp, err = http.Get(fmt.Sprintf("%s/v1/buildpacks/%s", api, id))
		if err != nil {
			return &backoff.PermanentError{Err: err}
		}

		// only retry when the CNB registry status code is http.StatusTooManyRequests (429)
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("unexpected response status: %s", resp.Status)
			if resp.StatusCode == http.StatusTooManyRequests {
				return err
			}
			return &backoff.PermanentError{Err: err}
		}
		return nil
	},
		exponentialBackoff,
		func(err error, t time.Duration) {
			fmt.Println(err)
			fmt.Printf("Retrying in %s\n", t)
		},
	)
	if err != nil {
		return Image{}, err
	}
	defer func() {
		if err2 := resp.Body.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	var metadata struct {
		Latest struct {
			Version string `json:"version"`
		} `json:"latest"`
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	}

	err = json.NewDecoder(resp.Body).Decode(&metadata)
	if err != nil {
		return Image{}, err
	}

	// If a patch version is passed in, get the highest patch in the same minor version line
	if patchVersion != "" {
		versions := []string{}
		for _, v := range metadata.Versions {
			versions = append(versions, v.Version)
		}

		highestPatch, err := getHighestPatch(patchVersion, versions)
		if err != nil {
			return Image{}, fmt.Errorf("could not get the highest patch in the %s line: %w", patchVersion, err)
		}
		return Image{
			Name:    fmt.Sprintf("urn:cnb:registry:%s", id),
			Path:    id,
			Version: highestPatch,
		}, err // err should be nil here, but return err to catch deferred error
	}

	return Image{
		Name:    fmt.Sprintf("urn:cnb:registry:%s", id),
		Path:    id,
		Version: metadata.Latest.Version,
	}, err // err should be nil here, but return err to catch deferred error
}

func FindLatestImage(uri, patchVersion string) (Image, error) {
	named, err := reference.ParseNormalizedNamed(uri)
	if err != nil {
		return Image{}, fmt.Errorf("failed to parse image reference %q: %w", uri, err)
	}

	repo, err := name.NewRepository(reference.Path(named))
	if err != nil {
		return Image{}, fmt.Errorf("failed to parse image repository: %w", err)
	}

	repo.Registry, err = name.NewRegistry(reference.Domain(named))
	if err != nil {
		return Image{}, fmt.Errorf("failed to parse image registry: %w", err)
	}

	tags, err := remote.List(repo, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return Image{}, fmt.Errorf("failed to list tags: %w", err)
	}

	if patchVersion != "" {
		highestPatch, err := getHighestPatch(patchVersion, tags)
		if err != nil {
			return Image{}, fmt.Errorf("could not get the highest patch in the %s line: %w", patchVersion, err)
		}
		return Image{
			Name:    named.Name(),
			Path:    reference.Path(named),
			Version: highestPatch,
		}, nil
	}

	var versions []*semver.Version
	for _, tag := range tags {
		version, err := semver.StrictNewVersion(tag)
		if err != nil {
			continue
		}
		if version.Prerelease() != "" {
			continue
		}
		versions = append(versions, version)
	}

	if len(versions) == 0 {
		return Image{}, fmt.Errorf("could not find any valid tag for %s", repo.Name())
	}

	sort.Sort(semver.Collection(versions))

	return Image{
		Name:    named.Name(),
		Path:    reference.Path(named),
		Version: versions[len(versions)-1].String(),
	}, nil
}

// Finds latest build image, and the matching run image (if the run image has a semantic version, instead of latest)
func FindLatestStackImages(runURI, buildURI string) (Image, Image, error) {
	buildImage, err := FindLatestBuildImage(runURI, buildURI)
	if err != nil {
		return Image{}, Image{}, fmt.Errorf("failed to find latest build image: %w", err)
	}

	// If the run image tag is semver, update it to the same version as the build image
	var runImage Image
	runNamed, runTag, err := parseImageURI(runURI)
	if err != nil {
		return Image{}, Image{}, fmt.Errorf("failed to parse run image: %w", err)
	}
	runImageTagIsSemver, _ := checkRunImageTag(runTag)
	if runImageTagIsSemver {
		runImage = Image{
			Name:    runNamed.Name(),
			Path:    reference.Path(runNamed),
			Version: buildImage.Version,
		}
	}
	return runImage, buildImage, nil
}

func UpdateRunImageMirrors(version string, mirrors []string) ([]string, error) {
	for i, mirror := range mirrors {
		mirrorNamed, mirrorTag, err := parseImageURI(mirror)
		if err != nil {
			return []string{}, fmt.Errorf("failed to parse image '%s': %w", mirror, err)
		}
		// if the mirror URI is semver (ex. 1.2.3) (ex. 1.2.3-suffix)
		// update the version of the mirror to match the given version
		tagIsSemver, _ := checkRunImageTag(mirrorTag)
		if tagIsSemver {
			mirrors[i] = fmt.Sprintf("%s:%s", mirrorNamed.Name(), version)
		}
	}
	return mirrors, nil
}

func FindLatestBuildImage(runURI, buildURI string) (Image, error) {
	_, runTag, err := parseImageURI(runURI)
	if err != nil {
		return Image{}, fmt.Errorf("failed to parse run image: %w", err)
	}
	_, runTagSuffix := checkRunImageTag(runTag)

	buildNamed, err := reference.ParseNormalizedNamed(buildURI)
	if err != nil {
		return Image{}, fmt.Errorf("failed to parse build image reference %q: %w", buildURI, err)
	}

	repo, err := name.NewRepository(reference.Path(buildNamed))
	if err != nil {
		return Image{}, fmt.Errorf("failed to parse build image repository: %w", err)
	}

	repo.Registry, err = name.NewRegistry(reference.Domain(buildNamed))
	if err != nil {
		return Image{}, fmt.Errorf("failed to parse build image registry: %w", err)
	}

	tags, err := remote.List(repo, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return Image{}, fmt.Errorf("failed to list tags: %w", err)
	}

	var versions []*semver.Version
	for _, tag := range tags {

		version, err := semver.StrictNewVersion(tag)
		if err != nil {
			continue
		}

		// legacy case: if the build image tag has a suffix (ex.
		// <image>:1.2.3-suffix) (ex. <image>:suffix), it should be equal to the
		// run image tag suffix in order to be considered as a valid version
		// See this PR for more context: https://github.com/paketo-buildpacks/jam/pull/81
		if version.Prerelease() != "" && runTagSuffix != version.Prerelease() {
			fmt.Printf("Skipping build image version: %s, the tag suffix does not match run image tag: %s\n", tag, runTagSuffix)
			continue
		}

		versions = append(versions, version)
	}

	if len(versions) == 0 {
		return Image{}, fmt.Errorf("could not find any valid tag for %s", repo.Name())
	}

	sort.Sort(semver.Collection(versions))

	return Image{
		Name:    buildNamed.Name(),
		Path:    reference.Path(buildNamed),
		Version: versions[len(versions)-1].String(),
	}, nil
}

// Parse an image URI into a reference.Named type, and a tag
func parseImageURI(uri string) (reference.Named, string, error) {
	var imgNamed reference.Named
	var err error
	imgNamed, err = reference.ParseNormalizedNamed(uri)
	if err != nil {
		return imgNamed, "", fmt.Errorf("failed to parse image reference %q: %w", uri, err)
	}
	tagged, ok := imgNamed.(reference.Tagged)
	if !ok {
		return imgNamed, "", fmt.Errorf("expected the image to be tagged but it was not")
	}
	return imgNamed, tagged.Tag(), nil
}

// Given an image tag, check the tag for some rules pertaining to run images
// If the tag is:
// - `latest`: return runImageTagIsSemver=false, suffix=""
// - semantically versioned: return runImageTagIsSemver=true, suffix=""
// - semantically versioned with a suffix: return runImageTagIsSemver=true, suffix=<suffix>
// - not semantically versioned: return runImageTagIsSemver=false, suffix=<tag>
func checkRunImageTag(tag string) (bool, string) {
	var suffix string
	var runImageTagIsSemver bool

	if tag != "latest" {
		version, err := semver.StrictNewVersion(tag)
		if err != nil {
			// if the run image tag is NOT semantically versioned, follow legacy case:
			// one image repository is being used for multiple stacks;
			// tag suffixes are used to distinguish between them
			// ex. <image>:<non-semver-suffix>
			suffix = tag
		} else {
			// if the run image tag is semantically versioned, set runImageTagIsSemver to true
			// ex. <image>:1.2.3-suffix
			runImageTagIsSemver = true
			suffix = version.Prerelease()
		}
	}
	return runImageTagIsSemver, suffix
}

func GetBuildpackageID(uri string) (string, error) {
	ref, err := name.ParseReference(uri)
	if err != nil {
		return "", err
	}

	image, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", err
	}

	cfg, err := image.ConfigFile()
	if err != nil {
		return "", err
	}

	type BuildpackageMetadata struct {
		BuildpackageID string `json:"id"`
	}
	var metadataString string
	var ok bool
	if metadataString, ok = cfg.Config.Labels["io.buildpacks.buildpackage.metadata"]; !ok {
		return "", fmt.Errorf("could not get buildpackage id: image %s has no label 'io.buildpacks.buildpackage.metadata'", uri)
	}

	metadata := BuildpackageMetadata{}

	err = json.Unmarshal([]byte(metadataString), &metadata)
	if err != nil {
		return "", fmt.Errorf("could not unmarshal buildpackage metadata")
	}
	return metadata.BuildpackageID, nil
}

func getHighestPatch(patchVersion string, allVersions []string) (string, error) {
	versionConstraint, err := semver.NewConstraint(fmt.Sprintf("~%s", patchVersion))
	if err != nil {
		return "", fmt.Errorf("version constraint ~%s is not a valid semantic version constraint: %w", patchVersion, err)
	}
	highestPatch, err := semver.NewVersion(patchVersion)
	if err != nil {
		return "", fmt.Errorf("cannot convert %s to a semantic version: %w", patchVersion, err)
	}
	for _, versionEntry := range allVersions {
		version, err := semver.NewVersion(versionEntry)
		// do not error, since some upstream versions may not be semantic versions
		if err != nil {
			continue
		}
		if version.Prerelease() != "" {
			continue
		}

		if versionConstraint.Check(version) {
			if version.GreaterThan(highestPatch) {
				highestPatch = version
			}
		}
	}
	return highestPatch.String(), nil
}
