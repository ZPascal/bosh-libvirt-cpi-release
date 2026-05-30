package fakes

import (
	bstem "bosh-libvirt-cpi/stemcell"
)

type FakeImporter struct {
	ImportFromPathArg string
	ImportResult      bstem.Stemcell
	ImportErr         error
}

var _ bstem.Importer = &FakeImporter{}

func (i *FakeImporter) ImportFromPath(path string) (bstem.Stemcell, error) {
	i.ImportFromPathArg = path
	return i.ImportResult, i.ImportErr
}
