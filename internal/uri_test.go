package internal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal"
)

func TestIsStaticURI(t *testing.T) {
	// Create temp directory and test files
	tempDir := t.TempDir()

	cnbFile := filepath.Join(tempDir, "buildpack.cnb")
	if err := os.WriteFile(cnbFile, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to create test .cnb file: %v", err)
	}

	subDir := filepath.Join(tempDir, "buildpack-dir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	tests := []struct {
		name     string
		uri      string
		expected bool
	}{
		// Local file paths - should be static if exist and are .cnb or directory
		{
			name:     "existing .cnb file",
			uri:      cnbFile,
			expected: true,
		},
		{
			name:     "existing directory",
			uri:      subDir,
			expected: true,
		},
		{
			name:     "non-existent relative path",
			uri:      "./non-existent-buildpack.cnb",
			expected: false,
		},
		{
			name:     "non-existent absolute path",
			uri:      "/non/existent/path/buildpack.cnb",
			expected: false,
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
			expected: true,
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
			expected: true,
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
