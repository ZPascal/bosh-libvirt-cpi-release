# BOSH Libvirt CPI Release

A BOSH Cloud Provider Interface (CPI) that uses **libvirt** to support multiple virtualization technologies including **QEMU/KVM**, **LXC**

## Features

- **Multi-Hypervisor Support**: Support for different virtualization backends through libvirt
  - QEMU/KVM - Full virtualization with KVM acceleration
  - LXC - Linux Containers
- **Unified Interface**: Single libvirt-based implementation for all hypervisors
- **Flexible Architecture**: Easy switching between hypervisors via configuration
- **Remote Management**: Support for managing VMs on remote hosts via SSH

## Choosing a backend

| Backend | URI | Use case | Disk format |
|---------|-----|----------|-------------|
| QEMU/KVM | `qemu:///system` | Production workloads on KVM-capable Linux hosts | qcow2 |
| LXC | `lxc:///` | Container workloads, low overhead, shared kernel | raw |

See [docs/HYPERVISOR_CONFIGURATION.md](docs/HYPERVISOR_CONFIGURATION.md) for per-backend installation prerequisites and known limitations.

## Quick Start

### Prerequisites

```bash
# Install libvirt
sudo apt-get install qemu-kvm libvirt-daemon-system libvirt-clients virtinst

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

For remote libvirt hosts add `Host`, `Username`, `PrivateKey`, and `HostKey` fields - see [`config/cpi-qemu-remote.json`](config/cpi-qemu-remote.json).


## Getting started as a user

To run a BOSH Director with this CPI on your own machine, follow [docs/INSTALL.md](docs/INSTALL.md). It covers host prerequisites, CPI configuration, and `bosh create-env` with the ops files in `manifests/`.

## Set up a BOSH Director in GitHub Actions

The reusable workflow `setup-bosh-director.yml` bootstraps a BOSH Director backed by this CPI (QEMU/KVM or LXC) on a GitHub Actions runner and returns its credentials as job outputs. Call it from your own repository:

```yaml
jobs:
  director:
    uses: ZPascal/bosh-libvirt-cpi-release/.github/workflows/setup-bosh-director.yml@main
    with:
      backend: qemu   # or lxc
      # cpi_ref: v1.2.3   # pin a CPI tag/branch/SHA (default: main)
```

If you need the Director in the same job as your own steps, use the composite action `.github/actions/setup-bosh-director` instead (see the docs). Outputs include `director-url`, `director-ca-cert`, `bosh-client`, `bosh-client-secret` and `bosh-env-alias`. The director's libvirt bridge (`192.168.122.x`) is only reachable from the machine that created it, so run your BOSH steps on that same runner. Inputs, outputs and a full example: [docs/e2e-backends.md](docs/e2e-backends.md#setup-bosh-director-workflow-setup-bosh-directoryml).

## CI (contributors)

The project's own pipeline (`.github/workflows/tests.yml` and `e2e.yml`) runs a throwaway director plus smoke test for QEMU and LXC on every push and PR. You do not need it to use the CPI; see [docs/e2e-backends.md](docs/e2e-backends.md).

## Documentation

- **[Installation Guide](docs/INSTALL.md)** - Installation and Director deployment
- **[CI/CD Workflows](docs/e2e-backends.md)** - E2E pipeline and the reusable `setup-bosh-director` workflow
- **[Hypervisor Configuration Guide](docs/HYPERVISOR_CONFIGURATION.md)** - Per-backend installation prerequisites and known limitations
- **Configuration Examples**:
  - [QEMU/KVM Local](config/cpi-qemu.json)
  - [QEMU/KVM Remote](config/cpi-qemu-remote.json)
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
    ↓           ↓           
┌─────────┐ ┌─────────┐ 
│QEMU/KVM │ │LXC      │
└─────────┘ └─────────┘
```

The `BackendURI` configuration field determines which virtualization backend libvirt uses.

## Supported Hypervisors

| Hypervisor | URI Format | Status | Use Case |
|------------|-----------|--------|----------|
| **lxc** (Containers) | `lxc:///` | ✅ Stable | Lightweight containers, fast setup |
| **qemu** (KVM) | `qemu:///system` | ✅ Stable | ARM support, good performance |

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



