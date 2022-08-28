package ihop

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface ImageClient --output fakes/image_client.go
type ImageClient interface {
	Build(definitionImage DefinitionImage, platform string) (Image, error)
	Update(Image) (Image, error)
}

//go:generate faux --interface ImageBuilder --output fakes/image_builder.go
type ImageBuilder interface {
	Execute(definitionImage DefinitionImage, platform string) ImageBuildPromise
}

//go:generate faux --interface LayerCreator --output fakes/layer_creator.go
type LayerCreator interface {
	Create(Image, DefinitionImage, SBOM) (Layer, error)
}

// A Stack holds all of the Build and Run images for a built stack.
type Stack struct {
	Build []Image
	Run   []Image
}

// A Creator can be used to generate a Stack.
type Creator struct {
	docker                ImageClient
	builder               ImageBuilder
	userLayerCreator      LayerCreator
	sbomLayerCreator      LayerCreator
	osReleaseLayerCreator LayerCreator
	now                   func() time.Time
	logger                scribe.Logger
}

// NewCreator returns a Creator configured with the given arguments.
func NewCreator(docker ImageClient, builder ImageBuilder, userLayerCreator, sbomLayerCreator LayerCreator, osReleaseLayerCreator LayerCreator, now func() time.Time, logger scribe.Logger) Creator {
	return Creator{
		docker:                docker,
		builder:               builder,
		userLayerCreator:      userLayerCreator,
		sbomLayerCreator:      sbomLayerCreator,
		osReleaseLayerCreator: osReleaseLayerCreator,
		now:                   now,
		logger:                logger,
	}
}

// Execute builds a Stack using the given Definition.
func (c Creator) Execute(def Definition) (Stack, error) {
	c.logger.Title("Building %s", def.ID)

	var stack Stack
	for _, platform := range def.Platforms {
		c.logger.Process("Building on %s", platform)

		build, run, err := c.create(def, platform)
		if err != nil {
			return Stack{}, err
		}

		stack.Build = append(stack.Build, build)
		stack.Run = append(stack.Run, run)
	}

	return stack, nil
}

func (c Creator) create(def Definition, platform string) (Image, Image, error) {
	c.logger.Subprocess("Building base images")

	// invoke the builder to start the build process for the build and run images
	buildPromise := c.builder.Execute(def.Build, platform)
	runPromise := c.builder.Execute(def.Run, platform)

	// wait for the build image to complete building
	build, buildSBOM, err := buildPromise.Resolve()
	if err != nil {
		return Image{}, Image{}, err
	}

	// wait for the run image to complete building
	run, runSBOM, err := runPromise.Resolve()
	if err != nil {
		return Image{}, Image{}, err
	}

	c.logger.Action("Build complete for base images")

	// determine which packages appear in each image
	packages := NewPackages(buildSBOM.Packages(), runSBOM.Packages())
	timestamp := c.now()

	c.logger.Subprocess("build: Decorating base image")

	c.logger.Action("Adding CNB_* environment variables")
	build.Env = append(build.Env, fmt.Sprintf("CNB_USER_ID=%d", def.Build.UID))
	build.Env = append(build.Env, fmt.Sprintf("CNB_GROUP_ID=%d", def.Build.GID))
	build.Env = append(build.Env, fmt.Sprintf("CNB_STACK_ID=%s", def.ID))

	// update the base build image with common configuration metadata
	build, err = c.mutate(build, def, def.Build, buildSBOM, packages.Intersection, packages.BuildComplement, timestamp)
	if err != nil {
		return Image{}, Image{}, err
	}

	c.logger.Subprocess("run: Decorating base image")
	// update the base run image with common configuration metadata
	run, err = c.mutate(run, def, def.Run, runSBOM, packages.Intersection, packages.RunComplement, timestamp)
	if err != nil {
		return Image{}, Image{}, err
	}

	if def.containsOsReleaseOverwrites() {
		// update /etc/os-release" in the run images in the Docker daemon
		c.logger.Action("Updating /etc/os-release")
		layer, err := c.osReleaseLayerCreator.Create(run, def.Run, runSBOM)
		if err != nil {
			return Image{}, Image{}, err
		}
		run.Layers = append(run.Layers, layer)
	}

	// if the EXPERIMENTAL_ATTACH_RUN_IMAGE_SBOM environment variable is set,
	// attach an SBOM layer to the run image
	if def.IncludeExperimentalSBOM {
		c.logger.Action("Attaching experimental SBOM")

		layer, err := c.sbomLayerCreator.Create(run, def.Run, runSBOM)
		if err != nil {
			return Image{}, Image{}, err
		}

		run.Labels["io.buildpacks.base.sbom"] = layer.DiffID
		run.Layers = append(run.Layers, layer)
	}

	// update the build and run images in the Docker daemon
	c.logger.Subprocess("build: Updating image")
	build, err = c.docker.Update(build)
	if err != nil {
		return Image{}, Image{}, err
	}

	c.logger.Subprocess("run: Updating image")
	run, err = c.docker.Update(run)
	if err != nil {
		return Image{}, Image{}, err
	}

	c.logger.Break()

	return build, run, nil
}

func (c Creator) mutate(image Image, def Definition, imageDef DefinitionImage, sbom SBOM, intersection, complement []string, now time.Time) (Image, error) {
	// add the common CNB labels to the given image
	c.logger.Action("Adding io.buildpacks.stack.* labels")
	image.Labels["io.buildpacks.stack.id"] = def.ID
	image.Labels["io.buildpacks.stack.description"] = imageDef.Description
	image.Labels["io.buildpacks.stack.distro.name"] = sbom.Distro.Name
	image.Labels["io.buildpacks.stack.distro.version"] = sbom.Distro.Version
	image.Labels["io.buildpacks.stack.homepage"] = def.Homepage
	image.Labels["io.buildpacks.stack.maintainer"] = def.Maintainer
	image.Labels["io.buildpacks.stack.metadata"] = "{}"
	image.Labels["io.buildpacks.stack.released"] = now.Format(time.RFC3339)

	// if the stack descriptor requests to use the depredated mixins feature,
	// include the mixins label as a JSON-encoded list of package names
	if def.Deprecated.Mixins {
		c.logger.Action("Adding io.buildpacks.stack.mixins label")

		var mixins []string
		mixins = append(mixins, intersection...)
		mixins = append(mixins, complement...)

		output, err := json.Marshal(mixins)
		if err != nil {
			return Image{}, err
		}

		image.Labels["io.buildpacks.stack.mixins"] = string(output)
	}

	// if the stack descriptor requests to use the deprecated legacy SBOM
	// feature, include the packages label as a JSON-encoded object
	if def.Deprecated.LegacySBOM {
		c.logger.Action("Adding io.paketo.stack.packages label")

		var err error
		image.Labels["io.paketo.stack.packages"], err = sbom.LegacyFormat()
		if err != nil {
			return Image{}, err
		}
	}

	// create and attach a layer that creates a cnb user in the container image
	// filesystem
	c.logger.Action("Creating cnb user")
	layer, err := c.userLayerCreator.Create(image, imageDef, sbom)
	if err != nil {
		return Image{}, err
	}

	image.Layers = append(image.Layers, layer)

	image.User = fmt.Sprintf("%d:%d", imageDef.UID, imageDef.GID)

	return image, nil
}
