package fakes

import (
	"os"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// FakeFS wraps the real OS filesystem and overrides TempDir.
type FakeFS struct {
	boshsys.FileSystem
	TempDirErr error
	tempDirs   []string
}

var _ boshsys.FileSystem = &FakeFS{}

func NewFakeFS(real boshsys.FileSystem) *FakeFS {
	return &FakeFS{FileSystem: real}
}

func (f *FakeFS) TempDir(prefix string) (string, error) {
	if f.TempDirErr != nil {
		return "", f.TempDirErr
	}
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}
	f.tempDirs = append(f.tempDirs, dir)
	return dir, nil
}

func (f *FakeFS) Cleanup() {
	for _, d := range f.tempDirs {
		_ = os.RemoveAll(d)
	}
}
