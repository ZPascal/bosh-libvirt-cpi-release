package fakes

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	bdisk "bosh-libvirt-cpi/disk"
)

type FakeDisk struct {
	cid apiv1.DiskCID

	PathResult      string
	ImagePathResult string
	ExistsResult    bool
	ExistsErr       error
	DeleteErr       error
}

var _ bdisk.Disk = &FakeDisk{}

func NewFakeDisk(cid string) *FakeDisk {
	return &FakeDisk{cid: apiv1.NewDiskCID(cid)}
}

func (d *FakeDisk) ID() apiv1.DiskCID     { return d.cid }
func (d *FakeDisk) Path() string          { return d.PathResult }
func (d *FakeDisk) ImagePath() string     { return d.ImagePathResult }
func (d *FakeDisk) Exists() (bool, error) { return d.ExistsResult, d.ExistsErr }
func (d *FakeDisk) Delete() error         { return d.DeleteErr }
