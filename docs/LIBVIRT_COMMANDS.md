# Libvirt Command Reference - BOSH CPI

## Native libvirt/virsh Commands Used

### VM Lifecycle

```bash
# Define domain (from XML)
virsh define /path/to/domain.xml

# Start domain
virsh start <domain-name>

# Stop domain (immediately)
virsh destroy <domain-name>

# Shut down domain (gracefully)
virsh shutdown <domain-name>

# Restart domain
virsh reboot <domain-name>

# Delete domain (with storage)
virsh undefine <domain-name> --remove-all-storage --snapshots-metadata

# Domain status
virsh domstate <domain-name>

# Domain info
virsh dominfo <domain-name>

# Export domain XML
virsh dumpxml <domain-name>
```

### Configuration

```bash
# Set memory (in KB)
virsh setmaxmem <domain> 2097152 --config  # 2GB
virsh setmem <domain> 2097152 --config

# Set vCPUs
virsh setvcpus <domain> 2 --config --maximum
virsh setvcpus <domain> 2 --config
```

### Disks

```bash
# Attach disk
virsh attach-disk <domain> \
    /path/to/disk.qcow2 \
    vda \
    --persistent \
    --subdriver qcow2

# Detach disk
virsh detach-disk <domain> vda --persistent
```

### Network

```bash
# Attach network interface
virsh attach-interface <domain> \
    network \
    --source default \
    --mac 52:54:00:xx:xx:xx \
    --model virtio \
    --config
```

### Snapshots

```bash
# Create snapshot
virsh snapshot-create-as <domain> \
    snapshot-name \
    --description "Description"

# Delete snapshot
virsh snapshot-delete <domain> snapshot-name

# Revert to snapshot
virsh snapshot-revert <domain> snapshot-name
```

### Disk Tools (qemu-img)

```bash
# Create qcow2 disk
qemu-img create -f qcow2 /path/to/disk.qcow2 10G

# Convert VMDK to qcow2
qemu-img convert -f vmdk -O qcow2 input.vmdk output.qcow2

# Disk info
qemu-img info /path/to/disk.qcow2
```

## Device Naming Conventions

### Virtio (recommended)
- vda, vdb, vdc, vdd, ...
- Best performance
- Paravirtualization

### SCSI
- sda, sdb, sdc, sdd, ...
- Compatibility

### IDE
- hda, hdb, hdc, hdd
- Legacy

## Network Types

- **network**: Libvirt-managed network (NAT, routed)
- **bridge**: Direct bridge attachment
- **direct**: Direct macvtap connection

## Example Domain XML (Minimal)

```xml
<domain type='kvm'>
  <name>my-vm</name>
  <memory unit='KiB'>2097152</memory>
  <vcpu>2</vcpu>
  <os>
    <type arch='x86_64'>hvm</type>
    <boot dev='hd'/>
  </os>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='/var/lib/libvirt/images/disk.qcow2'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <interface type='network'>
      <source network='default'/>
      <model type='virtio'/>
    </interface>
  </devices>
</domain>
```

## Usage in CPI

| Operation | Uses |
|-----------|------|
| **Create VM** | `define` |
| **Start VM** | `start` |
| **Stop VM** | `destroy` |
| **Delete VM** | `undefine --remove-all-storage` |
| **Change memory** | `setmem`, `setmaxmem` |
| **Change CPUs** | `setvcpus` |
| **Create disk** | `qemu-img create` |
| **Attach disk** | `attach-disk` |
| **Detach disk** | `detach-disk` |
| **Attach NIC** | `attach-interface` |
| **Snapshot** | `snapshot-create-as` |
| **Status** | `domstate` |
| **Info** | `dominfo` |

## Performance Tips

1. **Use Virtio**: Always use virtio devices for best performance
2. **qcow2 format**: Native snapshots, compression, thin provisioning
3. **KVM acceleration**: CPU host-passthrough mode
4. **Cache modes**: For disks use writeback/writethrough as needed

## Troubleshooting

```bash
# List all domains
virsh list --all

# Check domain XML
virsh dumpxml <domain> | less

# Libvirt logs
journalctl -u libvirtd -f

# QEMU logs
tail -f /var/log/libvirt/qemu/<domain>.log
```