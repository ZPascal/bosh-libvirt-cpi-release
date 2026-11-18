package driver

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	libvirt "libvirt.org/go/libvirt"
)

// LibvirtDriver implements Driver against a real libvirt connection.
type LibvirtDriver struct {
	conn       LibvirtConn
	domBuilder DomainBuilder
	logTag     string
	logger     boshlog.Logger

	reconnectDelay    time.Duration
	reconnectAttempts int
}

// NewLibvirtDriver returns a LibvirtDriver using conn and builder.
func NewLibvirtDriver(conn LibvirtConn, builder DomainBuilder, logger boshlog.Logger) LibvirtDriver {
	return LibvirtDriver{
		conn:       conn,
		domBuilder: builder,
		logTag:     "driver.LibvirtDriver",
		logger:     logger,

		reconnectDelay:    time.Second,
		reconnectAttempts: 3,
	}
}

// WithReconnectDelay sets the base delay between reconnect attempts
// (the n-th attempt waits n*delay). Mainly useful to speed up tests.
func (d LibvirtDriver) WithReconnectDelay(delay time.Duration) LibvirtDriver {
	d.reconnectDelay = delay
	return d
}

func (d LibvirtDriver) withDomain(id string, fn func(*libvirt.Domain) error) error {
	dom, err := d.conn.LookupDomainByName(id)
	if err != nil {
		return err
	}
	if dom == nil {
		return fmt.Errorf("domain '%s' not found", id)
	}
	defer dom.Free() //nolint // releases C-level reference per libvirt Go binding docs
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
	return d.withReconnect("shutdown of '"+id+"'", func() error {
		return d.withDomain(id, func(dom *libvirt.Domain) error { return dom.Shutdown() })
	})
}

// isConnectionLost reports whether err means the RPC connection to libvirtd
// was dropped, e.g. "End of file while reading data: Input/output error" or
// "internal error: client socket is closed".
func isConnectionLost(err error) bool {
	var lverr libvirt.Error
	if !errors.As(err, &lverr) {
		return false
	}
	if lverr.Code == libvirt.ERR_INVALID_CONN {
		return true
	}
	if lverr.Domain != libvirt.FROM_RPC {
		return false
	}
	switch lverr.Code {
	case libvirt.ERR_SYSTEM_ERROR, libvirt.ERR_RPC, libvirt.ERR_INTERNAL_ERROR:
		return true
	}
	return false
}

// withReconnect runs op and, if libvirtd dropped the connection, reconnects
// and runs it again. Only use it for idempotent operations.
func (d LibvirtDriver) withReconnect(what string, op func() error) error {
	err := op()
	for attempt := 1; attempt <= d.reconnectAttempts && err != nil && isConnectionLost(err); attempt++ {
		d.logger.Warn(d.logTag, "Connection lost during %s, reconnecting (attempt %d): %s", what, attempt, err)
		time.Sleep(time.Duration(attempt) * d.reconnectDelay)
		if rerr := d.conn.Reconnect(); rerr != nil {
			d.logger.Warn(d.logTag, "Reconnect failed: %s", rerr)
			err = fmt.Errorf("%w (reconnect failed: %s)", err, rerr)
			continue
		}
		err = op()
	}
	return err
}

// DestroyDomain stops and undefines the domain. It is idempotent, so it
// survives libvirtd dropping the connection mid-call (seen with the LXC driver).
func (d LibvirtDriver) DestroyDomain(id string) error {
	err := d.withReconnect("destroy of '"+id+"'", func() error { return d.destroyDomain(id) })
	if err != nil && errors.Is(err, libvirt.ERR_NO_DOMAIN) {
		return nil
	}
	return err
}

func (d LibvirtDriver) destroyDomain(id string) error {
	d.logger.Debug(d.logTag, "Destroying domain '%s'", id)
	return d.withDomain(id, func(dom *libvirt.Domain) error {
		if err := dom.Destroy(); err != nil {
			lverr, ok := err.(libvirt.Error)
			isNotRunning := ok && lverr.Code == libvirt.ERR_OPERATION_INVALID
			if !errors.Is(err, libvirt.ERR_NO_DOMAIN) && !isNotRunning {
				return err
			}
		}
		return dom.Undefine()
	})
}

func (d LibvirtDriver) RebootDomain(id string) error {
	d.logger.Debug(d.logTag, "Rebooting domain '%s'", id)
	return d.withDomain(id, func(dom *libvirt.Domain) error { return dom.Reboot(libvirt.DOMAIN_REBOOT_DEFAULT) })
}

func (d LibvirtDriver) LookupDomain(id string) (Domain, error) {
	d.logger.Debug(d.logTag, "Looking up domain '%s'", id)
	var dom *libvirt.Domain
	err := d.withReconnect("lookup of '"+id+"'", func() error {
		var lerr error
		dom, lerr = d.conn.LookupDomainByName(id)
		return lerr
	})
	if err != nil {
		return nil, err
	}
	return &LibvirtDomainWrapper{dom}, nil
}

func (d LibvirtDriver) UpdateDomainMemory(id string, memoryMB int) error {
	d.logger.Debug(d.logTag, "Updating memory for domain '%s' to %dMB", id, memoryMB)
	return d.withDomain(id, func(dom *libvirt.Domain) error {
		kib := uint64(memoryMB) * 1024
		// Set max first: libvirt enforces current <= max, so raising max before current allows scale-up.
		if err := dom.SetMemoryFlags(kib, libvirt.DOMAIN_MEM_CONFIG|libvirt.DOMAIN_MEM_MAXIMUM); err != nil {
			return err
		}
		return dom.SetMemoryFlags(kib, libvirt.DOMAIN_MEM_CONFIG)
	})
}

func (d LibvirtDriver) UpdateDomainCPUs(id string, cpus int) error {
	d.logger.Debug(d.logTag, "Updating CPUs for domain '%s' to %d", id, cpus)
	return d.withDomain(id, func(dom *libvirt.Domain) error {
		// Set max first: libvirt enforces current <= max, so raising max before current allows scale-up.
		if err := dom.SetVcpusFlags(uint(cpus), libvirt.DOMAIN_VCPU_CONFIG|libvirt.DOMAIN_VCPU_MAXIMUM); err != nil {
			return err
		}
		return dom.SetVcpusFlags(uint(cpus), libvirt.DOMAIN_VCPU_CONFIG)
	})
}

func (d LibvirtDriver) CreateStorageVol(poolName, volName string, sizeMB int) (string, error) {
	d.logger.Debug(d.logTag, "Creating storage vol '%s' in pool '%s'", volName, poolName)
	pool, err := d.conn.LookupStoragePoolByName(poolName)
	if err != nil {
		return "", err
	}
	defer pool.Free() //nolint
	sizeBytes := uint64(sizeMB) * 1024 * 1024
	xml := fmt.Sprintf(`<volume><name>%s</name><capacity unit="bytes">%d</capacity></volume>`, xmlEscape(volName), sizeBytes)
	vol, err := pool.StorageVolCreateXML(xml, 0)
	if err != nil {
		return "", err
	}
	defer vol.Free() //nolint
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
	defer pool.Free() //nolint
	vol, err := pool.LookupStorageVolByName(volName)
	if err != nil {
		if errors.Is(err, libvirt.ERR_NO_STORAGE_VOL) {
			return nil
		}
		return err
	}
	defer vol.Free() //nolint
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

func (w *LibvirtDomainWrapper) Free() error { return w.dom.Free() }

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
