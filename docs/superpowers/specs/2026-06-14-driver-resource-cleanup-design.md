# Driver Resource Cleanup & Bug Fixes

**Date:** 2026-06-14
**Scope:** `driver/`, `vm/`, `stemcell/`

## Problem

The libvirt Go bindings are reference-counted at the C level. `vendor/libvirt.org/go/libvirt/doc.go` requires calling `Free()` on every `*libvirt.Domain`, `*libvirt.StoragePool`, and `*libvirt.StorageVol` obtained from a lookup. None of the current driver code does this, causing C-level reference leaks on every domain or storage operation. Over time this exhausts file descriptors and connection handles.

Additionally:
- `UpdateDomainMemory` and `UpdateDomainCPUs` set current value before max, which libvirt rejects when increasing resources (it enforces `current <= max` at all times).
- `DestroyDomain` bypasses `withDomain`, has no nil-domain guard, and duplicates lookup logic.
- `LookupDomain` has no nil-domain guard (inconsistent with `withDomain`).

## Design

### 1. Interface: add `Free()` to `Domain`

`driver/interfaces.go`:

```go
type Domain interface {
    GetName()  (string, error)
    GetState() (int, int, error)
    IsActive() (bool, error)
    Free() error
}
```

`LibvirtDomainWrapper` gains:

```go
func (w *LibvirtDomainWrapper) Free() error { return w.dom.Free() }
```

`driver/fakes/fake_domain.go` gains a no-op stub:

```go
func (d *FakeDomain) Free() error { return nil }
```

### 2. `libvirt_driver.go` changes

**`withDomain`** — add `defer dom.Free()` after nil guard:

```go
func (d LibvirtDriver) withDomain(id string, fn func(*libvirt.Domain) error) error {
    dom, err := d.conn.LookupDomainByName(id)
    if err != nil { return err }
    if dom == nil { return fmt.Errorf("domain '%s' not found", id) }
    defer dom.Free()
    return fn(dom)
}
```

**`DestroyDomain`** — refactor to use `withDomain` (removes duplicated lookup, adds nil guard and Free automatically):

```go
func (d LibvirtDriver) DestroyDomain(id string) error {
    return d.withDomain(id, func(dom *libvirt.Domain) error {
        if err := dom.Destroy(); err != nil {
            lverr, ok := err.(libvirt.Error)
            isNotRunning := ok && lverr.Code == libvirt.ERR_OPERATION_INVALID
            if !errors.Is(err, libvirt.ERR_NO_DOMAIN) && !isNotRunning {
                return err
            }
        }
        return dom.Undefine()
    })
}
```

**`LookupDomain`** — add nil guard:

```go
func (d LibvirtDriver) LookupDomain(id string) (Domain, error) {
    dom, err := d.conn.LookupDomainByName(id)
    if err != nil { return nil, err }
    if dom == nil { return nil, fmt.Errorf("domain '%s' not found", id) }
    return &LibvirtDomainWrapper{dom}, nil
}
```

Caller owns the reference and must call `dom.Free()`.

**`UpdateDomainMemory` / `UpdateDomainCPUs`** — set max before current (safe for both increases and decreases):

```go
// Memory
dom.SetMemoryFlags(kib, libvirt.DOMAIN_MEM_CONFIG|libvirt.DOMAIN_MEM_MAXIMUM) // max first
dom.SetMemoryFlags(kib, libvirt.DOMAIN_MEM_CONFIG)                             // then current

// CPUs
dom.SetVcpusFlags(uint(cpus), libvirt.DOMAIN_VCPU_CONFIG|libvirt.DOMAIN_VCPU_MAXIMUM) // max first
dom.SetVcpusFlags(uint(cpus), libvirt.DOMAIN_VCPU_CONFIG)                             // then current
```

**Storage methods** — add `defer pool.Free()` and `defer vol.Free()` after each successful lookup in `CreateStorageVol` and `DeleteStorageVol`. Add nil guards before defers (since fakes return nil).

### 3. Callers of `LookupDomain`

Three call sites; all add `defer dom.Free()` immediately after a successful lookup:

- `vm/vm_state.go` `Exists()` — domain used only to confirm existence
- `vm/vm_state.go` `IsRunning()` — domain used to call `GetState()`
- `stemcell/stemcell.go` `Exists()` — domain used only to confirm existence

### 4. Tests

- `driver/fakes/fake_domain.go`: add `Free() error { return nil }` stub (required by compile-time interface check)
- `driver/libvirt_driver_test.go`: add test that `DestroyDomain` with nil-domain return errors rather than panics
- Existing tests unchanged — `FakeDriver.LookupDomain` returns `FakeDomain` which now satisfies the updated interface

## Files Changed

| File | Change |
|------|--------|
| `driver/interfaces.go` | Add `Free()` to `Domain` interface |
| `driver/libvirt_driver.go` | `withDomain` defer Free; refactor `DestroyDomain`; nil guard `LookupDomain`; fix resize order; storage defers |
| `driver/fakes/fake_domain.go` | Add `Free()` stub |
| `vm/vm_state.go` | `defer dom.Free()` in `Exists()` and `IsRunning()` |
| `stemcell/stemcell.go` | `defer dom.Free()` in `Exists()` |
| `driver/libvirt_driver_test.go` | Add `DestroyDomain` nil-domain test |
