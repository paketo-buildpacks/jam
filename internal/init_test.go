package internal_test

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitInternal(t *testing.T) {
	format.MaxLength = 0
	gomega.SetDefaultEventuallyTimeout(10 * time.Second)

	suite := spec.New("jam/internal", spec.Report(report.Terminal{}))
	suite("BuilderConfig", testBuilderConfig)
	suite("BuildpackConfig", testBuildpackConfig)
	suite("BuildpackInspector", testBuildpackInspector)
	suite("ExtensionInspector", testExtensionInspector)
	suite("DependencyCacher", testDependencyCacher)
	suite("Dependency", testDependency)
	suite("FileBundler", testFileBundler)
	suite("Formatter", testFormatter)
	suite("ExtensionFormatter", testExtensionFormatter)
	suite("Image", testImage)
	suite("PrePackager", testPrePackager)
	suite("PackageConfig", testPackageConfig)
	suite("TarBuilder", testTarBuilder)
	suite.Run(t)
}

func ExtractFile(file *os.File, name string) ([]byte, *tar.Header, error) {
	_, err := file.Seek(0, 0)
	if err != nil {
		return nil, nil, err
	}

	//TODO: Replace me with decompression library
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err2 := gzr.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, nil, err
		}

		if hdr.Name == name {
			contents, err := io.ReadAll(tr)
			if err != nil {
				return nil, nil, err
			}

			return contents, hdr, err // err should be nil here, but return err to catch deferred error
		}
	}

	return nil, nil, fmt.Errorf("no such file: %s", name)
}

type errorReader struct{}

func (r errorReader) Read(p []byte) (int, error) {
	return 0, errors.New("failed to read")
}
