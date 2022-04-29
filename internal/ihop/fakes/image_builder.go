package fakes

import (
	"sync"

	"github.com/paketo-buildpacks/jam/internal/ihop"
)

type ImageBuilder struct {
	ExecuteCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			DefinitionImage ihop.DefinitionImage
			Platform        string
		}
		Returns struct {
			ImageBuildPromise ihop.ImageBuildPromise
		}
		Stub func(ihop.DefinitionImage, string) ihop.ImageBuildPromise
	}
}

func (f *ImageBuilder) Execute(param1 ihop.DefinitionImage, param2 string) ihop.ImageBuildPromise {
	f.ExecuteCall.mutex.Lock()
	defer f.ExecuteCall.mutex.Unlock()
	f.ExecuteCall.CallCount++
	f.ExecuteCall.Receives.DefinitionImage = param1
	f.ExecuteCall.Receives.Platform = param2
	if f.ExecuteCall.Stub != nil {
		return f.ExecuteCall.Stub(param1, param2)
	}
	return f.ExecuteCall.Returns.ImageBuildPromise
}
