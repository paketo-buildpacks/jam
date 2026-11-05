package commands

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnitUpdateDependenciesReportsVersionNewForADependency(t *testing.T) {
	dir := t.TempDir()

	buildpackFile := filepath.Join(dir, "buildpack.toml")
	metadataFile := filepath.Join(dir, "metadata.json")

	if err := os.WriteFile(buildpackFile, []byte(`api = "0.2"

[buildpack]
  id = "some-buildpack"
  name = "Some Buildpack"
  version = "some-version"

[metadata]

  [[metadata.dependencies]]
    checksum = "sha256:jdk-sha"
    cpe = "some-cpe"
    id = "jdk-test"
    name = "Test JDK"
    purl = "some-purl"
    uri = "some-jdk-uri"
    version = "21.0.13+0.3"

  [[metadata.dependencies]]
    checksum = "sha256:jre-sha"
    cpe = "some-cpe"
    id = "jre-test"
    name = "Test JRE"
    purl = "some-purl"
    uri = "some-jre-uri"
    version = "21.0.13"

  [[metadata.dependency-constraints]]
    constraint = "^21"
    id = "jdk-test"
    patches = 1

  [[metadata.dependency-constraints]]
    constraint = "^21"
    id = "jre-test"
    patches = 1

[[stacks]]
  id = "*"
`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(metadataFile, []byte(`[
  {
    "checksum": "sha256:new-jre-sha",
    "cpe": "some-cpe",
    "id": "jre-test",
    "name": "Test JRE",
    "purl": "some-purl",
    "stacks": ["*"],
    "target": "linux-amd64",
    "uri": "new-jre-uri",
    "version": "21.0.13+0.3"
  }
]
`), 0600); err != nil {
		t.Fatal(err)
	}

	output := captureOutput(t, func() error {
		return updateDependenciesRun(updateDependenciesFlags{
			buildpackFile: buildpackFile,
			metadataFile:  metadataFile,
		})
	})

	// 21.0.13+0.3 already exists for jdk-test, but it is new for jre-test and
	// must still be reported, otherwise the update PR has no title.
	if !strings.Contains(output, "21.0.13+0.3") {
		t.Errorf("expected the new version to be reported, got %q", output)
	}

	updated, err := os.ReadFile(buildpackFile)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(updated), `uri = "new-jre-uri"`) {
		t.Errorf("expected jre-test to be updated, got:\n%s", updated)
	}
}

func captureOutput(t *testing.T, run func() error) string {
	t.Helper()

	original := os.Stdout

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	runErr := run()

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if runErr != nil {
		t.Fatalf("update dependencies failed: %v", runErr)
	}

	return string(output)
}
