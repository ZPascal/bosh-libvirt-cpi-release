package provider

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/qemu"
)

var (
	libvirtErrorRegex = regexp.MustCompile("error:")
)

// LibvirtProvider implements the Provider interface for libvirt
type LibvirtProvider struct {
	runner     driver.Runner
	retrier    driver.Retrier
	fs         boshsys.FileSystem
	opts       ProviderOptions
	driver     driver.Driver
	hypervisor HypervisorType

	uri string // libvirt connection URI

	logTag string
	logger boshlog.Logger
}

// NewLibvirtProvider creates a new libvirt provider with specified hypervisor
func NewLibvirtProvider(
	hypervisor HypervisorType,
	runner driver.Runner,
	retrier driver.Retrier,
	fs boshsys.FileSystem,
	opts ProviderOptions,
	logger boshlog.Logger,
) (*LibvirtProvider, error) {
	if opts.BinPath == "" {
		opts.BinPath = "virsh"
	}

	// Set hypervisor in opts if not already set
	if opts.Hypervisor == "" {
		opts.Hypervisor = hypervisor
	}

	// Get connection URI
	uri := opts.GetConnectionURI()

	execDriver := NewLibvirtDriver(runner, retrier, opts.BinPath, uri, logger)

	return &LibvirtProvider{
		runner:     runner,
		retrier:    retrier,
		fs:         fs,
		opts:       opts,
		driver:     execDriver,
		hypervisor: hypervisor,
		uri:        uri,
		logTag:     "provider.LibvirtProvider",
		logger:     logger,
	}, nil
}

func (p *LibvirtProvider) GetDriver() driver.Driver {
	return p.driver
}

func (p *LibvirtProvider) GetHypervisor() HypervisorType {
	return p.hypervisor
}

func (p *LibvirtProvider) Initialize() error {
	// Check if virsh is available and can connect
	_, err := p.driver.Execute("version")
	if err != nil {
		return bosherr.WrapError(err, "virsh not available or cannot connect")
	}
	return nil
}

func (p *LibvirtProvider) Cleanup() error {
	return nil
}

func (p *LibvirtProvider) CreateVM(name string, opts VMOptions) error {
	// Create VM definition XML
	domainXML := p.createDomainXML(name, opts)

	// Write XML to temporary file
	xmlPath := fmt.Sprintf("/tmp/%s.xml", name)
	err := p.fs.WriteFileString(xmlPath, domainXML)
	if err != nil {
		return bosherr.WrapErrorf(err, "Writing domain XML for VM '%s'", name)
	}
	defer p.fs.RemoveAll(xmlPath)

	// Define the domain
	_, err = p.driver.Execute("define", xmlPath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Defining VM '%s'", name)
	}

	return nil
}

func (p *LibvirtProvider) DeleteVM(name string) error {
	// First ensure VM is stopped
	state, err := p.GetVMState(name)
	if err == nil && state == VMStateRunning {
		p.StopVM(name, true)
	}

	// Undefine the domain
	_, err = p.driver.Execute("undefine", name, "--remove-all-storage")
	return err
}

func (p *LibvirtProvider) StartVM(name string) error {
	_, err := p.driver.Execute("start", name)
	return err
}

func (p *LibvirtProvider) StopVM(name string, force bool) error {
	if force {
		_, err := p.driver.Execute("destroy", name)
		return err
	}
	_, err := p.driver.Execute("shutdown", name)
	return err
}

func (p *LibvirtProvider) GetVMState(name string) (VMState, error) {
	output, err := p.driver.Execute("domstate", name)
	if err != nil {
		return VMStateUnknown, err
	}

	state := strings.TrimSpace(output)
	return parseLibvirtState(state), nil
}

func (p *LibvirtProvider) ModifyVM(name string, opts VMModifyOptions) error {
	// For libvirt, we need to modify the domain XML
	// This is more complex than VirtualBox

	// Get current domain XML
	xmlOutput, err := p.driver.Execute("dumpxml", name)
	if err != nil {
		return bosherr.WrapErrorf(err, "Getting domain XML for '%s'", name)
	}

	// Parse and modify XML
	var domain LibvirtDomain
	err = xml.Unmarshal([]byte(xmlOutput), &domain)
	if err != nil {
		return bosherr.WrapError(err, "Parsing domain XML")
	}

	// Apply modifications
	if opts.Memory != nil {
		domain.Memory.Value = *opts.Memory * 1024 // Convert MB to KB
		domain.CurrentMemory.Value = *opts.Memory * 1024
	}
	if opts.CPUs != nil {
		domain.VCPU.Value = *opts.CPUs
	}

	// Marshal back to XML
	modifiedXML, err := xml.MarshalIndent(domain, "", "  ")
	if err != nil {
		return bosherr.WrapError(err, "Marshaling domain XML")
	}

	// Write to temp file
	xmlPath := fmt.Sprintf("/tmp/%s-modified.xml", name)
	err = p.fs.WriteFileString(xmlPath, string(modifiedXML))
	if err != nil {
		return bosherr.WrapError(err, "Writing modified domain XML")
	}
	defer p.fs.RemoveAll(xmlPath)

	// Redefine the domain
	_, err = p.driver.Execute("define", xmlPath)
	return err
}

