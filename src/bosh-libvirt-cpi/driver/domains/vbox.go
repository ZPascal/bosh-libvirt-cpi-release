package domains

import (
	"fmt"

	"bosh-libvirt-cpi/driver"
)

var _ driver.DomainBuilder = VBoxDomainBuilder{}

type VBoxDomainBuilder struct{}

func (b VBoxDomainBuilder) DiskImageFormat() string { return "vmdk" }

func (b VBoxDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
	xml := fmt.Sprintf(`<domain type='vbox'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os><type>hvm</type></os>
  <devices>
    <disk type='file' device='disk'>
      <source file='%s'/>
      <target dev='sda' bus='ide'/>
    </disk>
    <disk type='file' device='disk'>
      <source file='%s'/>
      <target dev='sdb' bus='ide'/>
    </disk>
    <interface type='network'>
      <source network='default'/>
    </interface>
  </devices>
</domain>`, xmlEscape(id), props.MemoryMB*1024, props.CPUs, xmlEscape(disks.RootDisk), xmlEscape(disks.EphemeralDisk))
	return xml, nil
}

func (b VBoxDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
	xml := fmt.Sprintf(`<domain type='vbox'>
  <name>%s</name>
  <memory unit='KiB'>524288</memory>
  <vcpu>1</vcpu>
  <os><type>hvm</type></os>
  <devices>
    <disk type='file' device='disk'>
      <source file='%s'/>
      <target dev='sda' bus='ide'/>
    </disk>
  </devices>
</domain>`, xmlEscape(id), xmlEscape(imagePath))
	return xml, nil
}
