package stemcell

import (
	"path/filepath"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/driver"
)

type StemcellImpl struct {
	cid  apiv1.StemcellCID
	path string

	driver     driver.Driver
	domBuilder driver.DomainBuilder
	runner     driver.Runner

	logger boshlog.Logger
}

func NewStemcellImpl(
	cid apiv1.StemcellCID,
	path string,
	driver driver.Driver,
	domBuilder driver.DomainBuilder,
	runner driver.Runner,
	logger boshlog.Logger,
) StemcellImpl {
	return StemcellImpl{cid, path, driver, domBuilder, runner, logger}
}

func (s StemcellImpl) ID() apiv1.StemcellCID { return s.cid }

func (s StemcellImpl) Path() string { return s.path }

func (s StemcellImpl) ImagePath() string {
	return filepath.Join(s.path, "image."+s.domBuilder.DiskImageFormat())
}

func (s StemcellImpl) Prepare() error {
	xml, err := s.domBuilder.BuildStemcellDomain(s.cid.AsString(), s.ImagePath())
	if err != nil {
		return bosherr.WrapErrorf(err, "Building stemcell domain XML")
	}

	err = s.driver.DefineDomain(xml)
	if err != nil {
		return bosherr.WrapErrorf(err, "Defining stemcell domain")
	}

	return nil
}

func (s StemcellImpl) Exists() (bool, error) {
	_, err := s.driver.LookupDomain(s.cid.AsString())
	if err != nil {
		if s.driver.IsMissingDomainErr(err) {
			return false, nil
		}
		return false, bosherr.WrapErrorf(err, "Looking up stemcell domain '%s'", s.cid.AsString())
	}
	return true, nil
}

func (s StemcellImpl) Delete() error {
	err := s.driver.DestroyDomain(s.cid.AsString())
	if err != nil && !s.driver.IsMissingDomainErr(err) {
		return bosherr.WrapErrorf(err, "Destroying stemcell domain '%s'", s.cid.AsString())
	}

	_, _, err = s.runner.Execute("rm", "-rf", s.path)
	if err != nil {
		return bosherr.WrapErrorf(err, "Deleting stemcell '%s'", s.path)
	}

	return nil
}
