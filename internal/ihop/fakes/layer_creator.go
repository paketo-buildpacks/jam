package fakes

import (
	"sync"

	"github.com/paketo-buildpacks/jam/internal/ihop"
)

type LayerCreator struct {
	CreateCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Image           ihop.Image
			DefinitionImage ihop.DefinitionImage
			SBOM            ihop.SBOM
		}
		Returns struct {
			Layer ihop.Layer
			Error error
		}
		Stub func(ihop.Image, ihop.DefinitionImage, ihop.SBOM) (ihop.Layer, error)
	}
}

func (f *LayerCreator) Create(param1 ihop.Image, param2 ihop.DefinitionImage, param3 ihop.SBOM) (ihop.Layer, error) {
	f.CreateCall.mutex.Lock()
	defer f.CreateCall.mutex.Unlock()
	f.CreateCall.CallCount++
	f.CreateCall.Receives.Image = param1
	f.CreateCall.Receives.DefinitionImage = param2
	f.CreateCall.Receives.SBOM = param3
	if f.CreateCall.Stub != nil {
		return f.CreateCall.Stub(param1, param2, param3)
	}
	return f.CreateCall.Returns.Layer, f.CreateCall.Returns.Error
}
