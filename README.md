# BOSH Libvirt CPI Release

A BOSH Cloud Provider Interface (CPI) that uses **libvirt** to support multiple virtualization technologies including **QEMU/KVM**, **LXC**

## Features

- **Multi-Hypervisor Support**: Support for different virtualization backends through libvirt
  - QEMU/KVM - Full virtualization with KVM acceleration
  - VirtualBox - Desktop virtualization via libvirt-vbox
  - LXC - Linux Containers
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


## CI

Tests run on every push and PR to `main`. The pipeline is defined in `.github/workflows/tests.yml` and calls two reusable workflows:

**E2E (`e2e.yml`)** — spins up a real BOSH director on the CI runner, deploys a smoke test, and tears everything down. Runs for QEMU, LXC, and VBox backends in parallel. Everything runs in a single job: the director VM lives inside the runner's libvirt bridge network and is unreachable from any other runner, so setup, tests, and teardown must share one machine.

```yaml
jobs:
  e2e:
    uses: ./.github/workflows/e2e.yml
    with:
      backend: qemu   # or lxc, vbox
```

**Setup BOSH Director (`setup-bosh-director.yml`)** — bootstraps a director and emits its credentials as job outputs. Intended for callers that run their own persistent runner and want to drive their own test steps against a live director. Cannot be combined with `e2e.yml` — both require the same runner for libvirt networking to work.

Full workflow documentation (inputs, outputs, step-by-step breakdown) is in [docs/e2e-backends.md](docs/e2e-backends.md).

## Documentation

- **[CI/CD Workflows](docs/e2e-backends.md)** - E2E workflow inputs, backend comparison, diagnostics
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
| **vbox** (VirtualBox) | `vbox:///session` | ✅ Stable | Development, desktop (macOS/Windows) |
| **lxc** (Containers) | `lxc:///` | ✅ Stable | Lightweight containers, fast setup |

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



