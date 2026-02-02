package provider

import (
	"bosh-libvirt-cpi/driver"
)

// HypervisorType represents the libvirt hypervisor/driver type
type HypervisorType string

const (
	HypervisorTypeQEMU   HypervisorType = "qemu"   // QEMU/KVM
	HypervisorTypeVBox   HypervisorType = "vbox"   // VirtualBox via libvirt
	HypervisorTypeLXC    HypervisorType = "lxc"    // Linux Containers
	HypervisorTypeXen    HypervisorType = "xen"    // Xen
	HypervisorTypeVMware HypervisorType = "vmware" // VMware ESX
)

// Provider defines the interface for libvirt-based infrastructure management
type Provider interface {
	// GetDriver returns the driver for this provider
	GetDriver() driver.Driver

	// GetHypervisor returns the hypervisor type
	GetHypervisor() HypervisorType

	// Initialize initializes the provider
	Initialize() error

	// Cleanup performs any necessary cleanup
	Cleanup() error

	// VM operations
	CreateVM(name string, opts VMOptions) error
	DeleteVM(name string) error
	StartVM(name string) error
	StopVM(name string, force bool) error
	GetVMState(name string) (VMState, error)
	ModifyVM(name string, opts VMModifyOptions) error

	// Disk operations
	CreateDisk(path string, sizeMB int) error
	AttachDisk(vmName, diskPath string, port int, device int) error
	DetachDisk(vmName string, port int, device int) error

	// Network operations
	CreateNetwork(name string, opts NetworkOptions) error
	DeleteNetwork(name string) error
	AttachNIC(vmName string, nicIndex int, opts NICOptions) error

	// Snapshot operations
	CreateSnapshot(vmName, snapshotName string) error
	DeleteSnapshot(vmName, snapshotName string) error
	RestoreSnapshot(vmName, snapshotName string) error

	// Utility operations
	CloneVM(srcName, dstName string, opts CloneOptions) error
	ExportVM(vmName, outputPath string) error
	ImportVM(imagePath, vmName string) error

	// Info operations
	GetVMInfo(name string) (VMInfo, error)
	ListVMs() ([]string, error)
}

// VMOptions contains options for creating a VM
type VMOptions struct {
	Memory            int
	CPUs              int
	OSType            string
	ParavirtProvider  string
	Audio             string
	Firmware          string
	StorageController string
}

// VMModifyOptions contains options for modifying a VM
type VMModifyOptions struct {
	Memory           *int
	CPUs             *int
	ParavirtProvider *string
	Audio            *string
	Firmware         *string
}

// VMState represents the state of a VM
type VMState string

const (
	VMStateRunning  VMState = "running"
	VMStatePowerOff VMState = "poweroff"
	VMStatePaused   VMState = "paused"
	VMStateAborted  VMState = "aborted"
	VMStateSaved    VMState = "saved"
	VMStateUnknown  VMState = "unknown"
)

// VMInfo contains information about a VM
type VMInfo struct {
	Name   string
	UUID   string
	State  VMState
	Memory int
	CPUs   int
}

// NetworkOptions contains options for creating a network
type NetworkOptions struct {
	Type        string
	CIDR        string
	DHCPEnabled bool
}

// NICOptions contains options for attaching a NIC
type NICOptions struct {
	Type          string
	NetworkName   string
	MACAddress    string
	BridgeAdapter string
}

// CloneOptions contains options for cloning a VM
type CloneOptions struct {
	Linked   bool
	Snapshot string
}
