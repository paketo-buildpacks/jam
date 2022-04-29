package matchers

import (
	"archive/tar"
	"fmt"
	"io"

	"github.com/onsi/gomega/types"
)

func HaveFile(path interface{}) types.GomegaMatcher {
	return &haveFileMatcher{
		path: path,
	}
}

type haveFileMatcher struct {
	path interface{}
}

func (m haveFileMatcher) Match(actual interface{}) (bool, error) {
	return matchImage(m.path, actual, func(hdr *tar.Header, _ io.Reader) (bool, error) {
		return hdr.Typeflag == tar.TypeReg, nil
	})
}

func (m haveFileMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#v\nto have file with path\n\t%#v", actual, m.path)
}

func (m haveFileMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#v\nnot to have file with path\n\t%#v", actual, m.path)
}
