package driver

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

	// Storage
	CreateStorageVol(poolName, volName string, sizeMB int) (string, error)
	DeleteStorageVol(poolName, volName string) error

	// Error helpers
	IsMissingDomainErr(err error) bool
}

type Domain interface {
	GetName() (string, error)
	GetState() (int, int, error)
	IsActive() (bool, error)
	Free() error
}

type Runner interface {
	Execute(path string, args ...string) (string, int, error)
	Upload(srcDir, dstDir string) error
	Put(path string, contents []byte) error
	Get(path string) ([]byte, error)
}

var _ Runner = LocalRunner{}
var _ Runner = &SSHRunner{}
var _ Runner = &ExpandingPathRunner{}

type RawRunner interface {
	HomeDir() (string, error)
	Runner
}

var _ RawRunner = LocalRunner{}
var _ RawRunner = &SSHRunner{}
