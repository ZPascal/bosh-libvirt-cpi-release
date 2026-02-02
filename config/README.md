# CPI Configuration Examples

This directory contains example configurations for different hypervisors.

## Available Configurations

### QEMU/KVM (Recommended for Production)

**Local:**
- File: `cpi-libvirt.json`
- URI: `qemu:///system`
- Use case: Production deployments, best performance

**Remote (via SSH):**
- File: `cpi-libvirt-remote.json`
- URI: `qemu+ssh://user@host/system`
- Use case: Remote host management

### VirtualBox (Development)

- File: `cpi-vbox.json`
- URI: `vbox:///session`
- Use case: Desktop development, testing

### LXC (Containers)

- File: `cpi-lxc.json`
- URI: `lxc:///`
- Use case: Lightweight workloads, fast iteration, CI/CD

## Quick Start

1. Choose the hypervisor that fits your needs
2. Copy the corresponding configuration file
3. Adjust parameters (paths, credentials, etc.)
4. Use with BOSH Director

## Configuration Parameters

Common to all hypervisors:

- `hypervisor`: Type of hypervisor (qemu, vbox, lxc, xen, vmware)
- `uri`: Libvirt connection URI (auto-generated if not specified)
- `bin_path`: Path to virsh binary (default: "virsh")
- `store_dir`: Directory for VM/disk storage (required)
- `storage_controller`: Storage controller type (IDE, SCSI, SATA)
- `auto_enable_networks`: Auto-enable network interfaces
- `agent`: BOSH agent configuration

For remote connections, also specify:
- `host`: Remote host address
- `username`: SSH username
- `private_key`: Path to SSH private key

## Examples

### Minimal QEMU/KVM Configuration

```json
{
  "hypervisor": "qemu",
  "store_dir": "/var/vcap/store/libvirt",
  "storage_controller": "SATA",
  "agent": { ... }
}
```

### Switching Hypervisors

Simply change the `hypervisor` field:

```json
{
  "hypervisor": "vbox",  // Change to: qemu, lxc, xen, vmware
  ...
}
```

## Documentation

For detailed configuration options, see:
- [QUICKSTART.md](../QUICKSTART.md)
- [docs/HYPERVISOR_CONFIGURATION.md](../docs/HYPERVISOR_CONFIGURATION.md)
