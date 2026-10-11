package cpi

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
)

type Misc struct{}

func NewMisc() Misc {
	return Misc{}
}

func (m Misc) Info() (apiv1.Info, error) {
	// warden-tar is the only format this CPI can provision: stemcell.Factory
	// consumes a warden-tar rootfs tarball (extracted into a directory for LXC
	// warden-tar is the only format this CPI can provision: stemcell.Factory
	// consumes a warden-tar rootfs tarball (extracted into a directory for LXC
	// or an ext4 image for QEMU kernel boot). The general-ovf/vsphere-ovf
	// formats advertised previously were only ever usable by the old VBox
	// driver, which has been removed, so they are no longer offered.
	return apiv1.Info{
		StemcellFormats: []string{"warden-tar"},
	}, nil
}
