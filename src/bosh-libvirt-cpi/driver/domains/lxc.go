package domains

import (
	"fmt"

	"bosh-libvirt-cpi/driver"
)

var _ driver.DomainBuilder = LXCDomainBuilder{}

type LXCDomainBuilder struct{}

func (b LXCDomainBuilder) DiskImageFormat() string   { return "raw" }
func (b LXCDomainBuilder) StorageController() string { return "lxc" }

func (b LXCDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
	xml := fmt.Sprintf(`<domain type='lxc'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os><type>exe</type><init>/sbin/init</init></os>
  <devices>
    <filesystem type='ram'>
      <source usage='1048576'/>
      <target dir='/'/>
    </filesystem>
    <filesystem type='file'>
      <source file='%s'/>
      <target dir='/mnt/root'/>
    </filesystem>
    <filesystem type='file'>
      <source file='%s'/>
      <target dir='/mnt/ephemeral'/>
    </filesystem>
  </devices>
</domain>`, xmlEscape(id), props.MemoryMB*1024, props.CPUs, xmlEscape(disks.RootDisk), xmlEscape(disks.EphemeralDisk))
	return xml, nil
}

func (b LXCDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
	xml := fmt.Sprintf(`<domain type='lxc'>
  <name>%s</name>
  <memory unit='KiB'>524288</memory>
  <vcpu>1</vcpu>
  <os><type>exe</type><init>/sbin/init</init></os>
  <devices>
    <filesystem type='file'>
      <source file='%s'/>
      <target dir='/mnt/root'/>
    </filesystem>
  </devices>
</domain>`, xmlEscape(id), xmlEscape(imagePath))
	return xml, nil
}
