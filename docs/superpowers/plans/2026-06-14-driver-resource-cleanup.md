# Driver Resource Cleanup & Bug Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix C-level reference leaks by calling `Free()` on all libvirt objects, refactor `DestroyDomain` to use `withDomain`, add nil guards, and fix VM resize call order.

**Architecture:** Add `Free()` to the `Domain` interface so callers of `LookupDomain` can release references. Use `defer` inside `withDomain`, storage methods, and all `LookupDomain` call sites. Refactor `DestroyDomain` to delegate to `withDomain` to eliminate duplicated lookup logic and the missing nil guard.

**Tech Stack:** Go 1.21+, libvirt Go bindings (`libvirt.org/go/libvirt`), Ginkgo/Gomega test framework. Run tests with `CGO_ENABLED=1 go test ./...` from `src/bosh-libvirt-cpi/`.

---

## File Map

| File | Change |
|------|--------|
| `driver/interfaces.go` | Add `Free() error` to `Domain` interface |
| `driver/libvirt_driver.go` | `defer dom.Free()` in `withDomain`; refactor `DestroyDomain`; nil guard in `LookupDomain`; fix resize order; `defer pool/vol.Free()` in storage methods |
| `driver/fakes/fake_domain.go` | Add `Free() error` no-op stub |
| `vm/vm_state.go` | `defer dom.Free()` in `Exists()` and `IsRunning()` |
| `stemcell/stemcell.go` | `defer dom.Free()` in `Exists()` |
| `driver/libvirt_driver_test.go` | Add `DestroyDomain` nil-domain test |

---

### Task 1: Add `Free()` to the `Domain` interface and fake

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/interfaces.go`
- Modify: `src/bosh-libvirt-cpi/driver/fakes/fake_domain.go`
- Modify: `src/bosh-libvirt-cpi/driver/libvirt_driver.go`

The libvirt Go vendor docs (`vendor/libvirt.org/go/libvirt/doc.go`) require calling `Free()` on every `*libvirt.Domain`. The `Domain` interface is currently missing this method, which means callers of `LookupDomain` have no way to release the reference.

- [ ] **Step 1: Add `Free()` to `driver/interfaces.go`**

Replace the `Domain` interface block:

```go
type Domain interface {
	GetName() (string, error)
	GetState() (int, int, error)
	IsActive() (bool, error)
	Free() error
}
```

- [ ] **Step 2: Add `Free()` stub to `driver/fakes/fake_domain.go`**

Append after the existing `IsActive` method (the compile-time check `var _ driver.Domain = &FakeDomain{}` on line 17 will fail until this is added):

```go
func (d *FakeDomain) Free() error { return nil }
```

- [ ] **Step 3: Add `Free()` to `LibvirtDomainWrapper` in `driver/libvirt_driver.go`**

Append after the existing `IsActive` method on the wrapper (around line 168):

```go
func (w *LibvirtDomainWrapper) Free() error { return w.dom.Free() }
```

- [ ] **Step 4: Verify it compiles**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go build ./...
```

Expected: no errors (linker warning about duplicate libraries is pre-existing and harmless).

- [ ] **Step 5: Run tests to confirm nothing broke**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/interfaces.go \
        src/bosh-libvirt-cpi/driver/fakes/fake_domain.go \
        src/bosh-libvirt-cpi/driver/libvirt_driver.go
git commit -m "feat: add Free() to Domain interface and implement on wrapper and fake"
```

---

### Task 2: Fix `withDomain` — add `defer dom.Free()`

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/libvirt_driver.go:29-38`
- Test: `src/bosh-libvirt-cpi/driver/libvirt_driver_test.go`

`withDomain` looks up a `*libvirt.Domain` and passes it to a callback but never frees it. Since `FakeLibvirtConn.LookupDomainByName` returns `(nil, nil)` by default, the nil guard fires before `Free()` would be called in tests — so we can't write a unit test that directly verifies `Free()` was called. Instead we add a comment explaining this, and trust the defer is correct by inspection.

- [ ] **Step 1: Write a failing test for nil-domain guard in `withDomain` (via `ShutdownDomain`)**

Add to the `Describe("StartDomain / ShutdownDomain / RebootDomain")` block in `src/bosh-libvirt-cpi/driver/libvirt_driver_test.go`:

