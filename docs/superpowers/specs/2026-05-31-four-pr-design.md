# Four-PR Design: Tests, BOSH Release, LXC/VBox, Security

Date: 2026-05-31  
Branch: combined-libvirt-cpi  
Strategy: Four separate PRs, merged independently in the order listed.

---

## PR 1 — Security audit & fixes

### Issues

**1. SSH host key verification (`driver/ssh_runner.go`)**

`ssh.InsecureIgnoreHostKey()` means any server can impersonate the libvirt host.

Fix:
- Add `HostKey string` to `SSHRunnerOpts` and `FactoryOpts`.
- In `SSHRunner.client()`: parse `HostKey` with `ssh.FixedHostKey(pubkey)` and use it as `HostKeyCallback`.
- In `FactoryOpts.Validate()`: if `Host` is set and `HostKey` is empty, return an error — fail-closed.
- Add `host_key` property to `jobs/libvirt_cpi/spec` and wire it into `cpi.json.erb`.

Tests:
- `cpi/factory_options_test.go`: validation rejects SSH config missing `HostKey`.
- `driver/ssh_runner_test.go`: `client()` uses `FixedHostKey` when `HostKey` is set.

**2. HomeDir shell injection (`driver/ssh_runner.go:HomeDir`)**

Current: `` sh -c 'USER= HOME= eval echo ~`whoami`' ``  
The backtick subshell is an injection vector if username is attacker-controlled.

Fix: replace with `sh -c 'getent passwd $(id -u) | cut -d: -f6'` — no backtick subshell, no user-controlled input in the shell string.

**3. Path traversal in Store (`vm/store.go`)**

`filepath.Join(m.path, key)` is called with CID-derived keys. A key like `../../etc/passwd` would escape the store directory.

Fix:
- Add `sanitizeKey(key string) error` that rejects keys containing `..` or `/`.
- Call it at the top of `Put`, `Get`, and `DeleteOne`.

Tests:
- `vm/store_test.go`: `Put`/`Get`/`DeleteOne` reject `..` and `/` in key.

---

## PR 2 — Integration tests + unit test coverage

### Integration tests (fleshed out from skeletal placeholders)

**`driver/libvirt_driver_integration_test.go`** (gate: `LIBVIRT_URI`):
- Already has: define/lookup/destroy.
- Add: `StartDomain`, `ShutdownDomain`, `RebootDomain`, `CreateStorageVol`/`DeleteStorageVol`, `UpdateDomainMemory`/`UpdateDomainCPUs`, `IsMissingDomainErr`.

**`stemcell/stemcell_integration_test.go`** (gate: `LIBVIRT_URI` + `STEMCELL_PATH`):
- Full import flow: decompress tarball → upload → `Prepare()` → `Exists()` → `Delete()`.

**`vm/vm_integration_test.go`** (gate: `LIBVIRT_URI` + `STEMCELL_PATH`):
- Create VM from stemcell → attach persistent disk → `DiskIDs()` → detach → delete VM.

### Unit test gaps

| File | Tests to add |
|------|-------------|
| `disk/factory_test.go` | `Create()` success/error, `Find()`, disk path construction |
| `stemcell/factory_test.go` | `ImportFromPath()` success + partial-import cleanup |
| `vm/factory_test.go` | `Create()` success + `cleanUpPartialCreate` at each failure point |
| `vm/vm_disks_test.go` | `AttachDisk`, `DetachDisk`, `DiskIDs`, ephemeral vs persistent flag |
| `driver/libvirt_driver_test.go` | All `Driver` interface methods via `FakeLibvirtConn` |

### CI

`.github/workflows/tests.yml`:
- Existing unit test job runs `go test ./...` (no build tag).
- Add `integration` job: `go test -tags integration ./...`, requires `LIBVIRT_URI` secret, runs on `ubuntu-latest` with `libvirt-daemon-system` installed.

---

## PR 3 — BOSH release minimal correct rebuild

### Files changed

**`packages/libvirt_cpi/spec`**
- `name: virtualbox_cpi` → `name: libvirt_cpi`
- Files globs unchanged.

**`packages/libvirt_cpi/packaging`**
- Remove `CGO_ENABLED=0` (cgo required by `libvirt.org/go/libvirt`).
- Add `apt-get install -y libvirt-dev` (or document build host requirement in spec).
- Drop Darwin cross-compile — BOSH deployments run Linux, cross-cgo to Darwin is not supported. Build Linux only; rename output from `cpi-linux` to `cpi`.
- Use `golang-1-linux` as the sole golang package dependency; remove `golang-1-darwin` from the `spec` dependencies list.

