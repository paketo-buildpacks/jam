package internal

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
)

type PackageConfig struct {
	Buildpack    interface{}               `toml:"buildpack"`
	Dependencies []PackageConfigDependency `toml:"dependencies"`
	Targets      []PackageConfigTarget     `toml:"targets,omitempty"`
}

type PackageConfigDependency struct {
	URI string `toml:"uri"`
}

type PackageConfigTarget struct {
	OS   string `toml:"os,omitempty"`
	Arch string `toml:"arch,omitempty"`
}

// Note: this is to support that buildpackages can refer to this field as `image` or `uri`.
func (d *PackageConfigDependency) UnmarshalTOML(v interface{}) error {
	if m, ok := v.(map[string]interface{}); ok {
		if image, ok := m["image"].(string); ok {
			d.URI = image
		}

		if uri, ok := m["uri"].(string); ok {
			d.URI = uri
		}
	}

	if d.URI != "" {
		if !strings.HasPrefix(d.URI, "urn:cnb:registry") {
			uri, err := url.Parse(d.URI)
			if err != nil {
				return err
			}

			uri.Scheme = ""

			d.URI = strings.TrimPrefix(uri.String(), "//")
		}
	}

	return nil
}

func ParsePackageConfig(path string) (PackageConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return PackageConfig{}, fmt.Errorf("failed to open package config file: %w", err)
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	var config PackageConfig
	err = toml.NewDecoder(file).Decode(&config)
	if err != nil {
		return PackageConfig{}, fmt.Errorf("failed to parse package config: %w", err)
	}

	return config, err // err should be nil here, but return err to catch deferred error
}

func OverwritePackageConfig(path string, config PackageConfig) error {
	for i, dependency := range config.Dependencies {
		if !strings.HasPrefix(dependency.URI, "docker://") && !strings.HasPrefix(dependency.URI, "urn:cnb:registry") {
			config.Dependencies[i].URI = fmt.Sprintf("docker://%s", dependency.URI)
		}
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open package config file: %w", err)
	}

	err = toml.NewEncoder(file).Encode(config)
	if err != nil {
		return fmt.Errorf("failed to write package config: %w", err)
	}

	return nil
}
