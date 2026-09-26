# Stemcell Remote-Host Fix Design

## Problem

Two CI failures share the same root class: stemcell factory operations that must run on
the libvirt host are instead running locally (inside a deployed director VM).

### QEMU: Kernel panic — `/bosh-init` missing

`bosh create-env` fails: the director VM boots the ext4 disk image but immediately panics:

```
Kernel panic - not syncing: Requested init /bosh-init failed (error -2)
```

Diagnostic: the mounted per-VM `rootfs.img` is empty (no data/jobs, no log files).

**Root cause:** In `vm/factory.go`, after `qemu-img resize rootfs.img 65G` the file grows
from ~2 GB to 65 GB, but the ext4 superblock still records 2 GB.  The next step runs
`e2fsck -f -y rootfs.img`.  e2fsck detects the physical-size / superblock-size mismatch
and, because `-y` auto-confirms all repairs, it "fixes" the filesystem destructively —
leaving the ext4 unreadable.  The subsequent `resize2fs` and `mount` may appear to
succeed (or the mount error is swallowed non-fatally), but the filesystem contains no
usable data, so `/bosh-init` is never found at boot.

**Fix:** Remove `e2fsck` from the resize sequence.  `resize2fs` accepts a `-f` flag that
forces the resize without requiring a prior e2fsck pass; the image is a fresh `cp` of the
stemcell and is known-clean.

### LXC: `cp: cannot stat '.../image.dir/.'`

The deployed LXC director's CPI runs inside an LXC container.  During `bosh
upload-stemcell`, `stemcell/factory.go` `dir` case runs:

```go
os.MkdirAll(dstImage, 0755)
exec.Command("tar", "-xzf", imagePath, "-C", dstImage)
```

Both calls execute **inside the container**.  When `create_vm` later runs:

```go
f.runner.Execute("cp", "-a", stemcell.ImagePath()+"/.", vmRootfs)
```

…the SSH runner sends that `cp` to the libvirt **host**, where `image.dir` never existed.

**Fix:** In the `dir` case, upload the stemcell tarball to the remote host via
`runner.Upload`, then extract it there via `runner.Execute("tar", ...)`.  This mirrors
what the `qcow2` case already does (local decompress → Upload raw → remote qemu-img).

---

## Affected files

| File | Change |
|------|--------|
| `src/bosh-libvirt-cpi/vm/factory.go` | Remove `e2fsck -f -y` call; add `-f` to `resize2fs` |
| `src/bosh-libvirt-cpi/stemcell/factory.go` | `dir` case: upload tarball then extract on remote |
| `src/bosh-libvirt-cpi/stemcell/factory_test.go` | Add test: `dir` format triggers runner.Upload + runner.Execute tar |
| `src/bosh-libvirt-cpi/vm/factory_test.go` | Verify e2fsck is NOT called; resize2fs uses `-f` |

---

## Detailed design

### 1. vm/factory.go — remove e2fsck, add -f to resize2fs

Current code (~line 879):

```go
if out, _, err := f.runner.Execute("qemu-img", "resize", vmExt4, "65G"); err != nil {
    f.logger.Info(f.logTag, "qemu-img resize failed (non-fatal): %s %s", err, out)
} else {
    if out2, _, err2 := f.runner.Execute("e2fsck", "-f", "-y", vmExt4); err2 != nil {
        f.logger.Info(f.logTag, "e2fsck failed (non-fatal): %s %s", err2, out2)
    }
    if out3, _, err3 := f.runner.Execute("resize2fs", vmExt4); err3 != nil {
        f.logger.Info(f.logTag, "resize2fs failed (non-fatal): %s %s", err3, out3)
    }
}
```

Replace with:

```go
if out, _, err := f.runner.Execute("qemu-img", "resize", vmExt4, "65G"); err != nil {
    f.logger.Info(f.logTag, "qemu-img resize failed (non-fatal): %s %s", err, out)
} else {
    if out2, _, err2 := f.runner.Execute("resize2fs", "-f", vmExt4); err2 != nil {
        f.logger.Info(f.logTag, "resize2fs failed (non-fatal): %s %s", err2, out2)
    }
}
```

### 2. stemcell/factory.go — `dir` case: upload then extract on remote

Current code (~line 208):

```go
case "dir":
    if err := os.MkdirAll(dstImage, 0755); err != nil {
        return bosherr.WrapError(err, "Creating stemcell rootfs directory")
    }
    out, err := exec.Command("tar", "--no-same-devices", "-xzf", imagePath, "-C", dstImage).CombinedOutput()
    if err != nil {
        if strings.Contains(string(out), "unrecognized option") || strings.Contains(string(out), "unknown option") {
            cmd := exec.Command("tar", "-xzf", imagePath, "-C", dstImage)
            out2, err2 := cmd.CombinedOutput()
            if err2 != nil && cmd.ProcessState.ExitCode() != 2 {
                return bosherr.WrapErrorf(err2, "Extracting stemcell rootfs: %s", string(out2))
            }
        } else {
            return bosherr.WrapErrorf(err, "Extracting stemcell rootfs: %s", string(out))
        }
    }
