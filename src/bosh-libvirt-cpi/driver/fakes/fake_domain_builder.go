package fakes

import "bosh-libvirt-cpi/driver"

type FakeDomainBuilder struct {
	BuildDomainXML   string
	BuildDomainErr   error
	BuildDomainDisks driver.DomainDiskPaths // disks of the last BuildDomain call

	BuildStemcellDomainXML string
	BuildStemcellDomainErr error

	DiskImageFormatResult string

	PersistentDiskXMLResult string
	PersistentDiskXMLErr    error
	PersistentDiskXMLArgs   []string // imagePath, format, target, serial of the last call
}

var _ driver.DomainBuilder = &FakeDomainBuilder{}

func (b *FakeDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
	b.BuildDomainDisks = disks
	return b.BuildDomainXML, b.BuildDomainErr
}

func (b *FakeDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
	return b.BuildStemcellDomainXML, b.BuildStemcellDomainErr
}

func (b *FakeDomainBuilder) DiskImageFormat() string { return b.DiskImageFormatResult }

func (b *FakeDomainBuilder) BuildPersistentDiskXML(imagePath, format, target, serial string) (string, error) {
	b.PersistentDiskXMLArgs = []string{imagePath, format, target, serial}
	return b.PersistentDiskXMLResult, b.PersistentDiskXMLErr
}
