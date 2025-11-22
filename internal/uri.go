package internal

import (
	"strings"
)

// IsStaticURI returns true if the dependency URI is a static reference
// (local file path, HTTP/HTTPS URL, or file:// URI) that should not be updated.
// https://buildpacks.io/docs/reference/config/package-config/#dependencies-list-optional
func IsStaticURI(dependency PackageConfigDependency) bool {
	uri := dependency.URI

	// Check for local paths (relative or absolute)
	if strings.HasPrefix(uri, "/") ||
		strings.HasPrefix(uri, "./") ||
		strings.HasPrefix(uri, "../") {
		return true
	}

	// Check for URL schemes (http, https, file)
	if strings.HasPrefix(uri, "http://") ||
		strings.HasPrefix(uri, "https://") ||
		strings.HasPrefix(uri, "file://") {
		return true
	}

	return false
}

func IsCnbRegistry(dependency PackageConfigDependency) bool {
	return strings.HasPrefix(dependency.URI, "urn:cnb:registry")
}

func IsDocker(dependency PackageConfigDependency) bool {
	return strings.HasPrefix(dependency.URI, "docker://")
}
