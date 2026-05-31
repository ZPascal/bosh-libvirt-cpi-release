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

	It("connects and can define/start/lookup/shutdown/destroy a domain", func() {
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
