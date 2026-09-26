package stemcell_test

import (
	"errors"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	driverfakes "bosh-libvirt-cpi/driver/fakes"
	"bosh-libvirt-cpi/stemcell"
	stemcellfakes "bosh-libvirt-cpi/stemcell/fakes"
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
			DiskImageFormatResult:  "qcow2",
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
		factory.ConvertToQCOW2 = func(src, dst string) error { return nil }
		factory.DecompressImage = func(src, dst string) error { return nil }
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

		It("returns error when Upload fails", func() {
			factory.ConvertToQCOW2 = func(src, dst string) error { return errors.New("upload failed") }
			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Converting stemcell image to qcow2"))
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

	Describe("ImportFromPath (dir format)", func() {
		BeforeEach(func() {
			builder.DiskImageFormatResult = "dir"
		})

		It("uploads tarball to remote host and extracts via runner", func() {
			var uploadSrc, uploadDst string
			runner.UploadFunc = func(src, dst string) error {
				uploadSrc = src
				uploadDst = dst
				return nil
			}
			defer func() { runner.UploadFunc = nil }()

			var executedCmds []string
			runner.ExecuteFunc = func(name string, args ...string) (string, int, error) {
				executedCmds = append(executedCmds, name+" "+strings.Join(args, " "))
				return "", 0, nil
			}
			defer func() { runner.ExecuteFunc = nil }()

			sc, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).ToNot(HaveOccurred())
			Expect(sc.ID().AsString()).To(Equal("sc-uuid-1"))
			Expect(uploadSrc).To(Equal("/tmp/stemcell.tgz"))
			Expect(uploadDst).To(HaveSuffix(".tgz"))

			tarCalled := false
			for _, cmd := range executedCmds {
				if strings.HasPrefix(cmd, "tar") && strings.Contains(cmd, uploadDst) {
					tarCalled = true
				}
			}
			Expect(tarCalled).To(BeTrue(), "tar must be called with the uploaded remoteTar path")
		})

		It("returns error when tarball upload to remote fails", func() {
			runner.UploadFunc = func(src, dst string) error {
				return errors.New("upload failed")
			}
			defer func() { runner.UploadFunc = nil }()

			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Uploading stemcell tarball to remote host"))
		})
	})
})
