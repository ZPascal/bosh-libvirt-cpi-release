# BOSH Libvirt CPI - Architecture

## Overview

The BOSH Libvirt CPI provides a unified interface to multiple virtualization technologies through libvirt. This architecture leverages libvirt's abstraction layer to support various hypervisors without code duplication.

## Architecture Diagram

```
┌─────────────────────────────────────────────┐
│          BOSH Director                      │
│  (Orchestrates deployments)                 │
└────────────────┬────────────────────────────┘
                 │ CPI API
                 │
┌────────────────▼────────────────────────────┐
│          BOSH CPI (Go)                      │
│  ┌────────────────────────────────────┐    │
│  │      CPI Factory                   │    │
│  │  - Configuration parsing           │    │
│  │  - Provider instantiation          │    │
│  └────────────┬───────────────────────┘    │
│               │                             │
│  ┌────────────▼───────────────────────┐    │
│  │   Libvirt Provider                 │    │
│  │  - VM lifecycle management         │    │
│  │  - Disk operations                 │    │
│  │  - Network management              │    │
│  │  - Hypervisor-specific templates   │    │
│  └────────────┬───────────────────────┘    │
│               │                             │
│  ┌────────────▼───────────────────────┐    │
│  │   Libvirt Driver                   │    │
│  │  - virsh command execution         │    │
│  │  - Error handling & retries        │    │
│  │  - Connection management           │    │
│  └────────────┬───────────────────────┘    │
└───────────────┼─────────────────────────────┘
                │ virsh CLI
                │
┌───────────────▼─────────────────────────────┐
│          Libvirt Daemon                     │
│  (libvirtd)                                 │
│  ┌──────────────────────────────────────┐  │
│  │   Libvirt Core                       │  │
│  │   - API abstraction                  │  │
│  │   - XML domain management            │  │
│  │   - Resource management              │  │
│  │   - Security & permissions           │  │
│  └──────────┬───────────────────────────┘  │
└─────────────┼───────────────────────────────┘
              │
    ┌─────────┼─────────┬─────────┐
    │         │         │         │
┌───▼────┐ ┌──▼───┐ ┌──▼──┐ ┌────▼────┐
│ QEMU/  │ │VBox  │ │ LXC │ │   Xen   │
│  KVM   │ │      │ │     │ │         │
└────────┘ └──────┘ └─────┘ └─────────┘
   VMs       VMs    Containers  VMs
```

## Component Details

### 1. CPI Factory

**Responsibilities:**
- Parse CPI configuration
- Validate configuration parameters
- Instantiate libvirt provider with correct hypervisor
- Manage SSH connections for remote hosts

**Key Code:**
```go
// cpi/factory.go
func (f Factory) New(ctx apiv1.CallContext) (apiv1.CPI, error) {
    // Setup runner (local or SSH)
    // Create libvirt provider
    // Initialize driver
    // Return CPI implementation
}
```

### 2. Libvirt Provider

**Responsibilities:**
- Implement BOSH CPI interface
- VM lifecycle management (create, delete, start, stop)
- Disk operations (create, attach, detach)
- Network configuration
- Generate hypervisor-specific XML templates
- Snapshot management

**Key Features:**
- Single implementation for all hypervisors
- Hypervisor-specific domain XML generation
- Automatic URI generation based on hypervisor type
- Support for local and remote connections

**Key Code:**
```go
// provider/libvirt_provider.go
type LibvirtProvider struct {
    hypervisor HypervisorType  // qemu, vbox, lxc, etc.
    uri        string           // Connection URI
    driver     driver.Driver    // Command executor
    // ...
}

func (p *LibvirtProvider) CreateVM(name string, opts VMOptions) error {
    xml := p.createDomainXML(name, opts)  // Hypervisor-specific
    // Define domain via virsh
}
```

### 3. Libvirt Driver

**Responsibilities:**
- Execute virsh commands
- Handle connection URIs
- Retry logic for transient failures
- Error parsing and reporting

**Key Code:**
```go
// provider/libvirt_driver.go
func (d LibvirtDriver) Execute(args ...string) (string, error) {
    fullArgs := []string{"-c", d.uri}
    fullArgs = append(fullArgs, args...)
    return d.runner.Execute(d.binPath, fullArgs...)
}
```

### 4. Hypervisor Support

Each hypervisor is supported through libvirt's driver system:

| Hypervisor | Libvirt Driver | Domain Type | VM Format |
|------------|---------------|-------------|-----------|
| QEMU/KVM | qemu | kvm | qcow2 |
| VirtualBox | vbox | vbox | VMDK/VDI |
| LXC | lxc | lxc | Directory |
| Xen | xen | xen | raw/qcow2 |
| VMware | vmware | vmware | VMDK |

## Data Flow

### VM Creation Flow

```
BOSH Director
    │
    │ CreateVM(stemcell_cid, cloud_properties, networks, ...)
    ▼
CPI Factory
    │
    │ Delegate to Provider
    ▼
Libvirt Provider
    │
    ├─► Generate hypervisor-specific XML
    │   (QEMU: kvm type with virtio)
    │   (VBox: vbox type with RDP)
    │   (LXC: lxc type with container init)
    │
    ├─► Write XML to temp file
    │
    ├─► Execute: virsh -c URI define /tmp/domain.xml
    │
    └─► Return VM CID
        │
        ▼
    Libvirt Driver
        │
        │ virsh -c qemu:///system define /tmp/domain.xml
        ▼
    Libvirt Daemon
        │
        │ Parse XML, validate, store configuration
        ▼
    Hypervisor
        │
        └─► VM created but not started
```

