//go:build integration

package stemcell_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	libvirt "libvirt.org/go/libvirt"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
	"bosh-libvirt-cpi/stemcell"
)

var _ = Describe("Stemcell (integration)", func() {
	var (
		factory stemcell.Factory
		conn    *libvirt.Connect
		tmpDir  string
		logger  boshlog.Logger
	)

	BeforeEach(func() {
		uri := os.Getenv("LIBVIRT_URI")
		if uri == "" {
			Skip("LIBVIRT_URI not set")
		}

		stemcellPath := os.Getenv("STEMCELL_PATH")
		if stemcellPath == "" {
			Skip("STEMCELL_PATH not set — provide path to a BOSH stemcell tarball")
		}

		var err error
		conn, err = libvirt.NewConnect(uri)
		if err != nil {
			Skip("libvirt connection unavailable: " + err.Error())
		}

		logger = boshlog.NewWriterLogger(boshlog.LevelDebug, os.Stderr)
		fs := boshsys.NewOsFileSystem(logger)
		uuidGen := boshuuid.NewGenerator()
		compressor := boshcmd.NewTarballCompressor(
			boshsys.NewExecCmdRunner(logger), fs)
		localRunner := driver.NewLocalRunner(fs, boshsys.NewExecCmdRunner(logger), logger)
		runner := driver.NewExpandingPathRunner(localRunner)

		tmpDir, err = os.MkdirTemp("", "stemcell-integration-test")
		Expect(err).ToNot(HaveOccurred())

		libvirtConn := driver.NewLibvirtConnImpl(conn)
		domBuilder := domains.QEMUDomainBuilder{}
		d := driver.NewLibvirtDriver(libvirtConn, domBuilder, logger)

		opts := stemcell.FactoryOpts{DirPath: tmpDir}
		factory = stemcell.NewFactory(opts, d, domBuilder, runner, fs, uuidGen, compressor, logger)
	})

	AfterEach(func() {
		if conn != nil {
			conn.Close()
		}
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	})

	It("imports a stemcell tarball and creates a domain, then deletes it", func() {
		stemcellPath := os.Getenv("STEMCELL_PATH")

		sc, err := factory.ImportFromPath(stemcellPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(sc).ToNot(BeNil())
		Expect(sc.ID().AsString()).To(HavePrefix("sc-"))

		exists, err := sc.Exists()
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		imagePath := sc.ImagePath()
		Expect(imagePath).To(HaveSuffix(".qcow2"))
		Expect(filepath.Dir(imagePath)).To(Equal(filepath.Join(tmpDir, sc.ID().AsString())))

		err = sc.Delete()
		Expect(err).ToNot(HaveOccurred())

		exists, err = sc.Exists()
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})
})
