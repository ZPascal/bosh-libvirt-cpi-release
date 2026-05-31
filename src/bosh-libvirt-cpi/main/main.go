package main

import (
	"flag"
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

	conn, err := libvirt.NewConnect(config.BackendURI)
	if err != nil {
		logger.Error("main", "Connecting to libvirt: %s", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

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
