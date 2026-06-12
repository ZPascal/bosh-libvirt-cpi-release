package domains_test

import (
	"encoding/xml"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

var _ = Describe("QEMUDomainBuilder", func() {
	var builder domains.QEMUDomainBuilder

	BeforeEach(func() {
		builder = domains.QEMUDomainBuilder{}
	})

	It("returns qcow2 as disk format", func() {
		Expect(builder.DiskImageFormat()).To(Equal("qcow2"))
	})

	Describe("BuildDomain", func() {
		It("contains domain name, bus, and disk paths", func() {
			xml, err := builder.BuildDomain("vm-kvm-1", driver.VMDomainProps{CPUs: 4, MemoryMB: 2048},
				driver.DomainDiskPaths{RootDisk: "/root.qcow2", EphemeralDisk: "/eph.qcow2"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("vm-kvm-1"))
			Expect(xml).To(ContainSubstring("virtio"))
			Expect(xml).To(ContainSubstring("/root.qcow2"))
			Expect(xml).To(ContainSubstring("/eph.qcow2"))
		})

		It("includes a network interface using the default network when Network is empty", func() {
			xml, err := builder.BuildDomain("vm-kvm-net", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/r.qcow2", EphemeralDisk: "/e.qcow2"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("<interface"))
			Expect(xml).To(ContainSubstring("network='default'"))
		})

		It("uses the configured network name when Network is set", func() {
			xml, err := builder.BuildDomain("vm-kvm-net2", driver.VMDomainProps{CPUs: 1, MemoryMB: 512, Network: "bosh"},
				driver.DomainDiskPaths{RootDisk: "/r.qcow2", EphemeralDisk: "/e.qcow2"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("network='bosh'"))
		})

		It("uses kvm domain type", func() {
			xml, err := builder.BuildDomain("vm-kvm-2", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/r.qcow2", EphemeralDisk: "/e.qcow2"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("type='kvm'"))
		})

		It("encodes memory as KiB", func() {
			xml, err := builder.BuildDomain("vm-kvm-3", driver.VMDomainProps{CPUs: 2, MemoryMB: 2048},
				driver.DomainDiskPaths{RootDisk: "/r.qcow2", EphemeralDisk: "/e.qcow2"})
			Expect(err).To(BeNil())
			// 2048 MB * 1024 = 2097152 KiB
			Expect(xml).To(ContainSubstring("2097152"))
		})

		It("specifies qcow2 disk driver type", func() {
			xml, err := builder.BuildDomain("vm-kvm-4", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/r.qcow2", EphemeralDisk: "/e.qcow2"})
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("type='qcow2'"))
		})

		It("escapes XML special characters in id and disk paths", func() {
			result, err := builder.BuildDomain("vm&<1>", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
				driver.DomainDiskPaths{RootDisk: "/path&root.qcow2", EphemeralDisk: "/path&eph.qcow2"})
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("vm&amp;&lt;1&gt;"))
			Expect(result).To(ContainSubstring("/path&amp;root.qcow2"))
			Expect(result).To(ContainSubstring("/path&amp;eph.qcow2"))
			var v interface{}
			Expect(xml.NewDecoder(strings.NewReader(result)).Decode(&v)).To(Succeed())
		})
	})

	Describe("BuildStemcellDomain", func() {
		It("contains stemcell name and image path", func() {
			xml, err := builder.BuildStemcellDomain("sc-kvm-1", "/image.qcow2")
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("sc-kvm-1"))
			Expect(xml).To(ContainSubstring("/image.qcow2"))
		})

		It("uses kvm domain type", func() {
			xml, err := builder.BuildStemcellDomain("sc-kvm-2", "/img.qcow2")
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("type='kvm'"))
		})
	})
})
