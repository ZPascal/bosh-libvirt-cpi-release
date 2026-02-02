# BOSH Libvirt CPI Release

A BOSH Cloud Provider Interface (CPI) that uses **libvirt** to support multiple virtualization technologies including **QEMU/KVM**, **VirtualBox**, **LXC**, and more.

## Features

- **Multi-Hypervisor Support**: Support for different virtualization backends through libvirt
  - QEMU/KVM - Full virtualization with KVM acceleration
  - VirtualBox - Desktop virtualization via libvirt-vbox
  - LXC - Linux Containers
  - Xen - Xen hypervisor
  - VMware - VMware ESX (experimental)
- **Unified Interface**: Single libvirt-based implementation for all hypervisors
- **Flexible Architecture**: Easy switching between hypervisors via configuration
- **Remote Management**: Support for managing VMs on remote hosts via SSH

## Quick Start

### Prerequisites

```bash
# Install libvirt
sudo apt-get install qemu-kvm libvirt-daemon-system libvirt-clients virtinst

# For VirtualBox support (optional)
sudo apt-get install virtualbox libvirt-daemon-driver-vbox

# For LXC support (optional)
sudo apt-get install lxc libvirt-daemon-driver-lxc

# Start and enable libvirt service
sudo systemctl start libvirtd
sudo systemctl enable libvirtd

# Add user to libvirt group
sudo usermod -aG libvirt $USER
```

### Configuration

Create a CPI configuration file (e.g., `cpi.json`):

**For QEMU/KVM:**
```json
{
  "hypervisor": "qemu",
  "uri": "qemu:///system",
  "bin_path": "virsh",
  "store_dir": "/var/vcap/store/libvirt",
  "storage_controller": "SATA",
  "auto_enable_networks": true,
  "agent": {
    "mbus": "https://mbus:mbus-password@0.0.0.0:6868",
    "ntp": ["0.pool.ntp.org", "1.pool.ntp.org"],
    "blobstore": {
      "provider": "local",
      "options": {
        "blobstore_path": "/var/vcap/micro_bosh/data/cache"
      }
    }
  }
}
```

**For VirtualBox (via libvirt):**
```json
{
  "hypervisor": "vbox",
  "uri": "vbox:///session",
  ...
}
```

**For LXC:**
```json
{
  "hypervisor": "lxc",
  "uri": "lxc:///",
  ...
}
```

## Documentation

- **[Provider Configuration Guide](docs/PROVIDER_CONFIGURATION.md)** - Detailed guide on configuring different hypervisors
- **Configuration Examples**:
  - [QEMU/KVM Local](config/cpi-libvirt.json)
  - [QEMU/KVM Remote](config/cpi-libvirt-remote.json)
  - [VirtualBox](config/cpi-vbox.json)
  - [LXC](config/cpi-lxc.json)

## Architecture

The CPI uses libvirt as a unified interface to different virtualization technologies:

```
BOSH Director
    ↓
CPI Factory
    ↓
Libvirt Provider
    ↓
┌────────────────────────────────────┐
│         Libvirt API                │
└────────────────────────────────────┘
    ↓           ↓           ↓
┌─────────┐ ┌─────────┐ ┌─────────┐
│QEMU/KVM │ │VirtualBox│ │   LXC   │
└─────────┘ └─────────┘ └─────────┘
```

The `hypervisor` configuration field determines which virtualization backend libvirt uses.

## Supported Hypervisors

| Hypervisor | URI Format | Status | Use Case |
|------------|-----------|--------|----------|
| **qemu** (KVM) | `qemu:///system` | ✅ Stable | Production, best performance |
| **vbox** (VirtualBox) | `vbox:///session` | ✅ Stable | Development, desktop |
| **lxc** (Containers) | `lxc:///` | ✅ Stable | Lightweight containers |
| **xen** | `xen:///` | ⚠️ Experimental | Xen environments |
| **vmware** | `vmware:///session` | ⚠️ Experimental | VMware workstation |

## Building

```bash
cd src/bosh-libvirt-cpi
go build -o ../../bin/cpi ./main
```

## Testing

```bash
cd src/bosh-libvirt-cpi
go test ./...
```

## License

See [LICENSE](LICENSE) file.



