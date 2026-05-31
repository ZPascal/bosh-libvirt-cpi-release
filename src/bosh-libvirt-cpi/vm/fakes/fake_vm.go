package fakes

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	bdisk "bosh-libvirt-cpi/disk"
	bvm "bosh-libvirt-cpi/vm"
)

type FakeVM struct {
	cid apiv1.VMCID

	SetMetadataArg apiv1.VMMeta
	SetMetadataErr error

	RebootErr    error
	ExistsResult bool
	ExistsErr    error
	DeleteErr    error

	DiskIDsResult []apiv1.DiskCID
	DiskIDsErr    error

	AttachDiskArg  bdisk.Disk
	AttachDiskHint apiv1.DiskHint
	AttachDiskErr  error

	AttachEphemeralDiskArg bdisk.Disk
	AttachEphemeralDiskErr error

	DetachDiskArg bdisk.Disk
	DetachDiskErr error
}

var _ bvm.VM = &FakeVM{}

func NewFakeVM(cid string) *FakeVM {
	return &FakeVM{cid: apiv1.NewVMCID(cid)}
}

func (v *FakeVM) ID() apiv1.VMCID { return v.cid }

func (v *FakeVM) SetMetadata(meta apiv1.VMMeta) error {
	v.SetMetadataArg = meta
	return v.SetMetadataErr
}

func (v *FakeVM) Reboot() error         { return v.RebootErr }
func (v *FakeVM) Exists() (bool, error) { return v.ExistsResult, v.ExistsErr }
func (v *FakeVM) Delete() error         { return v.DeleteErr }
func (v *FakeVM) DiskIDs() ([]apiv1.DiskCID, error) {
	return v.DiskIDsResult, v.DiskIDsErr
}

func (v *FakeVM) AttachDisk(d bdisk.Disk) (apiv1.DiskHint, error) {
	v.AttachDiskArg = d
	return v.AttachDiskHint, v.AttachDiskErr
}

func (v *FakeVM) AttachEphemeralDisk(d bdisk.Disk) error {
	v.AttachEphemeralDiskArg = d
	return v.AttachEphemeralDiskErr
}

func (v *FakeVM) DetachDisk(d bdisk.Disk) error {
	v.DetachDiskArg = d
	return v.DetachDiskErr
}
