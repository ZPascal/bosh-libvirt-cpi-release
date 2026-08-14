package domains

import (
	"fmt"

	"bosh-libvirt-cpi/driver"
)

var _ driver.DomainBuilder = QEMUDomainBuilder{}

type QEMUDomainBuilder struct{}

func (b QEMUDomainBuilder) DiskImageFormat() string { return "qcow2" }

func (b QEMUDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
	network := props.Network
	if network == "" {
		network = "default"
	}
	macElem := ""
	if props.MAC != "" {
		macElem = fmt.Sprintf("\n      <mac address='%s'/>", xmlEscape(props.MAC))
	}
	xml := fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os><type arch='x86_64' machine='pc'>hvm</type></os>
  <features><acpi/><apic/></features>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vdb' bus='virtio'/>
    </disk>
    <interface type='network'>%s
      <source network='%s'/>
      <model type='virtio'/>
    </interface>
    <serial type='file'>
      <source path='/tmp/bosh-vm-%s-console.log'/>
      <target port='0'/>
    </serial>
    <console type='file'>
      <source path='/tmp/bosh-vm-%s-console.log'/>
      <target type='serial' port='0'/>
    </console>
  </devices>
</domain>`, xmlEscape(id), props.MemoryMB*1024, props.CPUs, xmlEscape(disks.RootDisk), xmlEscape(disks.EphemeralDisk), macElem, xmlEscape(network), xmlEscape(id), xmlEscape(id))
	return xml, nil
}

func (b QEMUDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
	xml := fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='KiB'>524288</memory>
  <vcpu>1</vcpu>
  <os><type arch='x86_64' machine='pc'>hvm</type></os>
  <features><acpi/><apic/></features>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <serial type='pty'><target port='0'/></serial>
    <console type='pty'><target type='serial' port='0'/></console>
  </devices>
</domain>`, xmlEscape(id), xmlEscape(imagePath))
	return xml, nil
}
