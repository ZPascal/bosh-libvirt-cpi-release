package driver_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	libvirt "libvirt.org/go/libvirt"

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
		It("returns true for a libvirt ERR_NO_DOMAIN error", func() {
			err := libvirt.Error{Code: libvirt.ERR_NO_DOMAIN}
			Expect(d.IsMissingDomainErr(err)).To(BeTrue())
		})

		It("returns false for a generic error", func() {
			Expect(d.IsMissingDomainErr(errors.New("something else"))).To(BeFalse())
		})

		It("returns false for nil", func() {
			Expect(d.IsMissingDomainErr(nil)).To(BeFalse())
		})
	})

	Describe("DestroyDomain", func() {
		It("returns error when domain not found", func() {
			conn.LookupDomainByNameErr = errors.New("not found")
			Expect(d.DestroyDomain("vm-1")).To(HaveOccurred())
		})

		It("returns error when lookup returns nil domain with no error", func() {
			// FakeLibvirtConn returns (nil, nil) by default
			Expect(d.DestroyDomain("vm-1")).To(HaveOccurred())
		})

		Context("when libvirtd drops the connection", func() {
			connLost := libvirt.Error{Code: libvirt.ERR_SYSTEM_ERROR, Domain: libvirt.FROM_RPC}

			It("reconnects and treats a now-missing domain as success", func() {
				conn.LookupDomainByNameErr = connLost
				conn.OnReconnect = func() {
					conn.LookupDomainByNameErr = libvirt.Error{Code: libvirt.ERR_NO_DOMAIN}
				}
				Expect(d.DestroyDomain("vm-1")).To(Succeed())
				Expect(conn.ReconnectCalls).To(Equal(1))
			})

			It("gives up after 3 failed reconnects", func() {
				conn.LookupDomainByNameErr = connLost
				conn.ReconnectErr = errors.New("still down")
				Expect(d.DestroyDomain("vm-1")).To(HaveOccurred())
				Expect(conn.ReconnectCalls).To(Equal(3))
			})

			It("does not reconnect for other libvirt errors", func() {
				conn.LookupDomainByNameErr = libvirt.Error{Code: libvirt.ERR_INTERNAL_ERROR, Domain: libvirt.FROM_RPC}
				Expect(d.DestroyDomain("vm-1")).To(HaveOccurred())
				Expect(conn.ReconnectCalls).To(Equal(0))
			})
		})
	})

	Describe("DeleteStorageVol", func() {
		It("returns nil when pool not found (idempotent)", func() {
			conn.LookupStoragePoolByNameErr = libvirt.Error{Code: libvirt.ERR_NO_STORAGE_POOL}
			Expect(d.DeleteStorageVol("default", "vol-1")).To(Succeed())
		})

		It("returns error for non-pool-not-found pool lookup failure", func() {
			conn.LookupStoragePoolByNameErr = errors.New("unexpected pool error")
			Expect(d.DeleteStorageVol("default", "vol-1")).To(HaveOccurred())
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

		It("returns error when lookup returns nil domain with no error", func() {
			// conn returns (nil, nil) by default - withDomain must guard against nil
			Expect(d.StartDomain("vm-1")).To(HaveOccurred())
		})

		It("returns error when lookup returns nil domain with no error for Shutdown", func() {
			// conn returns (nil, nil) by default - withDomain must guard against nil
			Expect(d.ShutdownDomain("vm-1")).To(HaveOccurred())
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

		It("returns error when lookup returns nil domain with no error for UpdateDomainMemory", func() {
			Expect(d.UpdateDomainMemory("vm-1", 512)).To(HaveOccurred())
		})

		It("returns error when lookup returns nil domain with no error for UpdateDomainCPUs", func() {
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
