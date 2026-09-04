package disk

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	"bosh-libvirt-cpi/driver"
)

type Factory struct {
	dirPath string
	uuidGen boshuuid.Generator

	driver driver.Driver
	runner driver.Runner

	logger boshlog.Logger
}

func NewFactory(
	dirPath string,
	uuidGen boshuuid.Generator,
	driver driver.Driver,
	runner driver.Runner,
	logger boshlog.Logger,
) Factory {
	return Factory{
		dirPath: dirPath,
		uuidGen: uuidGen,

		driver: driver,
		runner: runner,

		logger: logger,
	}
}

func (f Factory) Create(size int) (Disk, error) {
	id, err := f.uuidGen.Generate()
	if err != nil {
		return nil, bosherr.WrapError(err, "Generating disk id")
	}

	id = "disk-" + id

	disk := f.newDisk(apiv1.NewDiskCID(id))

	if err := os.MkdirAll(disk.Path(), 0755); err != nil {
		return nil, bosherr.WrapError(err, "Creating disk parent")
	}

	// Try qemu-img first (produces qcow2 for QEMU/KVM); fall back to dd (raw).
	sizeStr := strconv.Itoa(size) + "M"
	out, qemuErr := exec.Command("qemu-img", "create", "-f", "qcow2", disk.ImagePath(), sizeStr).CombinedOutput()
	if qemuErr != nil {
		// Fall back to sparse raw image via dd.
		_, _, err = f.runner.Execute(
			"dd",
			"if=/dev/zero",
			"of="+disk.ImagePath(),
			"bs=1M",
			"count=0",
			"seek="+strconv.Itoa(size),
		)
		if err != nil {
			return nil, bosherr.WrapErrorf(err, "Creating disk image (qemu-img failed: %s)", string(out))
		}
	} else {
		if err := os.Chmod(disk.ImagePath(), 0644); err != nil {
			return nil, bosherr.WrapError(err, "Setting disk image permissions")
		}
	}

	return disk, nil
}

func (f Factory) Find(cid apiv1.DiskCID) (Disk, error) {
	return f.newDisk(cid), nil
}

func (f Factory) newDisk(cid apiv1.DiskCID) DiskImpl {
	diskPath := filepath.Join(f.dirPath, cid.AsString())
	return NewDiskImpl(cid, diskPath, f.runner, f.logger)
}
