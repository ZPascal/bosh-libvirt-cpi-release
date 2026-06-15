//go:build integration

package driver_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
	libvirt "libvirt.org/go/libvirt"
)

var _ = Describe("LibvirtDriver (integration)", func() {
	var (
		d    driver.Driver
		conn *libvirt.Connect
		uri  string
	)

	BeforeEach(func() {
		uri = os.Getenv("LIBVIRT_URI")
		if uri == "" {
			Skip("LIBVIRT_URI not set")
		}

		var err error
		conn, err = libvirt.NewConnect(uri)
		if err != nil {
			Skip("libvirt connection unavailable: " + err.Error())
		}

		logger := boshlog.NewWriterLogger(boshlog.LevelDebug, os.Stderr)
		libvirtConn := driver.NewLibvirtConnImpl(conn)
		d = driver.NewLibvirtDriver(libvirtConn, domains.QEMUDomainBuilder{}, logger)
	})

	AfterEach(func() {
		if conn != nil {
			conn.Close()
		}
	})

	Describe("DefineDomain / LookupDomain / DestroyDomain", func() {
		It("defines, looks up, and destroys a domain", func() {
			xml := `<domain type='kvm'>
  <name>bosh-integration-test</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
</domain>`

			err := d.DefineDomain(xml)
			Expect(err).ToNot(HaveOccurred())

			dom, err := d.LookupDomain("bosh-integration-test")
			Expect(err).ToNot(HaveOccurred())
			Expect(dom).ToNot(BeNil())

			err = d.DestroyDomain("bosh-integration-test")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("IsMissingDomainErr", func() {
		It("returns true for a not-found error", func() {
			_, err := d.LookupDomain("does-not-exist-xyz")
			Expect(err).To(HaveOccurred())
			Expect(d.IsMissingDomainErr(err)).To(BeTrue())
		})

		It("returns false for nil", func() {
			Expect(d.IsMissingDomainErr(nil)).To(BeFalse())
		})
	})

	Describe("UpdateDomainMemory / UpdateDomainCPUs", func() {
		const updateTestXML = `<domain type='kvm'>
  <name>bosh-integration-update-test</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
</domain>`

		BeforeEach(func() {
			_ = d.DestroyDomain("bosh-integration-update-test")
			Expect(d.DefineDomain(updateTestXML)).To(Succeed())
		})

		AfterEach(func() {
			_ = d.DestroyDomain("bosh-integration-update-test")
		})

		It("updates memory on a defined (offline) domain", func() {
			err := d.UpdateDomainMemory("bosh-integration-update-test", 128)
			Expect(err).ToNot(HaveOccurred())
		})

		It("updates CPUs on a defined (offline) domain", func() {
			err := d.UpdateDomainCPUs("bosh-integration-update-test", 2)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
