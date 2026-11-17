package driver

import "errors"

// ErrPersistentDiskUnsupported is returned by domain builders whose backend
// cannot hot-plug a persistent disk into a running domain.
var ErrPersistentDiskUnsupported = errors.New("persistent disks are not supported by this backend")

// VMDomainProps holds backend-agnostic VM parameters for domain building.
type VMDomainProps struct {
	CPUs     int
	MemoryMB int
	// Network is the libvirt network name for the VM's interface (e.g. "default").
	// If empty, builders use "default".
	Network string
	// MAC is an optional fixed MAC address for the VM's network interface.
	// If empty, libvirt auto-generates one.
	MAC string
	// Kernel enables direct kernel boot (skips the BIOS/disk bootloader). When
	// set, the VM boots this kernel with the ext4 root disk mounted at /dev/vda.
	Kernel string
}

// DomainDiskPaths holds the paths to disk images for a VM domain.
type DomainDiskPaths struct {
	RootDisk      string
	EphemeralDisk string
	// EphemeralFormat is the image format of EphemeralDisk ("qcow2" or "raw");
	// empty means qcow2.
	EphemeralFormat string
}

// DomainBuilder produces libvirt XML domain definitions for a specific backend.
type DomainBuilder interface {
	BuildDomain(id string, props VMDomainProps, disks DomainDiskPaths) (string, error)
	BuildStemcellDomain(id string, imagePath string) (string, error)
	DiskImageFormat() string // "vmdk", "raw", "qcow2"
	// BuildPersistentDiskXML returns the <disk> device XML that attaches the
	// persistent disk image (format "qcow2" or "raw") at target (e.g. "vdc")
	// with the given serial, or ErrPersistentDiskUnsupported.
	BuildPersistentDiskXML(imagePath, format, target, serial string) (string, error)
}
