package cpi

import (
	"net/url"
	"path/filepath"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type FactoryOpts struct {
	// Connection
	BackendURI string // e.g. "vbox:///session", "lxc:///", "qemu:///system"
	Host       string
	Port       int // SSH port for remote Host connections; defaults to 22 if zero
	Username   string
	PrivateKey string
	HostKey    string // SSH host public key in authorized_keys format; required when Host is set

	// Network is the libvirt network name for VM interfaces. Defaults to "default" if empty.
	Network string

	StoreDir string

	// InjectHost/Username/PrivateKey/HostKey/StoreDir carry the SSH credentials
	// that the deployed director's CPI must use. They are written into the VM
	// rootfs at create_vm time so the agent-rendered cpi.json is overwritten on
	// first boot. When unset they fall back to Host/Username/PrivateKey/HostKey/
	// StoreDir respectively (useful when bootstrap and deployed CPI share the
	// same credentials, e.g. the QEMU case).
	InjectHost       string
	InjectUsername   string
	InjectPrivateKey string
	InjectHostKey    string
	InjectStoreDir   string

	// MbusBootstrapSSL is the TLS cert/key for the agent mbus bootstrap listener.
	// When provided (via cloud_provider.properties.mbus_bootstrap_ssl in the manifest),
	// it is injected into the agent env so bosh create-env can verify the mbus endpoint
	// using cloud_provider.cert: ((mbus_bootstrap_ssl)).
	MbusBootstrapSSL struct {
		CA          string `json:"ca"`
		Certificate string `json:"certificate"`
		PrivateKey  string `json:"private_key"`
	}

	Agent apiv1.AgentOptions
}

func (o FactoryOpts) Validate() error {
	if len(o.Host) > 0 {
		if o.Username == "" {
			return bosherr.Error("Must provide non-empty Username")
		}
		if o.PrivateKey == "" {
			return bosherr.Error("Must provide non-empty PrivateKey")
		}
		if o.HostKey == "" {
			return bosherr.Error("Must provide non-empty HostKey when Host is set")
		}
	}

	if o.BackendURI == "" {
		return bosherr.Error("Must provide non-empty BackendURI")
	}

	u, err := url.Parse(o.BackendURI)
	if err != nil {
		return bosherr.WrapError(err, "Parsing BackendURI")
	}

	switch u.Scheme {
	case "vbox", "lxc", "qemu":
		// valid
	default:
		return bosherr.Errorf("Unsupported BackendURI scheme '%s': expected 'vbox', 'lxc', or 'qemu'", u.Scheme)
	}

	if o.StoreDir == "" {
		return bosherr.Error("Must provide non-empty StoreDir")
	}

	err = o.Agent.Validate()
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
