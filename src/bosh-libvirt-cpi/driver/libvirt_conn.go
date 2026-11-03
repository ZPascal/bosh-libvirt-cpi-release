package driver

import (
	"errors"

	libvirt "libvirt.org/go/libvirt"
)

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
	uri  string
}

// NewLibvirtConnImpl wraps a real *libvirt.Connect in a LibvirtConnImpl.
func NewLibvirtConnImpl(conn *libvirt.Connect) *LibvirtConnImpl {
	return &LibvirtConnImpl{conn: conn}
}

// NewReconnectableLibvirtConn wraps conn and remembers uri so that Reconnect
// can re-establish the connection after libvirtd drops it.
func NewReconnectableLibvirtConn(conn *libvirt.Connect, uri string) *LibvirtConnImpl {
	return &LibvirtConnImpl{conn: conn, uri: uri}
}

// Reconnect opens a fresh connection to the original URI.
func (c *LibvirtConnImpl) Reconnect() error {
	if c.uri == "" {
		return errors.New("cannot reconnect: no URI recorded")
	}
	conn, err := libvirt.NewConnect(c.uri)
	if err != nil {
		return err
	}
	if c.conn != nil {
		_, _ = c.conn.Close()
	}
	c.conn = conn
	return nil
}

func (c *LibvirtConnImpl) DomainDefineXML(xml string) (*libvirt.Domain, error) {
	return c.conn.DomainDefineXML(xml)
}
func (c *LibvirtConnImpl) LookupDomainByName(id string) (*libvirt.Domain, error) {
	return c.conn.LookupDomainByName(id)
}
func (c *LibvirtConnImpl) LookupStoragePoolByName(name string) (*libvirt.StoragePool, error) {
	return c.conn.LookupStoragePoolByName(name)
}
func (c *LibvirtConnImpl) Close() (int, error) {
	return c.conn.Close()
}
