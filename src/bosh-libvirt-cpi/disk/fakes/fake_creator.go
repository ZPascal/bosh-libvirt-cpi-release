package fakes

import (
	bdisk "bosh-libvirt-cpi/disk"
)

type FakeDiskCreator struct {
	CreateSizeArg int
	CreateResult  bdisk.Disk
	CreateErr     error
}

var _ bdisk.Creator = &FakeDiskCreator{}

func (c *FakeDiskCreator) Create(size int) (bdisk.Disk, error) {
	c.CreateSizeArg = size
	return c.CreateResult, c.CreateErr
}
