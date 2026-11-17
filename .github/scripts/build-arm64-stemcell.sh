#!/usr/bin/env bash
# Builds an ARM64 (aarch64) warden-tar stemcell for this CPI from a checkout of
# aws-samples/sample-cloud-foundry-graviton (MIT-0): its stemcell image (Ubuntu
# 24.04 arm64, arm64 bosh-agent, monit), with the agent config moved to where
# this CPI expects it, exported as a rootfs tarball. Run on an arm64 host.
#
# Usage: build-arm64-stemcell.sh <sample-dir> <out.tgz>
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <sample-dir> <out.tgz>" >&2
  exit 2
fi

sample_dir=$(realpath "$1")
out=$(realpath -m "$2")
name="${STEMCELL_NAME:-bosh-libvirt-ubuntu-noble-arm64-test}"
version="${STEMCELL_VERSION:-1.0-arm64}"

# Patched arm64 bosh-agent, davcli and nats-server into <sample-dir>/bin.
"${sample_dir}/scripts/build-binaries.sh"

docker build --platform linux/arm64 -t bosh-stemcell-arm64:sample \
  -f "${sample_dir}/Dockerfile.stemcell" "$sample_dir"

# The sample keeps the agent config at /var/vcap/bosh/etc/agent.json (Docker
# CPI). This CPI starts the agent with /var/vcap/bosh/agent.json, like the
# official warden stemcells; the settings source (warden-cpi-agent-env.json)
# is already the one this CPI writes. Persistent disks: bind-mount, as on warden.
docker build --platform linux/arm64 -t bosh-stemcell-arm64:libvirt \
  --build-arg BASE=bosh-stemcell-arm64:sample - <<'DOCKERFILE'
ARG BASE
FROM ${BASE}
RUN jq '.Platform.Linux += {"UsePreformattedPersistentDisk": true, "BindMountPersistentDisk": true}' \
      /var/vcap/bosh/etc/agent.json > /var/vcap/bosh/agent.json && \
    jq -e '.Infrastructure.Settings.Sources[0].SettingsPath == "/var/vcap/bosh/warden-cpi-agent-env.json"' \
      /var/vcap/bosh/agent.json
DOCKERFILE

arch=$(docker inspect bosh-stemcell-arm64:libvirt --format '{{.Architecture}}')
[ "$arch" = arm64 ] || { echo "stemcell image is ${arch}, expected arm64" >&2; exit 1; }

work=$(mktemp -d)
cid=""
trap '[ -n "$cid" ] && docker rm "$cid" >/dev/null; rm -rf "$work"' EXIT

# A warden-tar stemcell's image is a flat rootfs tarball: docker export, not save.
cid=$(docker create --platform linux/arm64 bosh-stemcell-arm64:libvirt /bin/true)
docker export "$cid" | gzip > "${work}/image"

cat > "${work}/stemcell.MF" <<EOF
---
name: ${name}
version: '${version}'
bosh_protocol: 1
api_version: 3
sha1: $(sha1sum "${work}/image" | awk '{print $1}')
operating_system: ubuntu-noble
stemcell_formats:
- warden-tar
cloud_properties:
  architecture: aarch64
EOF

mkdir -p "$(dirname "$out")"
tar -czf "$out" -C "$work" stemcell.MF image
echo "built ${out} (${name}/${version}, $(du -h "$out" | cut -f1))"
