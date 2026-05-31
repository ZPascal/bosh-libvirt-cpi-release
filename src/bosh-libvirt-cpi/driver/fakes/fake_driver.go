package fakes

import "bosh-libvirt-cpi/driver"

type FakeDriver struct {
	DefineDomainXML string
	DefineDomainErr error

	StartDomainID  string
	StartDomainErr error

	ShutdownDomainID  string
	ShutdownDomainErr error

	DestroyDomainID  string
	DestroyDomainErr error

	RebootDomainID  string
	RebootDomainErr error

	LookupDomainID  string
	LookupDomainDom driver.Domain
	LookupDomainErr error

	UpdateMemoryID  string
	UpdateMemoryMB  int
	UpdateMemoryErr error

	UpdateCPUsID  string
	UpdateCPUs    int
	UpdateCPUsErr error

	CreateStorageVolPool   string
	CreateStorageVolName   string
	CreateStorageVolSizeMB int
	CreateStorageVolPath   string
	CreateStorageVolErr    error

	DeleteStorageVolPool string
	DeleteStorageVolName string
	DeleteStorageVolErr  error

	IsMissingDomainErrInput  error
	IsMissingDomainErrResult bool
}

var _ driver.Driver = &FakeDriver{}

func (d *FakeDriver) DefineDomain(xml string) error {
	d.DefineDomainXML = xml
	return d.DefineDomainErr
}

func (d *FakeDriver) StartDomain(id string) error {
	d.StartDomainID = id
	return d.StartDomainErr
}

func (d *FakeDriver) ShutdownDomain(id string) error {
	d.ShutdownDomainID = id
	return d.ShutdownDomainErr
}

func (d *FakeDriver) DestroyDomain(id string) error {
	d.DestroyDomainID = id
	return d.DestroyDomainErr
}

func (d *FakeDriver) RebootDomain(id string) error {
	d.RebootDomainID = id
	return d.RebootDomainErr
}

func (d *FakeDriver) LookupDomain(id string) (driver.Domain, error) {
	d.LookupDomainID = id
	return d.LookupDomainDom, d.LookupDomainErr
}

func (d *FakeDriver) UpdateDomainMemory(id string, memoryMB int) error {
	d.UpdateMemoryID = id
	d.UpdateMemoryMB = memoryMB
	return d.UpdateMemoryErr
}

func (d *FakeDriver) UpdateDomainCPUs(id string, cpus int) error {
	d.UpdateCPUsID = id
	d.UpdateCPUs = cpus
	return d.UpdateCPUsErr
}

func (d *FakeDriver) CreateStorageVol(poolName, volName string, sizeMB int) (string, error) {
	d.CreateStorageVolPool = poolName
	d.CreateStorageVolName = volName
	d.CreateStorageVolSizeMB = sizeMB
	return d.CreateStorageVolPath, d.CreateStorageVolErr
}

func (d *FakeDriver) DeleteStorageVol(poolName, volName string) error {
	d.DeleteStorageVolPool = poolName
	d.DeleteStorageVolName = volName
	return d.DeleteStorageVolErr
}

func (d *FakeDriver) IsMissingDomainErr(err error) bool {
	d.IsMissingDomainErrInput = err
	return d.IsMissingDomainErrResult
}
