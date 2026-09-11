# LXC Integration Test Design

Date: 2026-08-12

## Summary

Add a full end-to-end LXC integration test to GitHub Actions that:
1. Runs the existing Go integration tests against `lxc:///`
2. Bootstraps a BOSH Director via `bosh create-env` using the LXC CPI
3. Deploys a minimal one-instance deployment via `bosh deploy`
4. Provides a standalone `config/cpi-lxc-local.json` for colleague handoff

## CI Job Structure

The existing `integration-tests` job is renamed to `integration-tests-qemu`. A new `integration-tests-lxc` job runs in parallel after `build`:

```
unit-tests ──► build ──► integration-tests-qemu
                     └──► integration-tests-lxc
                     └──► code-quality
```

Both integration jobs depend on `build` and are otherwise independent.

### LXC job setup

Install: `lxc libvirt-daemon-driver-lxc libvirt-dev` (no `qemu-kvm`).

```yaml
- name: Install LXC + libvirt
  run: |
    sudo apt-get update
    sudo apt-get install -y lxc libvirt-daemon-system libvirt-daemon-driver-lxc libvirt-dev
    sudo systemctl start libvirtd
    sudo usermod -aG libvirt $USER
```

Environment:
```yaml
env:
  LIBVIRT_URI: lxc:///
  STEMCELL_PATH: /tmp/bosh-stemcell.tgz
  INTERNAL_IP: 127.0.0.1
```

The `INTERNAL_IP` is set to `127.0.0.1` since the BOSH Director and CPI run on the same runner (no SSH hop needed).

## Go Integration Test Changes

### Problem

All three existing integration test files hardcode `domains.QEMUDomainBuilder{}`:
- `driver/libvirt_driver_integration_test.go`
- `stemcell/stemcell_integration_test.go`
- `vm/vm_integration_test.go`

### Solution

Add a shared helper per package that selects the domain builder from `LIBVIRT_URI`, mirroring what `cpi/factory.go` already does:

```go
// integration_helpers_test.go (added to each affected package)
func domBuilderFromEnv() driver.DomainBuilder {
    uri := os.Getenv("LIBVIRT_URI")
    u, _ := url.Parse(uri)
    switch u.Scheme {
    case "lxc":
        return domains.LXCDomainBuilder{}
    case "vbox":
        return domains.VBoxDomainBuilder{}
    default:
        return domains.QEMUDomainBuilder{}
    }
}
```

Each `BeforeEach` in the three test files replaces `domains.QEMUDomainBuilder{}` with `domBuilderFromEnv()`. No test logic changes, no duplicate files — same suite covers QEMU and LXC based on the environment variable.

The driver integration test also uses a hardcoded `<domain type='kvm'>` XML block for the define/lookup/destroy and update tests. A helper `testDomainXML(name string) string` replaces it, returning the correct XML based on the URI scheme:

```go
func testDomainXML(name string) string {
    uri := os.Getenv("LIBVIRT_URI")
    u, _ := url.Parse(uri)
    if u.Scheme == "lxc" {
        return fmt.Sprintf(`<domain type='lxc'>
  <name>%s</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type>exe</type><init>/sbin/init</init></os>
  <devices><emulator>/usr/lib/libvirt/libvirt_lxc</emulator></devices>
</domain>`, name)
    }
    return fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
</domain>`, name)
}
```

This helper lives in `driver/integration_helpers_test.go` alongside `domBuilderFromEnv()`.

## Manifests

### Existing `manifests/lxc-cpi.yml`

Kept intact for remote/colleague use (requires SSH vars). Not used by CI directly.

### New `manifests/ops/lxc-local.yml`

Ops file that removes SSH connection fields so `bosh create-env` can run locally on the runner without any secrets:

```yaml
- type: remove
  path: /instance_groups/name=bosh/properties/libvirt_cpi/host
- type: remove
  path: /instance_groups/name=bosh/properties/libvirt_cpi/username
- type: remove
  path: /instance_groups/name=bosh/properties/libvirt_cpi/private_key
- type: remove
  path: /instance_groups/name=bosh/properties/libvirt_cpi/host_key
- type: remove
  path: /cloud_provider/properties/libvirt_cpi/host
- type: remove
  path: /cloud_provider/properties/libvirt_cpi/username
- type: remove
  path: /cloud_provider/properties/libvirt_cpi/private_key
- type: remove
  path: /cloud_provider/properties/libvirt_cpi/host_key
```

CI invocation:
```bash
bosh create-env manifests/lxc-cpi.yml \
  -o manifests/ops/lxc-local.yml \
  -o manifests/local-release.yml \
  -v director_name=bosh-lxc-ci \
  -v internal_ip=127.0.0.1 \
  -v stemcell_url=/tmp/bosh-stemcell.tgz \
  --state /tmp/bosh-state.json
```

### New `manifests/lxc-deployment.yml`

Minimal single-instance deployment using only the `nats` job (smallest self-contained BOSH job). Deployed after `bosh create-env` succeeds:

```yaml
name: lxc-test-deployment
releases:
- name: bosh
  version: latest
instance_groups:
- name: nats
  instances: 1
  jobs:
  - name: nats
    release: bosh
  vm_type: default
  stemcell: default
  networks:
  - name: default
stemcells:
- alias: default
  os: ubuntu-jammy
  version: latest
update:
  canaries: 1
  max_in_flight: 1
  canary_watch_time: 1000-30000
  update_watch_time: 1000-30000
```

## Colleague Config

### New `config/cpi-lxc-local.json`

Standalone config for local LXC use without SSH. Forwarded to colleagues who want to run the CPI locally:

```json
{
  "BackendURI": "lxc:///",
  "StoreDir": "~/.bosh_libvirt_cpi",
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

Prerequisites for the colleague:
```bash
sudo apt-get install -y lxc libvirt-daemon-system libvirt-daemon-driver-lxc libvirt-dev
sudo systemctl start libvirtd
sudo usermod -aG libvirt $USER
# re-login or: newgrp libvirt
virsh -c lxc:/// list --all   # verify connection
```

## Files Changed / Added

| File | Change |
|------|--------|
| `.github/workflows/tests.yml` | Rename `integration-tests` → `integration-tests-qemu`; add `integration-tests-lxc` job |
| `driver/libvirt_driver_integration_test.go` | Replace hardcoded QEMU domain XML + builder with URI-driven selection |
| `stemcell/stemcell_integration_test.go` | Replace `QEMUDomainBuilder{}` with `domBuilderFromEnv()` |
| `vm/vm_integration_test.go` | Replace `QEMUDomainBuilder{}` with `domBuilderFromEnv()` |
| `driver/integration_helpers_test.go` | New — `domBuilderFromEnv()` helper |
| `stemcell/integration_helpers_test.go` | New — `domBuilderFromEnv()` helper |
| `vm/integration_helpers_test.go` | New — `domBuilderFromEnv()` helper |
| `manifests/ops/lxc-local.yml` | New — removes SSH fields for local CI use |
| `manifests/lxc-deployment.yml` | New — minimal nats deployment for smoke test |
| `config/cpi-lxc-local.json` | New — colleague handoff config |