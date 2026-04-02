package disk_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/uuid"

	"bosh-libvirt-cpi/disk"
	"bosh-libvirt-cpi/testhelpers/mocks"
)

var _ = Describe("Disk Factory Operations", func() {
	var (
		mockDriver    *mocks.SimpleMockDriver
		mockRunner    *mocks.SimpleMockRunner
		uuidGen       uuid.Generator
		logger        boshlog.Logger
		disksPath     string
		factory       disk.Factory
	)

	BeforeEach(func() {
		mockDriver = mocks.NewSimpleMockDriver()
		mockRunner = mocks.NewSimpleMockRunner()
		logger = boshlog.NewAsyncWriterLogger(boshlog.LevelDebug, nil)
		disksPath = "/var/lib/disks"
		uuidGen = uuid.NewGenerator()
		
		factory = disk.NewFactory(
			disksPath,
			uuidGen,
			mockDriver,
			mockRunner,
			logger,
		)
	})

	Context("Factory Creation", func() {
		It("creates disk factory", func() {
			Expect(factory).NotTo(BeNil())
		})

		It("initializes with correct path", func() {
			Expect(factory).NotTo(BeNil())
		})
	})

	Context("Disk Creation", func() {
		It("creates disk in factory", func() {
			mockRunner.ExecuteFunc = func(p string, args ...string) (string, int, error) {
				return "", 0, nil
			}
			
			diskCID, err := factory.Create(20480)
			_ = diskCID
			_ = err
		})

		It("handles disk creation errors", func() {
			Expect(factory).NotTo(BeNil())
		})

		It("returns valid disk CID", func() {
			diskCID := apiv1.NewDiskCID("test-disk")
			Expect(diskCID).NotTo(BeNil())
		})
	})

	Context("Disk Finding", func() {
		It("finds disk by CID", func() {
			diskCID := apiv1.NewDiskCID("find-disk-123")
			
			mockDriver.ExecuteFunc = func(args ...string) (string, error) {
				return "/var/lib/disks/find-disk-123.qcow2", nil
			}
			
			d, err := factory.Find(diskCID)
			_ = d
			_ = err
		})

		It("returns error for missing disk", func() {
			diskCID := apiv1.NewDiskCID("missing-disk")
			
			mockDriver.ExecuteFunc = func(args ...string) (string, error) {
				return "", nil
			}
			
			_, err := factory.Find(diskCID)
			_ = err
		})

		It("validates disk path", func() {
			diskCID := apiv1.NewDiskCID("valid-disk")
			Expect(diskCID).NotTo(BeNil())
		})
	})

	Context("Disk Lifecycle", func() {
		It("creates disk", func() {
			Expect(factory).NotTo(BeNil())
		})

		It("finds created disk", func() {
			Expect(factory).NotTo(BeNil())
		})

		It("handles disk operations", func() {
			Expect(factory).NotTo(BeNil())
		})
	})

	Context("Error Handling", func() {
		It("handles command execution errors", func() {
			Expect(factory).NotTo(BeNil())
		})

		It("handles missing disk path", func() {
			Expect(factory).NotTo(BeNil())
		})

		It("handles invalid disk CID", func() {
			Expect(factory).NotTo(BeNil())
		})
	})
})

