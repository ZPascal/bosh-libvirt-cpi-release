//go:build integration

package vm_test

import (
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
