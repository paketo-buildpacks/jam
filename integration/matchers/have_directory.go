package matchers

import (
	"archive/tar"
	"fmt"
	"io"

	"github.com/onsi/gomega/types"
)

func HaveDirectory(path interface{}) types.GomegaMatcher {
	return &haveDirectoryMatcher{
		path: path,
	}
}

type haveDirectoryMatcher struct {
	path interface{}
}

func (m haveDirectoryMatcher) Match(actual interface{}) (bool, error) {
	return matchImage(m.path, actual, func(hdr *tar.Header, _ io.Reader) (bool, error) {
		return hdr.Typeflag == tar.TypeDir, nil
	})
}

func (m haveDirectoryMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#v\nto have directory with path\n\t%#v", actual, m.path)
}

func (m haveDirectoryMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#v\nnot to have directory with path\n\t%#v", actual, m.path)
}
