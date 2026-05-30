package cpi

import (
	"net/url"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	libvirt "libvirt.org/go/libvirt"

	bdisk "bosh-libvirt-cpi/disk"
	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
	bstem "bosh-libvirt-cpi/stemcell"
	bvm "bosh-libvirt-cpi/vm"
)

type Factory struct {
	fs          boshsys.FileSystem
	cmdRunner   boshsys.CmdRunner
	uuidGen     boshuuid.Generator
	compressor  boshcmd.Compressor
	opts        FactoryOpts
	logger      boshlog.Logger
	libvirtConn driver.LibvirtConn // non-nil when conn is owned by the caller
}

var _ apiv1.CPIFactory = Factory{}

type CPI struct {
	Misc
	Stemcells
	VMs
	Disks
	Snapshots
}

var _ apiv1.CPI = CPI{}

func NewFactory(
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	uuidGen boshuuid.Generator,
	compressor boshcmd.Compressor,
	opts FactoryOpts,
	logger boshlog.Logger,
) Factory {
	return Factory{fs: fs, cmdRunner: cmdRunner, uuidGen: uuidGen, compressor: compressor, opts: opts, logger: logger}
}

// NewFactoryWithConn is like NewFactory but accepts a pre-opened LibvirtConn
// whose lifecycle is managed by the caller. Factory.New() will use this conn
// instead of opening its own, so the caller can defer conn.Close() to ensure
// the connection is closed after the CPI request completes.
func NewFactoryWithConn(
	conn driver.LibvirtConn,
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	uuidGen boshuuid.Generator,
	compressor boshcmd.Compressor,
	opts FactoryOpts,
	logger boshlog.Logger,
) Factory {
	return Factory{fs: fs, cmdRunner: cmdRunner, uuidGen: uuidGen, compressor: compressor, opts: opts, logger: logger, libvirtConn: conn}
}

func (f Factory) New(ctx apiv1.CallContext) (apiv1.CPI, error) {
	rawRunner := driver.RawRunner(driver.NewLocalRunner(f.fs, f.cmdRunner, f.logger))

	if len(f.opts.Host) > 0 {
		runnerOpts := driver.SSHRunnerOpts{
			Host:       f.opts.Host,
			Username:   f.opts.Username,
			PrivateKey: f.opts.PrivateKey,
		}
		rawRunner = driver.NewSSHRunner(runnerOpts, f.fs, f.logger)
	}

	runner := driver.NewExpandingPathRunner(rawRunner)

	u, _ := url.Parse(f.opts.BackendURI) // already validated in Validate()
	var domBuilder driver.DomainBuilder
	switch u.Scheme {
	case "vbox":
		domBuilder = domains.VBoxDomainBuilder{}
	case "lxc":
		domBuilder = domains.LXCDomainBuilder{}
	default: // "qemu"
		domBuilder = domains.QEMUDomainBuilder{}
	}

	var libvirtConn driver.LibvirtConn
	if f.libvirtConn != nil {
		libvirtConn = f.libvirtConn
	} else {
		conn, err := libvirt.NewConnect(f.opts.BackendURI)
		if err != nil {
			return nil, err
		}
		libvirtConn = driver.NewLibvirtConnImpl(conn)
	}
	d := driver.NewLibvirtDriver(libvirtConn, domBuilder, f.logger)

	stemcellsOpts := bstem.FactoryOpts{
		DirPath: f.opts.StemcellsDir(),
	}

	stemcells := bstem.NewFactory(
		stemcellsOpts, d, domBuilder, runner, f.fs, f.uuidGen, f.compressor, f.logger)

	disks := bdisk.NewFactory(f.opts.DisksDir(), f.uuidGen, d, runner, f.logger)

	vmsOpts := bvm.FactoryOpts{
		DirPath:            f.opts.VMsDir(),
		AutoEnableNetworks: f.opts.AutoEnableNetworks,
	}

	vms := bvm.NewFactory(
		vmsOpts, f.uuidGen, d, runner, domBuilder, disks,
		f.opts.Agent, apiv1.NewStemcellAPIVersion(ctx), f.logger)

	return CPI{
		NewMisc(),
		NewStemcells(stemcells, stemcells),
		NewVMs(stemcells, vms, vms),
		NewDisks(disks, disks, vms),
		NewSnapshots(),
	}, nil
}
