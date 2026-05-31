//go:build integration

package vm_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	libvirt "libvirt.org/go/libvirt"

	bdisk "bosh-libvirt-cpi/disk"
	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
	"bosh-libvirt-cpi/stemcell"
	bvm "bosh-libvirt-cpi/vm"
)

var _ = Describe("VM (integration)", func() {
	var (
		vmFactory   bvm.Factory
		diskFactory bdisk.Factory
		stemcellFac stemcell.Factory
		conn        *libvirt.Connect
		tmpDir      string
		logger      boshlog.Logger
	)

	BeforeEach(func() {
		uri := os.Getenv("LIBVIRT_URI")
		if uri == "" {
			Skip("LIBVIRT_URI not set")
		}
		if os.Getenv("STEMCELL_PATH") == "" {
			Skip("STEMCELL_PATH not set")
		}

		var err error
		conn, err = libvirt.NewConnect(uri)
		if err != nil {
			Skip("libvirt connection unavailable: " + err.Error())
		}

		logger = boshlog.NewWriterLogger(boshlog.LevelDebug, os.Stderr)
		fs := boshsys.NewOsFileSystem(logger)
		uuidGen := boshuuid.NewGenerator()
		compressor := boshcmd.NewTarballCompressor(boshsys.NewExecCmdRunner(logger), fs)
		localRunner := driver.NewLocalRunner(fs, boshsys.NewExecCmdRunner(logger), logger)
		runner := driver.NewExpandingPathRunner(localRunner)
		domBuilder := domains.QEMUDomainBuilder{}

		libvirtConn := driver.NewLibvirtConnImpl(conn)
		d := driver.NewLibvirtDriver(libvirtConn, domBuilder, logger)

		tmpDir, err = os.MkdirTemp("", "vm-integration-test")
		Expect(err).ToNot(HaveOccurred())

		stemcellFac = stemcell.NewFactory(
			stemcell.FactoryOpts{DirPath: tmpDir + "/stemcells"},
			d, domBuilder, runner, fs, uuidGen, compressor, logger)

		diskFactory = bdisk.NewFactory(tmpDir+"/disks", uuidGen, d, runner, logger)

		agentOpts := apiv1.AgentOptions{
			Mbus: "https://mbus:password@0.0.0.0:6868",
		}
		vmFactory = bvm.NewFactory(
			bvm.FactoryOpts{DirPath: tmpDir + "/vms"},
			uuidGen, d, runner, domBuilder, diskFactory,
			agentOpts, apiv1.NewStemcellAPIVersion(apiv1.CallContext{}),
			logger)
	})

	AfterEach(func() {
		if conn != nil {
			conn.Close()
		}
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	})

	It("creates a VM, attaches a disk, reports it, detaches, and deletes", func() {
		sc, err := stemcellFac.ImportFromPath(os.Getenv("STEMCELL_PATH"))
		Expect(err).ToNot(HaveOccurred())
		defer sc.Delete()

		agentID := apiv1.NewAgentID("agent-integration-1")
		cloudProps := apiv1.NewVMCloudPropsFromMap(map[string]interface{}{
			"memory": 256, "cpus": 1, "ephemeral_disk": 1000,
		})

		v, err := vmFactory.Create(agentID, sc, cloudProps, apiv1.Networks{}, apiv1.VMEnv{})
		Expect(err).ToNot(HaveOccurred())
		Expect(v.ID().AsString()).To(HavePrefix("vm-"))
		defer v.Delete()

		disk, err := diskFactory.Create(512)
		Expect(err).ToNot(HaveOccurred())
		defer disk.Delete()

		hint, err := v.AttachDisk(disk)
		Expect(err).ToNot(HaveOccurred())
		Expect(hint.AsString()).ToNot(BeEmpty())

		ids, err := v.DiskIDs()
		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(HaveLen(1))
		Expect(ids[0].AsString()).To(Equal(disk.ID().AsString()))

		err = v.DetachDisk(disk)
		Expect(err).ToNot(HaveOccurred())

		ids, err = v.DiskIDs()
		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(BeEmpty())
	})
})
