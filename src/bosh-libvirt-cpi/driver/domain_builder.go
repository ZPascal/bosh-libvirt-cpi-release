package driver

// VMDomainProps holds backend-agnostic VM parameters for domain building.
type VMDomainProps struct {
	CPUs     int
	MemoryMB int
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
	DiskImageFormat() string   // "vmdk", "raw", "qcow2"
	StorageController() string // "ide", "sata", "virtio", "lxc"
}
