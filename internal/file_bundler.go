package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type File struct {
	io.ReadCloser

	Name string
	Info os.FileInfo
	Link string
}

type FileInfo struct {
	name  string
	size  int
	mode  os.FileMode
	mtime time.Time
}

func NewFileInfo(name string, size int, mode os.FileMode, mtime time.Time) FileInfo {
	return FileInfo{
		name:  name,
		size:  size,
		mode:  mode,
		mtime: mtime,
	}
}

func (fi FileInfo) Name() string {
	return fi.name
}

func (fi FileInfo) Size() int64 {
	return int64(fi.size)
}

func (fi FileInfo) Mode() os.FileMode {
	return fi.mode
}

func (fi FileInfo) ModTime() time.Time {
	return fi.mtime
}

func (fi FileInfo) IsDir() bool {
	return fi.Mode().IsDir()
}

func (fi FileInfo) Sys() interface{} {
	return nil
}

type FileBundler struct{}

func NewFileBundler() FileBundler {
	return FileBundler{}
}

func (b FileBundler) bundling(root string, path string) (File, error) {

	file := File{Name: path}

	var err error
	file.Info, err = os.Lstat(filepath.Join(root, path))
	if err != nil {
		return file, fmt.Errorf("error stating included file: %s", err)
	}

	if file.Info.Mode()&os.ModeType != 0 {
		link, err := os.Readlink(filepath.Join(root, path))
		if err != nil {
			return file, fmt.Errorf("error readlinking included file: %s", err)
		}

		if !strings.HasPrefix(link, string(filepath.Separator)) {
			link = filepath.Clean(filepath.Join(root, link))
		}

		file.Link, err = filepath.Rel(root, link)
		if err != nil {
			return file, fmt.Errorf("error finding relative link path: %s", err)
		}
	} else {
		file.ReadCloser, err = os.Open(filepath.Join(root, path))
		if err != nil {
			return file, fmt.Errorf("error opening included file: %s", err)
		}
	}

	return file, nil
}

func (b FileBundler) Bundle(root string, paths []string, config cargo.Config) ([]File, error) {
	var files []File

	for _, path := range paths {
		file := File{Name: path}

		switch path {
		case "buildpack.toml":
			buf := bytes.NewBuffer(nil)
			err := cargo.EncodeConfig(buf, config)
			if err != nil {
				return nil, fmt.Errorf("error encoding buildpack.toml: %s", err)
			}

			file.ReadCloser = io.NopCloser(buf)
			file.Info = NewFileInfo("buildpack.toml", buf.Len(), 0644, time.Now())

		default:
			var err error
			file, err = b.bundling(root, path)

			if err != nil {
				return nil, fmt.Errorf("%s", err)
			}
		}

		files = append(files, file)

	}

	return files, nil
}

func (b FileBundler) BundleExtension(root string, paths []string, config cargo.ExtensionConfig) ([]File, error) {
	var files []File

	for _, path := range paths {
		file := File{Name: path}

		switch path {
		case "extension.toml":
			buf := bytes.NewBuffer(nil)
			err := cargo.EncodeExtensionConfig(buf, config)
			if err != nil {
				return nil, fmt.Errorf("error encoding extension.toml: %s", err)
			}

			file.ReadCloser = io.NopCloser(buf)
			file.Info = NewFileInfo("extension.toml", buf.Len(), 0644, time.Now())

		default:
			var err error
			file, err = b.bundling(root, path)

			if err != nil {
				return nil, fmt.Errorf("%s", err)
			}
		}

		files = append(files, file)
	}

	return files, nil
}
