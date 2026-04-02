package cpi

import (
	"path/filepath"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	bpds "bosh-libvirt-cpi/vm/portdevices"
)

type FactoryOpts struct {
	// Hypervisor configuration (libvirt driver type)
	Hypervisor string `json:"hypervisor"`

	// Connection settings
	Host       string `json:"host"`
	Username   string `json:"username"`
	PrivateKey string `json:"private_key"`

	// Libvirt settings
	BinPath  string `json:"bin_path"`
	StoreDir string `json:"store_dir"`
	URI      string `json:"uri"` // Libvirt connection URI (auto-generated if not provided)

	// VM settings
	StorageController  string `json:"storage_controller"`
	AutoEnableNetworks bool   `json:"auto_enable_networks"`

	Agent apiv1.AgentOptions `json:"agent"`
}

func (o FactoryOpts) Validate() error {
	// Note: Defaults should be applied before calling Validate()
	// Check if required defaults are present
	if o.Hypervisor == "" {
		return bosherr.Error("Hypervisor must be set (defaults should be applied before validation)")
	}

	if o.BinPath == "" {
		return bosherr.Error("BinPath must be set (defaults should be applied before validation)")
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
