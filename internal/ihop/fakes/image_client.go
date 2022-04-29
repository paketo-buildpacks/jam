package fakes

import (
	"sync"

	"github.com/paketo-buildpacks/jam/internal/ihop"
)

type ImageClient struct {
	BuildCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			DefinitionImage ihop.DefinitionImage
			Platform        string
		}
		Returns struct {
			Image ihop.Image
			Error error
		}
		Stub func(ihop.DefinitionImage, string) (ihop.Image, error)
	}
	UpdateCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Image ihop.Image
		}
		Returns struct {
			Image ihop.Image
			Error error
		}
		Stub func(ihop.Image) (ihop.Image, error)
	}
}

func (f *ImageClient) Build(param1 ihop.DefinitionImage, param2 string) (ihop.Image, error) {
	f.BuildCall.mutex.Lock()
	defer f.BuildCall.mutex.Unlock()
	f.BuildCall.CallCount++
	f.BuildCall.Receives.DefinitionImage = param1
	f.BuildCall.Receives.Platform = param2
	if f.BuildCall.Stub != nil {
		return f.BuildCall.Stub(param1, param2)
	}
	return f.BuildCall.Returns.Image, f.BuildCall.Returns.Error
}
func (f *ImageClient) Update(param1 ihop.Image) (ihop.Image, error) {
	f.UpdateCall.mutex.Lock()
	defer f.UpdateCall.mutex.Unlock()
	f.UpdateCall.CallCount++
	f.UpdateCall.Receives.Image = param1
	if f.UpdateCall.Stub != nil {
		return f.UpdateCall.Stub(param1)
	}
	return f.UpdateCall.Returns.Image, f.UpdateCall.Returns.Error
}
