# PR 3: BOSH Release Minimal Correct Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all VirtualBox copy-paste artifacts in the BOSH release packaging — rename the package, fix the binary path, add cgo support, rewrite the job spec and templates to match current FactoryOpts, and add a working QEMU/KVM manifest.

**Architecture:** Pure configuration/templating changes — no Go code modified. Each file has one clear purpose. `packages/libvirt_cpi/` builds the binary; `jobs/libvirt_cpi/` configures and runs it; `manifests/` shows operators how to deploy it.

**Tech Stack:** BOSH release format (YAML spec, ERB templates, bash packaging scripts)

**Prerequisite:** PR 1 must be merged first — the `host_key` property added here requires `HostKey` in `FactoryOpts`.

---

### Task 1: Rename and fix the package spec

**Files:**
- Modify: `packages/libvirt_cpi/spec`

- [ ] **Step 1: Read the current spec**

```bash
cat packages/libvirt_cpi/spec
```

Current contents:
```yaml
---
name: virtualbox_cpi

dependencies:
- golang-1-darwin
- golang-1-linux

files:
- "bosh-libvirt-cpi/**/*.go"
- "bosh-libvirt-cpi/**/*.s"
- "bosh-libvirt-cpi/go.{mod,sum}"
- "bosh-libvirt-cpi/vendor/modules.txt"
```

- [ ] **Step 2: Rewrite the spec**

Replace the entire contents of `packages/libvirt_cpi/spec`:

```yaml
---
name: libvirt_cpi

dependencies:
- golang-1-linux

files:
- "bosh-libvirt-cpi/**/*.go"
- "bosh-libvirt-cpi/**/*.s"
- "bosh-libvirt-cpi/**/*.h"
- "bosh-libvirt-cpi/**/*.c"
- "bosh-libvirt-cpi/go.{mod,sum}"
- "bosh-libvirt-cpi/vendor/modules.txt"
```

Changes: `name` corrected, `golang-1-darwin` removed, `.h` and `.c` globs added for cgo vendored headers.

- [ ] **Step 3: Commit**

```bash
git add packages/libvirt_cpi/spec
git commit -m "fix: rename package to libvirt_cpi, drop Darwin dependency"
```

---

### Task 2: Fix the packaging script

**Files:**
- Modify: `packages/libvirt_cpi/packaging`

- [ ] **Step 1: Read the current packaging script**

```bash
cat packages/libvirt_cpi/packaging
```

- [ ] **Step 2: Rewrite the packaging script**

Replace the entire contents of `packages/libvirt_cpi/packaging`:

```bash
set -e -x

if [ -z "$BOSH_PACKAGES_DIR" ]; then
  pkg_dir=$(readlink -nf /var/vcap/packages/golang-1-linux)
else
  pkg_dir=$BOSH_PACKAGES_DIR/golang-1-linux
fi

source ${pkg_dir}/bosh/compile.env

# CGO_ENABLED=1 is required by libvirt.org/go/libvirt (cgo bindings).
# The build host must have libvirt-dev installed:
#   apt-get install -y libvirt-dev
export CGO_ENABLED=1

mkdir -p /tmp/go/.cache
export GOPATH=/tmp/go
export GOCACHE=${GOPATH}/.cache

cd ${BOSH_COMPILE_TARGET}/bosh-libvirt-cpi
mkdir -p ${BOSH_INSTALL_TARGET}/bin

export GOARCH=amd64
export GOOS=linux

go build -mod=vendor -o ${BOSH_INSTALL_TARGET}/bin/cpi ./main/*.go
```

Changes: `CGO_ENABLED=1`, single Linux build only, output renamed to `cpi`, `golang-1-linux` only, comment documenting build host requirement.

- [ ] **Step 3: Commit**

```bash
git add packages/libvirt_cpi/packaging
git commit -m "fix: enable CGO, build Linux only, output binary as 'cpi'"
```

---

### Task 3: Rewrite the job spec

**Files:**
- Modify: `jobs/libvirt_cpi/spec`

- [ ] **Step 1: Rewrite the job spec**

Replace the entire contents of `jobs/libvirt_cpi/spec`:

