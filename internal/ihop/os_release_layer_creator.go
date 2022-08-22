package ihop

import (
	"archive/tar"
	"bufio"
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
)

// A OsReleaseLayerCreator can be used to construct a layer that includes /etc/os-release.
type OsReleaseLayerCreator struct {
	Def Definition
}

func (o OsReleaseLayerCreator) Create(image Image, _ DefinitionImage, _ SBOM) (Layer, error) {
	img, err := image.ToDaemonImage()
	if err != nil {
		return Layer{}, err
	}

	tarBuffer, err := os.CreateTemp("", "")
	if err != nil {
		return Layer{}, err
	}
	defer tarBuffer.Close()
	tw := tar.NewWriter(tarBuffer)

	// find any existing /etc/ folder and copy the header
	hdr, _, err := findFile(img, "etc/")
	if err != nil {
		return Layer{}, err
	}
	err = tw.WriteHeader(hdr)
	if err != nil {
		return Layer{}, err
	}

	// find any existing /etc/os-release file and copy the header
	hdr, content, err := findFile(img, "etc/os-release")
	if err != nil {
		return Layer{}, err
	}

	buffer, err := updateOsRelease(content, o.Def)
	if err != nil {
		return Layer{}, err
	}

	err = tw.WriteHeader(&tar.Header{
		Name: "etc/os-release",
		Mode: hdr.Mode,
		Size: int64(buffer.Len()),
	})

	if err != nil {
		return Layer{}, err
	}

	_, err = io.Copy(tw, buffer)
	if err != nil {
		return Layer{}, err
	}

	err = tw.Close()
	if err != nil {
		return Layer{}, err
	}

	return tarToLayer(tarBuffer)
}

func updateOsRelease(content io.Reader, def Definition) (*bytes.Buffer, error) {
	scanner := bufio.NewScanner(content)
	updatedContent := map[string]string{}

	for scanner.Scan() {
		before, after, found := strings.Cut(scanner.Text(), "=")
		if found {
			updatedContent[before] = after
		}
	}
	err := scanner.Err()
	if err != nil {
		return nil, err
	}

	updatedContent["PRETTY_NAME"] = strconv.Quote(def.Name)
	updatedContent["HOME_URL"] = strconv.Quote(def.Homepage)
	updatedContent["SUPPORT_URL"] = strconv.Quote(def.SupportURL)
	updatedContent["BUG_REPORT_URL"] = strconv.Quote(def.BugReportURL)

	buffer := bytes.NewBuffer(nil)
	for key, value := range updatedContent {
		buffer.WriteString(key)
		buffer.WriteString("=")
		buffer.WriteString(value)
		buffer.WriteString("\n")
	}

	return buffer, nil
}
