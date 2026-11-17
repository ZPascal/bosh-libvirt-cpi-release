package fakes

import (
	bstem "bosh-libvirt-cpi/stemcell"
)

type FakeImporter struct {
	ImportFromPathArg  string
	ImportFromPathArch string
	ImportResult       bstem.Stemcell
	ImportErr          error
}

var _ bstem.Importer = &FakeImporter{}

func (i *FakeImporter) ImportFromPath(path, arch string) (bstem.Stemcell, error) {
	i.ImportFromPathArg = path
	i.ImportFromPathArch = arch
	return i.ImportResult, i.ImportErr
}