```yaml
---
name: libvirt_cpi

templates:
  cpi.erb: bin/cpi
  cpi.json.erb: config/cpi.json

packages:
- libvirt_cpi

properties:
  backend_uri:
    description: >
      Libvirt connection URI. Determines the hypervisor backend.
      Examples: "qemu:///system" (QEMU/KVM), "lxc:///" (LXC),
      "vbox:///session" (VirtualBox).
    default: "qemu:///system"

  host:
    description: >
      Hostname or IP of the machine running libvirtd.
      If not set, the CPI connects to the local libvirtd via the backend_uri.
    default: ""
    example: 192.168.50.1

  username:
    description: Username for SSH access to the libvirt host. Required when host is set.
    default: ""

  private_key:
    description: SSH private key (PEM format) for accessing the libvirt host. Required when host is set.
    default: ""

  host_key:
    description: >
      SSH host public key in authorized_keys format (e.g. "ssh-ed25519 AAAA...").
      Required when host is set. Used to verify the remote host identity.
    default: ""

  store_dir:
    description: >
      Directory on the libvirt host used to store stemcells, disks, and VM metadata.
      The '~' prefix is expanded to the connecting user's home directory.
    default: "~/.bosh_libvirt_cpi"

  ntp:
    description: List of NTP server addresses for the BOSH agent.
    default:
      - 0.pool.ntp.org
      - 1.pool.ntp.org

  agent.mbus:
    description: Agent mbus URL (NATS or HTTPS).

  nats.user:
    description: NATS username.
    default: nats
  nats.password:
    description: NATS password.
  agent.nats.address:
    description: NATS server address (used by agent).
  nats.address:
    description: NATS server address.
  nats.port:
    description: NATS server port.
    default: 4222

  env.http_proxy:
    description: HTTP proxy for outbound connections.
  env.https_proxy:
    description: HTTPS proxy for outbound connections.
  env.no_proxy:
    description: Comma-separated list of hosts to bypass the proxy.
```

- [ ] **Step 2: Commit**

```bash
git add jobs/libvirt_cpi/spec
git commit -m "fix: rewrite job spec — remove VBox properties, add backend_uri and host_key"
```

---

### Task 4: Fix cpi.erb

**Files:**
- Modify: `jobs/libvirt_cpi/templates/cpi.erb`

- [ ] **Step 1: Read current cpi.erb**

```bash
cat jobs/libvirt_cpi/templates/cpi.erb
```

- [ ] **Step 2: Rewrite cpi.erb**

Replace the entire contents of `jobs/libvirt_cpi/templates/cpi.erb`:

```bash
#!/bin/bash

BOSH_PACKAGES_DIR=${BOSH_PACKAGES_DIR:-/var/vcap/packages}
BOSH_JOBS_DIR=${BOSH_JOBS_DIR:-/var/vcap/jobs}

<% if_p('env.http_proxy') do |http_proxy| %>
export HTTP_PROXY="<%= http_proxy %>"
export http_proxy="<%= http_proxy %>"
<% end %>

<% if_p('env.https_proxy') do |https_proxy| %>
export HTTPS_PROXY="<%= https_proxy %>"
export https_proxy="<%= https_proxy %>"
<% end %>

<% if_p('env.no_proxy') do |no_proxy| %>
export NO_PROXY="<%= no_proxy %>"
export no_proxy="<%= no_proxy %>"
<% end %>

exec $BOSH_PACKAGES_DIR/libvirt_cpi/bin/cpi -configPath $BOSH_JOBS_DIR/libvirt_cpi/config/cpi.json
```

Changes: removed `platform` variable and cross-platform binary selection (Linux only now), fixed package name from `virtualbox_cpi` to `libvirt_cpi`, fixed job config path.

- [ ] **Step 3: Commit**

```bash
git add jobs/libvirt_cpi/templates/cpi.erb
git commit -m "fix: correct binary and config paths in cpi.erb"
```

---

### Task 5: Rewrite cpi.json.erb

**Files:**
- Modify: `jobs/libvirt_cpi/templates/cpi.json.erb`

- [ ] **Step 1: Read current cpi.json.erb**

```bash
cat jobs/libvirt_cpi/templates/cpi.json.erb
```

- [ ] **Step 2: Rewrite cpi.json.erb**

Replace the entire contents of `jobs/libvirt_cpi/templates/cpi.json.erb`:

```erb
<%=
require 'json'

params = {
  "BackendURI"  => p("backend_uri"),
  "Host"        => p("host"),
  "Username"    => p("username"),
  "PrivateKey"  => p("private_key"),
  "HostKey"     => p("host_key"),
  "StoreDir"    => p("store_dir"),
  "Agent"       => {
    "ntp" => p("ntp")
  }
}

agent_params = params["Agent"]

if_p("agent.mbus") do |mbus|
  agent_params["mbus"] = mbus
end.else_if_p("nats") do
  agent_params["mbus"] = "nats://#{p("nats.user")}:#{p("nats.password")}@#{p(["agent.nats.address", "nats.address"])}:#{p("nats.port")}"
end

JSON.pretty_generate(params)
%>
```

Changes: emits `BackendURI`, `HostKey`, `StoreDir` matching `FactoryOpts` struct. Removes `BinPath`, `StorageController`, `AutoEnableNetworks`.

- [ ] **Step 3: Commit**

```bash
git add jobs/libvirt_cpi/templates/cpi.json.erb
git commit -m "fix: rewrite cpi.json.erb to match current FactoryOpts struct"
```

---

### Task 6: Add QEMU/KVM deployment manifest

**Files:**
- Create: `manifests/qemu-cpi.yml`