### Disk Attachment Flow

```
BOSH Director
    │ AttachDisk(vm_cid, disk_cid)
    ▼
Libvirt Provider
    │
    ├─► Determine device name (vda, vdb, ...)
    │
    ├─► Execute: virsh attach-disk VM_NAME DISK_PATH DEVICE --persistent
    │
    └─► Update VM metadata
        │
        ▼
    Hypervisor
        │
        └─► Disk attached and available to VM
```

## Configuration Flow

```
cpi.json
    │
    ├─► hypervisor: "qemu"
    ├─► uri: "qemu:///system"  (or auto-generated)
    ├─► store_dir: "/var/vcap/store/libvirt"
    └─► agent: { ... }
        │
        ▼
    CPI Factory (factory.go)
        │
        ├─► Parse configuration
        ├─► Validate hypervisor type
        └─► Set defaults
            │
            ▼
        Provider Factory
            │
            └─► Create LibvirtProvider
                    │
                    ├─► hypervisor = HypervisorTypeQEMU
                    ├─► uri = "qemu:///system"
                    └─► Initialize driver
                        │
                        ▼
                    Ready to accept CPI calls
```

## Hypervisor-Specific Templates

The provider generates different XML templates based on hypervisor:

### QEMU/KVM Template
```xml
<domain type='kvm'>
  <name>vm-name</name>
  <memory>2097152</memory>  <!-- 2GB in KB -->
  <vcpu>2</vcpu>
  <cpu mode='host-passthrough'/>
  <devices>
    <emulator>/usr/bin/qemu-system-x86_64</emulator>
    <!-- virtio devices for performance -->
  </devices>
</domain>
```

### VirtualBox Template
```xml
<domain type='vbox'>
  <name>vm-name</name>
  <memory>2097152</memory>
  <vcpu>2</vcpu>
  <devices>
    <graphics type='rdp' autoport='yes'/>
  </devices>
</domain>
```

### LXC Template
```xml
<domain type='lxc'>
  <name>container-name</name>
  <memory>2097152</memory>
  <os>
    <type arch='x86_64'>exe</type>
    <init>/sbin/init</init>
  </os>
  <devices>
    <emulator>/usr/lib/libvirt/libvirt_lxc</emulator>
  </devices>
</domain>
```

## Extension Points

### Adding a New Hypervisor

1. **Add hypervisor type:**
   ```go
   // provider/interfaces.go
   const HypervisorTypeNewHV HypervisorType = "newhv"
   ```

2. **Update URI auto-generation:**
   ```go
   // provider/factory.go
   func (o ProviderOptions) GetConnectionURI() string {
       switch o.Hypervisor {
       case HypervisorTypeNewHV:
           return "newhv:///system"
       // ...
       }
   }
   ```

3. **Add XML template:**
   ```go
   // provider/libvirt_provider.go
   func (p *LibvirtProvider) createDomainXML(...) {
       if p.hypervisor == HypervisorTypeNewHV {
           return `<domain type='newhv'>...</domain>`
       }
   }
   ```

4. **Update validation:**
   ```go
   // cpi/factory_options.go
   validHypervisors := []string{"qemu", "vbox", "lxc", "newhv"}
   ```

## Security Considerations

### Access Control
- Libvirt uses PolicyKit for authorization
- SSH keys for remote access
- File permissions on `store_dir`

### Network Isolation
- Default NAT networks isolate VMs
- Custom bridge networks for specific topologies
- Firewall rules managed by libvirt

### Storage Security
- qcow2 encryption support (QEMU)
- Secure disk deletion
- Proper permissions on disk files

## Performance Characteristics

### QEMU/KVM
- **Startup time:** 5-15 seconds
- **Overhead:** ~5-10%
- **Best for:** Production workloads

### VirtualBox
- **Startup time:** 10-30 seconds
- **Overhead:** ~10-15%
- **Best for:** Development

### LXC
- **Startup time:** 1-3 seconds
- **Overhead:** <1%
- **Best for:** Fast iteration, CI/CD

## Monitoring & Debugging

### Libvirt Logs
```bash
# Daemon logs
sudo journalctl -u libvirtd -f

# QEMU logs
tail -f /var/log/libvirt/qemu/vm-name.log
```

### CPI Logs
```bash
# BOSH Director logs
tail -f /var/vcap/sys/log/director/director.log
```

### Debugging Commands
```bash
# List VMs
virsh -c qemu:///system list --all

# Get VM info
virsh -c qemu:///system dominfo vm-name

# View VM XML
virsh -c qemu:///system dumpxml vm-name

# Check networks
virsh -c qemu:///system net-list --all
```

## Future Enhancements

- Native libvirt Go bindings (libvirt-go)
- Advanced network topologies
- Storage pool management
- Live migration support
- Enhanced monitoring and metrics
- GPU passthrough support
- NUMA awareness

## References

- [Libvirt Architecture](https://libvirt.org/goals.html)
- [BOSH CPI API](https://bosh.io/docs/cpi-api-v1/)
- [QEMU Documentation](https://www.qemu.org/docs/master/)
- [Libvirt Drivers](https://libvirt.org/drivers.html)
