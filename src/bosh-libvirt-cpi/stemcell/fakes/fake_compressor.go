package fakes

import boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"

type FakeCompressor struct {
	DecompressFileToDirErr error
}

var _ boshcmd.Compressor = &FakeCompressor{}

func (c *FakeCompressor) CompressFilesInDir(dir string) (string, error) {
	return "", nil
}

func (c *FakeCompressor) CompressSpecificFilesInDir(dir string, files []string) (string, error) {
	return "", nil
}

func (c *FakeCompressor) DecompressFileToDir(path, dir string, opts boshcmd.CompressorOptions) error {
	return c.DecompressFileToDirErr
}

func (c *FakeCompressor) CleanUp(tarballPath string) error {
	return nil
}
