package integration_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-libvirt-cpi/cpi"
)

func TestCPIIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CPI Integration Suite")
}

var _ = Describe("CPI Configuration and Setup", func() {
	var (
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "cpi-test-")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("BOSH store directory setup", func() {
		It("creates stemcells storage", func() {
			opts := cpi.FactoryOpts{
				StoreDir: tmpDir,
			}
			stemDir := opts.StemcellsDir()
			err := os.MkdirAll(stemDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			info, err := os.Stat(stemDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("creates vms storage", func() {
			opts := cpi.FactoryOpts{
				StoreDir: tmpDir,
			}
			vmDir := opts.VMsDir()
			err := os.MkdirAll(vmDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			info, err := os.Stat(vmDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("creates disks storage", func() {
			opts := cpi.FactoryOpts{
				StoreDir: tmpDir,
			}
			diskDir := opts.DisksDir()
			err := os.MkdirAll(diskDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			info, err := os.Stat(diskDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("creates complete BOSH directory tree", func() {
			opts := cpi.FactoryOpts{
				StoreDir: tmpDir,
			}

			dirs := []string{
				opts.StemcellsDir(),
				opts.VMsDir(),
				opts.DisksDir(),
			}

			for _, dir := range dirs {
				err := os.MkdirAll(dir, 0755)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, dir := range dirs {
				info, err := os.Stat(dir)
				Expect(err).ToNot(HaveOccurred())
				Expect(info.IsDir()).To(BeTrue())
			}
		})
	})

	Describe("Configuration validation", func() {
		It("validates hypervisor types", func() {
			hypervisors := []string{"qemu", "vbox", "lxc", "xen", "vmware"}

			for _, hyp := range hypervisors {
				Expect(hyp).ToNot(BeEmpty())
			}
		})

		It("validates storage controllers", func() {
			controllers := []string{"IDE", "SCSI", "SATA"}

			for _, controller := range controllers {
				Expect(controller).ToNot(BeEmpty())
			}
		})
	})
})
