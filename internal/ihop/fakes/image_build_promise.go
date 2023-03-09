package fakes

import (
	"sync"

	"github.com/paketo-buildpacks/jam/v2/internal/ihop"
)

type ImageBuildPromise struct {
	ResolveCall struct {
		mutex     sync.Mutex
		CallCount int
		Returns   struct {
			Image ihop.Image
			SBOM  ihop.SBOM
			Error error
		}
		Stub func() (ihop.Image, ihop.SBOM, error)
	}
}

func (f *ImageBuildPromise) Resolve() (ihop.Image, ihop.SBOM, error) {
	f.ResolveCall.mutex.Lock()
	defer f.ResolveCall.mutex.Unlock()
	f.ResolveCall.CallCount++
	if f.ResolveCall.Stub != nil {
		return f.ResolveCall.Stub()
	}
	return f.ResolveCall.Returns.Image, f.ResolveCall.Returns.SBOM, f.ResolveCall.Returns.Error
}
