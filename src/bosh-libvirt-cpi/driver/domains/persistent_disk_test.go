package domains_test

import (
	"encoding/xml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

type diskDeviceXML struct {
	XMLName xml.Name `xml:"disk"`
	Type    string   `xml:"type,attr"`
	Device  string   `xml:"device,attr"`
	Driver  struct {
		Name string `xml:"name,attr"`
		Type string `xml:"type,attr"`
	} `xml:"driver"`
	Source struct {
		File string `xml:"file,attr"`
	} `xml:"source"`
	Target struct {
		Dev string `xml:"dev,attr"`
		Bus string `xml:"bus,attr"`
	} `xml:"target"`
	Serial string `xml:"serial"`
}

var _ = Describe("BuildPersistentDiskXML", func() {
	for name, b := range map[string]driver.DomainBuilder{
		"qcow2 boot":  domains.QEMUDomainBuilder{},
		"kernel boot": domains.QEMUKernelDomainBuilder{},
	} {
		builder := b
		It("produces a hot-pluggable virtio disk for QEMU "+name, func() {
			out, err := builder.BuildPersistentDiskXML("/store/disk-1/disk.img", "qcow2", "vdc", "disk-0123456789abcde")
			Expect(err).ToNot(HaveOccurred())

			var d diskDeviceXML
			Expect(xml.Unmarshal([]byte(out), &d)).To(Succeed())
			Expect(d.Type).To(Equal("file"))
			Expect(d.Device).To(Equal("disk"))
			Expect(d.Driver.Name).To(Equal("qemu"))
			Expect(d.Driver.Type).To(Equal("qcow2"))
			Expect(d.Source.File).To(Equal("/store/disk-1/disk.img"))
			Expect(d.Target.Dev).To(Equal("vdc"))
			Expect(d.Target.Bus).To(Equal("virtio"))
			Expect(d.Serial).To(Equal("disk-0123456789abcde"))
		})
	}

	It("uses the raw driver for raw images (dd fallback)", func() {
		out, err := domains.QEMUKernelDomainBuilder{}.BuildPersistentDiskXML("/p/disk.img", "raw", "vdc", "disk-0123456789abcde")
		Expect(err).ToNot(HaveOccurred())

		var d diskDeviceXML
		Expect(xml.Unmarshal([]byte(out), &d)).To(Succeed())
		Expect(d.Driver.Type).To(Equal("raw"))
	})

	It("escapes XML special characters in the image path", func() {
		out, err := domains.QEMUDomainBuilder{}.BuildPersistentDiskXML("/a'b&c/disk.img", "qcow2", "vdd", "short-id")
		Expect(err).ToNot(HaveOccurred())

		var d diskDeviceXML
		Expect(xml.Unmarshal([]byte(out), &d)).To(Succeed())
		Expect(d.Source.File).To(Equal("/a'b&c/disk.img"))
		Expect(d.Serial).To(Equal("short-id"))
	})

	for name, b := range map[string]driver.DomainBuilder{
		"LXC":        domains.LXCDomainBuilder{},
		"VirtualBox": domains.VBoxDomainBuilder{},
	} {
		builder := b
		It("reports persistent disks as unsupported for "+name, func() {
			_, err := builder.BuildPersistentDiskXML("/p/disk.img", "qcow2", "vdc", "disk-0123456789abcde")
			Expect(err).To(MatchError(driver.ErrPersistentDiskUnsupported))
		})
	}
})

var _ = Describe("ephemeral disk format", func() {
	type domainDisks struct {
		Disks []diskDeviceXML `xml:"devices>disk"`
	}

	for name, p := range map[string]driver.VMDomainProps{
		"qcow2 boot":  {CPUs: 1, MemoryMB: 512},
		"kernel boot": {CPUs: 1, MemoryMB: 512, Kernel: "/boot/vmlinuz"},
	} {
		props := p
		It("uses the ephemeral image's format for vdb ("+name+")", func() {
			out, err := domains.QEMUDomainBuilder{}.BuildDomain("vm-1", props,
				driver.DomainDiskPaths{RootDisk: "/root.img", EphemeralDisk: "/eph.img", EphemeralFormat: "raw"})
			Expect(err).ToNot(HaveOccurred())

			var dom domainDisks
			Expect(xml.Unmarshal([]byte(out), &dom)).To(Succeed())
			Expect(dom.Disks).To(HaveLen(2))
			Expect(dom.Disks[1].Target.Dev).To(Equal("vdb"))
			Expect(dom.Disks[1].Driver.Type).To(Equal("raw"))
		})
	}

	It("defaults vdb to qcow2 when no format is given", func() {
		out, err := domains.QEMUDomainBuilder{}.BuildDomain("vm-1", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
			driver.DomainDiskPaths{RootDisk: "/root.img", EphemeralDisk: "/eph.img"})
		Expect(err).ToNot(HaveOccurred())

		var dom domainDisks
		Expect(xml.Unmarshal([]byte(out), &dom)).To(Succeed())
		Expect(dom.Disks[1].Driver.Type).To(Equal("qcow2"))
	})
})
