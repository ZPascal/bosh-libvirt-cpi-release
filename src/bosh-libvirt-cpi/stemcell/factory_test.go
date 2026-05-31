package stemcell_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"bosh-libvirt-cpi/stemcell"
	stemcellfakes "bosh-libvirt-cpi/stemcell/fakes"
	driverfakes "bosh-libvirt-cpi/driver/fakes"
)

var _ = Describe("stemcell.Factory", func() {
	var (
		uuidGen    *stemcellfakes.FakeUUIDGen
		compressor *stemcellfakes.FakeCompressor
		fakeFS     *stemcellfakes.FakeFS
		runner     *driverfakes.FakeRunner
		drv        *driverfakes.FakeDriver
		builder    *driverfakes.FakeDomainBuilder
		factory    stemcell.Factory
		logger     boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
		uuidGen = &stemcellfakes.FakeUUIDGen{GeneratedUUID: "uuid-1"}
		compressor = &stemcellfakes.FakeCompressor{}
		fakeFS = stemcellfakes.NewFakeFS(boshsys.NewOsFileSystem(logger))
		runner = &driverfakes.FakeRunner{}
		drv = &driverfakes.FakeDriver{}
		builder = &driverfakes.FakeDomainBuilder{
			DiskImageFormatResult: "qcow2",
			BuildStemcellDomainXML: "<domain/>",
		}
		factory = stemcell.NewFactory(
			stemcell.FactoryOpts{DirPath: "/store/stemcells"},
			drv,
			builder,
			runner,
			fakeFS,
			uuidGen,
			compressor,
			logger,
		)
	})

	AfterEach(func() {
		fakeFS.Cleanup()
	})

	Describe("ImportFromPath", func() {
		It("returns stemcell with 'sc-' prefixed ID on success", func() {
			sc, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).ToNot(HaveOccurred())
			Expect(sc.ID().AsString()).To(Equal("sc-uuid-1"))
		})

		It("returns error when UUID generation fails", func() {
			uuidGen.GenerateErr = errors.New("uuid failure")
			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Generating stemcell id"))
		})

		It("returns error when TempDir fails", func() {
			fakeFS.TempDirErr = errors.New("tempdir failed")
			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Creating tmp stemcell directory"))
		})

		It("returns error when decompress fails", func() {
			compressor.DecompressFileToDirErr = errors.New("decompress failed")
			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unpacking stemcell"))
		})

		It("returns error when runner Upload fails", func() {
			runner.UploadErr = errors.New("upload failed")
			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Uploading stemcell image"))
		})

		It("returns error when BuildStemcellDomain fails", func() {
			builder.BuildStemcellDomainErr = errors.New("build failed")
			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Building stemcell domain XML"))
		})

		It("returns error when DefineDomain fails", func() {
			drv.DefineDomainErr = errors.New("define failed")
			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Defining stemcell domain"))
		})
	})

	Describe("Find", func() {
		It("returns stemcell with the given CID", func() {
			sc, err := factory.Find(apiv1.NewStemcellCID("sc-abc"))
			Expect(err).ToNot(HaveOccurred())
			Expect(sc.ID().AsString()).To(Equal("sc-abc"))
		})
	})
})
