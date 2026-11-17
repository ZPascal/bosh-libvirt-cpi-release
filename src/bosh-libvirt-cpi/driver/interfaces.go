package driver

// Driver abstracts libvirt domain and storage operations.
type Driver interface {
	// Domain lifecycle
	DefineDomain(xml string) error
	StartDomain(id string) error
	ShutdownDomain(id string) error
	DestroyDomain(id string) error
	RebootDomain(id string) error
	LookupDomain(id string) (Domain, error)

	// Domain config
	UpdateDomainMemory(id string, memoryMB int) error
	UpdateDomainCPUs(id string, cpus int) error

	// Devices: attach/detach a device (e.g. a persistent <disk>) to the
	// domain's persistent config and, when it is running, to the live domain.
	AttachDevice(id, xml string) error
	DetachDevice(id, xml string) error

	// Storage
	CreateStorageVol(poolName, volName string, sizeMB int) (string, error)
	DeleteStorageVol(poolName, volName string) error

	// Host: CPU architecture of the libvirt host ("x86_64", "aarch64", ...),
	// which decides between KVM and emulation for a guest architecture.
	HostArch() (string, error)

	// Error helpers
	IsMissingDomainErr(err error) bool
}

// Domain wraps a libvirt domain for state queries.
type Domain interface {
	GetName() (string, error)
	GetState() (int, int, error)
	IsActive() (bool, error)
	Free() error
}

// Runner executes commands and transfers files on a target host.
type Runner interface {
	Execute(path string, args ...string) (string, int, error)
	Upload(srcDir, dstDir string) error
	Put(path string, contents []byte) error
	Get(path string) ([]byte, error)
}

var _ Runner = LocalRunner{}
var _ Runner = &SSHRunner{}
var _ Runner = &ExpandingPathRunner{}

// RawRunner extends Runner with a HomeDir lookup used by ExpandingPathRunner.
type RawRunner interface {
	HomeDir() (string, error)
	Runner
}

var _ RawRunner = LocalRunner{}
var _ RawRunner = &SSHRunner{}
