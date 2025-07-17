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
	img := image.Actual

	tarBuffer, err := os.CreateTemp("", "")
	if err != nil {
		return Layer{}, err
	}
	defer func() {
		if err2 := tarBuffer.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

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

	buffer, err := overwriteOsRelease(content, createOsReleaseOverwrites(o.Def))
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

	layer, err := tarToLayer(tarBuffer)
	return layer, err // err should be nil here, but return err to catch deferred error
}

func createOsReleaseOverwrites(def Definition) map[string]string {
	overwrites := map[string]string{}

	if def.Name != "" {
		overwrites["PRETTY_NAME"] = strconv.Quote(def.Name)
	}

	if def.Homepage != "" {
		overwrites["HOME_URL"] = strconv.Quote(def.Homepage)
	}

	if def.SupportURL != "" {
		overwrites["SUPPORT_URL"] = strconv.Quote(def.SupportURL)
	}

	if def.BugReportURL != "" {
		overwrites["BUG_REPORT_URL"] = strconv.Quote(def.BugReportURL)
	}

	return overwrites
}

func overwriteOsRelease(content io.Reader, overwrites map[string]string) (*bytes.Buffer, error) {
	scanner := bufio.NewScanner(content)
	releaseContent := map[string]string{}

	for scanner.Scan() {
		before, after, found := strings.Cut(scanner.Text(), "=")
		if found {
			releaseContent[before] = after
		}
	}
	err := scanner.Err()
	if err != nil {
		return nil, err
	}

	for key, value := range overwrites {
		releaseContent[key] = value
	}

	buffer := bytes.NewBuffer(nil)
	for key, value := range releaseContent {
		buffer.WriteString(key)
		buffer.WriteString("=")
		buffer.WriteString(value)
		buffer.WriteString("\n")
	}

	return buffer, nil
}
