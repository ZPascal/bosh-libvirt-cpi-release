package disk_test

import (
	"errors"
	"os"
	"path/filepath"

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
		tmpDir  string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "disk-factory-test")
		Expect(err).ToNot(HaveOccurred())

		logger = boshlog.NewLogger(boshlog.LevelNone)
		uuidGen = &stubUUIDGen{result: "abc-123"}
		runner = &driverfakes.FakeRunner{}
		d = &driverfakes.FakeDriver{}
		factory = disk.NewFactory(filepath.Join(tmpDir, "disks"), uuidGen, d, runner, logger)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("Create", func() {
		It("returns disk with ID prefixed 'disk-' and correct paths", func() {
			dk, err := factory.Create(1024)
			Expect(err).ToNot(HaveOccurred())
			Expect(dk.ID().AsString()).To(Equal("disk-abc-123"))
			Expect(dk.Path()).To(Equal(filepath.Join(tmpDir, "disks", "disk-abc-123")))
			Expect(dk.ImagePath()).To(Equal(filepath.Join(tmpDir, "disks", "disk-abc-123", "disk.img")))
		})

		It("returns error when UUID generation fails", func() {
			uuidGen.err = errors.New("uuid failure")
			_, err := factory.Create(1024)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Generating disk id"))
		})
	})

	Describe("Find", func() {
		It("returns disk with the given CID", func() {
			dk, err := factory.Find(apiv1.NewDiskCID("disk-xyz"))
			Expect(err).ToNot(HaveOccurred())
			Expect(dk.ID().AsString()).To(Equal("disk-xyz"))
			Expect(dk.Path()).To(Equal(filepath.Join(tmpDir, "disks", "disk-xyz")))
		})
	})
})
