package domains_test

import (
	"encoding/xml"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

type aarch64DomainXML struct {
	Type string `xml:"type,attr"`
	OS   struct {
		Type struct {
			Arch    string `xml:"arch,attr"`
			Machine string `xml:"machine,attr"`
		} `xml:"type"`
		Kernel  string `xml:"kernel"`
		Cmdline string `xml:"cmdline"`
	} `xml:"os"`
	Features *struct {
		ACPI *struct{} `xml:"acpi"`
	} `xml:"features"`
	CPU struct {
		Mode string `xml:"mode,attr"`
	} `xml:"cpu"`
	Disks []struct {
		Driver struct {
			Type string `xml:"type,attr"`
		} `xml:"driver"`
		Source struct {
			File string `xml:"file,attr"`
		} `xml:"source"`
		Target struct {
			Dev string `xml:"dev,attr"`
		} `xml:"target"`
	} `xml:"devices>disk"`
	Controllers []struct {
		Type  string `xml:"type,attr"`
		Index string `xml:"index,attr"`
		Model string `xml:"model,attr"`
	} `xml:"devices>controller"`
	Interfaces []struct{} `xml:"devices>interface"`
	Consoles   []struct {
		Source struct {
			Path string `xml:"path,attr"`
		} `xml:"source"`
	} `xml:"devices>console"`
}

var _ = Describe("aarch64 guests", func() {
	disks := driver.DomainDiskPaths{RootDisk: "/vms/vm-1/rootfs.img", EphemeralDisk: "/disks/eph/disk.img"}
	props := func(hostArch string) driver.VMDomainProps {
		return driver.VMDomainProps{CPUs: 2, MemoryMB: 2048, Kernel: "/boot/vmlinuz-arm64", Arch: "aarch64", HostArch: hostArch}
	}
	parse := func(out string) aarch64DomainXML {
		var d aarch64DomainXML
		Expect(xml.Unmarshal([]byte(out), &d)).To(Succeed(), out)
		return d
	}

	for name, b := range map[string]driver.DomainBuilder{
		"kernel builder": domains.QEMUKernelDomainBuilder{},
		"qemu builder":   domains.QEMUDomainBuilder{},
	} {
		builder := b

		It("emulates an aarch64 virt machine on an x86_64 host ("+name+")", func() {
			out, err := builder.BuildDomain("vm-1", props("x86_64"), disks)
			Expect(err).ToNot(HaveOccurred())
			d := parse(out)

			Expect(d.Type).To(Equal("qemu"))
			Expect(d.OS.Type.Arch).To(Equal("aarch64"))
			Expect(d.OS.Type.Machine).To(Equal("virt"))
			Expect(d.OS.Kernel).To(Equal("/boot/vmlinuz-arm64"))
			Expect(d.OS.Cmdline).To(ContainSubstring("console=ttyAMA0"))
			Expect(d.OS.Cmdline).To(ContainSubstring("root=/dev/vda"))
			Expect(d.OS.Cmdline).To(ContainSubstring("init=/bosh-init"))
			Expect(d.OS.Cmdline).ToNot(ContainSubstring("ttyS0"))
			Expect(d.Features == nil || d.Features.ACPI == nil).To(BeTrue(), "no ACPI on direct-kernel virt")
			Expect(d.CPU.Mode).To(Equal("maximum"))

			Expect(d.Disks).To(HaveLen(2))
			Expect(d.Disks[0].Target.Dev).To(Equal("vda"))
			Expect(d.Disks[0].Driver.Type).To(Equal("raw"))
			Expect(d.Disks[0].Source.File).To(Equal("/vms/vm-1/rootfs.img"))
			Expect(d.Disks[1].Target.Dev).To(Equal("vdb"))
			Expect(d.Disks[1].Driver.Type).To(Equal("qcow2"))
			Expect(d.Interfaces).To(HaveLen(1))
			Expect(d.Consoles).To(HaveLen(1))
			Expect(d.Consoles[0].Source.Path).To(Equal("/tmp/bosh-vm-vm-1-console.log"))

			// libvirt numbers PCI controllers in order; on virt, index 0 must be
			// the pcie-root (seen in CI: "The PCI controller with index='0' must
			// be model='pcie-root' for this machine type").
			Expect(d.Controllers).ToNot(BeEmpty())
			Expect(d.Controllers[0].Type).To(Equal("pci"))
			Expect(d.Controllers[0].Index).To(Equal("0"))
			Expect(d.Controllers[0].Model).To(Equal("pcie-root"))

			rootPorts := 0
			for _, c := range d.Controllers {
				if c.Type == "pci" && c.Model == "pcie-root-port" {
					rootPorts++
				}
			}
			Expect(rootPorts).To(Equal(8), "free PCIe root ports for hot-plugged persistent disks")
		})

		It("uses KVM with the host CPU on an aarch64 host ("+name+")", func() {
			out, err := builder.BuildDomain("vm-1", props("aarch64"), disks)
			Expect(err).ToNot(HaveOccurred())
			d := parse(out)
			Expect(d.Type).To(Equal("kvm"))
			Expect(d.CPU.Mode).To(Equal("host-passthrough"))
		})
	}

	It("rejects aarch64 disk boot (no kernel): the virt machine would need UEFI firmware", func() {
		p := props("x86_64")
		p.Kernel = ""
		_, err := domains.QEMUDomainBuilder{}.BuildDomain("vm-1", p, disks)
		Expect(err).To(MatchError(ContainSubstring("kernel")))
	})

	It("leaves x86_64 domains byte-identical", func() {
		for _, kernel := range []string{"", "/boot/vmlinuz"} {
			base := driver.VMDomainProps{CPUs: 1, MemoryMB: 512, Kernel: kernel}
			want, err := domains.QEMUDomainBuilder{}.BuildDomain("vm-1", base, disks)
			Expect(err).ToNot(HaveOccurred())

			for _, arch := range []string{"x86_64", ""} {
				p := base
				p.Arch, p.HostArch = arch, "x86_64"
				got, err := domains.QEMUDomainBuilder{}.BuildDomain("vm-1", p, disks)
				Expect(err).ToNot(HaveOccurred())
				Expect(got).To(Equal(want))
				Expect(strings.Contains(got, "arch='x86_64' machine='pc'")).To(BeTrue())
			}
		}
	})
})
