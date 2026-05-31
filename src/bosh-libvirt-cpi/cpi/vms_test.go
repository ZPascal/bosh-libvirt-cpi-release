package cpi_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	"bosh-libvirt-cpi/cpi"
	stemcellfakes "bosh-libvirt-cpi/stemcell/fakes"
	vmfakes "bosh-libvirt-cpi/vm/fakes"
)

var _ = Describe("VMs", func() {
	var (
		stemcellFinder *stemcellfakes.FakeStemcellFinder
		creator        *vmfakes.FakeCreator
		finder         *vmfakes.FakeVMFinder
		vms            cpi.VMs

		agentID     apiv1.AgentID
		stemcellCID apiv1.StemcellCID
		networks    apiv1.Networks
		env         apiv1.VMEnv
		cloudProps  apiv1.VMCloudProps
	)

	BeforeEach(func() {
		stemcellFinder = &stemcellfakes.FakeStemcellFinder{}
		creator = &vmfakes.FakeCreator{}
		finder = &vmfakes.FakeVMFinder{}
		vms = cpi.NewVMs(stemcellFinder, creator, finder)

		agentID = apiv1.NewAgentID("agent-1")
		stemcellCID = apiv1.NewStemcellCID("sc-1")
		networks = apiv1.Networks{}
		env = apiv1.VMEnv{}
		cloudProps = apiv1.NewVMCloudPropsFromMap(map[string]interface{}{})
	})

	Describe("CreateVM", func() {
		It("finds stemcell and creates VM", func() {
			fakeStemcell := stemcellfakes.NewFakeStemcell("sc-1")
			stemcellFinder.FindResult = fakeStemcell

			fakeVM := vmfakes.NewFakeVM("vm-1")
			creator.CreateResult = fakeVM

			cid, err := vms.CreateVM(agentID, stemcellCID, cloudProps, networks, nil, env)
			Expect(err).ToNot(HaveOccurred())
			Expect(cid.AsString()).To(Equal("vm-1"))
			Expect(stemcellFinder.FindArg.AsString()).To(Equal("sc-1"))
			Expect(creator.CreateStemcellArg).To(Equal(fakeStemcell))
		})

		It("returns error when stemcell finder fails", func() {
			stemcellFinder.FindErr = errors.New("stemcell not found")

			_, err := vms.CreateVM(agentID, stemcellCID, cloudProps, networks, nil, env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stemcell not found"))
		})

		It("returns error when creator fails", func() {
			fakeStemcell := stemcellfakes.NewFakeStemcell("sc-1")
			stemcellFinder.FindResult = fakeStemcell
			creator.CreateErr = errors.New("create failed")

			_, err := vms.CreateVM(agentID, stemcellCID, cloudProps, networks, nil, env)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("create failed"))
		})
	})

	Describe("CreateVMV2", func() {
		It("returns VM ID and networks", func() {
			fakeStemcell := stemcellfakes.NewFakeStemcell("sc-1")
			stemcellFinder.FindResult = fakeStemcell

			fakeVM := vmfakes.NewFakeVM("vm-2")
			creator.CreateResult = fakeVM

			cid, retNets, err := vms.CreateVMV2(agentID, stemcellCID, cloudProps, networks, nil, env)
			Expect(err).ToNot(HaveOccurred())
			Expect(cid.AsString()).To(Equal("vm-2"))
			Expect(retNets).To(Equal(networks))
		})
	})

	Describe("DeleteVM", func() {
		It("finds and deletes the VM", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			finder.FindResult = fakeVM

			err := vms.DeleteVM(apiv1.NewVMCID("vm-1"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when finder fails", func() {
			finder.FindErr = errors.New("not found")

			err := vms.DeleteVM(apiv1.NewVMCID("vm-1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("returns error when delete fails", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			fakeVM.DeleteErr = errors.New("delete failed")
			finder.FindResult = fakeVM

			err := vms.DeleteVM(apiv1.NewVMCID("vm-1"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("delete failed"))
		})
	})

	Describe("HasVM", func() {
		It("returns true when VM exists", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			fakeVM.ExistsResult = true
			finder.FindResult = fakeVM

			exists, err := vms.HasVM(apiv1.NewVMCID("vm-1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("returns error when finder fails", func() {
			finder.FindErr = errors.New("nope")

			_, err := vms.HasVM(apiv1.NewVMCID("vm-1"))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("RebootVM", func() {
		It("reboots the VM", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			finder.FindResult = fakeVM

			err := vms.RebootVM(apiv1.NewVMCID("vm-1"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when finder fails", func() {
			finder.FindErr = errors.New("nope")

			err := vms.RebootVM(apiv1.NewVMCID("vm-1"))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("SetVMMetadata", func() {
		It("sets metadata on the VM", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			finder.FindResult = fakeVM

			err := vms.SetVMMetadata(apiv1.NewVMCID("vm-1"), apiv1.VMMeta{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error when finder fails", func() {
			finder.FindErr = errors.New("nope")

			err := vms.SetVMMetadata(apiv1.NewVMCID("vm-1"), apiv1.VMMeta{})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetDisks", func() {
		It("returns disk IDs from the VM", func() {
			fakeVM := vmfakes.NewFakeVM("vm-1")
			fakeVM.DiskIDsResult = []apiv1.DiskCID{apiv1.NewDiskCID("disk-1")}
			finder.FindResult = fakeVM

			ids, err := vms.GetDisks(apiv1.NewVMCID("vm-1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(ids).To(HaveLen(1))
			Expect(ids[0].AsString()).To(Equal("disk-1"))
		})

		It("returns error when finder fails", func() {
			finder.FindErr = errors.New("nope")

			_, err := vms.GetDisks(apiv1.NewVMCID("vm-1"))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("CalculateVMCloudProperties", func() {
		It("returns cloud props from resources", func() {
			props, err := vms.CalculateVMCloudProperties(apiv1.VMResources{
				RAM:               1024,
				CPU:               2,
				EphemeralDiskSize: 10000,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(props).ToNot(BeNil())
		})
	})
})
