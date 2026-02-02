package provider

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"bosh-libvirt-cpi/driver"
)

// ProviderFactory creates provider instances
type ProviderFactory struct {
	logger boshlog.Logger
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(logger boshlog.Logger) ProviderFactory {
	return ProviderFactory{logger: logger}
}

// Create creates a libvirt provider with the specified hypervisor type
func (f ProviderFactory) Create(
	hypervisor HypervisorType,
	runner driver.Runner,
	retrier driver.Retrier,
	fs boshsys.FileSystem,
	opts ProviderOptions,
) (Provider, error) {
	return NewLibvirtProvider(hypervisor, runner, retrier, fs, opts, f.logger)
}

// ProviderOptions contains libvirt provider options
type ProviderOptions struct {
	BinPath    string         // Path to virsh binary
	StoreDir   string         // Storage directory for VMs and disks
	Host       string         // Remote host (for SSH connections)
	URI        string         // Libvirt connection URI (auto-generated if not provided)
	Hypervisor HypervisorType // Hypervisor type (qemu, vbox, lxc, etc.)
}

// GetConnectionURI returns the libvirt connection URI based on hypervisor and settings
func (o ProviderOptions) GetConnectionURI() string {
	if o.URI != "" {
		return o.URI
	}

	// Auto-generate URI based on the hypervisor type
	switch o.Hypervisor {
	case HypervisorTypeQEMU:
		return "qemu:///system"
	case HypervisorTypeVBox:
		return "vbox:///session"
	case HypervisorTypeLXC:
		return "lxc:///"
	case HypervisorTypeXen:
		return "xen:///"
	case HypervisorTypeVMware:
		return "vmware:///session"
	default:
		return "qemu:///system"
	}
}
