---
name: ext4-bug-fixes
description: Fix four pre-existing bugs in the QEMU ext4 VM creation path in vm/factory.go
metadata:
  type: project
---

## Context

All four bugs are in the `else if f.domBuilder.DiskImageFormat() == "ext4"` branch of
`Create()` in `src/bosh-libvirt-cpi/vm/factory.go`. This branch handles QEMU kernel-boot
VMs by loop-mounting the stemcell ext4 image and injecting files into it.

## Bug 1 (Critical): Silent mount failure → VM created with uninitialised rootfs

**Location:** Line 607.

`if _, err := execCommand("mount", "-o", "loop", vmExt4, mntDir); err == nil {` — when
mount fails the entire injection block is skipped. Execution falls through to
`disks = driver.DomainDiskPaths{RootDisk: vmExt4}` and `Create()` returns nil. The
resulting VM has no `warden-cpi-agent-env.json`, no `/bosh-init`, and no mbus cert —
the agent cannot bootstrap.

**Fix:** Check the mount error explicitly. On failure: remove mntDir, call
`cleanUpPartialCreate`, return a wrapped error. On success: proceed with injection,
then unconditionally unmount+remove after (whether or not inner steps succeed).

Structure change — from nested `if err == nil` guards to:
```go
if out, mountErr := execCommand("mount", "-o", "loop", vmExt4, mntDir); mountErr != nil {
    _ = os.RemoveAll(mntDir)
    f.cleanUpPartialCreate(vm)
    return nil, bosherr.WrapErrorf(mountErr, "Mounting ext4 for VM injection: %s", string(out))
}
// ... injection ...
_, _ = execCommand("umount", mntDir)
_ = os.RemoveAll(mntDir)
```

## Bug 2 (Critical): `AsBytes()` error discarded → zero-byte agent env written

**Location:** Line 614.

`envBytes, _ := initialAgentEnv.AsBytes()` discards the error. If marshalling fails,
`envBytes` is nil, `addBlobstoreToEnv(nil)` and `injectMbusCert(nil)` both return nil,
and a zero-byte `warden-cpi-agent-env.json` is written. `Create()` returns success
while the agent cannot register.

**Fix:** Check the error. On failure: unmount cleanly, remove mntDir, call
`cleanUpPartialCreate`, return a wrapped error. Mirrors the dir branch at line 211.

```go
envBytes, err := initialAgentEnv.AsBytes()
if err != nil {
    _, _ = execCommand("umount", mntDir)
    _ = os.RemoveAll(mntDir)
    f.cleanUpPartialCreate(vm)
    return nil, bosherr.WrapError(err, "Marshalling agent env for ext4 rootfs injection")
}
```

## Bug 3 (Minor): Double `injectMbusCert` call generates two independent TLS keypairs

**Location:** Lines 617 and 620.

`agentEnvBytes2 := f.injectMbusCert(addBlobstoreToEnv(envBytes))` then immediately
`qemuStaticIP, _ := extractNetworkFromEnv(f.injectMbusCert(addBlobstoreToEnv(envBytes)))`.
Each call to `injectMbusCert` (when `MbusBootstrapSSL` is unset) generates a fresh ECDSA
keypair via `ecdsa.GenerateKey`. The second keypair is discarded immediately — wasteful
and the two values are silently inconsistent.

**Fix:** Call `injectMbusCert` once, reuse the result:
```go
agentEnvBytes2 := f.injectMbusCert(addBlobstoreToEnv(envBytes))
_ = os.WriteFile(boshDir+"/warden-cpi-agent-env.json", agentEnvBytes2, 0644)
qemuStaticIP, _ := extractNetworkFromEnv(agentEnvBytes2)
```

## Bug 4 (Minor): `injectCert` replaces entire `mbus` map, silently drops sibling keys

**Location:** `injectCert` function, line ~1046.

`bosh["mbus"] = map[string]interface{}{"cert": {...}}` replaces the whole `mbus` map.
Any existing `mbus.url` (or future sibling keys) is silently dropped.

**Fix:** Read the existing `mbus` map and merge `cert` into it:
```go
mbus, _ := bosh["mbus"].(map[string]interface{})
if mbus == nil {
    mbus = map[string]interface{}{}
}
mbus["cert"] = map[string]interface{}{
    "ca":          ca,
    "certificate": cert,
    "private_key": key,
}
bosh["mbus"] = mbus
```

## Files Changed

- `src/bosh-libvirt-cpi/vm/factory.go` — four targeted edits, no interface changes, no new files
