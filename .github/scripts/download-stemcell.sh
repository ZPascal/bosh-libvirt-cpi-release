#!/usr/bin/env bash
# Downloads the latest bosh-warden-boshlite-ubuntu-noble stemcell from bosh.io
# and verifies it against the SHA-256 bosh.io publishes.
#
#   download-stemcell.sh --version         print the latest version (cache key)
#   download-stemcell.sh <out-dir>         ensure <out-dir>/stemcell.tgz is that
#                                          version (no download if a cached copy
#                                          already matches the checksum)
set -euo pipefail

NAME=bosh-warden-boshlite-ubuntu-noble
API="https://bosh.io/api/v1/stemcells/${NAME}"

meta=$(curl -fsSL "$API" | python3 -c '
import json, sys
s = json.load(sys.stdin)[0]
r = s["regular"]
print(s["version"], r["url"], r["sha256"])
')
read -r version url sha256 <<<"$meta"

if [ "${1:-}" = "--version" ]; then
  echo "$version"
  exit 0
fi

out=${1:?usage: $0 --version | <out-dir>}
mkdir -p "$out"
target="${out}/stemcell.tgz"

if [ -f "$target" ] && echo "${sha256}  ${target}" | sha256sum -c --status -; then
  echo "${NAME} ${version} already present (checksum OK)"
  exit 0
fi

echo "Downloading ${NAME} ${version}"
curl -fsSL -o "${target}.part" "$url"
echo "${sha256}  ${target}.part" | sha256sum -c -
mv "${target}.part" "$target"
chmod 0644 "$target"
