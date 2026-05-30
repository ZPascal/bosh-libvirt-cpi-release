package fakes

import "bosh-libvirt-cpi/driver"

type FakeRunner struct {
	ExecuteOutput string
	ExecuteStatus int
	ExecuteErr    error

	UploadErr error

	PutErr error

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
	return r.PutErr
}

func (r *FakeRunner) Get(path string) ([]byte, error) {
	return r.GetResult, r.GetErr
}
