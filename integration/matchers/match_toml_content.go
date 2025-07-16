package matchers

import (
	"fmt"
	"os"

	"github.com/onsi/gomega/types"
	"github.com/paketo-buildpacks/packit/v2/matchers"
)

func MatchTomlContent(expectedFilePath string) types.GomegaMatcher {
	return &matchTomlContentMatcher{
		expectedFilePath: expectedFilePath,
	}
}

type matchTomlContentMatcher struct {
	expectedFilePath string
}

func (m matchTomlContentMatcher) Match(actual interface{}) (bool, error) {
	actualFilePath, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("MatchTomlContent matcher expects a file path")
	}
	actualContents, err := os.ReadFile(actualFilePath)
	if err != nil {
		return false, err
	}

	expectedContents, err := os.ReadFile(m.expectedFilePath)
	if err != nil {
		return false, err
	}

	matchTomlMatcher := matchers.MatchTOML(expectedContents)
	return matchTomlMatcher.Match(actualContents)
}

func (m matchTomlContentMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n%s contents\nto match the contents of \n%s", actual, m.expectedFilePath)
}

func (m matchTomlContentMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n%s contents\n not to match the contents of \n%s", actual, m.expectedFilePath)
}
