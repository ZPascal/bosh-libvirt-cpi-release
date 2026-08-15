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

	// Direct kernel boot: mount the stemcell rootfs directory via virtio-9p
	// and exec bosh-agent directly, bypassing disk boot.
	if props.Kernel != "" {
		xml := fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os>
    <type arch='x86_64' machine='pc'>hvm</type>
    <kernel>%s</kernel>
    <cmdline>root=rootfs rw rootfstype=9p rootflags=trans=virtio,version=9p2000.L console=ttyS0 init=/var/vcap/bosh/bin/bosh-agent -- -C /var/vcap/bosh/agent.json -P warden</cmdline>
  </os>
  <features><acpi/><apic/></features>
  <devices>
    <filesystem type='mount' accessmode='passthrough'>
      <source dir='%s'/>
      <target dir='rootfs'/>
    </filesystem>
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
</domain>`, xmlEscape(id), props.MemoryMB*1024, props.CPUs,
			xmlEscape(props.Kernel),
			xmlEscape(disks.RootDisk), xmlEscape(disks.EphemeralDisk),
			macElem, xmlEscape(network), xmlEscape(id), xmlEscape(id))
		return xml, nil
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

// QEMUKernelDomainBuilder is like QEMUDomainBuilder but uses direct kernel
// boot with the stemcell extracted as a directory (virtio-9p rootfs).
// The kernel path is supplied per-VM via VMDomainProps.Kernel.
type QEMUKernelDomainBuilder struct{}

var _ driver.DomainBuilder = QEMUKernelDomainBuilder{}

// DiskImageFormat returns "dir" so the stemcell factory extracts the rootfs
// tarball into a directory instead of converting it to qcow2.
func (b QEMUKernelDomainBuilder) DiskImageFormat() string { return "dir" }

func (b QEMUKernelDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
	return QEMUDomainBuilder{}.BuildDomain(id, props, disks)
}

func (b QEMUKernelDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
	return QEMUDomainBuilder{}.BuildStemcellDomain(id, imagePath)
}
