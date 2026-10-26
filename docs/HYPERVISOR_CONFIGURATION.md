# BOSH Libvirt CPI - Hypervisor Configuration Guide

This guide explains how to configure the BOSH Libvirt CPI to use different hypervisors through libvirt.

## Overview

The BOSH Libvirt CPI uses libvirt as a unified interface to multiple virtualization technologies. You can easily switch between different hypervisors by changing the `hypervisor` field in your configuration.

## Supported Hypervisors

| Hypervisor | Type | Status | Best For |
|------------|------|--------|----------|
| **lxc** (Containers) | `lxc:///` | ✅ Stable | Lightweight containers, fast setup |
| **qemu** (KVM) | `qemu:///system` | ✅ Stable | ARM support, good performance |

## Configuration Parameters

### Common Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `hypervisor` | string | No | `qemu` | Hypervisor type: qemu, vbox, lxc, xen, vmware |
| `uri` | string | No | Auto-generated | Libvirt connection URI |
| `bin_path` | string | No | `virsh` | Path to virsh binary |
| `store_dir` | string | Yes | - | Directory for VM and disk storage |
| `storage_controller` | string | Yes | - | Storage controller type (IDE, SCSI, SATA) |
| `auto_enable_networks` | boolean | No | `false` | Auto-enable network interfaces |
| `agent` | object | Yes | - | BOSH agent configuration |

### Remote Connection Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `host` | string | For remote | Remote host address |
| `username` | string | For remote | SSH username |
| `private_key` | string | For remote | Path to SSH private key |

### URI Auto-Generation

If you don't specify a `uri`, it will be automatically generated based on the `hypervisor`:

- `qemu` → `qemu:///system`
- `lxc` → `lxc:///`

## Hypervisor-Specific Configuration

### QEMU/KVM (Production)

**Prerequisites:**
```bash
# Install QEMU/KVM and libvirt
sudo apt-get install qemu-kvm libvirt-daemon-system libvirt-clients

# Start libvirt
sudo systemctl start libvirtd
sudo systemctl enable libvirtd

# Add user to groups
sudo usermod -aG libvirt $USER
sudo usermod -aG kvm $USER
```

**Configuration (Local):**
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

**Configuration (Remote via SSH):**
```json
{
  "hypervisor": "qemu",
  "uri": "qemu+ssh://user@remote-host/system",
  "host": "remote-host",
  "username": "user",
  "private_key": "/home/user/.ssh/id_rsa",
  "bin_path": "virsh",
  "store_dir": "/var/vcap/store/libvirt",
  "storage_controller": "SATA",
  "auto_enable_networks": true,
  "agent": { ... }
}
```

**Features:**
- Full hardware virtualization with KVM acceleration
- Best performance for production workloads
- Support for live migration
- Advanced CPU and memory management
- qcow2 disk format with compression and snapshots

### LXC (Containers)

**Prerequisites:**
```bash
# Install LXC and libvirt-lxc driver
sudo apt-get install lxc lxc-templates
sudo apt-get install libvirt-daemon-driver-lxc

# Restart libvirt
sudo systemctl restart libvirtd
```

**Configuration:**
```json
{
  "hypervisor": "lxc",
  "uri": "lxc:///",
  "bin_path": "virsh",
  "store_dir": "/var/vcap/store/libvirt-lxc",
  "storage_controller": "SATA",
  "auto_enable_networks": true,
  "agent": { ... }
}
```

**Features:**
- Lightweight OS-level virtualization
- Fast startup times
- Lower resource overhead
- Shared kernel with host
- Good for development and CI/CD

**Limitations:**
- Linux containers only
- Less isolation than full VMs
- Shared kernel version with host

## Network Configuration

### Default Networks

Each hypervisor may have different default network setups:

**QEMU/KVM:**
- Default network: `default` (virbr0)
- NAT networking enabled by default
- DHCP range: 192.168.122.2-254

**VirtualBox:**
- Uses VirtualBox's network adapters
- Host-only, NAT, or bridged modes

**LXC:**
- Bridge networking (lxcbr0)
- Direct host networking possible

