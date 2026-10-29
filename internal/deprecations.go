package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

// WarnDeprecatedDependencyKeys writes a warning for each deprecated dependency
// metadata key found in the given buildpack.toml or extension.toml file. This
// exists because Go cannot report when a deprecated struct field is read, so
// the best signal is that the document itself still uses the older format.
func WarnDeprecatedDependencyKeys(path string, writer io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	deprecations, err := cargo.DeprecatedDependencyKeys(file)
	if err != nil {
		return err
	}

	for _, deprecation := range deprecations {
		if deprecation.ID != "" {
			_, _ = fmt.Fprintf(writer, "WARNING: %s: dependency %q (%s) uses deprecated key %q, use %q instead\n",
				filepath.Base(path), deprecation.ID, deprecation.Path, deprecation.Key, deprecation.Replacement)
			continue
		}

		_, _ = fmt.Fprintf(writer, "WARNING: %s: %s uses deprecated key %q, use %q instead\n",
			filepath.Base(path), deprecation.Path, deprecation.Key, deprecation.Replacement)
	}

	return nil
}
