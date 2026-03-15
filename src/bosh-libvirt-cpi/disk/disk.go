package disk

import (
	"path/filepath"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/driver"
)

type DiskImpl struct {
	cid  apiv1.DiskCID
	path string

	runner driver.Runner
	logger boshlog.Logger
}

func NewDiskImpl(
	cid apiv1.DiskCID,
	path string,
	runner driver.Runner,
	logger boshlog.Logger,
) DiskImpl {
	return DiskImpl{cid, path, runner, logger}
}

func (d DiskImpl) ID() apiv1.DiskCID { return d.cid }

func (d DiskImpl) Path() string { return d.path }

// VMDKPath returns the disk file path (qcow2 for libvirt)
func (d DiskImpl) VMDKPath() string {
	// For libvirt, we use qcow2 format instead of VMDK
	return filepath.Join(d.path, "disk.qcow2")
}

// DiskPath returns the actual disk file path
func (d DiskImpl) DiskPath() string {
	return d.VMDKPath()
}

func (d DiskImpl) Exists() (bool, error) {
	_, _, err := d.runner.Execute("ls", d.path)
	if err != nil {
		return false, bosherr.WrapErrorf(err, "Checking disk '%s'", d.path)
	}

	// todo check status

	return true, nil
}

func (d DiskImpl) Delete() error {
	_, _, err := d.runner.Execute("rm", "-rf", d.path)
	if err != nil {
		return bosherr.WrapErrorf(err, "Deleting disk '%s'", d.path)
	}

	return nil
}
