package driver_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	libvirt "libvirt.org/go/libvirt"

	"bosh-libvirt-cpi/driver"
)

var _ = Describe("device detach helpers", func() {
	Describe("IsDeviceMissing", func() {
		It("recognises VIR_ERR_DEVICE_MISSING", func() {
			Expect(driver.IsDeviceMissing(libvirt.Error{Code: libvirt.ERR_DEVICE_MISSING})).To(BeTrue())
		})

		It("recognises older libvirt's 'operation failed: disk vdc not found'", func() {
			err := libvirt.Error{Code: libvirt.ERR_OPERATION_FAILED, Message: "operation failed: disk vdc not found"}
			Expect(driver.IsDeviceMissing(err)).To(BeTrue())
		})

		It("does not treat other failures as missing", func() {
			Expect(driver.IsDeviceMissing(libvirt.Error{Code: libvirt.ERR_OPERATION_FAILED, Message: "operation failed: device busy"})).To(BeFalse())
			Expect(driver.IsDeviceMissing(errors.New("disk vdc not found"))).To(BeFalse())
			Expect(driver.IsDeviceMissing(nil)).To(BeFalse())
		})
	})

	Describe("DiskTargetFromXML", func() {
		It("returns the target dev of a disk device", func() {
			Expect(driver.DiskTargetFromXML(`<disk type='file' device='disk'><target dev='vdc' bus='virtio'/></disk>`)).To(Equal("vdc"))
		})

		It("returns empty for non-disk XML", func() {
			Expect(driver.DiskTargetFromXML(`<interface/>`)).To(Equal(""))
			Expect(driver.DiskTargetFromXML(`not xml`)).To(Equal(""))
		})
	})

	Describe("DomainHasTarget", func() {
		const domXML = `<domain><devices>
  <disk type='file' device='disk'><target dev='vda' bus='virtio'/></disk>
  <disk type='file' device='disk'><target dev='vdc' bus='virtio'/></disk>
</devices></domain>`

		It("finds an attached disk target", func() {
			Expect(driver.DomainHasTarget(domXML, "vdc")).To(BeTrue())
		})

		It("reports a detached target as gone", func() {
			Expect(driver.DomainHasTarget(domXML, "vdd")).To(BeFalse())
		})
	})
})
