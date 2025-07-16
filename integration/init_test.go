package integration_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var path string

func TestJam(t *testing.T) {
	format.MaxLength = 0
	SetDefaultEventuallyTimeout(10 * time.Minute)

	suite := spec.New("jam", spec.Report(report.Terminal{}))
	suite("Errors", testErrors)
	suite("create-stack", testCreateStack)
	suite("publish-image", testPublishImage)
	suite("pack", testPack)
	suite("summarize", testSummarize)
	suite("update-builder", testUpdateBuilder)
	suite("update-buildpack", testUpdateBuildpack)
	suite("update-dependencies-from-metadata", testUpdateDependenciesFromMetadata)
	suite("version", testVersion)
	suite("pack extension", testPackExtension)

	var (
		Expect = NewWithT(t).Expect
		err    error
	)

	path, err = gexec.Build("github.com/paketo-buildpacks/jam/v2", "-ldflags", `-X github.com/paketo-buildpacks/jam/v2/commands.jamVersion=1.2.3`)
	Expect(err).NotTo(HaveOccurred())

	suite.Run(t)

	gexec.CleanupBuildArtifacts()
}

func by(_ string, f func()) { f() }

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

			return contents, hdr, nil
		}

	}

	return nil, nil, fmt.Errorf("no such file: %s", name)
}

type Buffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *Buffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}
func (b *Buffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}
