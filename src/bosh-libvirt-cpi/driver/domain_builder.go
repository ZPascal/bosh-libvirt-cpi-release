package driver

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
	// Kernel and Initrd enable direct kernel boot (skips BIOS/disk bootloader).
	// When set, the VM boots the given kernel with the rootfs as a 9p filesystem.
	Kernel string
	Initrd string
}

// DomainDiskPaths holds the paths to disk images for a VM domain.
type DomainDiskPaths struct {
	RootDisk      string
	EphemeralDisk string
}

// DomainBuilder produces libvirt XML domain definitions for a specific backend.
type DomainBuilder interface {
	BuildDomain(id string, props VMDomainProps, disks DomainDiskPaths) (string, error)
	BuildStemcellDomain(id string, imagePath string) (string, error)
	DiskImageFormat() string // "vmdk", "raw", "qcow2"
}