func (p *LibvirtProvider) CreateDisk(path string, sizeMB int) error {
	qemuImg := qemu.NewImage()
	return qemuImg.Create(path, sizeMB)
}

func (p *LibvirtProvider) AttachDisk(vmName, diskPath string, port int, device int) error {
	// Generate device name (vda, vdb, vdc, etc.)
	deviceName := fmt.Sprintf("vd%c", 'a'+port)

	_, err := p.driver.Execute(
		"attach-disk", vmName,
		diskPath,
		deviceName,
		"--persistent",
		"--subdriver", "qcow2",
	)
	return err
}

func (p *LibvirtProvider) DetachDisk(vmName string, port int, device int) error {
	deviceName := fmt.Sprintf("vd%c", 'a'+port)

	_, err := p.driver.Execute(
		"detach-disk", vmName,
		deviceName,
		"--persistent",
	)
	return err
}

func (p *LibvirtProvider) CreateNetwork(name string, opts NetworkOptions) error {
	networkXML := p.createNetworkXML(name, opts)

	xmlPath := fmt.Sprintf("/tmp/%s-network.xml", name)
	err := p.fs.WriteFileString(xmlPath, networkXML)
	if err != nil {
		return bosherr.WrapError(err, "Writing network XML")
	}
	defer p.fs.RemoveAll(xmlPath)

	_, err = p.driver.Execute("net-define", xmlPath)
	if err != nil {
		return err
	}

	// Auto-start the network
	_, err = p.driver.Execute("net-autostart", name)
	if err != nil {
		return err
	}

	// Start the network
	_, err = p.driver.Execute("net-start", name)
	return err
}

func (p *LibvirtProvider) DeleteNetwork(name string) error {
	// Stop the network
	p.driver.Execute("net-destroy", name)

	// Undefine the network
	_, err := p.driver.Execute("net-undefine", name)
	return err
}

func (p *LibvirtProvider) AttachNIC(vmName string, nicIndex int, opts NICOptions) error {
	// For libvirt, this requires modifying the domain XML
	// We'll use attach-interface for runtime attachment

	args := []string{
		"attach-interface", vmName,
		opts.Type,
	}

	if opts.NetworkName != "" {
		args = append(args, "--source", opts.NetworkName)
	}

	if opts.MACAddress != "" {
		args = append(args, "--mac", opts.MACAddress)
	}

	args = append(args, "--persistent")

	_, err := p.driver.Execute(args...)
	return err
}

func (p *LibvirtProvider) CreateSnapshot(vmName, snapshotName string) error {
	_, err := p.driver.Execute("snapshot-create-as", vmName, snapshotName)
	return err
}

func (p *LibvirtProvider) DeleteSnapshot(vmName, snapshotName string) error {
	_, err := p.driver.Execute("snapshot-delete", vmName, snapshotName)
	return err
}

func (p *LibvirtProvider) RestoreSnapshot(vmName, snapshotName string) error {
	_, err := p.driver.Execute("snapshot-revert", vmName, snapshotName)
	return err
}

func (p *LibvirtProvider) CloneVM(srcName, dstName string, opts CloneOptions) error {
	args := []string{"virt-clone", "--original", srcName, "--name", dstName, "--auto-clone"}

	// Note: virt-clone is a separate command, not virsh
	output, status, err := p.runner.Execute("virt-clone", args[1:]...)
	if err != nil || status != 0 {
		return bosherr.Errorf("Error cloning VM: %s", output)
	}

	return nil
}

func (p *LibvirtProvider) ExportVM(vmName, outputPath string) error {
	// Export domain XML
	xmlOutput, err := p.driver.Execute("dumpxml", vmName)
	if err != nil {
		return err
	}

	xmlPath := outputPath + ".xml"
	return p.fs.WriteFileString(xmlPath, xmlOutput)
}

func (p *LibvirtProvider) ImportVM(imagePath, vmName string) error {
	// Import from XML
	_, err := p.driver.Execute("define", imagePath)
	return err
}

func (p *LibvirtProvider) GetVMInfo(name string) (VMInfo, error) {
	xmlOutput, err := p.driver.Execute("dumpxml", name)
	if err != nil {
		return VMInfo{}, err
	}

	var domain LibvirtDomain
	err = xml.Unmarshal([]byte(xmlOutput), &domain)
	if err != nil {
		return VMInfo{}, bosherr.WrapError(err, "Parsing domain XML")
	}

	state, _ := p.GetVMState(name)

	info := VMInfo{
		Name:   domain.Name,
		UUID:   domain.UUID,
		State:  state,
		Memory: domain.Memory.Value / 1024, // Convert KB to MB
		CPUs:   domain.VCPU.Value,
	}

	return info, nil
}

