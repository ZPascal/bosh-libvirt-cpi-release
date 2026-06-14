package vm_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	diskfakes "bosh-libvirt-cpi/disk/fakes"
	driverfakes "bosh-libvirt-cpi/driver/fakes"
	"bosh-libvirt-cpi/vm"
)

// stubCallContext returns a stemcell API version of 2, which causes the
// per-disk AttachDisk path to skip reconfigureAgent for persistent disks.
type stubCallContext struct {
	version int
}

func (c *stubCallContext) As(val interface{}) error {
	type stemcellStruct struct {
		VM struct {
			Stemcell struct {
				APIVersion int `json:"api_version"`
			} `json:"stemcell"`
		} `json:"vm"`
	}
	if out, ok := val.(*stemcellStruct); ok {
		out.VM.Stemcell.APIVersion = c.version
	}
	return nil
}

var _ = Describe("VMImpl disk operations", func() {
	var (
		vmImpl vm.VMImpl
		runner *driverfakes.FakeRunner
		drv    *driverfakes.FakeDriver
		logger boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
		runner = &driverfakes.FakeRunner{}
		drv = &driverfakes.FakeDriver{}
		// GetResult is used when reconfigureAgent reads env.json — provide
		// minimal valid JSON so FromBytes succeeds.
		runner.GetResult = []byte("{}")
		store := vm.NewStore("/vms/vm-1", runner)
		// Use stemcell API version 2 so persistent-disk AttachDisk skips
		// reconfigureAgent.
		stemVer := apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2})
		vmImpl = vm.NewVMImpl(
			apiv1.NewVMCID("vm-1"),
			store,
			stemVer,
			drv,
			logger,
		)
	})

	Describe("AttachDisk", func() {
		It("saves attachment and returns hint with image path", func() {
			disk := diskfakes.NewFakeDisk("disk-1")
			disk.ImagePathResult = "/disks/disk-1/disk.img"

			hint, err := vmImpl.AttachDisk(disk)
			Expect(err).ToNot(HaveOccurred())
			Expect(hint).To(Equal(apiv1.NewDiskHintFromString("/disks/disk-1/disk.img")))
		})

		It("returns error when store Put fails", func() {
			runner.PutErr = errors.New("put failed")
			disk := diskfakes.NewFakeDisk("disk-1")
			disk.ImagePathResult = "/disks/disk-1/disk.img"

			_, err := vmImpl.AttachDisk(disk)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("DetachDisk", func() {
		It("removes attachment record after attaching", func() {
			disk := diskfakes.NewFakeDisk("disk-1")
			disk.ImagePathResult = "/disks/disk-1/disk.img"

			_, err := vmImpl.AttachDisk(disk)
			Expect(err).ToNot(HaveOccurred())

			err = vmImpl.DetachDisk(disk)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when reconfigureAgent fails due to Get error", func() {
			runner.GetErr = errors.New("get failed")
			disk := diskfakes.NewFakeDisk("disk-1")
			disk.ImagePathResult = "/disks/disk-1/disk.img"

			err := vmImpl.DetachDisk(disk)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("DiskIDs", func() {
		It("returns empty slice when no persistent disks attached", func() {
			// runner.Execute returns empty string (the ls output), so List returns []
			runner.ExecuteOutput = ""
			ids, err := vmImpl.DiskIDs()
			Expect(err).ToNot(HaveOccurred())
			Expect(ids).To(BeEmpty())
		})
	})
})
