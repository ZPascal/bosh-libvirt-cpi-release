package stemcell

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"bosh-libvirt-cpi/driver"
)

type StemcellImpl struct {
	cid  apiv1.StemcellCID
	path string

	driver driver.Driver
	runner driver.Runner

	logger boshlog.Logger
}

func NewStemcellImpl(
	cid apiv1.StemcellCID,
	path string,
	driver driver.Driver,
	runner driver.Runner,
	logger boshlog.Logger,
) StemcellImpl {
	return StemcellImpl{cid, path, driver, runner, logger}
}

func (s StemcellImpl) ID() apiv1.StemcellCID { return s.cid }

func (s StemcellImpl) SnapshotName() string { return "prepared-clone" }

func (s StemcellImpl) Prepare() error {
	// Create a snapshot of the stemcell for future cloning
	_, err := s.driver.Execute("snapshot-create-as", s.cid.AsString(), s.SnapshotName(), "--description", "Prepared for cloning")
	if err != nil {
		return bosherr.WrapErrorf(err, "Preparing stemcell for future cloning")
	}

	return nil
}

func (s StemcellImpl) Exists() (bool, error) {
	output, err := s.driver.Execute("dominfo", s.cid.AsString())
	if err != nil {
		if s.driver.IsMissingVMErr(output) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s StemcellImpl) Delete() error {
	// Undefine the stemcell domain
	output, err := s.driver.Execute("undefine", s.cid.AsString(), "--remove-all-storage", "--snapshots-metadata")
	if err != nil {
		if !s.driver.IsMissingVMErr(output) {
			return bosherr.WrapErrorf(err, "Undefining stemcell domain")
		}
	}

	// Remove stemcell directory
	_, _, err = s.runner.Execute("rm", "-rf", s.path)
	if err != nil {
		return bosherr.WrapErrorf(err, "Deleting stemcell directory '%s'", s.path)
	}

	return nil
}