**`jobs/libvirt_cpi/spec`**
- Remove: `storage_controller`, `auto_enable_networks`.
- Add: `backend_uri` (e.g. `qemu:///system`), `host_key` (SSH host public key, required when `host` is set).
- Fix all description strings referencing "VirtualBox".

**`jobs/libvirt_cpi/templates/cpi.erb`**
- Fix binary path: `virtualbox_cpi/bin/cpi-${platform}` → `libvirt_cpi/bin/cpi`.
- Fix config path: `virtualbox_cpi/config/cpi.json` → `libvirt_cpi/config/cpi.json`.

**`jobs/libvirt_cpi/templates/cpi.json.erb`**
- Rewrite to emit `FactoryOpts`-compatible JSON:
  ```json
  {
    "BackendURI": "<%= p('backend_uri') %>",
    "Host": "<%= p('host') %>",
    "Username": "<%= p('username') %>",
    "PrivateKey": "<%= p('private_key') %>",
    "HostKey": "<%= p('host_key') %>",
    "StoreDir": "<%= p('store_dir') %>",
    "Agent": {
      "mbus": "<%= p('agent.mbus') %>",
      "ntp": <%= p('ntp').to_json %>
    }
  }
  ```
- Remove: `BinPath`, `StorageController`, `AutoEnableNetworks`.

**`manifests/qemu-cpi.yml`** (new)
- Minimal BOSH manifest deploying `libvirt_cpi` job against a QEMU/KVM host.

---

## PR 4 — LXC and VBox support & examples

### Config examples

`config/cpi-lxc.json` — minimal valid `FactoryOpts` for LXC:
```json
{
  "BackendURI": "lxc:///",
  "StoreDir": "~/.bosh_libvirt_cpi",
  "Agent": {
    "mbus": "https://mbus:mbus-password@0.0.0.0:6868",
    "ntp": ["0.pool.ntp.org", "1.pool.ntp.org"],
    "blobstore": { "provider": "local", "options": { "blobstore_path": "/var/vcap/micro_bosh/data/cache" } }
  }
}
```

`config/cpi-vbox.json` — minimal valid `FactoryOpts` for VirtualBox:
```json
{
  "BackendURI": "vbox:///session",
  "StoreDir": "~/.bosh_libvirt_cpi",
  "Agent": {
    "mbus": "https://mbus:mbus-password@0.0.0.0:6868",
    "ntp": ["0.pool.ntp.org", "1.pool.ntp.org"],
    "blobstore": { "provider": "local", "options": { "blobstore_path": "/var/vcap/micro_bosh/data/cache" } }
  }
}
```

### Manifests

- `manifests/lxc-cpi.yml` — mirrors `qemu-cpi.yml` with `backend_uri: lxc:///`.
- `manifests/vbox-cpi.yml` — mirrors `qemu-cpi.yml` with `backend_uri: vbox:///session`.

### Docs

`docs/HYPERVISOR_CONFIGURATION.md` — fill in LXC and VBox sections:
- Prerequisites (packages to install per backend).
- Known limitations: VBox requires `libvirt-daemon-driver-vbox` and the libvirt user session; LXC requires root or unprivileged cgroup config.
- Working end-to-end config snippet per backend.

### Tests

`cpi/factory_options_test.go`:
- Add validation tests for `vbox:///session` and `lxc:///` URI schemes (currently untested despite the scheme switch in `factory.go`).

### README

Add a "Choosing a backend" decision table:

| Backend | URI | Use case |
|---------|-----|----------|
| QEMU/KVM | `qemu:///system` | Production, KVM-capable hosts |
| VirtualBox | `vbox:///session` | Desktop development |
| LXC | `lxc:///` | Container workloads, low overhead |

Link to `docs/HYPERVISOR_CONFIGURATION.md` for per-backend prerequisites.

---

## Merge order

1. PR 1 (security) — unblocks PR 3 (`host_key` property)
2. PR 2 (tests) — independent, can merge in parallel with PR 1
3. PR 3 (BOSH release) — depends on PR 1 for `host_key`
4. PR 4 (LXC/VBox examples) — independent, can merge any time after PR 3
