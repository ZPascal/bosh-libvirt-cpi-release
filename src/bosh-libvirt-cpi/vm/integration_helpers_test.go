//go:build integration

package vm_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"net/url"
	"os"
	"path/filepath"

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

// stemcellForImport returns a path to STEMCELL_PATH that the spec may hand to
// ImportFromPath. Importing moves (renames) the tarball into the store, so each
// spec gets its own hard link (or copy) and STEMCELL_PATH stays usable for the
// next spec.
func stemcellForImport(dir string) string {
	src := os.Getenv("STEMCELL_PATH")
	dst := filepath.Join(dir, "stemcell-import.tgz")
	if err := os.Link(src, dst); err == nil {
		return dst
	}
	in, err := os.Open(src)
	Expect(err).ToNot(HaveOccurred())
	defer in.Close()
	out, err := os.Create(dst)
	Expect(err).ToNot(HaveOccurred())
	defer out.Close()
	_, err = io.Copy(out, in)
	Expect(err).ToNot(HaveOccurred())
	return dst
}
