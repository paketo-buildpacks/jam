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
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/command/image/build"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/opts"
	buildtypes "github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/image"
	docker "github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	control "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/moby/go-archive"
	"github.com/opencontainers/go-digest"
)

// An Image is a representation of a container image that can be built,
// updated, or exported to an OCI-archive format.
type Image struct {
	Digest       string
	OS           string
	Architecture string
	User         string

	Env    []string
	Labels map[string]string

	Layers []Layer

	Actual v1.Image
	Path   string
}

func FromImage(path string, image v1.Image) (Image, error) {
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
		User:         file.Config.User,
		OS:           file.OS,
		Architecture: file.Architecture,
		Actual:       image,
		Path:         path,
	}, nil
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
	client, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
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
	sess, err := session.NewSession(context.Background(), hex.EncodeToString(sum[:]))
	if err != nil {
		return Image{}, err
	}

	// associate an authentication provider with the session
	// dockerAuthProvider := authprovider.NewDockerAuthProvider(config.LoadDefaultConfigFile(os.Stderr))
	dockerAuthProvider := authprovider.NewDockerAuthProvider(authprovider.DockerAuthProviderConfig{ConfigFile: config.LoadDefaultConfigFile(os.Stderr)})
	sess.Allow(dockerAuthProvider)

	// if the DefinitionImage contains secrets, add them to the session using the
	// secrets API
	if len(def.Secrets) > 0 {
		secretsDir, err := os.MkdirTemp("", "docker-secrets")
		if err != nil {
			return Image{}, err
		}
		defer func() {
			if err := os.RemoveAll(secretsDir); err != nil {
				log.Fatalln(err)
			}
		}()

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
	defer func() {
		if err := sess.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

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
		ChownOpts:       &archive.ChownOpts{UID: 0, GID: 0},
	})
	if err != nil {
		return Image{}, err
	}
	defer func() {
		if err := buildContext.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	// generate a random name for the image that is being built
	tag, err := randomName()
	if err != nil {
		return Image{}, err
	}
	tag = fmt.Sprintf("paketo.io/stack/%s", tag)

	buildArgs, err := def.Arguments(platform)
	if err != nil {
		return Image{}, err
	}
	// send a request to the Docker daemon to build the image
	resp, err := c.docker.ImageBuild(context.Background(), buildContext, buildtypes.ImageBuildOptions{
		BuildArgs:  opts.ConvertKVStringsToMapWithNil(buildArgs),
		Dockerfile: relDockerfile,
		NoCache:    true,
		Remove:     true,
		Tags:       []string{tag},
		Version:    buildtypes.BuilderBuildKit,
		SessionID:  sess.ID(),
		Platform:   platform,
	})
	if err != nil {
		return Image{}, fmt.Errorf("failed to initiate image build: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	// parse the streaming response body which is a JSON-encoded stream of
	// objects with output from the commands run in the Dockerfile
	buffer := bytes.NewBuffer(nil)
	displayChan := make(chan *client.SolveStatus)
	go func() {
		// _, _ = progressui.DisplaySolveStatus(context.Background(), nil, buffer, displayChan)
		d, _ := progressui.NewDisplay(buffer, progressui.PlainMode)
		_, err = d.UpdateFrom(context.Background(), displayChan)
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
			if err := (&resp).UnmarshalVT(dt); err != nil {
				return Image{}, err
			}

			solveStatus := client.SolveStatus{}
			for _, v := range resp.Vertexes {
				inputs := make([]digest.Digest, 0, len(v.Inputs))
				for _, input := range v.Inputs {
					inputs = append(inputs, digest.Digest(input))
				}
				started := v.Started.AsTime()
				completed := v.Completed.AsTime()
				solveStatus.Vertexes = append(solveStatus.Vertexes, &client.Vertex{
					Digest:    digest.Digest(v.Digest),
					Inputs:    inputs,
					Name:      v.Name,
					Started:   &started,
					Completed: &completed,
					Error:     v.Error,
					Cached:    v.Cached,
				})
			}
			for _, v := range resp.Statuses {
				started := v.Started.AsTime()
				completed := v.Completed.AsTime()
				solveStatus.Statuses = append(solveStatus.Statuses, &client.VertexStatus{
					ID:        v.ID,
					Vertex:    digest.Digest(v.Vertex),
					Name:      v.Name,
					Total:     v.Total,
					Current:   v.Current,
					Timestamp: v.Timestamp.AsTime(),
					Started:   &started,
					Completed: &completed,
				})
			}
			for _, v := range resp.Logs {
				solveStatus.Logs = append(solveStatus.Logs, &client.VertexLog{
					Vertex:    digest.Digest(v.Vertex),
					Stream:    int(v.Stream),
					Data:      v.Msg,
					Timestamp: v.Timestamp.AsTime(),
				})
			}

			displayChan <- &solveStatus

		// if the stream includes an error message, return that message as an error
		// to the caller
		case message.Error != "":
			return Image{}, fmt.Errorf("build failed:\n%s\n%s", buffer.String(), message.Error)
		}
	}

	defer func() {
		_, err := c.docker.ImageRemove(context.Background(), tag, image.RemoveOptions{})
		if err != nil {
			log.Fatalln(err)
		}
	}()

	ref, err := name.ParseReference(tag)
	if err != nil {
		return Image{}, err
	}

	image, err := daemon.Image(ref)
	if err != nil {
		return Image{}, err
	}

	name, err := randomName()
	if err != nil {
		return Image{}, err
	}

	path := filepath.Join(c.dir, name)
	index, err := layout.Write(path, empty.Index)
	if err != nil {
		return Image{}, fmt.Errorf("failed to write image layout: %w", err)
	}

	file, err := image.ConfigFile()
	if err != nil {
		return Image{}, err
	}

	err = index.AppendImage(image, layout.WithPlatform(v1.Platform{
		OS:           file.OS,
		Architecture: file.Architecture,
	}))
	if err != nil {
		return Image{}, err
	}

	digest, err := image.Digest()
	if err != nil {
		return Image{}, err
	}

	image, err = index.Image(digest)
	if err != nil {
		return Image{}, err
	}

	// fetch and return a reference to the built image
	return FromImage(string(path), image)
}

// Update will apply any modifications made to the Image reference onto the
// actual container image in the Docker daemon.
func (c Client) Update(image Image) (Image, error) {
	img := image.Actual

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
			if strings.Contains(err.Error(), "unknown diffID") {
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

	path, err := layout.FromPath(image.Path)
	if err != nil {
		return Image{}, fmt.Errorf("could not load layout from path %q: %w", image.Path, err)
	}

	err = path.AppendImage(updatedImage, layout.WithPlatform(v1.Platform{
		OS:           image.OS,
		Architecture: image.Architecture,
	}))
	if err != nil {
		return Image{}, fmt.Errorf("could not append image to layout: %w", err)
	}

	digest, err := updatedImage.Digest()
	if err != nil {
		return Image{}, err
	}

	updatedImage, err = path.Image(digest)
	if err != nil {
		return Image{}, err
	}

	return FromImage(image.Path, updatedImage)
}

// Export creates an OCI-archive tarball at the path location that includes the
// given Images.
func (c Client) Export(path string, images ...Image) error {
	directory, err := c.imageToDirectory(images)
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	tw := tar.NewWriter(file)
	defer func() {
		if err2 := tw.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

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

func (c Client) UploadImages(refName string, images ...Image) error {
	directory, err := c.imageToDirectory(images)
	if err != nil {
		return err
	}

	return c.Upload(refName, directory)
}

func (c Client) Upload(refName string, fromDir string) error {
	path, err := layout.FromPath(fromDir)
	if err != nil {
		return err
	}

	imageIndex, err := path.ImageIndex()
	if err != nil {
		return err
	}

	ref, err := name.ParseReference(refName)
	if err != nil {
		return err
	}

	return remote.WriteIndex(ref, imageIndex, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

func (c Client) imageToDirectory(images []Image) (string, error) {
	directory, err := randomName()
	if err != nil {
		return "", err
	}

	directory = filepath.Join(c.dir, directory)
	err = os.Mkdir(directory, os.ModePerm)
	if err != nil {
		return "", err
	}

	index, err := layout.Write(directory, empty.Index)
	if err != nil {
		return "", err
	}

	for _, image := range images {
		err = index.AppendImage(image.Actual, layout.WithPlatform(v1.Platform{
			OS:           image.OS,
			Architecture: image.Architecture,
		}))
		if err != nil {
			return "", err
		}
	}

	return directory, nil
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
