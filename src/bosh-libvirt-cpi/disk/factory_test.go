package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/disk"
	driverfakes "bosh-libvirt-cpi/driver/fakes"
)

type stubUUIDGen struct {
	result string
	err    error
}

func (g *stubUUIDGen) Generate() (string, error) { return g.result, g.err }

var _ = Describe("disk.Factory", func() {
	var (
		uuidGen *stubUUIDGen
		runner  *driverfakes.FakeRunner
		d       *driverfakes.FakeDriver
		factory disk.Factory
		logger  boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
		uuidGen = &stubUUIDGen{result: "abc-123"}
		runner = &driverfakes.FakeRunner{}
		d = &driverfakes.FakeDriver{}
		factory = disk.NewFactory("/store/disks", uuidGen, d, runner, logger)
	})

	Describe("Create", func() {
		It("returns disk with ID prefixed 'disk-' and correct paths", func() {
			dk, err := factory.Create(1024)
			Expect(err).ToNot(HaveOccurred())
			Expect(dk.ID().AsString()).To(Equal("disk-abc-123"))
			Expect(dk.Path()).To(Equal("/store/disks/disk-abc-123"))
			Expect(dk.ImagePath()).To(Equal("/store/disks/disk-abc-123/disk.img"))
		})

		It("returns error when UUID generation fails", func() {
			uuidGen.err = errors.New("uuid failure")
			_, err := factory.Create(1024)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Generating disk id"))
		})

		It("returns error when runner.Execute fails", func() {
			runner.ExecuteErr = errors.New("exec failed")
			_, err := factory.Create(1024)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Find", func() {
		It("returns disk with the given CID", func() {
			dk, err := factory.Find(apiv1.NewDiskCID("disk-xyz"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dk.ID().AsString()).To(Equal("disk-xyz"))
			Expect(dk.Path()).To(Equal("/store/disks/disk-xyz"))
		})
	})
})
