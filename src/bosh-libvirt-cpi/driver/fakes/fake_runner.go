package fakes

import "bosh-libvirt-cpi/driver"

type FakeRunner struct {
	ExecuteOutput string
	ExecuteStatus int
	ExecuteErr    error

	UploadErr error

	PutContents map[string][]byte // keyed by path; populated by Put calls
	PutErr      error

	// GetResult, if non-nil, is returned for every Get call regardless of path.
	// If nil, Get returns whatever was last Put to the same path.
	GetResult []byte
	GetErr    error
}

var _ driver.Runner = &FakeRunner{}

func (r *FakeRunner) Execute(path string, args ...string) (string, int, error) {
	return r.ExecuteOutput, r.ExecuteStatus, r.ExecuteErr
}

func (r *FakeRunner) Upload(srcDir, dstDir string) error {
	return r.UploadErr
}

func (r *FakeRunner) Put(path string, contents []byte) error {
	if r.PutContents == nil {
		r.PutContents = make(map[string][]byte)
	}
	r.PutContents[path] = contents
	return r.PutErr
}

func (r *FakeRunner) Get(path string) ([]byte, error) {
	if r.GetResult != nil {
		return r.GetResult, r.GetErr
	}
	if r.PutContents != nil {
		if data, ok := r.PutContents[path]; ok {
			return data, r.GetErr
		}
	}
	return nil, r.GetErr
}
