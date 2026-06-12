package vm_test

import (
	"encoding/json"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	bdisk "bosh-libvirt-cpi/disk"
	driverfakes "bosh-libvirt-cpi/driver/fakes"
	stemcellfakes "bosh-libvirt-cpi/stemcell/fakes"
	"bosh-libvirt-cpi/vm"
)

// stubVMUUIDGen is a UUID generator for vm factory tests.
type stubVMUUIDGen struct {
	result string
	err    error
}

func (g *stubVMUUIDGen) Generate() (string, error) { return g.result, g.err }

// stubDiskUUIDGen is a UUID generator for the disk sub-factory.
type stubDiskUUIDGen struct {
	result string
	err    error
}

func (g *stubDiskUUIDGen) Generate() (string, error) { return g.result, g.err }

var _ = Describe("vm.Factory", func() {
	var (
		vmUUIDGen   *stubVMUUIDGen
		diskUUIDGen *stubDiskUUIDGen
		runner      *driverfakes.FakeRunner
		drv         *driverfakes.FakeDriver
		builder     *driverfakes.FakeDomainBuilder
		diskFactory bdisk.Factory
		factory     vm.Factory
		logger      boshlog.Logger
		stemcell    *stemcellfakes.FakeStemcell
		cloudProps  apiv1.VMCloudProps
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
		vmUUIDGen = &stubVMUUIDGen{result: "uuid-vm-1"}
		diskUUIDGen = &stubDiskUUIDGen{result: "disk-uuid-1"}
		runner = &driverfakes.FakeRunner{}
		drv = &driverfakes.FakeDriver{
			// Make LookupDomain appear as "domain not found" so that
			// HaltIfRunning (called by cleanUpPartialCreate → Delete) returns
			// immediately instead of trying to call GetState on a nil domain.
			LookupDomainErr:          errors.New("domain not found"),
			IsMissingDomainErrResult: true,
		}
		builder = &driverfakes.FakeDomainBuilder{
			BuildDomainXML:        "<domain/>",
			DiskImageFormatResult: "qcow2",
		}

		diskFactory = bdisk.NewFactory("/store/disks", diskUUIDGen, drv, runner, logger)

		stemcell = stemcellfakes.NewFakeStemcell("sc-1")
		stemcell.ImagePathResult = "/stemcells/sc-1/image.qcow2"

		// CloudPropsImpl with an empty JSON object uses VMProps defaults.
		cloudProps = apiv1.CloudPropsImpl{RawMessage: json.RawMessage("{}")}

		factory = vm.NewFactory(
			vm.FactoryOpts{DirPath: "/vms"},
			vmUUIDGen,
			drv,
			runner,
			builder,
			diskFactory,
			apiv1.AgentOptions{Mbus: "nats://nats:nats-password@127.0.0.1:4222"},
			apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2}),
			logger,
		)
	})

	Describe("Create", func() {
		It("returns VM with 'vm-' prefixed ID on success", func() {
			v, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(v.ID().AsString()).To(Equal("vm-uuid-vm-1"))
		})

		It("propagates mbus from AgentOptions into the agent env", func() {
			v, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(v).ToNot(BeNil())
			// Locate the env.json write among all Put calls.
			var envJSON []byte
			for path, contents := range runner.PutContents {
				if strings.HasSuffix(path, "env.json") {
					envJSON = contents
					break
				}
			}
			Expect(envJSON).ToNot(BeNil(), "env.json was never written")
			Expect(string(envJSON)).To(ContainSubstring("nats://nats:nats-password@127.0.0.1:4222"))
		})

		It("returns error when UUID generation fails", func() {
			vmUUIDGen.err = errors.New("uuid failure")
			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Generating VM id"))
		})

		It("returns error when ephemeral disk creation fails", func() {
			runner.ExecuteErr = errors.New("exec failed")
			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Creating ephemeral disk"))
		})

		It("returns error when BuildDomain fails", func() {
			builder.BuildDomainErr = errors.New("build failed")
			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Building domain XML"))
		})

		It("returns error when DefineDomain fails", func() {
			builder.BuildDomainXML = "<domain/>"
			drv.DefineDomainErr = errors.New("define failed")
			_, err := factory.Create(
				apiv1.NewAgentID("agent-1"),
				stemcell,
				cloudProps,
				apiv1.Networks{},
				apiv1.NewVMEnv(nil),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Defining domain"))
		})
	})

	Describe("Find", func() {
		It("returns VM with the given CID", func() {
			v, err := factory.Find(apiv1.NewVMCID("vm-xyz"))
			Expect(err).ToNot(HaveOccurred())
			Expect(v.ID().AsString()).To(Equal("vm-xyz"))
		})
	})
})
