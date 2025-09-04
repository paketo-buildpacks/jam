package internal

import (
	"net/url"
	"strings"
)

// https://buildpacks.io/docs/reference/config/package-config/#dependencies-list-optional
func isArchive(dependency PackageConfigDependency) bool {
	if isCnbRegistry(dependency) {
		return false
	}

	if isDocker(dependency) {
		return false
	}

	if uri, err := url.Parse(dependency.URI); err != nil {
		// fallback to the default: URI is likely malformed
		return false
	} else {
		return strings.HasSuffix(uri.Path, ".cnb")
	}
}

func isCnbRegistry(dependency PackageConfigDependency) bool {
	return strings.HasPrefix(dependency.URI, "urn:cnb:registry")
}

func isDocker(dependency PackageConfigDependency) bool {
	return strings.HasPrefix(dependency.URI, "docker://")
}
