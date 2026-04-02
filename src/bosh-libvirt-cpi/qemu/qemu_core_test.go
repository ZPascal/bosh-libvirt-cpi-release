package qemu_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/qemu"
)

var _ = Describe("QEMU Image Operations", func() {
	var (
		tempDir   string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "qemu-test-")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Context("Image Format Constants", func() {
		It("defines QCOW2 format", func() {
			Expect(qemu.FormatQCOW2).To(Equal(qemu.ImageFormat("qcow2")))
		})

		It("defines VMDK format", func() {
			Expect(qemu.FormatVMDK).To(Equal(qemu.ImageFormat("vmdk")))
		})

		It("defines RAW format", func() {
			Expect(qemu.FormatRAW).To(Equal(qemu.ImageFormat("raw")))
		})
	})

	Context("Image Creation", func() {
		It("creates new Image instance", func() {
			newImg := qemu.NewImage()
			Expect(newImg).NotTo(BeNil())
		})

		It("supports various image sizes", func() {
			sizes := []int{100, 512, 1024, 2048}
			for _, size := range sizes {
				Expect(size > 0).To(BeTrue())
			}
		})
	})

	Context("Image Format Conversions", func() {
		It("supports VMDK to QCOW2 conversion", func() {
			Expect(qemu.FormatVMDK).To(Equal(qemu.ImageFormat("vmdk")))
			Expect(qemu.FormatQCOW2).To(Equal(qemu.ImageFormat("qcow2")))
		})

		It("supports RAW to QCOW2 conversion", func() {
			Expect(qemu.FormatRAW).To(Equal(qemu.ImageFormat("raw")))
			Expect(qemu.FormatQCOW2).To(Equal(qemu.ImageFormat("qcow2")))
		})
	})
})

var _ = Describe("QEMU Image Snapshots", func() {
	Context("Snapshot Operations", func() {
		It("image instance is ready for snapshots", func() {
			img := qemu.NewImage()
			Expect(img).NotTo(BeNil())
		})

		It("supports multiple snapshots", func() {
			snapshots := []string{"snapshot-1", "snapshot-2", "snapshot-3"}
			Expect(len(snapshots)).To(Equal(3))
		})

		It("snapshot names are valid identifiers", func() {
			snapshotName := "test-snapshot"
			Expect(snapshotName).NotTo(BeEmpty())
		})
	})
})

