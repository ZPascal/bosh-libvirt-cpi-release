package fakes

import libvirt "libvirt.org/go/libvirt"

// FakeLibvirtConn implements driver.LibvirtConn for unit testing.
type FakeLibvirtConn struct {
	DomainDefineXMLArg string
	DomainDefineXMLErr error

	LookupDomainByNameErr error

	LookupStoragePoolByNameErr error
}

func (c *FakeLibvirtConn) DomainDefineXML(xml string) (*libvirt.Domain, error) {
	c.DomainDefineXMLArg = xml
	if c.DomainDefineXMLErr != nil {
		return nil, c.DomainDefineXMLErr
	}
	return nil, nil
}

func (c *FakeLibvirtConn) LookupDomainByName(id string) (*libvirt.Domain, error) {
	if c.LookupDomainByNameErr != nil {
		return nil, c.LookupDomainByNameErr
	}
	return nil, nil
}

func (c *FakeLibvirtConn) LookupStoragePoolByName(name string) (*libvirt.StoragePool, error) {
	if c.LookupStoragePoolByNameErr != nil {
		return nil, c.LookupStoragePoolByNameErr
	}
	return nil, nil
}

func (c *FakeLibvirtConn) Close() (int, error) {
	return 0, nil
}
