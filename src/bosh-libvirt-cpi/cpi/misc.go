package cpi

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
)

// Misc implements the CPI Info method.
type Misc struct{}

// NewMisc returns a Misc.
func NewMisc() Misc {
	return Misc{}
}

// Info reports the stemcell formats this CPI accepts.
func (m Misc) Info() (apiv1.Info, error) {
	// warden-tar is the only format this CPI can provision: stemcell.Factory
	// consumes a warden-tar rootfs tarball (extracted into a directory for LXC
	// or an ext4 image for QEMU kernel boot).
	return apiv1.Info{
		StemcellFormats: []string{"warden-tar"},
	}, nil
}
