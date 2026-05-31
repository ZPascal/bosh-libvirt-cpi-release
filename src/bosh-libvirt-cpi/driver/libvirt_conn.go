package driver

import libvirt "libvirt.org/go/libvirt"

// LibvirtConn wraps *libvirt.Connect to make LibvirtDriver unit-testable.
type LibvirtConn interface {
	DomainDefineXML(xml string) (*libvirt.Domain, error)
	LookupDomainByName(id string) (*libvirt.Domain, error)
	LookupStoragePoolByName(name string) (*libvirt.StoragePool, error)
	Close() (int, error)
}

// LibvirtConnImpl wraps a real *libvirt.Connect.
type LibvirtConnImpl struct {
	conn *libvirt.Connect
}

// NewLibvirtConnImpl wraps a real *libvirt.Connect in a LibvirtConnImpl.
func NewLibvirtConnImpl(conn *libvirt.Connect) LibvirtConnImpl {
	return LibvirtConnImpl{conn: conn}
}

func (c LibvirtConnImpl) DomainDefineXML(xml string) (*libvirt.Domain, error) {
	return c.conn.DomainDefineXML(xml)
}
func (c LibvirtConnImpl) LookupDomainByName(id string) (*libvirt.Domain, error) {
	return c.conn.LookupDomainByName(id)
}
func (c LibvirtConnImpl) LookupStoragePoolByName(name string) (*libvirt.StoragePool, error) {
	return c.conn.LookupStoragePoolByName(name)
}
func (c LibvirtConnImpl) Close() (int, error) {
	return c.conn.Close()
}
