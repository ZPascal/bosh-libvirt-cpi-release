package main

import (
	"flag"
	"net/url"
	"os"

	"github.com/cloudfoundry/bosh-cpi-go/rpc"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	libvirt "libvirt.org/go/libvirt"

	"bosh-libvirt-cpi/cpi"
	"bosh-libvirt-cpi/driver"
)

var (
	configPathOpt = flag.String("configPath", "", "Path to configuration file")
)

func main() {
	logger, fs, cmdRunner, uuidGen := basicDeps()
	defer logger.HandlePanic("Main")

	flag.Parse()

	config, err := NewConfigFromPath(*configPathOpt, fs)
	if err != nil {
		logger.Error("main", "Loading config %s", err.Error())
		os.Exit(1)
	}

	compressor := boshcmd.NewTarballCompressor(cmdRunner, fs)

	libvirtURI := config.BackendURI
	if config.Host != "" {
		u, _ := url.Parse(config.BackendURI)
		tunnelURI, cleanupTunnel, tunnelErr := sshLibvirtURI(
			u.Scheme, config.Host, config.Port,
			config.Username, config.PrivateKey, config.HostKey,
			logger,
		)
		if tunnelErr != nil {
			logger.Error("main", "Setting up libvirt SSH tunnel: %s", tunnelErr.Error())
			os.Exit(1)
		}
		defer cleanupTunnel()
		libvirtURI = tunnelURI
	}

	conn, err := libvirt.NewConnect(libvirtURI)
	if err != nil {
		logger.Error("main", "Connecting to libvirt: %s", err.Error())
		os.Exit(1)
	}
	defer conn.Close() //nolint:errcheck

	libvirtConn := driver.NewLibvirtConnImpl(conn)

	cpiFactory := cpi.NewFactoryWithConn(
		libvirtConn, fs, cmdRunner, uuidGen, compressor, cpi.FactoryOpts(config), logger)

	cli := rpc.NewFactory(logger).NewCLI(cpiFactory)

	err = cli.ServeOnce()
	if err != nil {
		logger.Error("main", "Serving once: %s", err)
		os.Exit(1)
	}
}

func basicDeps() (boshlog.Logger, boshsys.FileSystem, boshsys.CmdRunner, boshuuid.Generator) {
	logger := boshlog.NewWriterLogger(boshlog.LevelDebug, os.Stderr)
	fs := boshsys.NewOsFileSystem(logger)
	cmdRunner := boshsys.NewExecCmdRunner(logger)
	uuidGen := boshuuid.NewGenerator()
	return logger, fs, cmdRunner, uuidGen
}
