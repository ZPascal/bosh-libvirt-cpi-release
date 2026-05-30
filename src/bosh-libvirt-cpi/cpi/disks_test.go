package cpi_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	"bosh-libvirt-cpi/cpi"
	diskfakes "bosh-libvirt-cpi/disk/fakes"
	vmfakes "bosh-libvirt-cpi/vm/fakes"
)

var _ = Describe("Disks", func() {
	var (
		creator  *diskfakes.FakeDiskCreator
		finder   *diskfakes.FakeDiskFinder
		vmFinder *vmfakes.FakeVMFinder
		disks    cpi.Disks
	)

	BeforeEach(func() {
		creator = &diskfakes.FakeDiskCreator{}
		finder = &diskfakes.FakeDiskFinder{}
		vmFinder = &vmfakes.FakeVMFinder{}
		disks = cpi.NewDisks(creator, finder, vmFinder)
	})

	Describe("CreateDisk", func() {
		It("creates disk and returns disk ID", func() {
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			creator.CreateResult = fakeDisk

			cid, err := disks.CreateDisk(1024, apiv1.CloudPropsImpl{}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(cid.AsString()).To(Equal("disk-1"))
			Expect(creator.CreateSizeArg).To(Equal(1024))
		})

		It("returns error when creator fails", func() {
			creator.CreateErr = errors.New("create failed")

			_, err := disks.CreateDisk(1024, apiv1.CloudPropsImpl{}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("create failed"))
		})
	})

	Describe("DeleteDisk", func() {
		It("finds and deletes disk", func() {
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			finder.FindResult = fakeDisk

			err := disks.DeleteDisk(apiv1.NewDiskCID("disk-1"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when finder fails", func() {
			finder.FindErr = errors.New("not found")

			err := disks.DeleteDisk(apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("returns error when delete fails", func() {
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			fakeDisk.DeleteErr = errors.New("delete failed")
			finder.FindResult = fakeDisk

			err := disks.DeleteDisk(apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("delete failed"))
		})
	})

	Describe("HasDisk", func() {
		It("returns true when disk exists", func() {
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			fakeDisk.ExistsResult = true
			finder.FindResult = fakeDisk

			exists, err := disks.HasDisk(apiv1.NewDiskCID("disk-1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("returns error when finder fails", func() {
			finder.FindErr = errors.New("nope")

			_, err := disks.HasDisk(apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("AttachDisk", func() {
		It("attaches the disk to the VM", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			vmFinder.FindResult = fakeVM
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			finder.FindResult = fakeDisk

			err := disks.AttachDisk(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeVM.AttachDiskArg).To(Equal(fakeDisk))
		})

		It("returns error when VM finder fails", func() {
			vmFinder.FindErr = errors.New("vm missing")

			err := disks.AttachDisk(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("vm missing"))
		})

		It("returns error when disk finder fails", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			vmFinder.FindResult = fakeVM
			finder.FindErr = errors.New("disk missing")

			err := disks.AttachDisk(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("disk missing"))
		})

		It("returns error when attach fails", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			fakeVM.AttachDiskErr = errors.New("attach failed")
			vmFinder.FindResult = fakeVM
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			finder.FindResult = fakeDisk

			err := disks.AttachDisk(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("attach failed"))
		})
	})

	Describe("AttachDiskV2", func() {
		It("returns the disk hint from VM", func() {
			expectedHint := apiv1.NewDiskHintFromString("some-hint")
			fakeVM := vmfakes.NewFakeVM("vm-1")
			fakeVM.AttachDiskHint = expectedHint
			vmFinder.FindResult = fakeVM
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			finder.FindResult = fakeDisk

			hint, err := disks.AttachDiskV2(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(hint).To(Equal(expectedHint))
		})
	})

	Describe("DetachDisk", func() {
		It("detaches the disk from the VM", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			vmFinder.FindResult = fakeVM
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			finder.FindResult = fakeDisk

			err := disks.DetachDisk(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeVM.DetachDiskArg).To(Equal(fakeDisk))
		})

		It("returns error when VM finder fails", func() {
			vmFinder.FindErr = errors.New("vm missing")

			err := disks.DetachDisk(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
		})

		It("returns error when disk finder fails", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			vmFinder.FindResult = fakeVM
			finder.FindErr = errors.New("disk missing")

			err := disks.DetachDisk(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
		})

		It("returns error when detach fails", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			fakeVM.DetachDiskErr = errors.New("detach failed")
			vmFinder.FindResult = fakeVM
			fakeDisk := diskfakes.NewFakeDisk("disk-1")
			finder.FindResult = fakeDisk

			err := disks.DetachDisk(apiv1.NewVMCID("vm-1"), apiv1.NewDiskCID("disk-1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("detach failed"))
		})
	})
})
