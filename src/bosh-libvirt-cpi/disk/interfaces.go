package disk

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
)

// Creator creates persistent disks.
type Creator interface {
	Create(int) (Disk, error)
}

var _ Creator = Factory{}

// Finder looks up a disk by CID.
type Finder interface {
	Find(apiv1.DiskCID) (Disk, error)
}

var _ Finder = Factory{}

// Disk represents a persistent disk managed by the CPI.
type Disk interface {
	ID() apiv1.DiskCID

	Path() string
	ImagePath() string

	Exists() (bool, error)
	Delete() error
}

var _ Disk = DiskImpl{}
