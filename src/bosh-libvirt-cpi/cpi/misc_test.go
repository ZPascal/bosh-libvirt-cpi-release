package cpi_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/cpi"
)

var _ = Describe("Misc", func() {
	Describe("Info", func() {
		It("advertises only the warden-tar stemcell format", func() {
			// The libvirt CPI consumes warden-tar rootfs tarballs (see
			// stemcell.Factory.upload).
			info, err := cpi.NewMisc().Info()
			Expect(err).ToNot(HaveOccurred())
			Expect(info.StemcellFormats).To(Equal([]string{"warden-tar"}))
		})
	})
})
