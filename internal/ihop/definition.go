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

	// Name is a human readable name of the stack.
	Name string `toml:"name"`

	// Homepage is the homepage for the stack.
	Homepage string `toml:"homepage"`

	// SupportURL is the support homepage for the stack.
	SupportURL string `toml:"support-url"`

	// BugReportURL is the bug report homepage for the stack.
	BugReportURL string `toml:"bug-report-url"`

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

	// Labels can be used to add custom labels to the build and run image.
	Labels []string `toml:"-"`
}

type DefinitionImagePlatforms struct {
	Args map[string]any `toml:"args"`
}

// DefinitionImage defines the definition of a build or run stack image.
type DefinitionImage struct {
	// Args can be used to pass arguments to the Dockerfile as might be done with
	// the --build-arg docker CLI flag.
	Args map[string]any `toml:"args"`

	// Platforms is a map of additional platform specific arguments where the key
	// is the platform name. Platform args of the same name override Args.
	Platforms map[string]DefinitionImagePlatforms `toml:"platforms"`

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
func (i DefinitionImage) Arguments(platform string) ([]string, error) {
	var args []string
	for key, value := range i.mergeArgs(platform) {
		var v string

		switch valTyped := value.(type) {
		case string:
			v = valTyped
		case int, int64, int32, int16, int8:
			v = fmt.Sprintf("%d", valTyped)
		case []string:
			v = strings.Join(valTyped, " ")
		case []any:
			typedSlice := make([]string, len(valTyped))
			for i, e := range valTyped {
				switch elementTyped := e.(type) {
				case string:
					typedSlice[i] = elementTyped
				case int, int64, int32, int16, int8:
					typedSlice[i] = fmt.Sprintf("%d", elementTyped)
				default:
					return nil, fmt.Errorf("unsupported type %T for the argument element %q.%d", elementTyped, key, i)
				}
			}
			v = strings.Join(typedSlice, " ")
		default:
			return nil, fmt.Errorf("unsupported type %T for the argument %q", valTyped, key)
		}

		args = append(args, fmt.Sprintf("%s=%s", key, v))
	}

	return args, nil
}

// mergeArgs merges Args and Platform Args if any exist for the platform with Platform Args overriding Args
func (d DefinitionImage) mergeArgs(platform string) map[string]any {
	allArgs := make(map[string]any)

	for key, value := range d.Args {
		allArgs[key] = value
	}

	if platform, ok := d.Platforms[platform]; ok {
		for key, value := range platform.Args {
			allArgs[key] = value
		}
	}

	return allArgs
}

// NewDefinitionFromFile parses the stack descriptor from a file location.
func NewDefinitionFromFile(path string, secrets ...string) (Definition, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return Definition{}, err
	}

	file, err := os.Open(path)
	if err != nil {
		return Definition{}, err
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

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

	if definition.SupportURL == "" && strings.Contains(definition.Homepage, "github.com") {
		definition.SupportURL = fmt.Sprintf("%s/blob/main/README.md", strings.TrimSuffix(definition.Homepage, "/"))
	}

	if definition.BugReportURL == "" && strings.Contains(definition.Homepage, "github.com") {
		definition.BugReportURL = fmt.Sprintf("%s/issues/new", strings.TrimSuffix(definition.Homepage, "/"))
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

	return definition, nil
}

func (d Definition) containsOsReleaseOverwrites() bool {
	return d.Name != "" || d.Homepage != "" || d.SupportURL != "" || d.BugReportURL != ""
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
