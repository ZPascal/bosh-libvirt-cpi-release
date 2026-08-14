package domains

import (
	"fmt"

	"bosh-libvirt-cpi/driver"
)

var _ driver.DomainBuilder = LXCDomainBuilder{}

type LXCDomainBuilder struct{}

func (b LXCDomainBuilder) DiskImageFormat() string { return "dir" }

func (b LXCDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
	network := props.Network
	if network == "" {
		network = "default"
	}
	xml := fmt.Sprintf(`<domain type='lxc'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os><type>exe</type><init>/sbin/init</init></os>
  <devices>
    <filesystem type='mount'>
      <source dir='%s'/>
      <target dir='/'/>
    </filesystem>
    <filesystem type='mount'>
      <source dir='%s'/>
      <target dir='/mnt/ephemeral'/>
    </filesystem>
    <interface type='network'>
      <source network='%s'/>
    </interface>
    <console type='pty'/>
  </devices>
</domain>`, xmlEscape(id), props.MemoryMB*1024, props.CPUs, xmlEscape(disks.RootDisk), xmlEscape(disks.EphemeralDisk), xmlEscape(network))
	return xml, nil
}

func (b LXCDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
	xml := fmt.Sprintf(`<domain type='lxc'>
  <name>%s</name>
  <memory unit='KiB'>524288</memory>
  <vcpu>1</vcpu>
  <os><type>exe</type><init>/sbin/init</init></os>
  <devices>
    <filesystem type='mount'>
      <source dir='%s'/>
      <target dir='/'/>
    </filesystem>
  </devices>
</domain>`, xmlEscape(id), xmlEscape(imagePath))
	return xml, nil
}