```go
It("returns error when lookup returns nil domain with no error for Shutdown", func() {
    // conn returns (nil, nil) by default — withDomain must guard against nil
    Expect(d.ShutdownDomain("vm-1")).To(HaveOccurred())
})
```

- [ ] **Step 2: Run to verify test already passes (nil guard was added earlier)**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./driver/...
```

Expected: PASS (nil guard already exists; this test documents the contract).

- [ ] **Step 3: Add `defer dom.Free()` to `withDomain`**

Replace the `withDomain` function body in `src/bosh-libvirt-cpi/driver/libvirt_driver.go`:

```go
func (d LibvirtDriver) withDomain(id string, fn func(*libvirt.Domain) error) error {
	dom, err := d.conn.LookupDomainByName(id)
	if err != nil {
		return err
	}
	if dom == nil {
		return fmt.Errorf("domain '%s' not found", id)
	}
	defer dom.Free() //nolint // releases C-level reference per libvirt Go binding docs
	return fn(dom)
}
```

- [ ] **Step 4: Run tests**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./driver/...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/libvirt_driver.go \
        src/bosh-libvirt-cpi/driver/libvirt_driver_test.go
git commit -m "fix: defer dom.Free() in withDomain to release C-level reference"
```

---

### Task 3: Refactor `DestroyDomain` to use `withDomain`

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/libvirt_driver.go:56-70`
- Test: `src/bosh-libvirt-cpi/driver/libvirt_driver_test.go`

`DestroyDomain` currently calls `LookupDomainByName` directly, duplicating the nil-guard and Free logic. Routing it through `withDomain` removes all three problems at once.

- [ ] **Step 1: Write a failing test for the nil-domain case in `DestroyDomain`**

Add to the `Describe("DestroyDomain")` block in `src/bosh-libvirt-cpi/driver/libvirt_driver_test.go`:

```go
It("returns error when lookup returns nil domain with no error", func() {
    // FakeLibvirtConn returns (nil, nil) by default
    Expect(d.DestroyDomain("vm-1")).To(HaveOccurred())
})
```

- [ ] **Step 2: Run to verify the test fails (currently DestroyDomain panics on nil)**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./driver/... -run "DestroyDomain"
```

Expected: FAIL (panic or test failure because the current code calls `dom.Destroy()` on a nil pointer).

- [ ] **Step 3: Replace `DestroyDomain` to route through `withDomain`**

Replace the entire `DestroyDomain` function in `src/bosh-libvirt-cpi/driver/libvirt_driver.go`:

```go
func (d LibvirtDriver) DestroyDomain(id string) error {
	d.logger.Debug(d.logTag, "Destroying domain '%s'", id)
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

- [ ] **Step 4: Run tests**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./driver/...
```

Expected: all pass including the new nil-domain test.

- [ ] **Step 5: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/libvirt_driver.go \
        src/bosh-libvirt-cpi/driver/libvirt_driver_test.go
git commit -m "fix: refactor DestroyDomain to use withDomain (nil guard + Free)"
```

---

### Task 4: Fix `LookupDomain` nil guard and add `defer Free()` to callers

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/libvirt_driver.go:77-84`
- Modify: `src/bosh-libvirt-cpi/vm/vm_state.go`
- Modify: `src/bosh-libvirt-cpi/stemcell/stemcell.go`
- Test: `src/bosh-libvirt-cpi/driver/libvirt_driver_test.go`

`LookupDomain` returns a `Domain` to the caller who then owns the C reference. The caller must `defer dom.Free()`. Currently `LookupDomain` also has no nil guard (inconsistent with `withDomain`).

- [ ] **Step 1: Write a failing test for nil-domain in `LookupDomain`**

Add to the `Describe("LookupDomain")` block in `src/bosh-libvirt-cpi/driver/libvirt_driver_test.go`:

```go
It("returns error when lookup returns nil domain with no error", func() {
    // FakeLibvirtConn returns (nil, nil) by default
    _, err := d.LookupDomain("vm-1")
    Expect(err).To(HaveOccurred())
})
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./driver/... -run "LookupDomain"
```

Expected: FAIL (currently returns `(&LibvirtDomainWrapper{nil}, nil)` — no error).

- [ ] **Step 3: Add nil guard to `LookupDomain` in `driver/libvirt_driver.go`**

Replace the `LookupDomain` function:

