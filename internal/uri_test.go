package internal_test

import (
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal"
)

func TestIsStaticURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected bool
	}{
		// Local file paths - should be static
		{
			name:     "relative path with ./",
			uri:      "./local-buildpack.cnb",
			expected: true,
		},
		{
			name:     "relative path with ../",
			uri:      "../buildpacks/my-buildpack.cnb",
			expected: true,
		},
		{
			name:     "absolute path",
			uri:      "/usr/local/buildpacks/my-buildpack.cnb",
			expected: true,
		},

		// HTTP/HTTPS URLs - should be static
		{
			name:     "http URL with .cnb",
			uri:      "http://example.com/buildpacks/my-buildpack.cnb",
			expected: true,
		},
		{
			name:     "https URL with .cnb",
			uri:      "https://example.com/buildpacks/my-buildpack.cnb",
			expected: true,
		},
		{
			name:     "http URL with .tar.gz",
			uri:      "http://example.com/buildpacks/my-buildpack.tar.gz",
			expected: true,
		},

		// File URLs - should be static
		{
			name:     "file:// URL",
			uri:      "file:///path/to/buildpack.cnb",
			expected: true,
		},

		// Docker images - should NOT be static (dynamic refs)
		{
			name:     "docker:// URI",
			uri:      "docker://registry/image:tag",
			expected: false,
		},
		{
			name:     "bare image reference (docker:// stripped)",
			uri:      "registry.example.com/my-org/my-buildpack:1.0.0",
			expected: false,
		},
		{
			name:     "simple image reference",
			uri:      "paketobuildpacks/go-dist:0.20.1",
			expected: false,
		},

		// CNB registry - should NOT be static (dynamic refs)
		{
			name:     "urn:cnb:registry URI",
			uri:      "urn:cnb:registry:paketo-buildpacks/go-dist@0.20.1",
			expected: false,
		},

		// Edge cases
		{
			name:     "empty URI",
			uri:      "",
			expected: false,
		},
		{
			name:     "bare filename without path prefix",
			uri:      "buildpack.cnb",
			expected: false,
		},
		{
			name:     "registry path",
			uri:      "gcr.io/buildpacks/builder:latest",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep := internal.PackageConfigDependency{URI: tt.uri}
			result := internal.IsStaticURI(dep)
			if result != tt.expected {
				t.Errorf("IsStaticURI(%q) = %v, expected %v", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestIsCnbRegistry(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected bool
	}{
		{
			name:     "urn:cnb:registry URI",
			uri:      "urn:cnb:registry:paketo-buildpacks/go-dist@0.20.1",
			expected: true,
		},
		{
			name:     "docker URI",
			uri:      "docker://registry/image:tag",
			expected: false,
		},
		{
			name:     "http URI",
			uri:      "http://example.com/buildpack.cnb",
			expected: false,
		},
		{
			name:     "local path",
			uri:      "./buildpack.cnb",
			expected: false,
		},
		{
			name:     "bare image reference",
			uri:      "registry/image:tag",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep := internal.PackageConfigDependency{URI: tt.uri}
			result := internal.IsCnbRegistry(dep)
			if result != tt.expected {
				t.Errorf("IsCnbRegistry(%q) = %v, expected %v", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestIsDocker(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected bool
	}{
		{
			name:     "docker:// URI",
			uri:      "docker://registry/image:tag",
			expected: true,
		},
		{
			name:     "bare image reference",
			uri:      "registry/image:tag",
			expected: false,
		},
		{
			name:     "urn:cnb:registry URI",
			uri:      "urn:cnb:registry:paketo-buildpacks/go-dist@0.20.1",
			expected: false,
		},
		{
			name:     "http URI",
			uri:      "http://example.com/buildpack.cnb",
			expected: false,
		},
		{
			name:     "local path",
			uri:      "./buildpack.cnb",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep := internal.PackageConfigDependency{URI: tt.uri}
			result := internal.IsDocker(dep)
			if result != tt.expected {
				t.Errorf("IsDocker(%q) = %v, expected %v", tt.uri, result, tt.expected)
			}
		})
	}
}
