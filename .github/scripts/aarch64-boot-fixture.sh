#!/usr/bin/env bash
# Builds the inputs for the aarch64 TCG boot test (driver/aarch64_boot_integration_test.go):
#   <out>/vmlinuz      Ubuntu noble arm64 kernel (cloud image, unpacked)
#   <out>/rootfs.img   ext4 from ubuntu-base arm64 with a tiny /bosh-init that
#                      reports the architecture and the virtio disks it sees
#   <out>/ephemeral.qcow2, <out>/persistent.qcow2
# Downloads are verified against the published SHA256SUMS. Needs sudo, curl,
# e2fsprogs (mkfs.ext4 -d) and qemu-img.
set -euo pipefail

out=${1:?usage: $0 <out-dir>}
mkdir -p "$out"
cd "$out"

fetch_verified() { # <base-url> <file> <local-name>
  curl -fsSL -o "$3" "$1/$2"
  curl -fsSL -o "$3.SHA256SUMS" "$1/SHA256SUMS"
  expected=$(awk -v f="$2" '$2 == f || $2 == "*" f {print $1}' "$3.SHA256SUMS")
  [ -n "$expected" ] || { echo "no checksum for $2" >&2; exit 1; }
  echo "$expected  $3" | sha256sum -c -
}

kernel_base=https://cloud-images.ubuntu.com/noble/current/unpacked
fetch_verified "$kernel_base" noble-server-cloudimg-arm64-vmlinuz-generic vmlinuz

base_url=https://cdimage.ubuntu.com/ubuntu-base/releases/noble/release
base_tar=$(curl -fsSL "$base_url/" | grep -oE 'ubuntu-base-24\.04(\.[0-9]+)?-base-arm64\.tar\.gz' | sort -uV | tail -1)
[ -n "$base_tar" ] || { echo "ubuntu-base arm64 tarball not found" >&2; exit 1; }
fetch_verified "$base_url" "$base_tar" ubuntu-base.tar.gz

sudo rm -rf root && mkdir root
sudo tar -xzf ubuntu-base.tar.gz -C root
sudo tee root/bosh-init > /dev/null <<'INIT'
#!/bin/sh
mount -t proc proc /proc
mount -t sysfs sysfs /sys
mount -t devtmpfs devtmpfs /dev
echo "AARCH64_INIT_OK $(uname -m) $(uname -r)" > /dev/console
while true; do
  echo "DISKS: $(ls /dev/vd* 2>/dev/null | tr '\n' ' ')" > /dev/console
  sleep 3
done
INIT
sudo chmod 0755 root/bosh-init

truncate -s 1G rootfs.img
sudo mkfs.ext4 -q -F -d root rootfs.img
sudo rm -rf root
qemu-img create -q -f qcow2 ephemeral.qcow2 64M
qemu-img create -q -f qcow2 persistent.qcow2 64M

# libvirt's qemu user must read the kernel and write the disks.
chmod 0755 "$out"
chmod 0644 vmlinuz
chmod 0666 rootfs.img ephemeral.qcow2 persistent.qcow2
ls -la "$out"
