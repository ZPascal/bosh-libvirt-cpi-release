#!/bin/sh
export PATH=/var/vcap/bosh/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
# Mount essential pseudo-filesystems
mount -t proc proc /proc 2>/dev/null || true
mount -t sysfs sysfs /sys 2>/dev/null || true
mount -t devtmpfs devtmpfs /dev 2>/dev/null || true
mkdir -p /dev/shm && mount -t tmpfs -o size=256m tmpfs /dev/shm 2>/dev/null || true
mkdir -p /dev/pts && mount -t devpts devpts /dev/pts 2>/dev/null || true
# Remove stale supervise locks before starting runsv
rm -rf /etc/sv/*/supervise
# Start runsv directly for each service
for svc in /etc/sv/*/; do
  [ -d "$svc" ] && runsv "$svc" &
done
# Wait for monit supervise dir
for i in $(seq 1 30); do
  [ -d /etc/sv/monit/supervise ] && break
  sleep 1
done
# Bring up network via DHCP
IFACE=$(ip -o link show 2>/dev/null | awk -F': ' '$2 !~ /lo/ {print $2; exit}')
if [ -n "$IFACE" ]; then
  ip link set "$IFACE" up
  /usr/sbin/dhclient -v "$IFACE" 2>/tmp/dhclient.log || true
fi
exec /var/vcap/bosh/bin/bosh-agent -C /var/vcap/bosh/agent.json -P ubuntu
