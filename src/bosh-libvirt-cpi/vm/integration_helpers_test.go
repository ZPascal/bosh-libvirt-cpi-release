//go:build integration

package vm_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	libvirt "libvirt.org/go/libvirt"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
	"bosh-libvirt-cpi/stemcell"
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

// stemcellForImport returns what the director would hand the CPI's
// create_stemcell for STEMCELL_PATH: the stemcell tarball's `image` member
// (for warden-tar stemcells, the rootfs tarball), extracted into dir. Each spec
// gets its own copy because importing moves (renames) it into the store.
func stemcellForImport(dir string) string {
	in, err := os.Open(os.Getenv("STEMCELL_PATH"))
	Expect(err).ToNot(HaveOccurred())
	defer in.Close()
	gz, err := gzip.NewReader(in)
	Expect(err).ToNot(HaveOccurred())
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		Expect(err).ToNot(HaveOccurred(), "no image in STEMCELL_PATH")
		if strings.TrimPrefix(hdr.Name, "./") != "image" {
			continue
		}
		dst := filepath.Join(dir, "stemcell-image.tgz")
		out, err := os.Create(dst)
		Expect(err).ToNot(HaveOccurred())
		defer out.Close()
		_, err = io.Copy(out, tr) // #nosec G110 -- trusted test input
		Expect(err).ToNot(HaveOccurred())
		return dst
	}
}

// stemcellArchFromEnv is the architecture to import STEMCELL_PATH with:
// STEMCELL_ARCH (x86_64, aarch64 or arm64), x86_64 when unset.
func stemcellArchFromEnv() string {
	if arch := os.Getenv("STEMCELL_ARCH"); arch != "" {
		return stemcell.NormalizeArch(arch)
	}
	return stemcell.ArchX86_64
}

// dumpLXCGuest prints what explains an LXC guest whose agent never logged:
// the domain state, libvirt's log for the container, the guest's init and
// agent logs, and the host's view of the container processes.
func dumpLXCGuest(conn *libvirt.Connect, name, rootfs string) {
	w := GinkgoWriter
	if dom, err := conn.LookupDomainByName(name); err == nil {
		state, reason, err := dom.GetState()
		fmt.Fprintf(w, "domain %s: state=%d reason=%d err=%v\n", name, state, reason, err)
		_ = dom.Free()
	} else {
		fmt.Fprintf(w, "domain %s: lookup failed: %v\n", name, err)
	}
	for _, f := range []string{
		"/var/log/libvirt/lxc/" + name + ".log",
		filepath.Join(rootfs, "tmp", "dhclient.log"),
	} {
		b, err := os.ReadFile(f)
		fmt.Fprintf(w, "--- %s (err=%v)\n%s\n", f, err, b)
	}
	for _, args := range [][]string{
		{"ls", "-la", filepath.Join(rootfs, "var", "vcap", "bosh", "log"), filepath.Join(rootfs, "bosh-lxc-init")},
		{"head", "-c", "2000", filepath.Join(rootfs, "bosh-lxc-init")},
		{"pgrep", "-a", "-f", "bosh-agent|bosh-lxc-init|dhclient"},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		fmt.Fprintf(w, "--- %s (err=%v)\n%s\n", strings.Join(args, " "), err, out)
	}
}
