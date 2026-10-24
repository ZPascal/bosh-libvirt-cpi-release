package vm

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	bdisk "bosh-libvirt-cpi/disk"
	bstem "bosh-libvirt-cpi/stemcell"
)

// Creator creates VMs.
type Creator interface {
	Create(
		apiv1.AgentID,
		bstem.Stemcell,
		apiv1.VMCloudProps,
		apiv1.Networks,
		apiv1.VMEnv,
	) (VM, error)
}

var _ Creator = Factory{}

// Finder looks up a VM by CID.
type Finder interface {
	Find(apiv1.VMCID) (VM, error)
}

var _ Finder = Factory{}

// VM represents a VM managed by the CPI.
type VM interface {
	ID() apiv1.VMCID
	SetMetadata(apiv1.VMMeta) error

	Reboot() error
	Exists() (bool, error)
	Delete() error

	DiskIDs() ([]apiv1.DiskCID, error)
	AttachDisk(bdisk.Disk) (apiv1.DiskHint, error)
	AttachEphemeralDisk(bdisk.Disk) error
	DetachDisk(bdisk.Disk) error
}

var _ VM = VMImpl{}