### Creating Custom Networks

```bash
# Create network definition
cat > /tmp/bosh-net.xml <<EOF
<network>
  <name>bosh-network</name>
  <forward mode='nat'/>
  <bridge name='virbr-bosh' stp='on' delay='0'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
    </dhcp>
  </ip>
</network>
EOF

# Define and start network
virsh net-define /tmp/bosh-net.xml
virsh net-start bosh-network
virsh net-autostart bosh-network
```

## Storage Configuration

### Storage Locations

By default, VMs and disks are stored in `store_dir`:
- VMs: `{store_dir}/vms/`
- Disks: `{store_dir}/disks/`
- Stemcells: `{store_dir}/stemcells/`

### Disk Formats

Different hypervisors support different disk formats:

| Hypervisor | Default Format | Alternatives |
|------------|---------------|--------------|
| QEMU/KVM | qcow2 | raw, qcow, vmdk |
| VirtualBox | VMDK | VDI, VHD |
| LXC | Directory | - |
| Xen | raw/qcow2 | - |

## Performance Tuning

### QEMU/KVM Optimization

```bash
# Enable KVM nested virtualization (if needed)
sudo modprobe -r kvm_intel
sudo modprobe kvm_intel nested=1

# Verify
cat /sys/module/kvm_intel/parameters/nested

# Make permanent
echo "options kvm_intel nested=1" | sudo tee /etc/modprobe.d/kvm.conf
```

**VM Configuration:**
- Use virtio drivers for best performance
- Enable CPU host-passthrough mode
- Use huge pages for large VMs
- Enable disk caching (write-back mode)

### VirtualBox Optimization

- Enable VT-x/AMD-V in BIOS
- Allocate sufficient RAM
- Use paravirtualization provider
- Enable nested paging

## Troubleshooting

### Connection Issues

```bash
# Test libvirt connection
virsh -c qemu:///system list --all
virsh -c lxc:/// list --all

# Check libvirt service
sudo systemctl status libvirtd

# View logs
sudo journalctl -u libvirtd -f
```

### Permission Issues

```bash
# Check groups
groups $USER

# Add to libvirt group
sudo usermod -aG libvirt $USER

# For KVM
sudo usermod -aG kvm $USER

# Relogin or use
newgrp libvirt
```

### Hypervisor-Specific Issues

**QEMU/KVM:**
```bash
# Verify KVM module loaded
lsmod | grep kvm

# Check CPU virtualization support
egrep -c '(vmx|svm)' /proc/cpuinfo
```

**LXC:**
```bash
# Check LXC installation
lxc-checkconfig

# List containers
lxc-ls -f
```

## Migration Between Hypervisors

To switch from one hypervisor to another:

1. **Backup existing deployments**
2. **Update CPI configuration:**
   ```json
   {
     "hypervisor": "qemu",  // Change this
     "uri": "qemu:///system",  // Update URI
     ...
   }
   ```
3. **Redeploy BOSH Director**
4. **Redeploy workloads**

**Note:** VMs cannot be directly migrated between different hypervisors. You need to redeploy.

## Best Practices

1. **Production:** Use QEMU/KVM for best performance and stability
2. **Development:** Use VirtualBox or LXC for easy setup
3. **CI/CD:** Use LXC for fast, lightweight testing
4. **Remote Management:** Use QEMU with SSH for remote deployments
5. **Security:** Use isolated networks and proper firewall rules
6. **Backup:** Regularly backup `store_dir`
7. **Monitoring:** Monitor libvirt logs and VM metrics

## Example Configurations

See the `config/` directory for complete examples:
- `cpi-libvirt.json` - QEMU/KVM local
- `cpi-libvirt-remote.json` - QEMU/KVM remote
- `cpi-lxc.json` - LXC containers

## Further Reading

- [Libvirt Documentation](https://libvirt.org/docs.html)
- [KVM Documentation](https://www.linux-kvm.org/page/Documents)
- [LXC Libvirt Driver](https://libvirt.org/drvlxc.html)
- [BOSH CPI Documentation](https://bosh.io/docs/cpi-api-v1/)
