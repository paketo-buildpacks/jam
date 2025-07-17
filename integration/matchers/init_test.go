package matchers_test

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types/image"
	docker "github.com/docker/docker/client"
	"github.com/onsi/gomega/format"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

func TestMatchers(t *testing.T) {
	format.MaxLength = 0

	var Expect = NewWithT(t).Expect

	client, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	Expect(err).NotTo(HaveOccurred())

	stream, err := client.ImagePull(context.Background(), "alpine:3.19", image.PullOptions{})
	Expect(err).NotTo(HaveOccurred())

	_, err = io.Copy(io.Discard, stream)
	Expect(err).NotTo(HaveOccurred())
	Expect(stream.Close()).To(Succeed())

	suite := spec.New("matchers", spec.Report(report.Terminal{}))
	suite("HaveDirectory", testHaveDirectory)
	suite("HaveFile", testHaveFile)
	suite("HaveFileWithContent", testHaveFileWithContent)
	suite("MatchTomlContent", testMatchTomlContent)
	suite.Run(t)

	_, err = client.ImageRemove(context.Background(), "alpine:3.19", image.RemoveOptions{Force: true})
	Expect(err).NotTo(HaveOccurred())
}
