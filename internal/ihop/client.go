package ihop

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/command/image/build"
	"github.com/docker/cli/opts"
	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	control "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/util/progress/progressui"
)

// An Image is a representation of a container image that can be built,
// updated, or exported to an OCI-archive format.
type Image struct {
	Tag          string
	Digest       string
	OS           string
	Architecture string
	User         string

	Env    []string
	Labels map[string]string

	Layers []Layer

	unbuffered bool
}

// ToDaemonImage returns the GGCR v1.Image associated with this Image.
func (i Image) ToDaemonImage() (v1.Image, error) {
	ref, err := name.ParseReference(i.Tag)
	if err != nil {
		return nil, err
	}

	option := daemon.WithBufferedOpener()
	if i.unbuffered {
		option = daemon.WithUnbufferedOpener()
	}

	image, err := daemon.Image(ref, option)
	if err != nil {
		return nil, err
	}

	return image, nil
}

// A Layer is a representation of a container image layer.
type Layer struct {
	DiffID string

	v1.Layer
}

// A Client can be used to build, update, and export container images.
type Client struct {
	dir      string
	docker   *docker.Client
	keychain authn.Keychain
}

// NewClient returns a Client that has been configured to interact with the
// local Docker daemon using the environment configuration options.
func NewClient(dir string) (Client, error) {
	client, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		return Client{}, err
	}

	return Client{
		dir:      dir,
		docker:   client,
		keychain: authn.DefaultKeychain,
	}, nil
}

// Build uses BuildKit to build a container image from its reference Dockerfile
// as specified in the given DefinitionImage. Specifying a platform other than
// the native platform for the Docker daemon will require that the daemon is
// configured following the guidelines in
// https://docs.docker.com/buildx/working-with-buildx/#build-multi-platform-images.
func (c Client) Build(def DefinitionImage, platform string) (Image, error) {
	// create a session to interact with the Docker daemon
	sum := sha256.Sum256([]byte(def.Dockerfile))
	sess, err := session.NewSession(context.Background(), def.Dockerfile, hex.EncodeToString(sum[:]))
	if err != nil {
		return Image{}, err
	}

	// associate an authentication provider with the session
	dockerAuthProvider := authprovider.NewDockerAuthProvider(os.Stderr)
	sess.Allow(dockerAuthProvider)

	// if the DefinitionImage contains secrets, add them to the session using the
	// secrets API
	if len(def.Secrets) > 0 {
		secretsDir, err := os.MkdirTemp("", "docker-secrets")
		if err != nil {
			return Image{}, err
		}
		defer os.RemoveAll(secretsDir)

		fs := make([]secretsprovider.Source, 0, len(def.Secrets))
		for id, secret := range def.Secrets {
			path := filepath.Join(secretsDir, id)
			err = os.WriteFile(path, []byte(secret), 0600)
			if err != nil {
				return Image{}, err
			}

			fs = append(fs, secretsprovider.Source{
				ID:       id,
				FilePath: path,
			})
		}

		store, err := secretsprovider.NewStore(fs)
		if err != nil {
			return Image{}, err
		}

		sess.Allow(secretsprovider.NewSecretProvider(store))
	}

	// establish the session with the Docker daemon in the background
	go func() {
		_ = sess.Run(context.Background(), func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
			return c.docker.DialHijack(ctx, "/session", proto, meta)
		})
	}()
	defer sess.Close()

	// create the build context which includes the Dockerfile and any of the
	// files in its same directory that could be referenced in that file
	contextDir := filepath.Dir(def.Dockerfile)
	excludes, err := build.ReadDockerignore(contextDir)
	if err != nil {
		return Image{}, err
	}

	contextDir, relDockerfile, err := build.GetContextFromLocalDir(contextDir, def.Dockerfile)
	if err != nil {
		return Image{}, err
	}

	buildContext, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		ExcludePatterns: excludes,
		ChownOpts:       &idtools.Identity{UID: 0, GID: 0},
	})
	if err != nil {
		return Image{}, err
	}
	defer buildContext.Close()

	// generate a random name for the image that is being built
	tag, err := randomName()
	if err != nil {
		return Image{}, err
	}
	tag = fmt.Sprintf("paketo.io/stack/%s", tag)

	// send a request to the Docker daemon to build the image
	resp, err := c.docker.ImageBuild(context.Background(), buildContext, types.ImageBuildOptions{
		BuildArgs:  opts.ConvertKVStringsToMapWithNil(def.Arguments()),
		Dockerfile: relDockerfile,
		NoCache:    true,
		Remove:     true,
		Tags:       []string{tag},
		Version:    types.BuilderBuildKit,
		SessionID:  sess.ID(),
		Platform:   platform,
	})
	if err != nil {
		return Image{}, fmt.Errorf("failed to initiate image build: %w", err)
	}
	defer resp.Body.Close()

	// parse the streaming response body which is a JSON-encoded stream of
	// objects with output from the commands run in the Dockerfile
	buffer := bytes.NewBuffer(nil)
	displayChan := make(chan *client.SolveStatus)
	go func() {
		_ = progressui.DisplaySolveStatus(context.Background(), "", nil, buffer, displayChan)
	}()

	stream := json.NewDecoder(resp.Body)
	for {
		var message struct {
			ID    string          `json:"id"`
			Aux   json.RawMessage `json:"aux"`
			Error string          `json:"error"`
		}

		if err := stream.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}

			return Image{}, err
		}

		switch {
		case message.ID == "moby.buildkit.trace":
			var dt []byte
			if err := json.Unmarshal(message.Aux, &dt); err != nil {
				return Image{}, err
			}

			var resp control.StatusResponse
			if err := (&resp).Unmarshal(dt); err != nil {
				return Image{}, err
			}

			solveStatus := client.SolveStatus{}
			for _, v := range resp.Vertexes {
				solveStatus.Vertexes = append(solveStatus.Vertexes, &client.Vertex{
					Digest:    v.Digest,
					Inputs:    v.Inputs,
					Name:      v.Name,
					Started:   v.Started,
					Completed: v.Completed,
					Error:     v.Error,
					Cached:    v.Cached,
				})
			}
			for _, v := range resp.Statuses {
				solveStatus.Statuses = append(solveStatus.Statuses, &client.VertexStatus{
					ID:        v.ID,
					Vertex:    v.Vertex,
					Name:      v.Name,
					Total:     v.Total,
					Current:   v.Current,
					Timestamp: v.Timestamp,
					Started:   v.Started,
					Completed: v.Completed,
				})
			}
			for _, v := range resp.Logs {
				solveStatus.Logs = append(solveStatus.Logs, &client.VertexLog{
					Vertex:    v.Vertex,
					Stream:    int(v.Stream),
					Data:      v.Msg,
					Timestamp: v.Timestamp,
				})
			}

			displayChan <- &solveStatus

		// if the stream includes an error message, return that message as an error
		// to the caller
		case message.Error != "":
			return Image{}, fmt.Errorf("build failed:\n%s\n%s", buffer.String(), message.Error)
		}
	}

	// fetch and return a reference to the built image
	return c.get(Image{Tag: tag, unbuffered: def.unbuffered})
}