```go
func (d LibvirtDriver) LookupDomain(id string) (Domain, error) {
	d.logger.Debug(d.logTag, "Looking up domain '%s'", id)
	dom, err := d.conn.LookupDomainByName(id)
	if err != nil {
		return nil, err
	}
	if dom == nil {
		return nil, fmt.Errorf("domain '%s' not found", id)
	}
	return &LibvirtDomainWrapper{dom}, nil
}
```

- [ ] **Step 4: Run driver tests**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./driver/...
```

Expected: all pass including the new nil test.

- [ ] **Step 5: Add `defer dom.Free()` to `vm/vm_state.go` `Exists()`**

Replace the `Exists` function:

```go
func (vm VMImpl) Exists() (bool, error) {
	dom, err := vm.driver.LookupDomain(vm.cid.AsString())
	if err != nil {
		if vm.driver.IsMissingDomainErr(err) {
			return false, nil
		}
		return false, bosherr.WrapErrorf(err, "Looking up domain '%s'", vm.cid.AsString())
	}
	defer dom.Free()
	return true, nil
}
```

- [ ] **Step 6: Add `defer dom.Free()` to `vm/vm_state.go` `IsRunning()`**

Replace the `IsRunning` function:

```go
func (vm VMImpl) IsRunning() (bool, error) {
	dom, err := vm.driver.LookupDomain(vm.cid.AsString())
	if err != nil {
		if vm.driver.IsMissingDomainErr(err) {
			return false, nil
		}
		return false, bosherr.WrapErrorf(err, "Looking up domain '%s'", vm.cid.AsString())
	}
	defer dom.Free()

	state, _, err := dom.GetState()
	if err != nil {
		return false, bosherr.WrapErrorf(err, "Getting domain state '%s'", vm.cid.AsString())
	}

	return state == int(libvirt.DOMAIN_RUNNING), nil
}
```

- [ ] **Step 7: Add `defer dom.Free()` to `stemcell/stemcell.go` `Exists()`**

Replace the `Exists` function:

```go
func (s StemcellImpl) Exists() (bool, error) {
	dom, err := s.driver.LookupDomain(s.cid.AsString())
	if err != nil {
		if s.driver.IsMissingDomainErr(err) {
			return false, nil
		}
		return false, bosherr.WrapErrorf(err, "Looking up stemcell domain '%s'", s.cid.AsString())
	}
	defer dom.Free()
	return true, nil
}
```

- [ ] **Step 8: Run all tests**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./...
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/libvirt_driver.go \
        src/bosh-libvirt-cpi/driver/libvirt_driver_test.go \
        src/bosh-libvirt-cpi/vm/vm_state.go \
        src/bosh-libvirt-cpi/stemcell/stemcell.go
git commit -m "fix: nil guard in LookupDomain; defer dom.Free() in all LookupDomain callers"
```

---

### Task 5: Fix VM resize call order (`UpdateDomainMemory` / `UpdateDomainCPUs`)

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/libvirt_driver.go:86-107`

libvirt enforces `current <= max` at all times. Setting current before max works for decreases but fails for increases (libvirt rejects `current > max`). The correct general-purpose order is max first, then current.

The current code does `DOMAIN_MEM_CONFIG` (current) → `DOMAIN_MEM_CONFIG|DOMAIN_MEM_MAXIMUM` (max), which is wrong for increases. The previous version before our session did `CONFIG|MAXIMUM` → `CONFIG`, which was wrong for decreases. The fix is: always set max first, then current — this is safe in both directions.

- [ ] **Step 1: Replace `UpdateDomainMemory` in `driver/libvirt_driver.go`**

```go
func (d LibvirtDriver) UpdateDomainMemory(id string, memoryMB int) error {
	d.logger.Debug(d.logTag, "Updating memory for domain '%s' to %dMB", id, memoryMB)
	return d.withDomain(id, func(dom *libvirt.Domain) error {
		kib := uint64(memoryMB) * 1024
		// Set max before current: safe for both increases and decreases.
		// libvirt enforces current <= max; setting max first ensures the
		// constraint holds regardless of direction.
		if err := dom.SetMemoryFlags(kib, libvirt.DOMAIN_MEM_CONFIG|libvirt.DOMAIN_MEM_MAXIMUM); err != nil {
			return err
		}
		return dom.SetMemoryFlags(kib, libvirt.DOMAIN_MEM_CONFIG)
	})
}
```

- [ ] **Step 2: Replace `UpdateDomainCPUs` in `driver/libvirt_driver.go`**

```go
func (d LibvirtDriver) UpdateDomainCPUs(id string, cpus int) error {
	d.logger.Debug(d.logTag, "Updating CPUs for domain '%s' to %d", id, cpus)
	return d.withDomain(id, func(dom *libvirt.Domain) error {
		// Set max before current: safe for both increases and decreases.
		if err := dom.SetVcpusFlags(uint(cpus), libvirt.DOMAIN_VCPU_CONFIG|libvirt.DOMAIN_VCPU_MAXIMUM); err != nil {
			return err
		}
		return dom.SetVcpusFlags(uint(cpus), libvirt.DOMAIN_VCPU_CONFIG)
	})
}
```

- [ ] **Step 3: Run tests**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./driver/...
```

