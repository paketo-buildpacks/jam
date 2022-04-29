package fakes

import (
	"sync"

	"github.com/paketo-buildpacks/jam/internal/ihop"
)

type ImageScanner struct {
	ScanCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Tag string
		}
		Returns struct {
			SBOM  ihop.SBOM
			Error error
		}
		Stub func(string) (ihop.SBOM, error)
	}
}

func (f *ImageScanner) Scan(param1 string) (ihop.SBOM, error) {
	f.ScanCall.mutex.Lock()
	defer f.ScanCall.mutex.Unlock()
	f.ScanCall.CallCount++
	f.ScanCall.Receives.Tag = param1
	if f.ScanCall.Stub != nil {
		return f.ScanCall.Stub(param1)
	}
	return f.ScanCall.Returns.SBOM, f.ScanCall.Returns.Error
}
