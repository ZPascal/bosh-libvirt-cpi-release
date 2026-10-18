//go:build integration

package driver_test

import (
	"fmt"
	"net/url"
	"os"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

func domBuilderFromEnv() driver.DomainBuilder {
	uri := os.Getenv("LIBVIRT_URI")
	u, _ := url.Parse(uri)
	if u == nil {
		return domains.QEMUDomainBuilder{}
	}
	switch u.Scheme {
	case "lxc":
		return domains.LXCDomainBuilder{}
	case "vbox":
		return domains.VBoxDomainBuilder{}
	default:
		return domains.QEMUDomainBuilder{}
	}
}

func testDomainXML(name string) string {
	uri := os.Getenv("LIBVIRT_URI")
	u, _ := url.Parse(uri)
	if u == nil {
		u = &url.URL{}
	}
	switch u.Scheme {
	case "lxc":
		return fmt.Sprintf(`<domain type='lxc'>
  <name>%s</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type>exe</type><init>/sbin/init</init></os>
  <devices><emulator>/usr/lib/libvirt/libvirt_lxc</emulator></devices>
</domain>`, name)
	case "vbox":
		return fmt.Sprintf(`<domain type='vbox'>
  <name>%s</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type>hvm</type></os>
</domain>`, name)
	default:
		return fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
</domain>`, name)
	}
}
