package driver_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/fakes"
)

var _ = Describe("LibvirtDriver", func() {
	var (
		conn    *fakes.FakeLibvirtConn
		builder *fakes.FakeDomainBuilder
		d       driver.LibvirtDriver
		logger  boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
		conn = &fakes.FakeLibvirtConn{}
		builder = &fakes.FakeDomainBuilder{}
		d = driver.NewLibvirtDriver(conn, builder, logger)
	})

	Describe("DefineDomain", func() {
		It("calls DomainDefineXML with the provided XML", func() {
			err := d.DefineDomain("<domain/>")
			Expect(err).ToNot(HaveOccurred())
			Expect(conn.DomainDefineXMLArg).To(Equal("<domain/>"))
		})

		It("returns error when DomainDefineXML fails", func() {
			conn.DomainDefineXMLErr = errors.New("define failed")
			err := d.DefineDomain("<domain/>")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("LookupDomain", func() {
		It("returns error when LookupDomainByName fails", func() {
			conn.LookupDomainByNameErr = errors.New("domain not found")
			_, err := d.LookupDomain("missing")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("IsMissingDomainErr", func() {
		It("returns false for a generic error", func() {
			Expect(d.IsMissingDomainErr(errors.New("something else"))).To(BeFalse())
		})

		It("returns false for nil", func() {
			Expect(d.IsMissingDomainErr(nil)).To(BeFalse())
		})
	})

	Describe("StartDomain / ShutdownDomain / RebootDomain", func() {
		It("returns error when domain not found for Start", func() {
			conn.LookupDomainByNameErr = errors.New("not found")
			Expect(d.StartDomain("vm-1")).To(HaveOccurred())
		})

		It("returns error when domain not found for Shutdown", func() {
			conn.LookupDomainByNameErr = errors.New("not found")
			Expect(d.ShutdownDomain("vm-1")).To(HaveOccurred())
		})

		It("returns error when domain not found for Reboot", func() {
			conn.LookupDomainByNameErr = errors.New("not found")
			Expect(d.RebootDomain("vm-1")).To(HaveOccurred())
		})
	})

	Describe("UpdateDomainMemory / UpdateDomainCPUs", func() {
		It("returns error when domain not found for UpdateDomainMemory", func() {
			conn.LookupDomainByNameErr = errors.New("not found")
			Expect(d.UpdateDomainMemory("vm-1", 512)).To(HaveOccurred())
		})

		It("returns error when domain not found for UpdateDomainCPUs", func() {
			conn.LookupDomainByNameErr = errors.New("not found")
			Expect(d.UpdateDomainCPUs("vm-1", 2)).To(HaveOccurred())
		})
	})

	Describe("CreateStorageVol", func() {
		It("returns error when pool not found", func() {
			conn.LookupStoragePoolByNameErr = errors.New("pool not found")
			_, err := d.CreateStorageVol("default", "vol-1", 100)
			Expect(err).To(HaveOccurred())
		})
	})
})
