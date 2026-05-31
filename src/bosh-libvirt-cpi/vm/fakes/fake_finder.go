package fakes

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	bvm "bosh-libvirt-cpi/vm"
)

type FakeVMFinder struct {
	FindArg    apiv1.VMCID
	FindResult bvm.VM
	FindErr    error
}

var _ bvm.Finder = &FakeVMFinder{}

func (f *FakeVMFinder) Find(cid apiv1.VMCID) (bvm.VM, error) {
	f.FindArg = cid
	return f.FindResult, f.FindErr
}
