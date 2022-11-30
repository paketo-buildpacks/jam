package ihop

//go:generate faux --interface ImageScanner --output fakes/image_scanner.go
type ImageScanner interface {
	Scan(path string) (SBOM, error)
}

//go:generate faux --interface ImageBuildPromise --output fakes/image_build_promise.go
type ImageBuildPromise interface {
	Resolve() (Image, SBOM, error)
}

type builderInput struct {
	def      DefinitionImage
	platform string
	results  chan<- builderResult
}

type builderResult struct {
	image Image
	sbom  SBOM
	err   error
}

// A Builder is used to asynchronously build and scan a stack image.
type Builder struct {
	client  ImageClient
	scanner ImageScanner

	inputs chan builderInput
}

// NewBuilder returns a Builder that can build and scan stack images. The
// Builder will spin up a number of workers to process the build and scan of
// the stack image. These workers will use the promise returned by the Execute
// method to pass their results back to the caller.
func NewBuilder(client ImageClient, scanner ImageScanner, workerCount int) Builder {
	builder := Builder{
		client:  client,
		scanner: scanner,
		inputs:  make(chan builderInput),
	}

	for i := 0; i < workerCount; i++ {
		go worker(builder)
	}

	return builder
}

// Execute returns an ImageBuildPromise that can be waited upon to receive a
// built image and its SBOM scan results.
func (b Builder) Execute(def DefinitionImage, platform string) ImageBuildPromise {
	results := make(chan builderResult)
	b.inputs <- builderInput{def, platform, results}

	return BuilderPromise{results: results}
}

func (b Builder) build(def DefinitionImage, platform string) (Image, SBOM, error) {
	image, err := b.client.Build(def, platform)
	if err != nil {
		return Image{}, SBOM{}, err
	}

	sbom, err := b.scanner.Scan(image.Path)
	if err != nil {
		return Image{}, SBOM{}, err
	}

	return image, sbom, nil
}

// BuilderPromise implements the ImageBuildPromise interface.
type BuilderPromise struct {
	results chan builderResult
}

// Resolve blocks until the image is built and scanned before returning these
// results.
func (bp BuilderPromise) Resolve() (Image, SBOM, error) {
	result := <-bp.results

	return result.image, result.sbom, result.err
}

func worker(builder Builder) {
	for {
		input := <-builder.inputs

		image, sbom, err := builder.build(input.def, input.platform)
		input.results <- builderResult{image, sbom, err}
	}
}
