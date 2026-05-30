package fakes

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	bstem "bosh-libvirt-cpi/stemcell"
)

type FakeStemcellFinder struct {
	FindArg    apiv1.StemcellCID
	FindResult bstem.Stemcell
	FindErr    error
}

var _ bstem.Finder = &FakeStemcellFinder{}

func (f *FakeStemcellFinder) Find(cid apiv1.StemcellCID) (bstem.Stemcell, error) {
	f.FindArg = cid
	return f.FindResult, f.FindErr
}
