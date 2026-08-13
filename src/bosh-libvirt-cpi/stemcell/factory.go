package stemcell

import (
	"path/filepath"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	"bosh-libvirt-cpi/driver"
)

type FactoryOpts struct {
	DirPath string
}

type Factory struct {
	opts FactoryOpts

	driver     driver.Driver
	domBuilder driver.DomainBuilder
	runner     driver.Runner

	fs         boshsys.FileSystem
	uuidGen    boshuuid.Generator
	compressor boshcmd.Compressor

	logTag string
	logger boshlog.Logger
}

func NewFactory(
	opts FactoryOpts,
	driver driver.Driver,
	domBuilder driver.DomainBuilder,
	runner driver.Runner,
	fs boshsys.FileSystem,
	uuidGen boshuuid.Generator,
	compressor boshcmd.Compressor,
	logger boshlog.Logger,
) Factory {
	return Factory{
		opts: opts,

		driver:     driver,
		domBuilder: domBuilder,
		runner:     runner,

		fs:         fs,
		uuidGen:    uuidGen,
		compressor: compressor,

		logTag: "stemcell.Factory",
		logger: logger,
	}
}

func (f Factory) ImportFromPath(imagePath string) (Stemcell, error) {
	id, err := f.uuidGen.Generate()
	if err != nil {
		return nil, bosherr.WrapError(err, "Generating stemcell id")
	}

	id = "sc-" + id

	stemcellPath := filepath.Join(f.opts.DirPath, id)

	err = f.upload(imagePath, stemcellPath)
	if err != nil {
		return nil, err
	}

	stemcell := f.newStemcell(apiv1.NewStemcellCID(id))

	err = stemcell.Prepare()
	if err != nil {
		f.cleanUpPartialImport(stemcell)
		return nil, bosherr.WrapErrorf(err, "Preparing stemcell")
	}

	return stemcell, nil
}

func (f Factory) Find(cid apiv1.StemcellCID) (Stemcell, error) {
	return f.newStemcell(cid), nil
}

func (f Factory) newStemcell(cid apiv1.StemcellCID) StemcellImpl {
	path := filepath.Join(f.opts.DirPath, cid.AsString())
	return NewStemcellImpl(cid, path, f.driver, f.domBuilder, f.runner, f.logger)
}

func (f Factory) upload(imagePath, stemcellPath string) error {
	err := f.fs.MkdirAll(stemcellPath, 0750)
	if err != nil {
		return bosherr.WrapError(err, "Creating stemcell parent")
	}

	dstImage := filepath.Join(stemcellPath, "image."+f.domBuilder.DiskImageFormat())

	err = f.fs.CopyFile(imagePath, dstImage)
	if err != nil {
		return bosherr.WrapErrorf(err, "Uploading stemcell image")
	}

	return nil
}

func (f Factory) cleanUpPartialImport(stemcell StemcellImpl) {
	err := stemcell.Delete()
	if err != nil {
		f.logger.Error(f.logTag, "Failed to clean up partially imported stemcell: %s", err)
	}
}
