package domains_test

import (
	"encoding/xml"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

var _ = Describe("VBoxDomainBuilder", func() {
	var builder domains.VBoxDomainBuilder

	BeforeEach(func() {
		builder = domains.VBoxDomainBuilder{}
	})

	It("returns vmdk as disk format", func() {
		Expect(builder.DiskImageFormat()).To(Equal("vmdk"))
	})

	Describe("BuildDomain", func() {
		It("contains domain name, controller, and disk paths", func() {
			xml, err := builder.BuildDomain("vm-123", driver.VMDomainProps{CPUs: 2, MemoryMB: 1024},
				driver.DomainDiskPaths{RootDisk: "/root.vmdk", EphemeralDisk: "/eph.vmdk"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("vm-123"))
			Expect(xml).To(ContainSubstring("ide"))
			Expect(xml).To(ContainSubstring("/root.vmdk"))
			Expect(xml).To(ContainSubstring("/eph.vmdk"))
		})

		It("includes a network interface using the default network when Network is empty", func() {
			xml, err := builder.BuildDomain("vm-vbox-net", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/r.vmdk", EphemeralDisk: "/e.vmdk"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("<interface"))
			Expect(xml).To(ContainSubstring("network='default'"))
		})

		It("uses the configured network name when Network is set", func() {
			xml, err := builder.BuildDomain("vm-vbox-net2", driver.VMDomainProps{CPUs: 1, MemoryMB: 512, Network: "bosh"},
				driver.DomainDiskPaths{RootDisk: "/r.vmdk", EphemeralDisk: "/e.vmdk"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("network='bosh'"))
		})

		It("does not use vboxsf driver", func() {
			xml, err := builder.BuildDomain("vm-vbox-drv", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/r.vmdk", EphemeralDisk: "/e.vmdk"})
			Expect(err).To(BeNil())
			Expect(xml).ToNot(ContainSubstring("vboxsf"))
		})

		It("encodes memory as KiB", func() {
			xml, err := builder.BuildDomain("vm-mem", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/r.vmdk", EphemeralDisk: "/e.vmdk"})
			Expect(err).To(BeNil())
			// 512 MB * 1024 = 524288 KiB
			Expect(xml).To(ContainSubstring("524288"))
		})

		It("escapes XML special characters in id and disk paths", func() {
			result, err := builder.BuildDomain("vm&<vbox>", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/path&root.vmdk", EphemeralDisk: "/path&eph.vmdk"})
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("vm&amp;&lt;vbox&gt;"))
			Expect(result).To(ContainSubstring("/path&amp;root.vmdk"))
			Expect(result).To(ContainSubstring("/path&amp;eph.vmdk"))
			var v interface{}
			Expect(xml.NewDecoder(strings.NewReader(result)).Decode(&v)).To(Succeed())
		})
	})

	Describe("BuildStemcellDomain", func() {
		It("contains stemcell name and image path", func() {
			xml, err := builder.BuildStemcellDomain("sc-123", "/image.vmdk")
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("sc-123"))
			Expect(xml).To(ContainSubstring("/image.vmdk"))
		})

		It("uses vbox domain type", func() {
			xml, err := builder.BuildStemcellDomain("sc-vbox", "/img.vmdk")
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("type='vbox'"))
		})
	})
})
