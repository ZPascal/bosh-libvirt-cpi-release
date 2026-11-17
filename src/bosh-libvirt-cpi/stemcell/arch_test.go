package stemcell_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	driverfakes "bosh-libvirt-cpi/driver/fakes"
	"bosh-libvirt-cpi/stemcell"
	stemcellfakes "bosh-libvirt-cpi/stemcell/fakes"
)

var _ = Describe("stemcell architecture", func() {
	It("normalizes architecture names", func() {
		for in, want := range map[string]string{
			"arm64": "aarch64", "ARM64": "aarch64", "aarch64": "aarch64",
			"x86_64": "x86_64", "amd64": "x86_64", "": "x86_64", "ppc64le": "x86_64",
		} {
			Expect(stemcell.NormalizeArch(in)).To(Equal(want), "input %q", in)
		}
	})

	Describe("import and lookup", func() {
		var (
			runner  *driverfakes.FakeRunner
			factory stemcell.Factory
			fakeFS  *stemcellfakes.FakeFS
		)

		BeforeEach(func() {
			logger := boshlog.NewLogger(boshlog.LevelNone)
			fakeFS = stemcellfakes.NewFakeFS(boshsys.NewOsFileSystem(logger))
			runner = &driverfakes.FakeRunner{}
			factory = stemcell.NewFactory(
				stemcell.FactoryOpts{DirPath: "/store/stemcells"},
				&driverfakes.FakeDriver{},
				&driverfakes.FakeDomainBuilder{DiskImageFormatResult: "qcow2", BuildStemcellDomainXML: "<domain/>"},
				runner, fakeFS,
				&stemcellfakes.FakeUUIDGen{GeneratedUUID: "uuid-1"},
				&stemcellfakes.FakeCompressor{},
				logger,
			)
			factory.ConvertToQCOW2 = func(src, dst string) error { return nil }
			factory.DecompressImage = func(src, dst string) error { return nil }
		})

		AfterEach(func() { fakeFS.Cleanup() })

		It("records the architecture next to the image and reads it back", func() {
			sc, err := factory.ImportFromPath("/tmp/stemcell.tgz", "aarch64")
			Expect(err).ToNot(HaveOccurred())
			Expect(string(runner.PutContents["/store/stemcells/sc-uuid-1/architecture"])).To(Equal("aarch64"))
			Expect(sc.Architecture()).To(Equal("aarch64"))

			found, err := factory.Find(apiv1.NewStemcellCID("sc-uuid-1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(found.Architecture()).To(Equal("aarch64"))
		})

		It("treats stemcells imported before this change (no file) as x86_64", func() {
			runner.GetErr = errors.New("no such file")
			found, err := factory.Find(apiv1.NewStemcellCID("sc-old"))
			Expect(err).ToNot(HaveOccurred())
			Expect(found.Architecture()).To(Equal("x86_64"))
		})
	})
})
