package internal

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
)

type BuilderConfig struct {
	Description    string                        `toml:"description"`
	Buildpacks     []BuilderConfigBuildpack      `toml:"buildpacks"`
	Lifecycle      BuilderConfigLifecycle        `toml:"lifecycle"`
	Order          []BuilderConfigOrder          `toml:"order"`
	Extensions     []BuilderConfigExtension      `toml:"extensions"`
	OrderExtension []BuilderExtensionConfigOrder `toml:"order-extensions"`
	Stack          BuilderConfigStack            `toml:"stack"`
	Targets        []BuilderConfigTarget         `toml:"targets"`
}

type BuilderConfigBuildpack struct {
	URI     string `toml:"uri"`
	Version string `toml:"version"`
}

type BuilderConfigExtension struct {
	ID      string `toml:"id"`
	URI     string `toml:"uri"`
	Version string `toml:"version"`
}

type BuilderConfigLifecycle struct {
	Version string `toml:"version"`
}

type BuilderConfigOrder struct {
	Group []BuilderConfigOrderGroup `toml:"group"`
}

type BuilderConfigOrderGroup struct {
	ID       string `toml:"id"`
	Version  string `toml:"version,omitempty"`
	Optional bool   `toml:"optional,omitempty"`
}

type BuilderExtensionConfigOrder struct {
	Group []BuilderExtensionConfigOrderGroup `toml:"group"`
}

type BuilderExtensionConfigOrderGroup struct {
	ID       string `toml:"id"`
	Version  string `toml:"version,omitempty"`
	Optional bool   `toml:"optional,omitempty"`
}

type BuilderConfigStack struct {
	ID              string   `toml:"id"`
	BuildImage      string   `toml:"build-image"`
	RunImage        string   `toml:"run-image"`
	RunImageMirrors []string `toml:"run-image-mirrors"`
}

type BuilderConfigTarget struct {
	OS   string `toml:"os"`
	Arch string `toml:"arch"`
}

// Note: this is to support that buildpackages can refer to this field as `image` or `uri`.
func (b *BuilderConfigBuildpack) UnmarshalTOML(v interface{}) error {
	if m, ok := v.(map[string]interface{}); ok {
		if image, ok := m["image"].(string); ok {
			b.URI = image
		}

		if uri, ok := m["uri"].(string); ok {
			b.URI = uri
		}

		if version, ok := m["version"].(string); ok {
			b.Version = version
		}
	}

	if b.URI != "" {
		uri, err := url.Parse(b.URI)
		if err != nil {
			return err
		}

		uri.Scheme = ""

		b.URI = strings.TrimPrefix(uri.String(), "//")
	}

	return nil
}

func (b *BuilderConfigExtension) UnmarshalTOML(v interface{}) error {
	if m, ok := v.(map[string]interface{}); ok {
		if image, ok := m["image"].(string); ok {
			b.URI = image
		}

		if uri, ok := m["uri"].(string); ok {
			b.URI = uri
		}

		if version, ok := m["version"].(string); ok {
			b.Version = version
		}

		if id, ok := m["id"].(string); ok {
			b.ID = id
		}
	}

	if b.URI != "" {
		uri, err := url.Parse(b.URI)
		if err != nil {
			return err
		}

		uri.Scheme = ""

		b.URI = strings.TrimPrefix(uri.String(), "//")
	}

	return nil
}

func ParseBuilderConfig(path string) (BuilderConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return BuilderConfig{}, fmt.Errorf("failed to open builder config file: %w", err)
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	var config BuilderConfig
	err = toml.NewDecoder(file).Decode(&config)
	if err != nil {
		return BuilderConfig{}, fmt.Errorf("failed to parse builder config: %w", err)
	}

	return config, err // err should be nil here, but return err to catch deferred error
}

func OverwriteBuilderConfig(path string, config BuilderConfig) error {
	for i, buildpack := range config.Buildpacks {
		if !strings.HasPrefix(buildpack.URI, "docker://") {
			config.Buildpacks[i].URI = fmt.Sprintf("docker://%s", buildpack.URI)
		}
	}

	for i, extension := range config.Extensions {
		if !strings.HasPrefix(extension.URI, "docker://") {
			config.Extensions[i].URI = fmt.Sprintf("docker://%s", extension.URI)
		}
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open builder config file: %w", err)
	}

	err = toml.NewEncoder(file).Encode(config)
	if err != nil {
		return fmt.Errorf("failed to write builder config: %w", err)
	}

	return nil
}
