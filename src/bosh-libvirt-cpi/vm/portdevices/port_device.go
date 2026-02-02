package portdevices

import (
	"fmt"
	"strconv"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	"bosh-libvirt-cpi/driver"
)

type PortDevice struct {
	driver driver.Driver
	vmCID  apiv1.VMCID

	controller string // e.g. scsi, ide, sata
	name       string // e.g. IDE, SCSI, AHCI Controller

	port   string
	device string
}

func NewPortDevice(driver driver.Driver, vmCID apiv1.VMCID, controller, name, port, device string) PortDevice {
	if len(controller) == 0 {
		panic("Internal inconsistency: PD's controller must not be empty")
	}
	if len(name) == 0 {
		panic("Internal inconsistency: PD's name must not be empty")
	}
	if len(port) == 0 {
		panic("Internal inconsistency: PD's port must not be empty")
	}
	if len(device) == 0 {
		panic("Internal inconsistency: PD's device must not be empty")
	}
	return PortDevice{
		driver: driver,
		vmCID:  vmCID,

		controller: controller,
		name:       name,

		port:   port,
		device: device,
	}
}

func (d PortDevice) Controller() string { return d.controller }

func (d PortDevice) Port() string   { return d.port }
func (d PortDevice) Device() string { return d.device }

func (d PortDevice) Hint() apiv1.DiskHint {
	// For libvirt, devices are typically named vda, vdb, vdc, etc. (virtio)
	// or sda, sdb, sdc, etc. (SCSI/SATA)
	// or hda, hdb, hdc, etc. (IDE)

	portNum, _ := strconv.Atoi(d.port)
	deviceNum, _ := strconv.Atoi(d.device)
	diskIndex := portNum*2 + deviceNum

	switch d.controller {
	case IDEController:
		// IDE: hda, hdb, hdc, hdd
		diskLetter := string(rune('a' + diskIndex))
		return apiv1.NewDiskHintFromString("hd" + diskLetter)

	case SCSIController, SATAController:
		// SCSI/SATA with virtio: vda, vdb, vdc, etc.
		// Or SCSI: sda, sdb, sdc, etc.
		diskLetter := string(rune('a' + diskIndex))
		// Use virtio naming for better performance
		return apiv1.NewDiskHintFromString("vd" + diskLetter)

	default:
		panic(fmt.Sprintf("Unexpected storage controller '%s'", d.name))
	}
}

func (d PortDevice) Attach(path string) error {
	// Generate device target name
	portNum, _ := strconv.Atoi(d.port)
	deviceNum, _ := strconv.Atoi(d.device)
	diskIndex := portNum*2 + deviceNum

	var targetDev string
	switch d.controller {
	case IDEController:
		targetDev = fmt.Sprintf("hd%c", 'a'+diskIndex)
	case SCSIController, SATAController:
		// Use virtio block device naming
		targetDev = fmt.Sprintf("vd%c", 'a'+diskIndex)
	default:
		return bosherr.Errorf("Unknown controller type: %s", d.controller)
	}

	// Attach disk using virsh attach-disk
	_, err := d.driver.Execute(
		"attach-disk", d.vmCID.AsString(),
		path,
		targetDev,
		"--persistent",
		"--subdriver", "qcow2",
	)
	return err
}

func (d PortDevice) Detach() error {
	// Generate device target name (same logic as Attach)
	portNum, _ := strconv.Atoi(d.port)
	deviceNum, _ := strconv.Atoi(d.device)
	diskIndex := portNum*2 + deviceNum

	var targetDev string
	switch d.controller {
	case IDEController:
		targetDev = fmt.Sprintf("hd%c", 'a'+diskIndex)
	case SCSIController, SATAController:
		targetDev = fmt.Sprintf("vd%c", 'a'+diskIndex)
	default:
		return bosherr.Errorf("Unknown controller type: %s", d.controller)
	}

	// Detach disk using virsh detach-disk
	_, err := d.driver.Execute(
		"detach-disk", d.vmCID.AsString(),
		targetDev,
		"--persistent",
	)
	return err
}
