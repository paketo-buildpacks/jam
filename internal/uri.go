package internal

import (
	"net/url"
	"os"
	"strings"
)

// https://buildpacks.io/docs/reference/config/package-config/#dependencies-list-optional
func IsArchive(dependency PackageConfigDependency) bool {
	if IsCnbRegistry(dependency) {
		return false
	}

	if IsDocker(dependency) {
		return false
	}

	return IsCnbFile(dependency)
}

func IsCnbFile(dependency PackageConfigDependency) bool {
	if uri, err := url.Parse(dependency.URI); err != nil {
		// fallback to the default: URI is likely malformed
		return false
	} else {
		return strings.HasSuffix(uri.Path, ".cnb")
	}
}

func IsCnbRegistry(dependency PackageConfigDependency) bool {
	return strings.HasPrefix(dependency.URI, "urn:cnb:registry")
}

func IsDirectory(dependency PackageConfigDependency) bool {
	info, err := os.Stat(dependency.URI)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func IsDocker(dependency PackageConfigDependency) bool {
	return strings.HasPrefix(dependency.URI, "docker://")
}
