# BOSH Libvirt CPI — Installation and Configuration Guide

This guide covers everything needed to install the CPI binary on a host machine, configure it for QEMU/KVM or LXC, and connect it to a BOSH Director. It also covers remote-host setups where the Director runs inside a VM and controls libvirt on the host over SSH.

---

## Table of contents

1. [How the CPI fits into BOSH](#1-how-the-cpi-fits-into-bosh)
2. [Choosing a backend](#2-choosing-a-backend)
3. [Linux — QEMU/KVM](#3-linux--qemukvm)
4. [Linux — LXC](#4-linux--lxc)
5. [macOS — VirtualBox via libvirt](#5-macos--virtualbox-via-libvirt)
6. [Windows — not supported](#6-windows--not-supported)
7. [Remote-host mode (Director inside a VM)](#7-remote-host-mode-director-inside-a-vm)
8. [CPI configuration reference](#8-cpi-configuration-reference)
9. [BOSH manifest integration](#9-bosh-manifest-integration)
10. [Storage layout](#10-storage-layout)
11. [Networking](#11-networking)
12. [Kernel-boot mode (QEMU only)](#12-kernel-boot-mode-qemu-only)
13. [Troubleshooting](#13-troubleshooting)

---

## 1. How the CPI fits into BOSH

The CPI is the binary that BOSH Director calls whenever it needs to create, delete, or inspect infrastructure. The Director does not talk to libvirt directly — it forks the CPI binary, writes a JSON request to its stdin, and reads the response from stdout.

```
BOSH Director
    │  fork + stdin/stdout JSON RPC
    ▼
bosh-libvirt-cpi  ──── libvirt API (local socket or SSH tunnel) ────▶  libvirtd
                                                                           │
                                                        ┌──────────────────┼──────────────────┐
                                                        ▼                  ▼                  ▼
                                                    QEMU/KVM             LXC            VirtualBox
```

The binary reads a single JSON config file on startup. That file's path is passed via the `-configPath` flag and is set in the CPI job template when you deploy with `bosh create-env`.

---

## 2. Choosing a backend

| Backend | URI | OS requirement | Disk format | Good for |
|---------|-----|---------------|-------------|----------|
| QEMU/KVM | `qemu:///system` | Linux with KVM | qcow2 | Production, full isolation |
| QEMU/KVM kernel-boot | `qemu:///system` | Linux with KVM | ext4 raw | Package compilation VMs |
| LXC | `lxc:///` | Linux only | directory tree | Low-overhead CI workloads |
| VirtualBox | `vbox:///session` | macOS or Linux | vmdk | Local development |

Stick to QEMU/KVM unless you have a specific reason not to. LXC shares the host kernel so it starts faster but offers less isolation. VirtualBox is the only option on macOS because KVM requires hardware virtualisation extensions that the macOS hypervisor does not expose to user-space libvirt.

---

## 3. Linux — QEMU/KVM

### 3.1 Verify hardware support

```bash
egrep -c '(vmx|svm)' /proc/cpuinfo
```

Any number above 0 means the CPU supports virtualisation. If the result is 0, KVM will not work — use QEMU in TCG (software emulation) mode or switch to LXC.

```bash
ls /dev/kvm
```

If the device does not exist, load the module:

```bash
sudo modprobe kvm_intel   # Intel CPUs
sudo modprobe kvm_amd     # AMD CPUs
```

### 3.2 Install packages

**Debian / Ubuntu:**

```bash
sudo apt-get update
sudo apt-get install -y \
    qemu-kvm \
    libvirt-daemon-system \
    libvirt-clients \
    virtinst \
    libvirt-dev \
    e2fsprogs \
    bridge-utils
```

`e2fsprogs` provides `e2fsck`, `tune2fs`, and `resize2fs`. The CPI uses these when it creates ext4 root disks for kernel-boot VMs.

**Fedora / RHEL / Rocky:**

```bash
sudo dnf install -y \
    qemu-kvm \
    libvirt \
    libvirt-devel \
    virt-install \
    e2fsprogs \
    bridge-utils
```

### 3.3 Start and enable libvirt

```bash
sudo systemctl enable --now libvirtd
sudo systemctl status libvirtd
```

### 3.4 Add your user to the libvirt group

```bash
sudo usermod -aG libvirt $USER
sudo usermod -aG kvm $USER
newgrp libvirt
```

Log out and back in if `newgrp` does not take effect globally.

### 3.5 Verify the connection

```bash
virsh -c qemu:///system list --all
```

Expected output: an empty table (no VMs yet). If virsh returns a permission error, the group membership has not been picked up — relogin.

### 3.6 Build the CPI binary

```bash
git clone https://github.com/ZPascal/bosh-libvirt-cpi-release.git
cd bosh-libvirt-cpi-release/src/bosh-libvirt-cpi
go build -mod=vendor -o ../../bin/cpi ./main
```

The resulting binary is at `bin/cpi` relative to the repo root.

### 3.7 Create the store directory

The CPI needs a directory to store stemcell images, VM disks, and persistent disks. It creates subdirectories automatically, but the parent must exist and be writable.

```bash
sudo mkdir -p /var/lib/bosh-libvirt-cpi
sudo chown $USER:libvirt /var/lib/bosh-libvirt-cpi
chmod 0775 /var/lib/bosh-libvirt-cpi
```

### 3.8 Write the config file

```json
{
  "BackendURI": "qemu:///system",
  "StoreDir": "/var/lib/bosh-libvirt-cpi",
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

Save this to `/etc/bosh-libvirt-cpi/cpi.json` or any path you prefer. When using `bosh create-env` the path is managed for you by the job template.

### 3.9 Smoke-test the binary

```bash
echo '{"method":"info","arguments":[],"context":{}}' \
  | bin/cpi -configPath /etc/bosh-libvirt-cpi/cpi.json
```

You should get back a JSON response with `"stemcell_formats"` in the result.

---

## 4. Linux — LXC

LXC containers share the host kernel. They start in under a second and consume far less RAM than full VMs, but every container runs the same kernel version as the host. Use LXC when you need many lightweight instances and do not require strict kernel isolation.

### 4.1 Kernel parameters

LXC with unprivileged user namespaces and the postgres workload that ships in BOSH requires a few sysctl knobs:

```bash
sudo tee /etc/sysctl.d/60-bosh-lxc.conf <<'EOF'
kernel.unprivileged_userns_clone = 1
user.max_user_namespaces = 15000
kernel.shmmax = 67108864
kernel.shmall = 4194304
EOF

sudo sysctl --system
```

### 4.2 Install packages

```bash
sudo apt-get update
sudo apt-get install -y \
    lxc \
    libvirt-daemon-system \
    libvirt-daemon-driver-lxc \
    libvirt-clients \
    libvirt-dev \
    systemd-container
```

### 4.3 Enable the LXC driver in libvirt

On some distributions the LXC driver is disabled by default. Check `/etc/libvirt/libvirtd.conf` and make sure these lines are present or uncommented:

```
listen_tls = 0
listen_tcp = 0
```

Restart libvirt after any config changes:

```bash
sudo systemctl restart libvirtd
```

### 4.4 Verify

```bash
virsh -c lxc:/// list --all
```

### 4.5 Write the config file

```json
{
  "BackendURI": "lxc:///",
  "StoreDir": "/var/lib/bosh-libvirt-cpi",
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

### 4.6 Known limitations

- LXC is Linux-only. There is no macOS or Windows equivalent.
- All containers share the host's kernel. A kernel panic in a container can affect the host.
- Some BOSH jobs that rely on kernel modules or device nodes may not work inside LXC without extra capabilities.

---

## 5. macOS — VirtualBox via libvirt

KVM is not available on macOS, but VirtualBox runs fine and libvirt can drive it through the `vbox:///session` URI. This is useful for local development when you do not have a Linux host handy.

### 5.1 Install VirtualBox

Download the installer from [virtualbox.org](https://www.virtualbox.org/wiki/Downloads) and run it. The Extension Pack is not required.

Alternatively with Homebrew:

```bash
brew install --cask virtualbox
```

macOS Ventura and later will prompt you to allow the kernel extension in System Settings → Privacy & Security. Do this before continuing.

### 5.2 Install libvirt

```bash
brew install libvirt
```

This installs `virsh`, the libvirt client library, and the VirtualBox driver. The daemon runs as a user-session process, not a system service.

Start it:

```bash
brew services start libvirt
```

### 5.3 Verify

```bash
virsh -c vbox:///session list --all
```

If VirtualBox is not running yet, libvirt will start it in headless mode automatically on the first connection.

### 5.4 Install the BOSH CLI

```bash
brew install cloudfoundry/tap/bosh-cli
```

### 5.5 Build the CPI binary

The Go build requires the libvirt C headers. On macOS these come with the Homebrew libvirt package:

```bash
export PKG_CONFIG_PATH="$(brew --prefix libvirt)/lib/pkgconfig"
export CGO_LDFLAGS="-L$(brew --prefix libvirt)/lib"

cd bosh-libvirt-cpi-release/src/bosh-libvirt-cpi
go build -mod=vendor -o ../../bin/cpi ./main
```

### 5.6 Write the config file

```json
{
  "BackendURI": "vbox:///session",
  "StoreDir": "/Users/yourname/.bosh-libvirt-cpi",
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

### 5.7 macOS-specific notes

- **Disk format**: VirtualBox uses VMDK. The CPI handles the conversion automatically.
- **Networking**: VirtualBox's default NAT network is `192.168.56.0/24`. The BOSH manifest's `internal_ip` must be in that range.
- **Performance**: Without KVM, every VM runs in software emulation. Package compilation is noticeably slower than on a Linux KVM host.
- **Screen sharing**: VirtualBox VMs started by libvirt run headless. To attach a console, use `virsh -c vbox:///session console <vm-name>`.

---

## 6. Windows — not supported

The CPI is a Linux/macOS binary. It links against libvirt's C library (`libvirt.so`) which has no native Windows build. There is no workaround for this.

If you need to develop on Windows:

- Run a Linux VM in WSL2 or a Hyper-V VM and follow the Linux QEMU/KVM instructions inside it.
- Use Windows Subsystem for Linux 2 with a bridged network adapter so the CPI can reach the NAT network.

Note that KVM is available inside WSL2 on Windows 11 with the right Hyper-V settings, but the setup is outside the scope of this guide.

---

## 7. Remote-host mode (Director inside a VM)

In a typical BOSH deployment the Director is itself a VM. That VM needs to be able to call `virsh` and manage libvirt on the physical host. Because the Director VM does not have direct access to the host's libvirt socket, the CPI opens an SSH connection back to the host and runs every command through it.

This is the default mode when you deploy with the provided manifests. The flow is:

```
Director VM (192.168.122.10)
    │
    │  SSH to 192.168.122.1 (host)
    ▼
libvirtd on host  ──────────────────▶  QEMU/KVM or LXC
```

### 7.1 Prepare the host for SSH access from the Director

Generate a dedicated key pair. Do not reuse your personal key.

```bash
ssh-keygen -t ed25519 -f /tmp/bosh-libvirt-key -N ""
```

This creates `/tmp/bosh-libvirt-key` (private) and `/tmp/bosh-libvirt-key.pub` (public).

Add the public key to root's authorized_keys on the host:

```bash
sudo mkdir -p /root/.ssh
sudo chmod 700 /root/.ssh
cat /tmp/bosh-libvirt-key.pub | sudo tee -a /root/.ssh/authorized_keys
sudo chmod 600 /root/.ssh/authorized_keys
```

Retrieve the host's SSH public key in authorized_keys format:

```bash
ssh-keyscan -t ed25519 192.168.122.1 | awk '{print $2, $3}'
```

Keep this output — you will need it as the `libvirt_host_key` variable in your manifest.

### 7.2 Inject credentials via the manifest ops file

The repo ships with ops files that wire the SSH credentials into the deployed Director's CPI config. When the Director's CPI binary runs inside the Director VM it reads `inject_*` fields that override the connection for child deployments:

```bash
bosh create-env manifests/qemu-cpi.yml \
  -o manifests/ops/qemu-local.yml \
  -o manifests/local-release.yml \
  -o manifests/ops/local-store-dir.yml \
  -o manifests/ops/qemu-mac.yml \
  -o manifests/ops/qemu-kernel-boot.yml \
  -o manifests/ops/qemu-director-ssh.yml \
  --state /tmp/bosh-state.json \
  --vars-store /tmp/bosh-vars.yml \
  -v director_name=bosh-qemu \
  -v internal_ip=192.168.122.10 \
  -v stemcell_url=file:///path/to/bosh-stemcell.tgz \
  -v kernel_path=/boot/vmlinuz \
  -v libvirt_host=192.168.122.1 \
  -v libvirt_username=root \
  --var-file libvirt_private_key=/tmp/bosh-libvirt-key \
  -v "libvirt_host_key=ssh-ed25519 AAAA..."
```

For LXC replace the qemu-specific ops files with their lxc equivalents:

```bash
bosh create-env manifests/lxc-cpi.yml \
  -o manifests/ops/lxc-local.yml \
  -o manifests/local-release.yml \
  -o manifests/ops/local-store-dir.yml \
  -o manifests/ops/lxc-mac.yml \
  -o manifests/ops/lxc-director-ssh.yml \
  --state /tmp/bosh-state.json \
  --vars-store /tmp/bosh-vars.yml \
  -v director_name=bosh-lxc \
  -v internal_ip=192.168.122.11 \
  -v stemcell_url=file:///path/to/bosh-stemcell.tgz \
  -v libvirt_host=192.168.122.1 \
  -v libvirt_username=root \
  --var-file libvirt_private_key=/tmp/bosh-libvirt-key \
  -v "libvirt_host_key=ssh-ed25519 AAAA..."
```

### 7.3 What the ops files do

| Ops file | Effect |
|----------|--------|
| `qemu-local.yml` | Sets `internal_ip` to `192.168.122.10`, network to libvirt default |
| `lxc-local.yml` | Sets `internal_ip` to `192.168.122.11`, network to libvirt default |
| `qemu-kernel-boot.yml` | Switches backend URI to `qemu:///system` and disk format to ext4 for direct kernel boot |
| `qemu-mac.yml` | Pins the Director VM's MAC address so it always gets the same DHCP lease |
| `lxc-mac.yml` | Same as above for LXC |
| `qemu-director-ssh.yml` | Injects `Host`, `Username`, `PrivateKey`, `HostKey` into the Director's CPI config so the deployed Director can SSH back to the host |
| `lxc-director-ssh.yml` | Same as above for LXC |
| `local-store-dir.yml` | Overrides `StoreDir` to `/tmp/bosh_libvirt_cpi` — useful for CI |
| `local-release.yml` | Points the CPI release to a local file path instead of fetching from the internet |

---

## 8. CPI configuration reference

The CPI reads a single JSON file. All fields are case-sensitive.

### 8.1 Top-level fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BackendURI` | string | yes | libvirt connection URI. See section 2 for valid values. |
| `StoreDir` | string | yes | Absolute path to the directory where stemcells, VM disks, and persistent disks are stored. |
| `Network` | string | no | libvirt network name. Defaults to `"default"` if empty. |
| `Host` | string | no | IP or hostname of the remote libvirt host. Leave empty for local connections. |
| `Port` | int | no | SSH port on the remote host. Defaults to 22. |
| `Username` | string | if Host set | SSH user on the remote host. Typically `"root"`. |
| `PrivateKey` | string | if Host set | PEM-encoded SSH private key. This is the key content, not a file path. |
| `HostKey` | string | if Host set | SSH host public key in authorized_keys format, e.g. `"ssh-ed25519 AAAA..."`. |
| `Agent` | object | yes | BOSH agent configuration passed to every VM at creation time. |

### 8.2 Inject fields (deployed Director only)

When the Director is itself a VM it needs a separate set of credentials to SSH back to the host for any subsequent deployments. The `Inject*` fields provide those overrides.

| Field | Description |
|-------|-------------|
| `InjectBackendURI` | Override `BackendURI` for the deployed Director's CPI. |
| `InjectHost` | Override `Host`. |
| `InjectUsername` | Override `Username`. |
| `InjectPrivateKey` | Override `PrivateKey`. |
| `InjectHostKey` | Override `HostKey`. |
| `InjectStoreDir` | Override `StoreDir`. |

If these fields are absent the deployed Director's CPI inherits the same values as the bootstrap CPI.

### 8.3 MbusBootstrapSSL fields

These are optional. When present they configure TLS for the BOSH agent's mbus bootstrap listener.

| Field | Description |
|-------|-------------|
| `MbusBootstrapSSL.CA` | PEM-encoded CA certificate. |
| `MbusBootstrapSSL.Certificate` | PEM-encoded server certificate signed by the CA. |
| `MbusBootstrapSSL.PrivateKey` | PEM-encoded private key for the server certificate. |

### 8.4 Agent block

The `Agent` block is passed verbatim to the BOSH agent running inside every VM. Refer to the [BOSH agent documentation](https://bosh.io/docs/agent/) for all available options. The fields you always need are:

```json
"Agent": {
  "mbus": "https://mbus:password@0.0.0.0:6868",
  "ntp": ["pool.ntp.org"],
  "blobstore": {
    "provider": "dav",
    "options": {
      "endpoint": "http://10.0.0.1:25250",
      "user": "agent",
      "password": "agent-password"
    }
  }
}
```

When using `bosh create-env` these values come from the manifest and are substituted by the CPI job template. You do not write them by hand.

### 8.5 Complete local example (QEMU/KVM)

```json
{
  "BackendURI": "qemu:///system",
  "StoreDir": "/var/lib/bosh-libvirt-cpi",
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

### 8.6 Complete remote example (QEMU/KVM via SSH)

```json
{
  "BackendURI": "qemu:///system",
  "StoreDir": "/var/lib/bosh-libvirt-cpi",
  "Host": "192.168.122.1",
  "Username": "root",
  "PrivateKey": "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----",
  "HostKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...",
  "InjectHost": "192.168.122.1",
  "InjectUsername": "root",
  "InjectPrivateKey": "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----",
  "InjectHostKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...",
  "Agent": {
    "mbus": "https://mbus:mbus-password@0.0.0.0:6868",
    "ntp": ["0.pool.ntp.org"],
    "blobstore": {
      "provider": "dav",
      "options": {
        "endpoint": "https://192.168.122.10:25250",
        "user": "agent",
        "password": "agent-password"
      }
    }
  }
}
```

---

## 9. BOSH manifest integration

When you deploy the Director with `bosh create-env` you do not write the JSON config by hand. The CPI release ships a job template that generates it from manifest variables. The variables you set on the command line map to config fields as follows:

| Manifest variable | Config field |
|-------------------|-------------|
| `libvirt_host` | `Host` / `InjectHost` |
| `libvirt_username` | `Username` / `InjectUsername` |
| `libvirt_private_key` | `PrivateKey` / `InjectPrivateKey` |
| `libvirt_host_key` | `HostKey` / `InjectHostKey` |
| `store_dir` | `StoreDir` |
| `backend_uri` | `BackendURI` |

The `qemu-director-ssh.yml` and `lxc-director-ssh.yml` ops files set these variables automatically from what you passed to `create-env`, so the deployed Director's CPI config is consistent with the bootstrap config.

### 9.1 Cloud config

After `create-env` succeeds, upload a cloud config before running `bosh deploy`:

```bash
bosh -e your-director update-cloud-config manifests/cloud-config-qemu.yml \
  -v kernel_path=/boot/vmlinuz \
  --client admin --client-secret admin-password
```

The cloud config defines the VM types and networks available to deployments. The `kernel_path` variable is required for QEMU kernel-boot mode (see section 12) and should point to a kernel image on the host that matches the stemcell's userspace.

### 9.2 Stemcell

Download the warden-boshlite stemcell and upload it:

```bash
bosh -e your-director upload-stemcell \
  https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-jammy-go_agent \
  --client admin --client-secret admin-password
```

The CPI will convert the stemcell to the correct disk format for your backend automatically:

- QEMU standard mode: qcow2
- QEMU kernel-boot mode: ext4 raw image
- LXC: directory tree

---

## 10. Storage layout

Everything the CPI creates on disk lives under `StoreDir`. The directory tree looks like this:

```
StoreDir/
├── stemcells/
│   └── sc-<uuid>/
│       └── image.qcow2        (or image.ext4, or a directory)
├── vms/
│   └── vm-<uuid>/
│       ├── rootfs.img         (root disk — copy of stemcell image, resized to 65G)
│       └── ephemeral.qcow2    (ephemeral disk)
└── disks/
    └── disk-<uuid>.qcow2      (persistent disks)
```

Root disks are created as sparse files so they do not consume 65GB of actual space immediately. Use `du -sh` rather than `ls -lh` to see actual disk usage.

You can place `StoreDir` on any filesystem that supports sparse files (ext4, xfs, btrfs, APFS). NFS is not recommended because loop-device attachment and the sync operations the CPI uses do not work reliably over NFS.

---

## 11. Networking

### 11.1 Default libvirt NAT network

libvirt creates a NAT bridge called `virbr0` on first use. Its default address space is `192.168.122.0/24`. The host is reachable from VMs at `192.168.122.1`.

```bash
virsh net-list --all
virsh net-info default
```

To make the default network start automatically on boot:

```bash
virsh net-autostart default
virsh net-start default
```

### 11.2 Static IP assignment via MAC address

BOSH Directors need a stable IP so the manifest can reference them. The CPI supports pinning a MAC address in the cloud properties. libvirt's DHCP server then always gives the same IP to the same MAC. The provided ops files set:

- QEMU Director: MAC `52:54:00:ab:cd:10` → IP `192.168.122.10`
- LXC Director: MAC `52:54:00:ab:cd:11` → IP `192.168.122.11`

To configure a static MAC-to-IP mapping in libvirt:

```bash
virsh net-update default add ip-dhcp-host \
  '<host mac="52:54:00:ab:cd:10" ip="192.168.122.10"/>' \
  --live --config
```

### 11.3 Firewall rules

The CI host needs to allow inbound connections from VMs on the following ports:

| Port | Protocol | Purpose |
|------|----------|---------|
| 22 | TCP | SSH — the Director VM's CPI SSHes back to the host |
| 4222 | TCP | NATS — BOSH agents connect to the Director's NATS server |
| 25250 | TCP | Blobstore — agents download packages |
| 6868 | TCP | BOSH agent mbus bootstrap |
| 5432 | TCP | postgres — Director database (localhost only by default) |

On systems using `ufw`:

```bash
sudo ufw allow in on virbr0
```

On systems using `firewalld`:

```bash
sudo firewall-cmd --zone=libvirt --add-port=4222/tcp --permanent
sudo firewall-cmd --reload
```

---

## 12. Kernel-boot mode (QEMU only)

Standard QEMU mode boots VMs from a qcow2 disk image, which means the guest kernel comes from inside the stemcell. Kernel-boot mode skips that and tells QEMU to load a kernel from the host directly. This avoids grub and speeds up the boot sequence for compilation VMs.

When `kernel` is set in the VM's cloud properties the CPI:

1. Creates an ext4 raw disk image (instead of qcow2).
2. Mounts the image, copies the stemcell rootfs into it, and injects the BOSH agent env.
3. Passes the host kernel path to libvirt's `<kernel>` element.
4. The guest boots with `root=/dev/vda rw init=/bosh-init`.

To enable this mode include the `qemu-kernel-boot.yml` ops file in your `create-env` command and provide `kernel_path`:

```bash
-o manifests/ops/qemu-kernel-boot.yml \
-v kernel_path=/boot/vmlinuz-$(uname -r)
```

The kernel must be compatible with the Ubuntu Jammy userspace in the stemcell. The host's own kernel works if the host is also Ubuntu Jammy. If the host runs a different distribution, copy a compatible kernel to the host and point `kernel_path` at it.

---

## 13. Troubleshooting

### libvirt connection refused

```
error: failed to connect to the hypervisor
error: Failed to connect socket to '/run/libvirt/libvirt-sock': No such file or directory
```

libvirtd is not running. Start it:

```bash
sudo systemctl start libvirtd
```

### Permission denied on /dev/kvm

```
Could not access KVM kernel module: Permission denied
```

Add your user to the kvm group and re-login:

```bash
sudo usermod -aG kvm $USER
```

### CPI exits with "missing exit info" or times out

This happens when an SSH command to the host drops without sending an exit code. It usually means the host ran out of file descriptors or the SSH session hit the `MaxSessions` limit in `sshd_config`. Increase the limit:

```bash
# /etc/ssh/sshd_config
MaxSessions 100
MaxStartups 100:30:200
```

```bash
sudo systemctl reload sshd
```

### QEMU compilation VM kernel panics with "VFS: Unable to mount root fs"

The ext4 root image has the `needs_recovery` feature flag set. This happens when the kernel writes a journal entry during VM setup and the flag is not cleared before QEMU boots the guest. The CPI automatically runs `e2fsck`, `tune2fs -O ^needs_recovery`, and a verify pass after every VM creation. If you see this panic it means one of those steps failed.

Check the CPI log on the Director VM:

```bash
ssh root@192.168.122.10 \
  'grep -h "nr_check\|tune2fs\|e2fsck" /var/vcap/bosh/log/libvirt_cpi/*.log | tail -30'
```

The `nr_check:` line shows the superblock value after the clear. `nr=0` means the bit was cleared successfully. If it shows `nr=1`, the clear failed and you will need to check whether `tune2fs` and `e2fsck` are installed on the host:

```bash
which tune2fs e2fsck
```

### LXC container networking not working

Check that the libvirt default network is running:

```bash
virsh -c lxc:/// net-list --all
```

If the state is inactive:

```bash
virsh net-start default
```

If the network is active but containers still cannot reach the host, check that IP forwarding is enabled:

```bash
sysctl net.ipv4.ip_forward
```

Enable it if the result is 0:

```bash
sudo sysctl -w net.ipv4.ip_forward=1
echo "net.ipv4.ip_forward = 1" | sudo tee /etc/sysctl.d/99-ipforward.conf
```

### `virsh -c lxc:/// list` works but `virsh -c qemu:///system list` fails

The QEMU driver and LXC driver are separate libvirt plugins. Check which drivers are loaded:

```bash
sudo libvirtd --version
ls /usr/lib/libvirt/connection-driver/
```

Install the missing driver package if `libvirt_driver_qemu.so` or `libvirt_driver_lxc.so` is absent.

### Stemcell upload fails with "connection refused" on port 25250

The Director's blobstore (nginx on port 25250) has not started yet. Wait a few seconds after the Director comes up before uploading stemcells. The CI workflow checks:

```bash
timeout 120 bash -c 'until nc -z 192.168.122.10 25250; do sleep 3; done'
```

You can run the same check manually.

### tune2fs is missing on the host

If you are using kernel-boot mode and `tune2fs` is not installed, the CPI's python3 fallback will attempt a raw superblock write instead. Install it to use the proper code path:

```bash
sudo apt-get install e2fsprogs
```

---

*This guide covers the bosh-libvirt-cpi-release as of the `feat/lxc-integration-tests` branch. For the current release notes see the repository's changelog.*
