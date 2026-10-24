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

## Choosing a backend

| Backend | URI | Use case | Disk format |
|---------|-----|----------|-------------|
| QEMU/KVM | `qemu:///system` | Production workloads on KVM-capable Linux hosts | qcow2 |
| VirtualBox | `vbox:///session` | Desktop development on macOS or Windows (no KVM available) | vmdk |
| LXC | `lxc:///` | Container workloads, low overhead, shared kernel | raw |

See [docs/HYPERVISOR_CONFIGURATION.md](docs/HYPERVISOR_CONFIGURATION.md) for per-backend installation prerequisites and known limitations.

For how these backends are exercised in CI (the reusable E2E workflow), see [docs/e2e-backends.md](docs/e2e-backends.md).

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

Create a CPI configuration file (e.g., `config/cpi.json`):

**For QEMU/KVM (local):**
```json
{
  "BackendURI": "qemu:///system",
  "StoreDir": "~/.bosh_libvirt_cpi",
  "Agent": {
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
  "BackendURI": "vbox:///session",
  "StoreDir": "~/.bosh_libvirt_cpi",
  "Agent": {
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

**For LXC:**
```json
{
  "BackendURI": "lxc:///",
  "StoreDir": "~/.bosh_libvirt_cpi",
  "Agent": {
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

For remote libvirt hosts add `Host`, `Username`, `PrivateKey`, and `HostKey` fields — see [`config/cpi-qemu-remote.json`](config/cpi-qemu-remote.json).


## Documentation

- **[Hypervisor Configuration Guide](docs/HYPERVISOR_CONFIGURATION.md)** - Per-backend installation prerequisites and known limitations
- **Configuration Examples**:
  - [QEMU/KVM Local](config/cpi-qemu.json)
  - [QEMU/KVM Remote](config/cpi-qemu-remote.json)
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
│QEMU/KVM │ │VBox     │ │ LXC     │
└─────────┘ └─────────┘ └─────────┘
```

The `BackendURI` configuration field determines which virtualization backend libvirt uses.

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



