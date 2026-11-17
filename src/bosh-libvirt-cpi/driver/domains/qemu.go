package domains

import (
	"fmt"
	"strings"

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

	if props.Arch == "aarch64" {
		return aarch64Domain(id, props, disks, macElem, network)
	}

	// Direct kernel boot: boot the given kernel with the ext4 rootfs image on
	// /dev/vda and init=/bosh-init, skipping the disk bootloader.
	if props.Kernel != "" {
		xml := fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os>
    <type arch='x86_64' machine='pc'>hvm</type>
    <kernel>%s</kernel>
    <cmdline>root=/dev/vda rw console=ttyS0 init=/bosh-init</cmdline>
  </os>
  <features><acpi/><apic/></features>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='disk'>
      <driver name='qemu' type='%s'/>
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
			xmlEscape(disks.RootDisk), xmlEscape(ephemeralFormat(disks)), xmlEscape(disks.EphemeralDisk),
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
      <driver name='qemu' type='%s'/>
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
</domain>`, xmlEscape(id), props.MemoryMB*1024, props.CPUs, xmlEscape(disks.RootDisk), xmlEscape(ephemeralFormat(disks)), xmlEscape(disks.EphemeralDisk), macElem, xmlEscape(network), xmlEscape(id), xmlEscape(id))
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
// boot: the stemcell is built as an ext4 image that the kernel mounts as root
// on /dev/vda. The kernel path is supplied per-VM via VMDomainProps.Kernel.
type QEMUKernelDomainBuilder struct{}

var _ driver.DomainBuilder = QEMUKernelDomainBuilder{}

// DiskImageFormat returns "ext4" so the stemcell factory creates an ext4
// disk image from the rootfs tarball that the kernel can mount as root.
func (b QEMUKernelDomainBuilder) DiskImageFormat() string { return "ext4" }

func (b QEMUKernelDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
	return QEMUDomainBuilder{}.BuildDomain(id, props, disks)
}

func (b QEMUKernelDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
	return QEMUDomainBuilder{}.BuildStemcellDomain(id, imagePath)
}

// qemuPersistentDiskXML is the hot-pluggable virtio disk for a persistent disk
// image. The serial shows up in the guest as /dev/disk/by-id/virtio-<serial>.
func qemuPersistentDiskXML(imagePath, format, target, serial string) string {
	if format == "" {
		format = "qcow2"
	}
	return fmt.Sprintf(`<disk type='file' device='disk'>
  <driver name='qemu' type='%s'/>
  <source file='%s'/>
  <target dev='%s' bus='virtio'/>
  <serial>%s</serial>
</disk>`, xmlEscape(format), xmlEscape(imagePath), xmlEscape(target), xmlEscape(serial))
}

func (b QEMUDomainBuilder) BuildPersistentDiskXML(imagePath, format, target, serial string) (string, error) {
	return qemuPersistentDiskXML(imagePath, format, target, serial), nil
}

func (b QEMUKernelDomainBuilder) BuildPersistentDiskXML(imagePath, format, target, serial string) (string, error) {
	return qemuPersistentDiskXML(imagePath, format, target, serial), nil
}

// ephemeralFormat is the libvirt driver type for the ephemeral disk image:
// qcow2 unless disk.Factory fell back to a raw dd image.
func ephemeralFormat(disks driver.DomainDiskPaths) string {
	if disks.EphemeralFormat == "" {
		return "qcow2"
	}
	return disks.EphemeralFormat
}

// aarch64RootPorts is the number of extra PCIe root ports on aarch64 (virt)
// domains: virt has no conventional PCI bus, so every hot-plugged persistent
// disk needs a free root port.
const aarch64RootPorts = 8

// aarch64Domain is an aarch64 "virt" machine with direct kernel boot. On an
// aarch64 host it runs under KVM with the host CPU, elsewhere it is emulated
// (TCG). There is no <acpi/>: direct kernel boot on virt uses a device tree.
func aarch64Domain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths, macElem, network string) (string, error) {
	if props.Kernel == "" {
		return "", fmt.Errorf("aarch64 guests need direct kernel boot (cloud_properties.kernel); disk boot on the virt machine would need UEFI firmware")
	}

	domType, cpu := "qemu", "<cpu mode='maximum'/>"
	if props.HostArch == "aarch64" {
		domType, cpu = "kvm", "<cpu mode='host-passthrough'/>"
	}

	rootPorts := strings.Repeat("\n    <controller type='pci' model='pcie-root-port'/>", aarch64RootPorts)

	return fmt.Sprintf(`<domain type='%s'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os>
    <type arch='aarch64' machine='virt'>hvm</type>
    <kernel>%s</kernel>
    <cmdline>root=/dev/vda rw console=ttyAMA0 init=/bosh-init</cmdline>
  </os>
  %s
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='disk'>
      <driver name='qemu' type='%s'/>
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
    </console>%s
  </devices>
</domain>`, domType, xmlEscape(id), props.MemoryMB*1024, props.CPUs,
		xmlEscape(props.Kernel), cpu,
		xmlEscape(disks.RootDisk), xmlEscape(ephemeralFormat(disks)), xmlEscape(disks.EphemeralDisk),
		macElem, xmlEscape(network), xmlEscape(id), xmlEscape(id), rootPorts), nil
}