Expected: all pass (existing nil-domain error tests still pass since `withDomain` fires before the flags calls).

- [ ] **Step 4: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/libvirt_driver.go
git commit -m "fix: set memory/CPU max before current to support both scale-up and scale-down"
```

---

### Task 6: Add `defer Free()` for storage pool and volume references

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/libvirt_driver.go:109-145`

`CreateStorageVol` and `DeleteStorageVol` look up `*libvirt.StoragePool` (and `*libvirt.StorageVol`) without ever calling `Free()`. Since `FakeLibvirtConn` returns nil for both, we must guard before deferring.

- [ ] **Step 1: Replace `CreateStorageVol` in `driver/libvirt_driver.go`**

```go
func (d LibvirtDriver) CreateStorageVol(poolName, volName string, sizeMB int) (string, error) {
	d.logger.Debug(d.logTag, "Creating storage vol '%s' in pool '%s'", volName, poolName)
	pool, err := d.conn.LookupStoragePoolByName(poolName)
	if err != nil {
		return "", err
	}
	if pool != nil {
		defer pool.Free()
	}
	sizeBytes := uint64(sizeMB) * 1024 * 1024
	xml := fmt.Sprintf(`<volume><name>%s</name><capacity unit="bytes">%d</capacity></volume>`, xmlEscape(volName), sizeBytes)
	vol, err := pool.StorageVolCreateXML(xml, 0)
	if err != nil {
		return "", err
	}
	if vol != nil {
		defer vol.Free()
	}
	path, err := vol.GetPath()
	if err != nil {
		return "", err
	}
	return path, nil
}
```

- [ ] **Step 2: Replace `DeleteStorageVol` in `driver/libvirt_driver.go`**

```go
func (d LibvirtDriver) DeleteStorageVol(poolName, volName string) error {
	d.logger.Debug(d.logTag, "Deleting storage vol '%s' from pool '%s'", volName, poolName)
	pool, err := d.conn.LookupStoragePoolByName(poolName)
	if err != nil {
		if errors.Is(err, libvirt.ERR_NO_STORAGE_POOL) {
			return nil
		}
		return err
	}
	if pool != nil {
		defer pool.Free()
	}
	vol, err := pool.LookupStorageVolByName(volName)
	if err != nil {
		if errors.Is(err, libvirt.ERR_NO_STORAGE_VOL) {
			return nil
		}
		return err
	}
	if vol != nil {
		defer vol.Free()
	}
	return vol.Delete(libvirt.STORAGE_VOL_DELETE_NORMAL)
}
```

- [ ] **Step 3: Run all tests**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./...
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/libvirt_driver.go
git commit -m "fix: defer pool.Free() and vol.Free() in storage operations"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run the full test suite**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go test ./...
```

Expected output (warnings are pre-existing and harmless):
```
ok  bosh-libvirt-cpi/cpi
ok  bosh-libvirt-cpi/disk
ok  bosh-libvirt-cpi/driver
ok  bosh-libvirt-cpi/driver/domains
ok  bosh-libvirt-cpi/main
ok  bosh-libvirt-cpi/stemcell
ok  bosh-libvirt-cpi/vm
```

- [ ] **Step 2: Verify build is clean**

```bash
cd src/bosh-libvirt-cpi && CGO_ENABLED=1 go build ./...
```

Expected: no errors.

- [ ] **Step 3: Review the full diff**

```bash
git diff main...HEAD -- src/bosh-libvirt-cpi/
```

Confirm: every `LookupDomainByName` call site either defers `dom.Free()` via `withDomain`, or returns a `Domain` whose caller immediately defers `Free()`. Every storage lookup defers `Free()` on success.
