package vm_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	diskfakes "bosh-libvirt-cpi/disk/fakes"
	"bosh-libvirt-cpi/driver"
	driverfakes "bosh-libvirt-cpi/driver/fakes"
	"bosh-libvirt-cpi/vm"
)

// These tests back the VM store with a real LocalRunner on a temp dir so
// attachment records are listed exactly as in production (target allocation
// depends on them).
var _ = Describe("VMImpl persistent disks (hot-plug)", func() {
	const diskCID = "disk-0123456789abcdef-0000"
	const serial = "disk-0123456789abcde" // first 20 chars of diskCID

	var (
		vmImpl  vm.VMImpl
		drv     *driverfakes.FakeDriver
		builder *driverfakes.FakeDomainBuilder
		tmpDir  string
	)

	newDisk := func(cid string) *diskfakes.FakeDisk {
		d := diskfakes.NewFakeDisk(cid)
		d.ImagePathResult = "/disks/" + cid + "/disk.img"
		return d
	}

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "vm-persistent-disks-test")
		Expect(err).ToNot(HaveOccurred())

		logger := boshlog.NewLogger(boshlog.LevelNone)
		runner := driver.NewLocalRunner(boshsys.NewOsFileSystem(logger), boshsys.NewExecCmdRunner(logger), logger)
		storeDir := filepath.Join(tmpDir, "vm-1")
		Expect(os.MkdirAll(storeDir, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(storeDir, "env.json"), []byte("{}"), 0644)).To(Succeed())

		drv = &driverfakes.FakeDriver{}
		builder = &driverfakes.FakeDomainBuilder{PersistentDiskXMLResult: "<disk/>"}
		vmImpl = vm.NewVMImpl(
			apiv1.NewVMCID("vm-1"),
			vm.NewStore(storeDir, runner),
			apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2}),
			drv,
			builder,
			logger,
		)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("attaches the first persistent disk as vdc and returns a virtio hint", func() {
		hint, err := vmImpl.AttachDisk(newDisk(diskCID))
		Expect(err).ToNot(HaveOccurred())

		Expect(builder.PersistentDiskXMLArgs).To(Equal([]string{"/disks/" + diskCID + "/disk.img", "qcow2", "vdc", serial}))
		Expect(drv.AttachDeviceCalls).To(Equal([]driverfakes.DeviceCall{{ID: "vm-1", XML: "<disk/>"}}))
		Expect(hint).To(Equal(apiv1.NewDiskHintFromMap(map[string]interface{}{
			"id":        serial,
			"volume_id": serial,
			"path":      "/dev/vdc",
		})))

		ids, err := vmImpl.DiskIDs()
		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(Equal([]apiv1.DiskCID{apiv1.NewDiskCID(diskCID)}))
	})

	It("is idempotent when the disk is already attached (e.g. bosh cck reattach)", func() {
		d := newDisk(diskCID)
		first, err := vmImpl.AttachDisk(d)
		Expect(err).ToNot(HaveOccurred())

		drv.AttachDeviceErr = errors.New("Failed to get write lock") // what QEMU says on a 2nd open
		second, err := vmImpl.AttachDisk(d)
		Expect(err).ToNot(HaveOccurred())
		Expect(second).To(Equal(first))
		Expect(drv.AttachDeviceCalls).To(HaveLen(1))

		ids, err := vmImpl.DiskIDs()
		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(Equal([]apiv1.DiskCID{apiv1.NewDiskCID(diskCID)}))
	})

	It("attaches and detaches a raw image (dd fallback) with the raw format", func() {
		d := newDisk(diskCID)
		d.FormatResult = "raw"
		_, err := vmImpl.AttachDisk(d)
		Expect(err).ToNot(HaveOccurred())
		Expect(builder.PersistentDiskXMLArgs[1]).To(Equal("raw"))

		builder.PersistentDiskXMLArgs = nil
		Expect(vmImpl.DetachDisk(newDisk(diskCID))).To(Succeed()) // format comes from the record
		Expect(builder.PersistentDiskXMLArgs[1]).To(Equal("raw"))
	})

	It("fails the attach when the image format can't be determined", func() {
		d := newDisk(diskCID)
		d.FormatErr = errors.New("no such file")
		_, err := vmImpl.AttachDisk(d)
		Expect(err).To(MatchError(ContainSubstring("no such file")))
		Expect(drv.AttachDeviceCalls).To(BeEmpty())
	})

	It("uses vdd for a second disk", func() {
		_, err := vmImpl.AttachDisk(newDisk(diskCID))
		Expect(err).ToNot(HaveOccurred())

		_, err = vmImpl.AttachDisk(newDisk("disk-fedcba9876543210-1111"))
		Expect(err).ToNot(HaveOccurred())
		Expect(builder.PersistentDiskXMLArgs[2]).To(Equal("vdd"))
	})

	It("reuses vdc after the first disk was detached", func() {
		first := newDisk(diskCID)
		_, err := vmImpl.AttachDisk(first)
		Expect(err).ToNot(HaveOccurred())
		Expect(vmImpl.DetachDisk(first)).To(Succeed())

		_, err = vmImpl.AttachDisk(newDisk("disk-fedcba9876543210-1111"))
		Expect(err).ToNot(HaveOccurred())
		Expect(builder.PersistentDiskXMLArgs[2]).To(Equal("vdc"))
	})

	It("detaches with the same device XML and removes the record", func() {
		d := newDisk(diskCID)
		_, err := vmImpl.AttachDisk(d)
		Expect(err).ToNot(HaveOccurred())
		builder.PersistentDiskXMLArgs = nil

		Expect(vmImpl.DetachDisk(d)).To(Succeed())
		Expect(builder.PersistentDiskXMLArgs).To(Equal([]string{"/disks/" + diskCID + "/disk.img", "qcow2", "vdc", serial}))
		Expect(drv.DetachDeviceCalls).To(Equal([]driverfakes.DeviceCall{{ID: "vm-1", XML: "<disk/>"}}))

		ids, err := vmImpl.DiskIDs()
		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(BeEmpty())
	})

	It("tolerates a missing domain on detach", func() {
		d := newDisk(diskCID)
		_, err := vmImpl.AttachDisk(d)
		Expect(err).ToNot(HaveOccurred())

		drv.DetachDeviceErr = errors.New("domain not found")
		drv.IsMissingDomainErrResult = true
		Expect(vmImpl.DetachDisk(d)).To(Succeed())

		ids, err := vmImpl.DiskIDs()
		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(BeEmpty())
	})

	It("returns other detach errors and keeps the record", func() {
		d := newDisk(diskCID)
		_, err := vmImpl.AttachDisk(d)
		Expect(err).ToNot(HaveOccurred())

		drv.DetachDeviceErr = errors.New("device busy")
		Expect(vmImpl.DetachDisk(d)).To(MatchError(ContainSubstring("device busy")))

		ids, err := vmImpl.DiskIDs()
		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(HaveLen(1))
	})

	It("fails detach without unplugging or forgetting the disk when its record can't be read", func() {
		d := newDisk(diskCID)
		_, err := vmImpl.AttachDisk(d)
		Expect(err).ToNot(HaveOccurred())

		// Replace the record with a directory: still listed, but unreadable even
		// for root (the integration jobs run the suite with sudo).
		record := filepath.Join(tmpDir, "vm-1", diskCID+"-disk-attachment.json")
		Expect(os.Remove(record)).To(Succeed())
		Expect(os.Mkdir(record, 0755)).To(Succeed())

		Expect(vmImpl.DetachDisk(d)).To(MatchError(ContainSubstring("disk attachment")))
		Expect(drv.DetachDeviceCalls).To(BeEmpty())
		_, statErr := os.Stat(record)
		Expect(statErr).ToNot(HaveOccurred())
	})

	It("leaves no record when attaching the device fails", func() {
		drv.AttachDeviceErr = errors.New("hotplug failed")
		_, err := vmImpl.AttachDisk(newDisk(diskCID))
		Expect(err).To(MatchError(ContainSubstring("hotplug failed")))

		ids, err := vmImpl.DiskIDs()
		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(BeEmpty())
	})

	It("falls back to the image-path hint when the backend cannot hot-plug", func() {
		builder.PersistentDiskXMLErr = driver.ErrPersistentDiskUnsupported
		hint, err := vmImpl.AttachDisk(newDisk(diskCID))
		Expect(err).ToNot(HaveOccurred())
		Expect(hint).To(Equal(apiv1.NewDiskHintFromString("/disks/" + diskCID + "/disk.img")))
		Expect(drv.AttachDeviceCalls).To(BeEmpty())

		Expect(vmImpl.DetachDisk(newDisk(diskCID))).To(Succeed())
		Expect(drv.DetachDeviceCalls).To(BeEmpty())
	})

	It("uses the whole CID as serial when it is shorter than 20 characters", func() {
		_, err := vmImpl.AttachDisk(newDisk("disk-short"))
		Expect(err).ToNot(HaveOccurred())
		Expect(builder.PersistentDiskXMLArgs[3]).To(Equal("disk-short"))
	})
})
