package fakes

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	bdisk "bosh-libvirt-cpi/disk"
)

type FakeDiskFinder struct {
	FindArg    apiv1.DiskCID
	FindResult bdisk.Disk
	FindErr    error
}

var _ bdisk.Finder = &FakeDiskFinder{}

func (f *FakeDiskFinder) Find(cid apiv1.DiskCID) (bdisk.Disk, error) {
	f.FindArg = cid
	return f.FindResult, f.FindErr
}
