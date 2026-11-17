//go:build integration_aarch64

package driver_test

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	libvirt "libvirt.org/go/libvirt"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

// Boots an aarch64 guest with the CPI's own domain builder and driver (TCG on
// an x86_64 host, KVM on an aarch64 host) and hot-plugs a persistent disk.
// Inputs come from .github/scripts/aarch64-boot-fixture.sh via AARCH64_DIR.
var _ = Describe("aarch64 guest boot (integration)", func() {
	const id = "bosh-aarch64-boot-test"
	var (
		d       driver.Driver
		conn    *libvirt.Connect
		dir     string
		console string
	)

	consoleLog := func() string {
		b, _ := os.ReadFile(console)
		return string(b)
	}

	BeforeEach(func() {
		dir = os.Getenv("AARCH64_DIR")
		Expect(dir).ToNot(BeEmpty(), "AARCH64_DIR must point at the fixture (aarch64-boot-fixture.sh)")
		uri := os.Getenv("LIBVIRT_URI")
		if uri == "" {
			uri = "qemu:///system"
		}

		var err error
		conn, err = libvirt.NewConnect(uri)
		Expect(err).ToNot(HaveOccurred())
		builder := domains.QEMUKernelDomainBuilder{}
		d = driver.NewLibvirtDriver(driver.NewLibvirtConnImpl(conn), builder, boshlog.NewWriterLogger(boshlog.LevelDebug, os.Stderr))

		console = "/tmp/bosh-vm-" + id + "-console.log"
		_ = os.Remove(console)
		_ = d.DestroyDomain(id)
	})

	AfterEach(func() {
		GinkgoWriter.Write([]byte("=== guest console ===\n" + consoleLog()))
		_ = d.DestroyDomain(id)
		if conn != nil {
			conn.Close()
		}
	})

	It("boots, runs /bosh-init and sees a hot-plugged persistent disk", func() {
		hostArch, err := d.HostArch()
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Write([]byte("libvirt host arch: " + hostArch + "\n"))

		xml, err := domains.QEMUKernelDomainBuilder{}.BuildDomain(id, driver.VMDomainProps{
			CPUs: 2, MemoryMB: 1024, Network: "default",
			Kernel: filepath.Join(dir, "vmlinuz"), Arch: "aarch64", HostArch: hostArch,
		}, driver.DomainDiskPaths{
			RootDisk: filepath.Join(dir, "rootfs.img"), EphemeralDisk: filepath.Join(dir, "ephemeral.qcow2"),
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(d.DefineDomain(xml)).To(Succeed())

		started := time.Now()
		Expect(d.StartDomain(id)).To(Succeed())
		Eventually(consoleLog, 15*time.Minute, 5*time.Second).Should(ContainSubstring("AARCH64_INIT_OK aarch64"))
		GinkgoWriter.Write([]byte("boot to /bosh-init: " + time.Since(started).String() + "\n"))
		Eventually(consoleLog, time.Minute, 3*time.Second).Should(MatchRegexp(`DISKS: /dev/vda /dev/vdb\s`))

		disk, err := domains.QEMUKernelDomainBuilder{}.BuildPersistentDiskXML(filepath.Join(dir, "persistent.qcow2"), "qcow2", "vdc", "aarch64-test-disk-01")
		Expect(err).ToNot(HaveOccurred())
		plugged := time.Now()
		Expect(d.AttachDevice(id, disk)).To(Succeed())
		Eventually(consoleLog, 3*time.Minute, 3*time.Second).Should(ContainSubstring("/dev/vdc"))
		GinkgoWriter.Write([]byte("hot-plug to guest /dev/vdc: " + time.Since(plugged).String() + "\n"))

		Expect(d.DetachDevice(id, disk)).To(Succeed())
	})
})
