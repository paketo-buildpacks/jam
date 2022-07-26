package ihop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// A Definition represents the content of the stack descriptor file.
type Definition struct {
	// ID is the stack id applied to the built images.
	ID string `toml:"id"`

	// Homepage is the homepage for the stack.
	Homepage string `toml:"homepage"`

	// Maintainer is the named individual or group responsible for maintaining
	// the stack.
	Maintainer string `toml:"maintainer"`

	// Platforms is a list of platforms the built stack should support. These
	// values must conform to values accepted by the --platform flag on the
	// docker CLI.
	Platforms []string `toml:"platforms"`

	// Build is the DefinitionImage for the stack build image.
	Build DefinitionImage `toml:"build"`

	// Run is the DefinitionImage for the stack run image.
	Run DefinitionImage `toml:"run"`

	// Deprecated contains fields enabling deprecated features of the stack
	// images.
	Deprecated DefinitionDeprecated `toml:"deprecated"`

	// IncludeExperimentalSBOM can be used to attach an experimental SBOM layer
	// to the run image.
	IncludeExperimentalSBOM bool `toml:"-"`
}

// DefinitionImage defines the definition of a build or run stack image.
type DefinitionImage struct {
	// Args can be used to pass arguments to the Dockerfile as might be done with
	// the --build-arg docker CLI flag.
	Args map[string]string `toml:"args"`

	// Description will be used to fill the io.buildpacks.stack.description image
	// label.
	Description string `toml:"description"`

	// Dockerfile is the path to the Dockerfile used to build the image. The
	// surrounding directory is used as the build context.
	Dockerfile string `toml:"dockerfile"`

	// GID is the cnb group id to be specified in the image.
	GID int `toml:"gid"`

	// Secrets can be used to pass secret arguments to the Dockerfile build.
	Secrets map[string]string `toml:"-"`

	// Shell is the default shell to be configured for the cnb user.
	Shell string `toml:"shell"`

	// UID is the cnb user id to be specified in the image.
	UID int `toml:"uid"`

	unbuffered bool
}

// DefinitionDeprecated defines the deprecated features of the stack.
type DefinitionDeprecated struct {
	// LegacySBOM can be set to true to include the io.paketo.stack.packages
	// image label.
	LegacySBOM bool `toml:"legacy-sbom"`

	// Mixins can be set to true to include the io.buildpacks.stack.mixins image
	// label.
	Mixins bool `toml:"mixins"`
}

// Arguments converts the Args map into a slice of strings of the form
// key=value.
func (i DefinitionImage) Arguments() []string {
	var args []string
	for key, value := range i.Args {
		args = append(args, fmt.Sprintf("%s=%s", key, value))
	}

	return args
}

// NewDefinitionFromFile parses the stack descriptor from a file location.
func NewDefinitionFromFile(path string, unbuffered bool, secrets ...string) (Definition, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return Definition{}, err
	}

	file, err := os.Open(path)
	if err != nil {
		return Definition{}, err
	}
	defer file.Close()

	var definition Definition
	_, err = toml.NewDecoder(file).Decode(&definition)
	if err != nil {
		return Definition{}, err
	}

	// check that all required fields are set
	for field, v := range map[string]any{
		"id":               definition.ID,
		"build.dockerfile": definition.Build.Dockerfile,
		"build.uid":        definition.Build.UID,
		"build.gid":        definition.Build.GID,
		"run.dockerfile":   definition.Run.Dockerfile,
		"run.uid":          definition.Run.UID,
		"run.gid":          definition.Run.GID,
	} {
		var err error
		switch value := v.(type) {
		case string:
			if value == "" {
				err = NewDefinitionRequiredFieldError(field)
			}
		case int:
			if value == 0 {
				err = NewDefinitionRequiredFieldError(field)
			}
		}
		if err != nil {
			return Definition{}, fmt.Errorf("failed to parse stack descriptor: %w", err)
		}
	}

	// default to "linux/amd64" if no platforms are specified
	if len(definition.Platforms) == 0 {
		definition.Platforms = []string{"linux/amd64"}
	}

	// default to using the nologin shell if none is specified
	if definition.Build.Shell == "" {
		definition.Build.Shell = "/sbin/nologin"
	}

	if definition.Run.Shell == "" {
		definition.Run.Shell = "/sbin/nologin"
	}

	// convert the Dockerfile paths given in the stack descriptor to absolute
	// paths
	dir := filepath.Dir(path)
	if definition.Build.Dockerfile != "" {
		definition.Build.Dockerfile = filepath.Clean(filepath.Join(dir, definition.Build.Dockerfile))
	}
	if definition.Run.Dockerfile != "" {
		definition.Run.Dockerfile = filepath.Clean(filepath.Join(dir, definition.Run.Dockerfile))
	}

	// if there were secrets passed to this function, attach them to each
	// DefinitionImage
	if len(secrets) > 0 {
		definition.Build.Secrets = make(map[string]string)
		for _, secret := range secrets {
			key, value, found := strings.Cut(secret, "=")
			if !found {
				return Definition{}, fmt.Errorf("malformed secret: %q must be in the form \"key=value\"", secret)
			}

			definition.Build.Secrets[key] = value
		}

		definition.Run.Secrets = definition.Build.Secrets
	}

	definition.Build.unbuffered = unbuffered
	definition.Run.unbuffered = unbuffered

	return definition, nil
}

// DefinitionRequiredFieldError defines the error message when a required field
// is missing from the stack descriptor.
type DefinitionRequiredFieldError string

// NewDefinitionRequiredFieldError returns a DefinitionRequiredFieldError to
// report the absence of the given field.
func NewDefinitionRequiredFieldError(field string) DefinitionRequiredFieldError {
	return DefinitionRequiredFieldError(field)
}

// Error returns an error message indicating that a required field is missing
// from the stack descriptor.
func (e DefinitionRequiredFieldError) Error() string {
	return fmt.Sprintf("'%s' is a required field", string(e))
}
