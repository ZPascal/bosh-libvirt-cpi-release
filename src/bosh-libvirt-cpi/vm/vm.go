package vm

import (
	"encoding/json"
	"strconv"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/driver"
	bpds "bosh-libvirt-cpi/vm/portdevices"
)

type VMImpl struct {
	cid         apiv1.VMCID
	portDevices bpds.PortDevices
	store       Store

	stemcellAPIVersion apiv1.StemcellAPIVersion

	driver driver.Driver
	logger boshlog.Logger
}

func NewVMImpl(
	cid apiv1.VMCID,
	portDevices bpds.PortDevices,
	store Store,
	stemcellAPIVersion apiv1.StemcellAPIVersion,
	driver driver.Driver,
	logger boshlog.Logger,
) VMImpl {
	return VMImpl{
		cid:                cid,
		portDevices:        portDevices,
		store:              store,
		stemcellAPIVersion: stemcellAPIVersion,
		driver:             driver,
		logger:             logger,
	}
}

func (vm VMImpl) ID() apiv1.VMCID { return vm.cid }

func (vm VMImpl) SetProps(props VMProps) error {
	// For libvirt, we modify domain properties using virsh commands

	// Memory modification (in KB)
	if props.Memory > 0 {
		memoryKB := strconv.Itoa(props.Memory * 1024)
		_, err := vm.driver.Execute("setmaxmem", vm.cid.AsString(), memoryKB, "--config")
		if err != nil {
			return bosherr.WrapErrorf(err, "Setting max memory")
		}
		_, err = vm.driver.Execute("setmem", vm.cid.AsString(), memoryKB, "--config")
		if err != nil {
			return bosherr.WrapErrorf(err, "Setting memory")
		}
	}

	// CPU modification
	if props.CPUs > 0 {
		_, err := vm.driver.Execute("setvcpus", vm.cid.AsString(), strconv.Itoa(props.CPUs), "--config", "--maximum")
		if err != nil {
			return bosherr.WrapErrorf(err, "Setting maximum vcpus")
		}
		_, err = vm.driver.Execute("setvcpus", vm.cid.AsString(), strconv.Itoa(props.CPUs), "--config")
		if err != nil {
			return bosherr.WrapErrorf(err, "Setting vcpus")
		}
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

func (vm VMImpl) ConfigureNICs(nets Networks, host Host) error {
	return NICs{vm.driver, vm.ID()}.Configure(nets, host)
}

func (vm VMImpl) Delete() error {
	err := vm.HaltIfRunning()
	if err != nil {
		return bosherr.WrapError(err, "Halting VM")
	}

	err = vm.detachPersistentDisks()
	if err != nil {
		return bosherr.WrapError(err, "Detaching persistent disks")
	}

	_, err = vm.driver.Execute("undefine", vm.cid.AsString(), "--remove-all-storage")
	if err != nil {
		return bosherr.WrapError(err, "Undefining domain")
	}

	err = vm.store.Delete()
	if err != nil {
		return bosherr.WrapError(err, "Deleting VM store")
	}

	return nil
}