- [ ] **Step 1: Create the manifest**

Create `manifests/qemu-cpi.yml`:

```yaml
---
# Minimal BOSH manifest for deploying the libvirt CPI against a QEMU/KVM host.
# Usage:
#   bosh create-env manifests/qemu-cpi.yml \
#     -v libvirt_host=<host-ip> \
#     -v libvirt_username=<ssh-user> \
#     --var-file libvirt_private_key=<path-to-key.pem> \
#     -v libvirt_host_key="ssh-ed25519 AAAA..." \
#     -v director_name=bosh-qemu \
#     -v internal_ip=<director-ip>

name: bosh-qemu

releases:
- name: bosh
  url: https://bosh.io/d/github.com/cloudfoundry/bosh
  version: latest
- name: bosh-libvirt-cpi
  url: file://..
  version: create

resource_pools:
- name: vms
  network: default
  stemcell:
    url: https://bosh.io/d/stemcells/bosh-libvirt-qemu-ubuntu-jammy-go_agent
    version: latest
  cloud_properties:
    memory: 2048
    cpus: 2
    ephemeral_disk: 25000

disk_pools:
- name: disks
  disk_size: 20000

networks:
- name: default
  type: manual
  subnets:
  - range: 192.168.1.0/24
    gateway: 192.168.1.1
    dns: [8.8.8.8]
    static: [192.168.1.6]

instance_groups:
- name: bosh
  instances: 1
  jobs:
  - name: nats
    release: bosh
  - name: postgres-10
    release: bosh
  - name: blobstore
    release: bosh
  - name: director
    release: bosh
  - name: health_monitor
    release: bosh
  - name: libvirt_cpi
    release: bosh-libvirt-cpi
  resource_pool: vms
  disk_pool: disks
  networks:
  - name: default
    static_ips: [((internal_ip))]
  properties:
    nats:
      address: 127.0.0.1
      user: nats
      password: nats-password
    postgres: &db
      listen_address: 127.0.0.1
      host: 127.0.0.1
      user: postgres
      password: postgres-password
      database: bosh
      adapter: postgres
    blobstore:
      address: ((internal_ip))
      port: 25250
      provider: dav
      director:
        user: director
        password: director-password
      agent:
        user: agent
        password: agent-password
    director:
      address: 127.0.0.1
      name: ((director_name))
      cpi_job: libvirt_cpi
      max_threads: 3
      db: *db
      backend_port: 25556
    hm:
      director_account:
        user: admin
        password: admin-password
      resurrector_enabled: true
    agent:
      mbus: nats://nats:nats-password@((internal_ip)):4222
    libvirt_cpi:
      backend_uri: qemu:///system
      host: ((libvirt_host))
      username: ((libvirt_username))
      private_key: ((libvirt_private_key))
      host_key: ((libvirt_host_key))
      store_dir: ~/.bosh_libvirt_cpi

cloud_provider:
  template:
    name: libvirt_cpi
    release: bosh-libvirt-cpi
  properties:
    libvirt_cpi:
      backend_uri: qemu:///system
      host: ((libvirt_host))
      username: ((libvirt_username))
      private_key: ((libvirt_private_key))
      host_key: ((libvirt_host_key))
      store_dir: ~/.bosh_libvirt_cpi
      agent:
        mbus: https://mbus:mbus-password@0.0.0.0:6868
        ntp: [0.pool.ntp.org, 1.pool.ntp.org]
        blobstore:
          provider: local
          options:
            blobstore_path: /var/vcap/micro_bosh/data/cache
```

- [ ] **Step 2: Commit**

```bash
git add manifests/qemu-cpi.yml
git commit -m "docs: add QEMU/KVM BOSH deployment manifest"
```

---

### Task 7: Validate the release can be created

- [ ] **Step 1: Check for any remaining VBox references**

```bash
grep -r "virtualbox_cpi\|VirtualBox\|vbox_cpi\|storage_controller\|auto_enable_networks" \
  jobs/ packages/ manifests/ --include="*.yml" --include="*.erb" --include="*.json" -l
```

Expected: no output (no files with VBox artifacts remaining).

- [ ] **Step 2: Confirm package and job names are consistent**

```bash
grep "^name:" jobs/libvirt_cpi/spec packages/libvirt_cpi/spec
```

Expected:
```
jobs/libvirt_cpi/spec:name: libvirt_cpi
packages/libvirt_cpi/spec:name: libvirt_cpi
```

- [ ] **Step 3: Verify ERB templates reference correct paths**

```bash
grep "libvirt_cpi/bin\|libvirt_cpi/config" jobs/libvirt_cpi/templates/cpi.erb
```

Expected: both lines contain `libvirt_cpi`.

- [ ] **Step 4: Commit any remaining fixups, then push**

```bash
git log --oneline -6
```

Confirm all five fix commits are present, then open the PR against `main`.
