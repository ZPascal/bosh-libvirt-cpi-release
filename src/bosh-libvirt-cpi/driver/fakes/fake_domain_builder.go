package fakes

import "bosh-libvirt-cpi/driver"

type FakeDomainBuilder struct {
	BuildDomainXML string
	BuildDomainErr error

	BuildStemcellDomainXML string
	BuildStemcellDomainErr error

	DiskImageFormatResult   string
	StorageControllerResult string
}

var _ driver.DomainBuilder = &FakeDomainBuilder{}

func (b *FakeDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
	return b.BuildDomainXML, b.BuildDomainErr
}

func (b *FakeDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
	return b.BuildStemcellDomainXML, b.BuildStemcellDomainErr
}

func (b *FakeDomainBuilder) DiskImageFormat() string   { return b.DiskImageFormatResult }
func (b *FakeDomainBuilder) StorageController() string { return b.StorageControllerResult }
