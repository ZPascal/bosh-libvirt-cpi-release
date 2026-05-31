package driver

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	libvirt "libvirt.org/go/libvirt"
)

type LibvirtDriver struct {
	conn       LibvirtConn
	domBuilder DomainBuilder
	logTag     string
	logger     boshlog.Logger
}

func NewLibvirtDriver(conn LibvirtConn, builder DomainBuilder, logger boshlog.Logger) LibvirtDriver {
	return LibvirtDriver{
		conn:       conn,
		domBuilder: builder,
		logTag:     "driver.LibvirtDriver",
		logger:     logger,
	}
}

func (d LibvirtDriver) withDomain(id string, fn func(*libvirt.Domain) error) error {
	dom, err := d.conn.LookupDomainByName(id)
	if err != nil {
		return err
	}
	return fn(dom)
}

func (d LibvirtDriver) DefineDomain(xml string) error {
	d.logger.Debug(d.logTag, "Defining domain")
	_, err := d.conn.DomainDefineXML(xml)
	return err
}

func (d LibvirtDriver) StartDomain(id string) error {
	d.logger.Debug(d.logTag, "Starting domain '%s'", id)
	return d.withDomain(id, func(dom *libvirt.Domain) error { return dom.Create() })
}

func (d LibvirtDriver) ShutdownDomain(id string) error {
	d.logger.Debug(d.logTag, "Shutting down domain '%s'", id)
	return d.withDomain(id, func(dom *libvirt.Domain) error { return dom.Shutdown() })
}

func (d LibvirtDriver) DestroyDomain(id string) error {
	d.logger.Debug(d.logTag, "Destroying domain '%s'", id)
	dom, err := d.conn.LookupDomainByName(id)
	if err != nil {
		return err
	}
	if err := dom.Destroy(); err != nil {
		lverr, ok := err.(libvirt.Error)
		isNotRunning := ok && lverr.Code == libvirt.ERR_OPERATION_INVALID
		if !errors.Is(err, libvirt.ERR_NO_DOMAIN) && !isNotRunning {
			return err
		}
	}
	return dom.Undefine()
}

func (d LibvirtDriver) RebootDomain(id string) error {
	d.logger.Debug(d.logTag, "Rebooting domain '%s'", id)
	return d.withDomain(id, func(dom *libvirt.Domain) error { return dom.Reboot(libvirt.DOMAIN_REBOOT_DEFAULT) })
}

func (d LibvirtDriver) LookupDomain(id string) (Domain, error) {
	d.logger.Debug(d.logTag, "Looking up domain '%s'", id)
	dom, err := d.conn.LookupDomainByName(id)
	if err != nil {
		return nil, err
	}
	return &LibvirtDomainWrapper{dom}, nil
}

func (d LibvirtDriver) UpdateDomainMemory(id string, memoryMB int) error {
	d.logger.Debug(d.logTag, "Updating memory for domain '%s' to %dMB", id, memoryMB)
	return d.withDomain(id, func(dom *libvirt.Domain) error { return dom.SetMemory(uint64(memoryMB) * 1024) })
}

func (d LibvirtDriver) UpdateDomainCPUs(id string, cpus int) error {
	d.logger.Debug(d.logTag, "Updating CPUs for domain '%s' to %d", id, cpus)
	return d.withDomain(id, func(dom *libvirt.Domain) error { return dom.SetVcpus(uint(cpus)) })
}

func (d LibvirtDriver) CreateStorageVol(poolName, volName string, sizeMB int) (string, error) {
	d.logger.Debug(d.logTag, "Creating storage vol '%s' in pool '%s'", volName, poolName)
	pool, err := d.conn.LookupStoragePoolByName(poolName)
	if err != nil {
		return "", err
	}
	sizeBytes := uint64(sizeMB) * 1024 * 1024
	xml := fmt.Sprintf(`<volume><name>%s</name><capacity unit="bytes">%d</capacity></volume>`, xmlEscape(volName), sizeBytes)
	vol, err := pool.StorageVolCreateXML(xml, 0)
	if err != nil {
		return "", err
	}
	path, err := vol.GetPath()
	if err != nil {
		return "", err
	}
	return path, nil
}

func (d LibvirtDriver) DeleteStorageVol(poolName, volName string) error {
	d.logger.Debug(d.logTag, "Deleting storage vol '%s' from pool '%s'", volName, poolName)
	pool, err := d.conn.LookupStoragePoolByName(poolName)
	if err != nil {
		if errors.Is(err, libvirt.ERR_NO_STORAGE_POOL) {
			return nil
		}
		return err
	}
	vol, err := pool.LookupStorageVolByName(volName)
	if err != nil {
		if errors.Is(err, libvirt.ERR_NO_STORAGE_VOL) {
			return nil
		}
		return err
	}
	return vol.Delete(libvirt.STORAGE_VOL_DELETE_NORMAL)
}

func (d LibvirtDriver) IsMissingDomainErr(err error) bool {
	return errors.Is(err, libvirt.ERR_NO_DOMAIN)
}

// LibvirtDomainWrapper wraps *libvirt.Domain to implement the Domain interface.
type LibvirtDomainWrapper struct {
	dom *libvirt.Domain
}

func (w *LibvirtDomainWrapper) GetName() (string, error) {
	return w.dom.GetName()
}

func (w *LibvirtDomainWrapper) GetState() (int, int, error) {
	state, reason, err := w.dom.GetState()
	return int(state), reason, err
}

func (w *LibvirtDomainWrapper) IsActive() (bool, error) {
	active, err := w.dom.IsActive()
	return active, err
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
