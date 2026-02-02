package cpi

import (
	"path/filepath"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	bpds "bosh-libvirt-cpi/vm/portdevices"
)

type FactoryOpts struct {
	// Hypervisor configuration (libvirt driver type)
	Hypervisor string // "qemu", "vbox", "lxc", "xen", "vmware"

	// Connection settings
	Host       string
	Username   string
	PrivateKey string

	// Libvirt settings
	BinPath  string
	StoreDir string
	URI      string // Libvirt connection URI (auto-generated if not provided)

	// VM settings
	StorageController  string
	AutoEnableNetworks bool

	Agent apiv1.AgentOptions
}

func (o FactoryOpts) Validate() error {
	// Default to qemu/kvm for backward compatibility
	if o.Hypervisor == "" {
		o.Hypervisor = "qemu"
	}

	// Validate hypervisor type
	validHypervisors := []string{"qemu", "vbox", "lxc", "xen", "vmware"}
	isValid := false
	for _, h := range validHypervisors {
		if o.Hypervisor == h {
			isValid = true
			break
		}
	}
	if !isValid {
		return bosherr.Errorf("Invalid hypervisor '%s'. Must be one of: qemu, vbox, lxc, xen, vmware", o.Hypervisor)
	}

	if len(o.Host) > 0 {
		if o.Username == "" {
			return bosherr.Error("Must provide non-empty Username")
		}

		if o.PrivateKey == "" {
			return bosherr.Error("Must provide non-empty PrivateKey")
		}
	}

	if o.BinPath == "" {
		o.BinPath = "virsh"
	}

	if o.StoreDir == "" {
		return bosherr.Error("Must provide non-empty StoreDir")
	}

	switch o.StorageController {
	case bpds.IDEController, bpds.SCSIController, bpds.SATAController:
		// valid
	default:
		return bosherr.Error("Unexpected StorageController")
	}

	err := o.Agent.Validate()
	if err != nil {
		return bosherr.WrapError(err, "Validating Agent configuration")
	}

	return nil
}

func (o FactoryOpts) StemcellsDir() string {
	return filepath.Join(o.StoreDir, "stemcells")
}

func (o FactoryOpts) VMsDir() string {
	return filepath.Join(o.StoreDir, "vms")
}

func (o FactoryOpts) DisksDir() string {
	return filepath.Join(o.StoreDir, "disks")
}
