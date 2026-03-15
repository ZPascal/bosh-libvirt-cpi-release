package main

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"bosh-libvirt-cpi/cpi"
)

type Config cpi.FactoryOpts

func NewConfigFromPath(path string, fs boshsys.FileSystem) (Config, error) {
	var config Config

	bytes, err := fs.ReadFile(path)
	if err != nil {
		return config, bosherr.WrapErrorf(err, "Reading config '%s'", path)
	}

	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return config, bosherr.WrapError(err, "Unmarshalling config")
	}

	// Apply defaults before validation
	if config.Hypervisor == "" {
		config.Hypervisor = "qemu" // Default to qemu/kvm for backward compatibility
	}

	if config.BinPath == "" {
		config.BinPath = "virsh"
	}

	err = cpi.FactoryOpts(config).Validate()
	if err != nil {
		return config, bosherr.WrapError(err, "Validating configuration")
	}

	return config, nil
}
