package matchers

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func matchImage(expected, actual interface{}, matcher func(*tar.Header, io.Reader) (bool, error)) (bool, error) {
	path, ok := expected.(string)
	if !ok {
		return false, fmt.Errorf("expected must be a <string>, received %#v", expected)
	}

	path = fmt.Sprintf("^%s$", strings.TrimPrefix(path, "/"))

	re, err := regexp.Compile(path)
	if err != nil {
		return false, err
	}

	var layers []v1.Layer
	if image, ok := actual.(v1.Image); ok {
		layers, err = image.Layers()
		if err != nil {
			return false, err
		}
	}

	if layer, ok := actual.(v1.Layer); ok {
		layers = append(layers, layer)
	}

	for i := len(layers) - 1; i >= 0; i-- {
		reader, err := layers[i].Uncompressed()
		if err != nil {
			return false, err
		}

		var found bool
		tr := tar.NewReader(reader)
		for {
			hdr, err := tr.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return false, err
			}

			if re.MatchString(strings.TrimSuffix(strings.TrimPrefix(hdr.Name, "/"), "/")) {
				result, err := matcher(hdr, tr)
				if err != nil {
					return false, err
				}

				if result {
					found = true
					break
				}
			}
		}

		err = reader.Close()
		if err != nil {
			return false, err
		}

		if found {
			return true, nil
		}
	}

	return false, nil
}
