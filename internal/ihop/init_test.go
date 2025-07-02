package ihop_test

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

func TestUnitIHOP(t *testing.T) {
	format.MaxLength = 0
	var Expect = NewWithT(t).Expect

	client, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	Expect(err).NotTo(HaveOccurred())

	stream, err := client.ImagePull(context.Background(), "busybox:latest", image.PullOptions{})
	Expect(err).NotTo(HaveOccurred())

	_, err = io.Copy(io.Discard, stream)
	Expect(err).NotTo(HaveOccurred())
	Expect(stream.Close()).To(Succeed())

	stream, err = client.ImagePull(context.Background(), "ubuntu:jammy", image.PullOptions{})
	Expect(err).NotTo(HaveOccurred())

	_, err = io.Copy(io.Discard, stream)
	Expect(err).NotTo(HaveOccurred())
	Expect(stream.Close()).To(Succeed())

	suite := spec.New("ihop", spec.Report(report.Terminal{}))
	suite("Builder", testBuilder)
	suite("Cataloger", testCataloger)
	suite("Client", testClient)
	suite("Creator", testCreator)
	suite("Definition", testDefinition)
	suite("Packages", testPackages)
	suite("SBOM", testSBOM)
	suite("SBOMLayerCreator", testSBOMLayerCreator)
	suite("UserLayerCreator", testUserLayerCreator)
	suite("OsReleaseLayerCreator", testOsReleaseLayerCreator)
	suite.Run(t)

	_, err = client.ImageRemove(context.Background(), "busybox:latest", image.RemoveOptions{Force: true})
	Expect(err).NotTo(HaveOccurred())

	_, err = client.ImageRemove(context.Background(), "ubuntu:jammy", image.RemoveOptions{Force: true})
	Expect(err).NotTo(HaveOccurred())
}