func (p *LibvirtProvider) ListVMs() ([]string, error) {
	output, err := p.driver.Execute("list", "--all", "--name")
	if err != nil {
		return nil, err
	}

	var vms []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			vms = append(vms, line)
		}
	}

	return vms, nil
}

// Helper functions

func (p *LibvirtProvider) createDomainXML(name string, opts VMOptions) string {
	memoryKB := opts.Memory * 1024

	// Determine domain type based on hypervisor
	domainType := string(p.hypervisor)
	if p.hypervisor == HypervisorTypeQEMU {
		domainType = "kvm" // Use kvm for better performance with QEMU
	}

	// For LXC, use a simpler template
	if p.hypervisor == HypervisorTypeLXC {
		return fmt.Sprintf(`<domain type='lxc'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <currentMemory unit='KiB'>%d</currentMemory>
  <vcpu placement='static'>%d</vcpu>
  <os>
    <type arch='x86_64'>exe</type>
    <init>/sbin/init</init>
  </os>
  <devices>
    <emulator>/usr/lib/libvirt/libvirt_lxc</emulator>
    <console type='pty'/>
  </devices>
</domain>`, name, memoryKB, memoryKB, opts.CPUs)
	}

	// For VirtualBox, use vbox type
	if p.hypervisor == HypervisorTypeVBox {
		return fmt.Sprintf(`<domain type='vbox'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <currentMemory unit='KiB'>%d</currentMemory>
  <vcpu placement='static'>%d</vcpu>
  <os>
    <type arch='x86_64'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <clock offset='utc'/>
  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>
  <devices>
    <graphics type='rdp' autoport='yes'/>
  </devices>
</domain>`, name, memoryKB, memoryKB, opts.CPUs)
	}

	// Default: KVM/QEMU template
	xml := fmt.Sprintf(`<domain type='%s'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <currentMemory unit='KiB'>%d</currentMemory>
  <vcpu placement='static'>%d</vcpu>
  <os>
    <type arch='x86_64' machine='pc'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <cpu mode='host-passthrough'/>
  <clock offset='utc'>
    <timer name='rtc' tickpolicy='catchup'/>
    <timer name='pit' tickpolicy='delay'/>
    <timer name='hpet' present='no'/>
  </clock>
  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>
  <devices>
    <emulator>/usr/bin/qemu-system-x86_64</emulator>
    <controller type='usb' index='0' model='ich9-ehci1'/>
    <controller type='usb' index='0' model='ich9-uhci1'/>
    <controller type='pci' index='0' model='pci-root'/>
    <input type='tablet' bus='usb'/>
    <input type='mouse' bus='ps2'/>
    <input type='keyboard' bus='ps2'/>
    <graphics type='vnc' port='-1' autoport='yes'/>
    <video>
      <model type='cirrus' vram='16384' heads='1'/>
    </video>
    <memballoon model='virtio'/>
  </devices>
</domain>`, domainType, name, memoryKB, memoryKB, opts.CPUs)

	return xml
}

func (p *LibvirtProvider) createNetworkXML(name string, opts NetworkOptions) string {
	xml := fmt.Sprintf(`<network>
  <name>%s</name>
  <forward mode='nat'/>
  <bridge name='virbr-%s' stp='on' delay='0'/>`, name, name)

	if opts.CIDR != "" {
		// Parse CIDR to extract network address
		// For simplicity, we'll use the CIDR as-is
		xml += fmt.Sprintf(`
  <ip address='%s' netmask='255.255.255.0'>`, opts.CIDR)

		if opts.DHCPEnabled {
			xml += `
    <dhcp>
      <range start='192.168.122.2' end='192.168.122.254'/>
    </dhcp>`
		}

		xml += `
  </ip>`
	}

	xml += `
</network>`

	return xml
}

func parseLibvirtState(state string) VMState {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running":
		return VMStateRunning
	case "shut off", "shutoff":
		return VMStatePowerOff
	case "paused":
		return VMStatePaused
	case "crashed":
		return VMStateAborted
	case "pmsuspended":
		return VMStateSaved
	default:
		return VMStateUnknown
	}
}

// LibvirtDomain represents a libvirt domain XML structure
type LibvirtDomain struct {
	XMLName       xml.Name      `xml:"domain"`
	Type          string        `xml:"type,attr"`
	Name          string        `xml:"name"`
	UUID          string        `xml:"uuid"`
	Memory        MemoryElement `xml:"memory"`
	CurrentMemory MemoryElement `xml:"currentMemory"`
	VCPU          VCPUElement   `xml:"vcpu"`
}

type MemoryElement struct {
	Unit  string `xml:"unit,attr"`
	Value int    `xml:",chardata"`
}

type VCPUElement struct {
	Placement string `xml:"placement,attr"`
	Value     int    `xml:",chardata"`
}
