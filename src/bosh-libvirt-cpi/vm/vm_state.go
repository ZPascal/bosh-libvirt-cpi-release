package vm

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	libvirt "libvirt.org/go/libvirt"
)

func (vm VMImpl) Exists() (bool, error) {
	_, err := vm.driver.LookupDomain(vm.cid.AsString())
	if err != nil {
		if vm.driver.IsMissingDomainErr(err) {
			return false, nil
		}
		return false, bosherr.WrapErrorf(err, "Looking up domain '%s'", vm.cid.AsString())
	}
	return true, nil
}

func (vm VMImpl) Start() error {
	return vm.driver.StartDomain(vm.cid.AsString())
}

func (vm VMImpl) Reboot() error {
	return vm.driver.RebootDomain(vm.cid.AsString())
}

func (vm VMImpl) HaltIfRunning() error {
	running, err := vm.IsRunning()
	if err != nil {
		return err
	}

	if running {
		return vm.driver.ShutdownDomain(vm.cid.AsString())
	}
	return nil
}

func (vm VMImpl) IsRunning() (bool, error) {
	dom, err := vm.driver.LookupDomain(vm.cid.AsString())
	if err != nil {
		if vm.driver.IsMissingDomainErr(err) {
			return false, nil
		}
		return false, bosherr.WrapErrorf(err, "Looking up domain '%s'", vm.cid.AsString())
	}

	state, _, err := dom.GetState()
	if err != nil {
		return false, bosherr.WrapErrorf(err, "Getting domain state '%s'", vm.cid.AsString())
	}

	return state == int(libvirt.DOMAIN_RUNNING), nil
}
