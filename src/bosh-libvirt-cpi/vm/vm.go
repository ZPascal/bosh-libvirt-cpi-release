package vm

import (
	"encoding/json"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/driver"
)

type VMImpl struct {
	cid   apiv1.VMCID
	store Store

	stemcellAPIVersion apiv1.StemcellAPIVersion

	driver driver.Driver
	logger boshlog.Logger
}

func NewVMImpl(
	cid apiv1.VMCID,
	store Store,
	stemcellAPIVersion apiv1.StemcellAPIVersion,
	driver driver.Driver,
	logger boshlog.Logger,
) VMImpl {
	return VMImpl{
		cid:                cid,
		store:              store,
		stemcellAPIVersion: stemcellAPIVersion,
		driver:             driver,
		logger:             logger,
	}
}

func (vm VMImpl) ID() apiv1.VMCID { return vm.cid }

func (vm VMImpl) SetProps(props VMProps) error {
	err := vm.driver.UpdateDomainMemory(vm.cid.AsString(), props.Memory)
	if err != nil {
		return bosherr.WrapError(err, "Updating domain memory")
	}

	err = vm.driver.UpdateDomainCPUs(vm.cid.AsString(), props.CPUs)
	if err != nil {
		return bosherr.WrapError(err, "Updating domain CPUs")
	}

	return nil
}

func (vm VMImpl) SetMetadata(meta apiv1.VMMeta) error {
	bytes, err := json.Marshal(meta)
	if err != nil {
		return bosherr.WrapError(err, "Marshaling VM metadata")
	}

	err = vm.store.Put("metadata.json", bytes)
	if err != nil {
		return bosherr.WrapError(err, "Saving VM metadata")
	}

	return nil
}

func (vm VMImpl) ConfigureNICs(networks apiv1.Networks) error {
	// libvirt domain XML already configures the NIC at definition time.
	// No-op here; agent receives network configuration via env.
	return nil
}

func (vm VMImpl) Delete() error {
	err := vm.HaltIfRunning()
	if err != nil {
		return err
	}

	err = vm.driver.DestroyDomain(vm.cid.AsString())
	if err != nil && !vm.driver.IsMissingDomainErr(err) {
		return bosherr.WrapErrorf(err, "Destroying VM domain '%s'", vm.cid.AsString())
	}

	return vm.store.Delete()
}