// Update will apply any modifications made to the Image reference onto the
// actual container image in the Docker daemon.
func (c Client) Update(image Image) (Image, error) {
	// Add a random tag to the original image to distinguish it from other
	// identical images
	random, err := randomName()
	if err != nil {
		return Image{}, err
	}
	originalImage := fmt.Sprintf("%s:%s", image.Tag, random)
	err = c.docker.ImageTag(context.Background(), image.Tag, originalImage)
	if err != nil {
		return Image{}, err
	}

	img, err := image.ToDaemonImage()
	if err != nil {
		return Image{}, err
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return Image{}, err
	}

	configFile.Config.Labels = image.Labels
	configFile.Config.User = image.User
	configFile.Config.Env = image.Env

	// find layers in the Image reference that are not yet applied to the
	// container image in the Docker daemon
	var layers []v1.Layer
	for _, layer := range image.Layers {
		hash, err := v1.NewHash(layer.DiffID)
		if err != nil {
			return Image{}, err
		}

		_, err = img.LayerByDiffID(hash)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				layers = append(layers, layer.Layer)
				continue
			}

			return Image{}, err
		}
	}

	updatedImage, err := mutate.ConfigFile(img, configFile)
	if err != nil {
		return Image{}, err
	}

	updatedImage, err = mutate.AppendLayers(updatedImage, layers...)
	if err != nil {
		return Image{}, err
	}

	tag, err := name.NewTag(image.Tag)
	if err != nil {
		return Image{}, err
	}

	_, err = daemon.Write(tag, updatedImage)
	if err != nil {
		return Image{}, err
	}

	err = c.Cleanup(Image{Tag: originalImage})
	if err != nil {
		return Image{}, err
	}

	return c.get(image)
}

// Export creates an OCI-archive tarball at the path location that includes the
// given Images.
func (c Client) Export(path string, images ...Image) error {
	directory, err := randomName()
	if err != nil {
		return err
	}

	directory = filepath.Join(c.dir, directory)
	err = os.Mkdir(directory, os.ModePerm)
	if err != nil {
		return err
	}

	index, err := layout.Write(directory, empty.Index)
	if err != nil {
		return err
	}

	for _, image := range images {
		img, err := image.ToDaemonImage()
		if err != nil {
			return err
		}

		err = index.AppendImage(img, layout.WithPlatform(v1.Platform{
			OS:           image.OS,
			Architecture: image.Architecture,
		}))
		if err != nil {
			return err
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	err = filepath.Walk(directory, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		hdr.Name, err = filepath.Rel(directory, path)
		if err != nil {
			return err
		}

		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fd, err := os.Open(path)
			if err != nil {
				return err
			}

			_, err = io.Copy(tw, fd)
			if err != nil {
				return err
			}

			err = fd.Close()
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// Cleanup deletes the container images from the Docker daemon that are
// referenced by the given Images.
func (c Client) Cleanup(images ...Image) error {
	for _, image := range images {
		_, err := c.docker.ImageRemove(context.Background(), image.Tag, types.ImageRemoveOptions{})
		if err != nil && !docker.IsErrNotFound(err) {
			return err
		}
	}

	return nil
}

func (c Client) get(img Image) (Image, error) {
	image, err := img.ToDaemonImage()
	if err != nil {
		return Image{}, err
	}

	file, err := image.ConfigFile()
	if err != nil {
		return Image{}, err
	}

	labels := file.Config.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	ls, err := image.Layers()
	if err != nil {
		return Image{}, err
	}

	var layers []Layer
	for _, layer := range ls {
		diffID, err := layer.DiffID()
		if err != nil {
			return Image{}, err
		}

		layers = append(layers, Layer{
			DiffID: diffID.String(),
			Layer:  layer,
		})
	}

	digest, err := image.Digest()
	if err != nil {
		return Image{}, err
	}

	return Image{
		Digest:       digest.String(),
		Env:          file.Config.Env,
		Labels:       labels,
		Layers:       layers,
		Tag:          img.Tag,
		User:         file.Config.User,
		OS:           file.OS,
		Architecture: file.Architecture,
		unbuffered:   img.unbuffered,
	}, nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomName() (string, error) {
	b := make([]byte, 10)
	for i := range b {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		if err != nil {
			return "", err
		}

		b[i] = letterBytes[index.Int64()]
	}

	return string(b), nil
}
