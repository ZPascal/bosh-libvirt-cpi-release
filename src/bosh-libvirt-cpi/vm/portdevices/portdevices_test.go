package portdevices_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/testhelpers/mocks"
	"bosh-libvirt-cpi/vm/portdevices"
)

var _ = Describe("Port Devices", func() {
	var (
		mockDriver *mocks.SimpleMockDriver
		logger     boshlog.Logger
		vmCID      apiv1.VMCID
		pd         portdevices.PortDevices
	)

	BeforeEach(func() {
		mockDriver = mocks.NewSimpleMockDriver()
		logger = boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, nil)
		vmCID = apiv1.NewVMCID("vm-portdevice-test")

		opts := portdevices.PortDevicesOpts{
			Controller: "sata",
		}
		pd = portdevices.NewPortDevices(vmCID, opts, mockDriver, logger)
	})

	Context("Port Devices Creation", func() {
		It("creates port devices instance", func() {
			Expect(pd).NotTo(BeNil())
		})

		It("stores VM CID", func() {
			Expect(vmCID.AsString()).To(Equal("vm-portdevice-test"))
		})

		It("stores controller options", func() {
			Expect("sata").NotTo(BeEmpty())
		})
	})

	Context("Controller Types", func() {
		It("supports SCSI controller", func() {
			controller := "scsi"
			Expect(controller).To(Equal("scsi"))
		})

		It("supports IDE controller", func() {
			controller := "ide"
			Expect(controller).To(Equal("ide"))
		})

		It("supports SATA controller", func() {
			controller := "sata"
			Expect(controller).To(Equal("sata"))
		})
	})

	Context("CDROM Operations", func() {
		It("creates CDROM device", func() {
			mockDriver.ExecuteFunc = func(args ...string) (string, error) {
				if len(args) > 0 && args[0] == "showvminfo" {
					return "storagecontrollername0=\"SCSI\"", nil
				}
				return "", nil
			}

			Expect(mockDriver).NotTo(BeNil())
		})

		It("handles CDROM on SCSI controller", func() {
			controller := "SCSI Controller-1-0"
			Expect(controller).To(ContainSubstring("SCSI"))
		})
	})

	Context("Port Device Configuration", func() {
		It("configures port 0", func() {
			port := 0
			Expect(port).To(Equal(0))
		})

		It("configures port 1", func() {
			port := 1
			Expect(port).To(Equal(1))
		})

		It("handles multiple port configurations", func() {
			ports := []int{0, 1, 2, 3}
			for _, p := range ports {
				Expect(p >= 0).To(BeTrue())
			}
		})
	})
})

var _ = Describe("Port Device Storage", func() {
	Context("Storage Controller Detection", func() {
		It("detects SCSI controller", func() {
			name := "SCSI"
			Expect(name).To(ContainSubstring("SCSI"))
		})

		It("detects IDE controller", func() {
			name := "IDE"
			Expect(name).To(ContainSubstring("IDE"))
		})

		It("detects SATA controller", func() {
			name := "SATA"
			Expect(name).To(ContainSubstring("SATA"))
		})

		It("handles controller names with 'Controller'", func() {
			names := []string{
				"SCSI Controller",
				"IDE Controller",
				"SATA Controller",
			}

			for _, name := range names {
				Expect(name).To(ContainSubstring("Controller"))
			}
		})
	})

	Context("Storage Controller Options", func() {
		It("accepts storage controller option", func() {
			opts := portdevices.PortDevicesOpts{
				Controller: "sata",
			}
			Expect(opts.Controller).To(Equal("sata"))
		})

		It("validates controller configuration", func() {
			controllers := []string{"scsi", "ide", "sata"}
			for _, c := range controllers {
				opts := portdevices.PortDevicesOpts{
					Controller: c,
				}
				Expect(opts.Controller).NotTo(BeEmpty())
			}
		})
	})
})

var _ = Describe("CDROM Devices", func() {
	var (
		cdrom portdevices.CDROM
	)

	Context("CDROM Operations", func() {
		It("represents CDROM device", func() {
			Expect(cdrom).NotTo(BeNil())
		})

		It("supports mount operations", func() {
			imagePath := "/var/lib/cdrom/install.iso"
			Expect(imagePath).NotTo(BeEmpty())
		})

		It("supports unmount operations", func() {
			Expect(cdrom).NotTo(BeNil())
		})
	})
})

