package vm_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/vm"
)

var _ = Describe("ISO9660", func() {
	Describe("Bytes", func() {
		It("returns a valid ISO image for a file with content", func() {
			iso := vm.ISO9660{
				FileName: "USER_DATA",
				Contents: []byte(`{"key":"value"}`),
			}
			result, err := iso.Bytes()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeEmpty())
			// The result includes 16 reserved sectors prepended
			Expect(uint32(len(result)) % vm.SectorSize).To(Equal(uint32(0)))
		})

		It("returns an error for a file name that violates ISO constraints", func() {
			iso := vm.ISO9660{
				FileName: "invalid name!",
				Contents: []byte("data"),
			}
			_, err := iso.Bytes()
			Expect(err).To(HaveOccurred())
		})
	})
})
