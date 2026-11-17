//go:build integration

package stemcell_test

import (
	. "github.com/onsi/ginkgo"
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

// skipUnlessLibvirtRequired skips a libvirt integration spec whose libvirt
// isn't configured or reachable, but fails it when REQUIRE_LIBVIRT is set
// (CI), so a dropped LIBVIRT_URI can't make the suite pass without testing.
func skipUnlessLibvirtRequired(reason string) {
	if os.Getenv("REQUIRE_LIBVIRT") != "" {
		Fail(reason + " (REQUIRE_LIBVIRT is set)")
	}
	Skip(reason)
}

// skipUnlessStemcellRequired skips a spec that needs STEMCELL_PATH, but fails
// it when REQUIRE_STEMCELL is set (CI), so it can't pass without running.
func skipUnlessStemcellRequired(reason string) {
	if os.Getenv("REQUIRE_STEMCELL") != "" {
		Fail(reason + " (REQUIRE_STEMCELL is set)")
	}
	Skip(reason)
}
