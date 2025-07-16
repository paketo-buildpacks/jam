package internal

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml"
)

type BuildpackConfig struct {
	API       interface{}             `toml:"api"`
	Buildpack interface{}             `toml:"buildpack"`
	Metadata  interface{}             `toml:"metadata"`
	Order     []BuildpackConfigOrder  `toml:"order"`
	Stacks    []BuildpackConfigStack  `toml:"stacks,omitempty"`
	Targets   []BuildpackConfigTarget `toml:"targets,omitempty"`
}

type BuildpackConfigOrder struct {
	Group []BuildpackConfigOrderGroup `toml:"group"`
}

type BuildpackConfigOrderGroup struct {
	ID       string `toml:"id"`
	Version  string `toml:"version,omitempty"`
	Optional bool   `toml:"optional,omitempty"`
}

type BuildpackConfigStack struct {
	ID     string   `toml:"id"`
	Mixins []string `toml:"mixins,omitempty"`
}

type BuildpackConfigTarget struct {
	OS   string `toml:"os,omitempty"`
	Arch string `toml:"arch,omitempty"`
}

func ParseBuildpackConfig(path string) (BuildpackConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return BuildpackConfig{}, fmt.Errorf("failed to open buildpack config file: %w", err)
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	var config BuildpackConfig
	err = toml.NewDecoder(file).Decode(&config)
	if err != nil {
		return BuildpackConfig{}, fmt.Errorf("failed to parse buildpack config: %w", err)
	}

	return config, err // err should be nil here, but return err to catch deferred error
}

func OverwriteBuildpackConfig(path string, config BuildpackConfig) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open buildpack config file: %w", err)
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	err = toml.NewEncoder(file).Encode(config)
	if err != nil {
		return fmt.Errorf("failed to write buildpack config: %w", err)
	}

	return nil
}
