# VBox Support Design

**Date:** 2026-10-26
**Issue:** [#21](https://github.com/ZPascal/bosh-libvirt-cpi-release/issues/21)

## Summary

Add VirtualBox (via libvirt's `vbox:///session` driver) as a first-class backend alongside the existing QEMU and LXC backends. The `VBoxDomainBuilder` and manifest already exist; the missing pieces are: a VBox VM provisioner in `vm/factory.go`, a config drive ISO attached as a third IDE disk, and CI wiring for integration and E2E tests.

---

## Architecture

VBox support splits into four coherent areas:

1. **`driver/domain_builder.go`** — add `ConfigDisk string` to `DomainDiskPaths` (optional; empty = no config drive, zero-value backward-compatible)
2. **`driver/domains/vbox.go`** — attach `ConfigDisk` as a third IDE cdrom slot (`sdc`) when non-empty
3. **`vm/factory.go`** — add `provisionVBoxConfigDrive()` and wire it into the `Create()` switch on `"vmdk"`
4. **CI** — extend workflows and add `manifests/cloud-config-vbox.yml`

No existing backends are affected — LXC and QEMU pass `ConfigDisk: ""` implicitly so their domain XML is unchanged.

---

## Components & Data Flow

### 1. `DomainDiskPaths` (`driver/domain_builder.go`)

Add an optional field:

```go
type DomainDiskPaths struct {
    RootDisk      string
    EphemeralDisk string
    ConfigDisk    string // optional; when non-empty, attached as a read-only cdrom
}
```

### 2. `VBoxDomainBuilder.BuildDomain` (`driver/domains/vbox.go`)

When `disks.ConfigDisk != ""`, append a third `<disk>` element to the domain XML:

```xml
<disk type='file' device='cdrom'>
  <source file='/path/to/env.iso'/>
  <target dev='sdc' bus='ide'/>
  <readonly/>
</disk>
```

When `disks.ConfigDisk == ""` the output is identical to today.

### 3. `provisionVBoxConfigDrive()` (`vm/factory.go`)

New method on `Factory`. Signature mirrors the other provisioners:

```go
func (f Factory) provisionVBoxConfigDrive(
    vm VMImpl,
    stemcell bstem.Stemcell,
    ephemeralDisk bdisk.Disk,
    initialAgentEnv apiv1.AgentEnv,
) (driver.DomainDiskPaths, error)
```

Steps:

1. Serialize `initialAgentEnv` to JSON bytes via `addBlobstoreToEnv` + `injectMbusCert` (same pipeline as the ext4 provisioner).
2. Build an ISO9660 image: `ISO9660{FileName: "ENV", Contents: agentEnvBytes}.Bytes()`.
3. Write to the VM store as `env.iso` via `vm.store.Put("env.iso", isoBytes)`.
4. Return `driver.DomainDiskPaths{RootDisk: stemcell.ImagePath(), EphemeralDisk: ephemeralDisk.ImagePath(), ConfigDisk: vm.store.Path("env.iso")}`.

The `Create()` switch gains a `case "vmdk":` alongside `"dir"` and `"ext4"`:

```go
case "vmdk":
    disks, err = f.provisionVBoxConfigDrive(vm, stemcell, ephemeralDisk, initialAgentEnv)
    if err != nil {
        f.cleanUpPartialCreate(vm)
        return nil, err
    }
```

### 4. `reconfigureAgent` (`vm/vm_agent.go`)

Already writes `env.iso` to the store on disk attach/detach — no change needed. The ISO path in the domain XML points into the store, so updates are picked up at next VM start.

### 5. CI

- **`e2e.yml`** and **`setup-bosh-director.yml`**: add `vbox` to accepted `inputs.backend` values; add install step for `virtualbox libvirt-daemon-driver-vbox`; set backend-specific env vars (director IP, MAC, alias, deployment name).
- **`tests.yml`**: add `integration-vbox` and `e2e-vbox` jobs, mirroring the existing `qemu`/`lxc` job shapes.
- **`manifests/cloud-config-vbox.yml`**: cloud config with subnets matching the `vbox-cpi.yml` range (`192.168.1.0/24`).

---

## Error Handling

`provisionVBoxConfigDrive()` returns `(driver.DomainDiskPaths, error)`. On any failure (ISO build error, store write error), it returns the error and `Create()` calls `cleanUpPartialCreate()` — the same pattern used by the LXC and ext4 provisioners. No new cleanup logic is required.

The `ConfigDisk` interpolation in `VBoxDomainBuilder.BuildDomain` uses the existing `xmlEscape()` helper — no new error surface.

---

## Testing

### Unit tests

- **`driver/domains/vbox_test.go`**:
  - "includes cdrom disk when ConfigDisk is set"
  - "omits cdrom disk when ConfigDisk is empty" (existing behavior, regression guard)
  - "escapes XML special characters in ConfigDisk path"

- **`vm/factory_test.go`** (or a new `vm/factory_vbox_test.go`):
  - `provisionVBoxConfigDrive` writes `env.iso` to the store
  - returned `ConfigDisk` equals `store.Path("env.iso")`
  - returned `RootDisk` and `EphemeralDisk` are the stemcell and ephemeral disk paths respectively

### Integration tests

- **`vm/vm_integration_test.go`**: add a VBox-backend `Create` test against a real `vbox:///session` libvirt connection, mirroring the existing QEMU integration test shape.

### E2E tests

- New `vbox` job in `tests.yml` calling `e2e.yml` / `setup-bosh-director.yml` with `backend: vbox`.
- Validates a full `bosh create-env` + `bosh deploy` cycle on VirtualBox via libvirt.

---

## Files Changed

| File | Change |
|---|---|
| `driver/domain_builder.go` | Add `ConfigDisk string` to `DomainDiskPaths` |
| `driver/domains/vbox.go` | Attach `ConfigDisk` as cdrom when non-empty |
| `driver/domains/vbox_test.go` | New test cases for ConfigDisk |
| `vm/factory.go` | Add `provisionVBoxConfigDrive()`, wire into `Create()` switch |
| `vm/factory_test.go` | New tests for `provisionVBoxConfigDrive` |
| `vm/vm_integration_test.go` | VBox Create integration test |
| `.github/workflows/e2e.yml` | Add `vbox` backend support |
| `.github/workflows/setup-bosh-director.yml` | Add `vbox` backend support |
| `.github/workflows/tests.yml` | Add `integration-vbox` and `e2e-vbox` jobs |
| `manifests/cloud-config-vbox.yml` | New cloud config for VBox subnet |

---

## Non-Goals

- Config drive support for QEMU (Option B from design discussion) — future enhancement.
- Injecting agent env directly into vmdk (Option C) — too fragile, requires FUSE/nbd.
- Any change to the LXC or QEMU provisioning paths.
