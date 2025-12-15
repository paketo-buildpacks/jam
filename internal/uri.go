package internal

import (
	"github.com/buildpacks/pack/pkg/buildpack"
)

// IsStaticURI returns true if the dependency URI is a static reference
// (local file path, HTTP/HTTPS URL, or file:// URI) that should not be updated.
// https://buildpacks.io/docs/reference/config/package-config/#dependencies-list-optional
func IsStaticURI(dependency PackageConfigDependency) bool {
	locatorType, err := buildpack.GetLocatorType(dependency.URI, ".", nil)
	if err != nil {
		return false
	}

	return locatorType == buildpack.URILocator
}

// IsCnbRegistry returns true if the dependency URI is a CNB registry reference.
// Example: urn:cnb:registry:paketo-buildpacks/go-dist@0.20.1
func IsCnbRegistry(dependency PackageConfigDependency) bool {
	locatorType, err := buildpack.GetLocatorType(dependency.URI, ".", nil)
	if err != nil {
		return false
	}

	return locatorType == buildpack.RegistryLocator
}

// IsDocker returns true if the dependency URI is a Docker image reference.
// This includes both docker:// URIs and bare image references like registry/image:tag
// (after docker:// prefix has been stripped by UnmarshalTOML).
func IsDocker(dependency PackageConfigDependency) bool {
	locatorType, err := buildpack.GetLocatorType(dependency.URI, ".", nil)
	if err != nil {
		return false
	}

	return locatorType == buildpack.PackageLocator
}
