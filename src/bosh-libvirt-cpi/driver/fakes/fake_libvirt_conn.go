package fakes

import (
	"bosh-libvirt-cpi/driver"
	libvirt "libvirt.org/go/libvirt"
)

type FakeLibvirtConn struct {
	DomainDefineXMLArg string
	DomainDefineXMLErr error

	LookupDomainByNameErr error

	LookupStoragePoolByNameErr error

	// ReconnectCalls counts Reconnect invocations; ReconnectErr is returned
	// from Reconnect; OnReconnect (if set) runs on each successful Reconnect.
	ReconnectCalls int
	ReconnectErr   error
	OnReconnect    func()
}

func (c *FakeLibvirtConn) Reconnect() error {
	c.ReconnectCalls++
	if c.ReconnectErr != nil {
		return c.ReconnectErr
	}
	if c.OnReconnect != nil {
		c.OnReconnect()
	}
	return nil
}

var _ driver.LibvirtConn = &FakeLibvirtConn{}

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