```

Replace with:

```go
case "dir":
    // For the dir format the stemcell tarball must be extracted on the libvirt host
    // (not locally) so that create_vm's runner.Execute("cp") can find image.dir.
    // Upload the tarball to a temp path on the remote host, extract there, then remove it.
    remoteTar := dstImage + ".tgz"
    if _, _, mkErr := f.runner.Execute("mkdir", "-p", dstImage); mkErr != nil {
        return bosherr.WrapError(mkErr, "Creating stemcell rootfs directory on remote")
    }
    if uploadErr := f.runner.Upload(imagePath, remoteTar); uploadErr != nil {
        return bosherr.WrapError(uploadErr, "Uploading stemcell tarball to remote host")
    }
    defer func() { _, _, _ = f.runner.Execute("rm", "-f", remoteTar) }()
    out, _, err := f.runner.Execute("tar", "--no-same-devices", "-xzf", remoteTar, "-C", dstImage)
    if err != nil {
        if strings.Contains(out, "unrecognized option") || strings.Contains(out, "unknown option") {
            out2, exitCode, err2 := f.runner.Execute("tar", "-xzf", remoteTar, "-C", dstImage)
            if err2 != nil && exitCode != 2 {
                return bosherr.WrapErrorf(err2, "Extracting stemcell rootfs on remote: %s", out2)
            }
        } else {
            return bosherr.WrapErrorf(err, "Extracting stemcell rootfs on remote: %s", out)
        }
    }
```

Note: `f.runner.Execute` returns `(string, int, error)` — the exit code is the second
return value.  The existing local `cmd.ProcessState.ExitCode()` check becomes a direct
exit-code check on the runner's return value.

Also: `f.fs.MkdirAll(stemcellPath, 0755)` (line 117) creates the PARENT directory
(`stemcells/sc-xxx`).  This still needs to happen on the remote for the `dir` case.
Since `runner.Execute("mkdir", "-p", dstImage)` creates `stemcells/sc-xxx/image.dir`
(which also creates the parent), the `f.fs.MkdirAll(stemcellPath)` call at line 117 can
remain as-is (it's a no-op if the parent already exists locally, and the remote `mkdir`
handles it on the host).

### 3. stemcell/factory_test.go — add dir-format test

Add to the existing `Describe("stemcell.Factory")` block:

```go
Describe("ImportFromPath (dir format)", func() {
    BeforeEach(func() {
        builder.DiskImageFormatResult = "dir"
    })

    It("uploads tarball to remote and extracts via runner when format is dir", func() {
        sc, err := factory.ImportFromPath("/tmp/stemcell.tgz")
        Expect(err).ToNot(HaveOccurred())
        Expect(sc.ID().AsString()).To(Equal("sc-uuid-1"))
        // runner.Upload must have been called (tarball → remote temp path)
        Expect(runner.UploadErr).To(BeNil()) // upload was attempted
        // runner.Execute must have been called for mkdir and tar
        Expect(runner.ExecuteOutput).To(Equal(""))
    })

    It("returns error when tarball upload to remote fails", func() {
        runner.UploadErr = errors.New("upload failed")
        _, err := factory.ImportFromPath("/tmp/stemcell.tgz")
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("Uploading stemcell tarball to remote host"))
    })
})
```

Note: `FakeRunner.Upload` returns `runner.UploadErr`.  To assert the upload was
*attempted*, check that `UploadErr` is nil and no error occurred.  For the error case,
set `runner.UploadErr` before calling `ImportFromPath`.

### 4. vm/factory_test.go — verify e2fsck not called, resize2fs uses -f

In the existing `Describe("Create (ext4 branch)")` block, add:

```go
It("calls resize2fs with -f and does not call e2fsck", func() {
    var executedCmds []string
    runner.ExecuteFunc = func(name string, args ...string) (string, int, error) {
        executedCmds = append(executedCmds, name+" "+strings.Join(args, " "))
        return "", 0, nil
    }
    defer func() { runner.ExecuteFunc = nil }()

    stemcellImg := filepath.Join(tmpDir, "stemcell.img")
    _ = os.WriteFile(stemcellImg, []byte("fake"), 0644)
    stemcell.ImagePathResult = stemcellImg

    _, err := factory.Create(
        apiv1.NewAgentID("agent-1"),
        stemcell,
        cloudProps,
        apiv1.Networks{},
        apiv1.NewVMEnv(nil),
    )
    Expect(err).ToNot(HaveOccurred())

    e2fsckCalled := false
    resize2fsFArgs := false
    for _, cmd := range executedCmds {
        if strings.HasPrefix(cmd, "e2fsck") {
            e2fsckCalled = true
        }
        if strings.HasPrefix(cmd, "resize2fs -f") {
            resize2fsFArgs = true
        }
    }
    Expect(e2fsckCalled).To(BeFalse(), "e2fsck must not be called")
    Expect(resize2fsFArgs).To(BeTrue(), "resize2fs must be called with -f")
})
```

---

## Testing

```bash
cd src/bosh-libvirt-cpi
go test ./stemcell/... ./vm/...
```

All existing tests must continue to pass.  The two new tests above must pass.
