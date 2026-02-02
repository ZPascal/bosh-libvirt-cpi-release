# Libvirt Command Reference - BOSH CPI

## Verwendete native libvirt/virsh Befehle

### VM-Lifecycle

```bash
# Domain definieren (aus XML)
virsh define /path/to/domain.xml

# Domain starten
virsh start <domain-name>

# Domain stoppen (sofort)
virsh destroy <domain-name>

# Domain herunterfahren (sauber)
virsh shutdown <domain-name>

# Domain neu starten
virsh reboot <domain-name>

# Domain löschen (mit Storage)
virsh undefine <domain-name> --remove-all-storage --snapshots-metadata

# Domain-Status
virsh domstate <domain-name>

# Domain-Info
virsh dominfo <domain-name>

# Domain-XML exportieren
virsh dumpxml <domain-name>
```

### Konfiguration

```bash
# Speicher setzen (in KB)
virsh setmaxmem <domain> 2097152 --config  # 2GB
virsh setmem <domain> 2097152 --config

# vCPUs setzen
virsh setvcpus <domain> 2 --config --maximum
virsh setvcpus <domain> 2 --config
```

### Disks

```bash
# Disk anhängen
virsh attach-disk <domain> \
    /path/to/disk.qcow2 \
    vda \
    --persistent \
    --subdriver qcow2

# Disk entfernen
virsh detach-disk <domain> vda --persistent
```

### Netzwerk

```bash
# Netzwerk-Interface anhängen
virsh attach-interface <domain> \
    network \
    --source default \
    --mac 52:54:00:xx:xx:xx \
    --model virtio \
    --config
```

### Snapshots

```bash
# Snapshot erstellen
virsh snapshot-create-as <domain> \
    snapshot-name \
    --description "Description"

# Snapshot löschen
virsh snapshot-delete <domain> snapshot-name

# Zu Snapshot zurückkehren
virsh snapshot-revert <domain> snapshot-name
```

### Disk-Tools (qemu-img)

```bash
# qcow2-Disk erstellen
qemu-img create -f qcow2 /path/to/disk.qcow2 10G

# VMDK zu qcow2 konvertieren
qemu-img convert -f vmdk -O qcow2 input.vmdk output.qcow2

# Disk-Info
qemu-img info /path/to/disk.qcow2
```

## Device-Naming-Konventionen

### Virtio (empfohlen)
- vda, vdb, vdc, vdd, ...
- Beste Performance
- Paravirtualisierung

### SCSI
- sda, sdb, sdc, sdd, ...
- Kompatibilität

### IDE
- hda, hdb, hdc, hdd
- Legacy

## Netzwerk-Typen

- **network**: Libvirt-managed network (NAT, routed)
- **bridge**: Direct bridge attachment
- **direct**: Direct macvtap connection

## Beispiel Domain-XML (Minimal)

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

## Verwendung im CPI

| Operation | Verwendet |
|-----------|-----------|
| **VM erstellen** | `define` |
| **VM starten** | `start` |
| **VM stoppen** | `destroy` |
| **VM löschen** | `undefine --remove-all-storage` |
| **Speicher ändern** | `setmem`, `setmaxmem` |
| **CPUs ändern** | `setvcpus` |
| **Disk erstellen** | `qemu-img create` |
| **Disk anhängen** | `attach-disk` |
| **Disk entfernen** | `detach-disk` |
| **NIC anhängen** | `attach-interface` |
| **Snapshot** | `snapshot-create-as` |
| **Status** | `domstate` |
| **Info** | `dominfo` |

## Performance-Tipps

1. **Virtio verwenden**: Immer virtio-Geräte für beste Performance
2. **qcow2-Format**: Native Snapshots, Kompression, Thin Provisioning
3. **KVM-Beschleunigung**: CPU host-passthrough mode
4. **Cache-Modi**: Für Disks writeback/writethrough je nach Bedarf

## Troubleshooting

```bash
# Alle Domains auflisten
virsh list --all

# Domain-XML prüfen
virsh dumpxml <domain> | less

# Libvirt-Logs
journalctl -u libvirtd -f

# QEMU-Logs
tail -f /var/log/libvirt/qemu/<domain>.log
```
