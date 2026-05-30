package domains_test

import (
	"encoding/xml"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

var _ = Describe("LXCDomainBuilder", func() {
	var builder domains.LXCDomainBuilder

	BeforeEach(func() {
		builder = domains.LXCDomainBuilder{}
	})

	It("returns raw as disk format", func() {
		Expect(builder.DiskImageFormat()).To(Equal("raw"))
	})

	It("returns lxc as storage controller", func() {
		Expect(builder.StorageController()).To(Equal("lxc"))
	})

	Describe("BuildDomain", func() {
		It("contains domain name and disk paths", func() {
			xml, err := builder.BuildDomain("vm-lxc-1", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/root.raw", EphemeralDisk: "/eph.raw"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("vm-lxc-1"))
			Expect(xml).To(ContainSubstring("/root.raw"))
			Expect(xml).To(ContainSubstring("/eph.raw"))
		})

		It("uses lxc domain type", func() {
			xml, err := builder.BuildDomain("vm-lxc-2", driver.VMDomainProps{CPUs: 1, MemoryMB: 256},
				driver.DomainDiskPaths{RootDisk: "/r.raw", EphemeralDisk: "/e.raw"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("type='lxc'"))
		})

		It("encodes memory as KiB", func() {
			xml, err := builder.BuildDomain("vm-lxc-3", driver.VMDomainProps{CPUs: 2, MemoryMB: 1024},
				driver.DomainDiskPaths{RootDisk: "/r.raw", EphemeralDisk: "/e.raw"})
			Expect(err).To(BeNil())
			// 1024 MB * 1024 = 1048576 KiB
			Expect(xml).To(ContainSubstring("1048576"))
		})

		It("escapes XML special characters in id and disk paths", func() {
			result, err := builder.BuildDomain("vm&<lxc>", driver.VMDomainProps{CPUs: 1, MemoryMB: 256},
				driver.DomainDiskPaths{RootDisk: "/path&root.raw", EphemeralDisk: "/path&eph.raw"})
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("vm&amp;&lt;lxc&gt;"))
			Expect(result).To(ContainSubstring("/path&amp;root.raw"))
			Expect(result).To(ContainSubstring("/path&amp;eph.raw"))
			var v interface{}
			Expect(xml.NewDecoder(strings.NewReader(result)).Decode(&v)).To(Succeed())
		})
	})

	Describe("BuildStemcellDomain", func() {
		It("contains stemcell name and image path", func() {
			xml, err := builder.BuildStemcellDomain("sc-lxc-1", "/image.raw")
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("sc-lxc-1"))
			Expect(xml).To(ContainSubstring("/image.raw"))
		})

		It("uses lxc domain type", func() {
			xml, err := builder.BuildStemcellDomain("sc-lxc-2", "/img.raw")
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("type='lxc'"))
		})
	})
})
