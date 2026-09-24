# Stemcell Remote-Host Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two CI failures — QEMU director kernel panic from e2fsck corrupting the ext4 resize, and LXC `create_vm` failing because `image.dir` was created inside the container instead of on the libvirt host.

**Architecture:** Two independent surgical edits. (1) Remove `e2fsck` from `vm/factory.go`'s ext4 resize sequence and add `-f` to `resize2fs`. (2) Change `stemcell/factory.go`'s `dir` case to upload the tarball to the remote libvirt host via `runner.Upload` then extract it there via `runner.Execute("tar")`, instead of extracting locally.

**Tech Stack:** Go 1.21, Ginkgo/Gomega test framework, bosh-utils, bosh-cpi-go SDK.

---

### Task 1: Fix ext4 resize — remove e2fsck, add -f to resize2fs

**Files:**
- Modify: `src/bosh-libvirt-cpi/vm/factory.go:879-888`
- Test: `src/bosh-libvirt-cpi/vm/factory_test.go`

**Background:** After `qemu-img resize rootfs.img 65G`, the ext4 superblock says ~2 GB but the file is 65 GB. Running `e2fsck -f -y` on this image treats the size mismatch as corruption and destructively "fixes" it, leaving the filesystem empty. `resize2fs -f` handles the grow directly without needing e2fsck; the image is a fresh `cp` of the stemcell and is known-clean.

- [ ] **Step 1: Write the failing test**

Open `src/bosh-libvirt-cpi/vm/factory_test.go`. In the `Describe("Create (ext4 branch)")` block (after the existing `It("returns error when mount fails", ...)` and the happy-path test), add this new `It` block:

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
    resize2fsFCalled := false
    for _, cmd := range executedCmds {
        if strings.HasPrefix(cmd, "e2fsck") {
            e2fsckCalled = true
        }
        if cmd == "resize2fs -f "+filepath.Join(tmpDir, "vms/vm-uuid-vm-1/rootfs.img") {
            resize2fsFCalled = true
        }
    }
    Expect(e2fsckCalled).To(BeFalse(), "e2fsck must not be called")
    Expect(resize2fsFCalled).To(BeTrue(), "resize2fs must be called with -f <path>")
})
```

The `strings` package is already imported in `factory_test.go`. `filepath`, `os`, `apiv1`, `errors` are also already imported.

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd src/bosh-libvirt-cpi
go test ./vm/... -run "calls resize2fs" -v 2>&1 | tail -20
```

Expected: FAIL — the test will find `e2fsck` called and `resize2fs -f` not called.

- [ ] **Step 3: Apply the fix in vm/factory.go**

Find this block in `src/bosh-libvirt-cpi/vm/factory.go` (around line 879):

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

Replace it with:

```go
		if out, _, err := f.runner.Execute("qemu-img", "resize", vmExt4, "65G"); err != nil {
			f.logger.Info(f.logTag, "qemu-img resize failed (non-fatal): %s %s", err, out)
		} else {
			if out2, _, err2 := f.runner.Execute("resize2fs", "-f", vmExt4); err2 != nil {
				f.logger.Info(f.logTag, "resize2fs failed (non-fatal): %s %s", err2, out2)
			}
		}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd src/bosh-libvirt-cpi
go test ./vm/... -v 2>&1 | grep -E "PASS|FAIL|resize2fs|e2fsck"
```

Expected: all `vm` tests pass, including the new one.

- [ ] **Step 5: Run full test suite**

```bash
cd src/bosh-libvirt-cpi
go test ./... 2>&1 | tail -20
```

Expected: no failures (only the known linker warning about duplicate libs).

- [ ] **Step 6: Commit**

```bash
cd /Users/I539231/Projects/Workspaces/Upstream/bosh-libvirt-cpi-release
git add src/bosh-libvirt-cpi/vm/factory.go src/bosh-libvirt-cpi/vm/factory_test.go
git commit -m "fix(cpi): remove e2fsck from ext4 resize; use resize2fs -f to prevent filesystem corruption"
```

---

### Task 2: Fix LXC dir stemcell — extract on remote host via runner

**Files:**
- Modify: `src/bosh-libvirt-cpi/stemcell/factory.go:208-230`
- Test: `src/bosh-libvirt-cpi/stemcell/factory_test.go`

**Background:** The deployed LXC director's CPI runs inside an LXC container. The `dir` case in `stemcell/factory.go` uses `os.MkdirAll` and `exec.Command("tar")` — both run locally (inside the container). When `create_vm` later calls `f.runner.Execute("cp", stemcell.ImagePath()+"/.", vmRootfs)` via SSH, the copy runs on the libvirt HOST where `image.dir` was never created. Fix: upload the tarball to a temp path on the remote host via `runner.Upload`, then extract via `runner.Execute("tar")`.

- [ ] **Step 1: Write the failing test**

Open `src/bosh-libvirt-cpi/stemcell/factory_test.go`. After the closing `})` of the `Describe("Find", ...)` block (around line 101), add:

```go
	Describe("ImportFromPath (dir format)", func() {
		BeforeEach(func() {
			builder.DiskImageFormatResult = "dir"
		})

		It("uploads tarball to remote host and extracts via runner", func() {
			var uploadSrc, uploadDst string
			runner.UploadFunc = func(src, dst string) error {
				uploadSrc = src
				uploadDst = dst
				return nil
			}
			defer func() { runner.UploadFunc = nil }()

			sc, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).ToNot(HaveOccurred())
			Expect(sc.ID().AsString()).To(Equal("sc-uuid-1"))
			Expect(uploadSrc).To(Equal("/tmp/stemcell.tgz"))
			Expect(uploadDst).To(HaveSuffix(".tgz"))
		})

		It("returns error when tarball upload to remote fails", func() {
			runner.UploadFunc = func(src, dst string) error {
				return errors.New("upload failed")
			}
			defer func() { runner.UploadFunc = nil }()

			_, err := factory.ImportFromPath("/tmp/stemcell.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Uploading stemcell tarball to remote host"))
		})
	})
```

`FakeRunner` currently has a static `UploadErr` field. We need an `UploadFunc` field too (same pattern as `ExecuteFunc`). Add it in the next step.

- [ ] **Step 2: Add UploadFunc to FakeRunner**

Open `src/bosh-libvirt-cpi/driver/fakes/fake_runner.go`. Add the `UploadFunc` field and update `Upload`:

```go
type FakeRunner struct {
	ExecuteOutput string
	ExecuteStatus int
	ExecuteErr    error

	ExecuteFunc func(path string, args ...string) (string, int, error)

	UploadErr  error
	UploadFunc func(srcDir, dstDir string) error

	PutContents map[string][]byte
	PutErr      error

	GetResult []byte
	GetErr    error
}

func (r *FakeRunner) Upload(srcDir, dstDir string) error {
	if r.UploadFunc != nil {
		return r.UploadFunc(srcDir, dstDir)
	}
	return r.UploadErr
}
```

- [ ] **Step 3: Run the new test to verify it fails**

```bash
cd src/bosh-libvirt-cpi
go test ./stemcell/... -run "dir format" -v 2>&1 | tail -20
```

Expected: FAIL — upload is not called and `uploadSrc` stays empty.

- [ ] **Step 4: Apply the fix in stemcell/factory.go**

Find the `case "dir":` block in `src/bosh-libvirt-cpi/stemcell/factory.go` (around line 208). Replace the entire `case "dir":` block with:

```go
	case "dir":
		// The stemcell tarball must be extracted on the libvirt host so that
		// create_vm's runner.Execute("cp") can find image.dir there.
		// Upload the tarball to a temp path on the remote, extract, then delete it.
		remoteTar := dstImage + ".tgz"
		if _, _, mkErr := f.runner.Execute("mkdir", "-p", dstImage); mkErr != nil {
			return bosherr.WrapError(mkErr, "Creating stemcell rootfs directory on remote")
		}
		if uploadErr := f.runner.Upload(imagePath, remoteTar); uploadErr != nil {
			return bosherr.WrapError(uploadErr, "Uploading stemcell tarball to remote host")
		}
		defer func() { _, _, _ = f.runner.Execute("rm", "-f", remoteTar) }()
		out, exitCode, err := f.runner.Execute("tar", "--no-same-devices", "-xzf", remoteTar, "-C", dstImage)
		if err != nil {
			if strings.Contains(out, "unrecognized option") || strings.Contains(out, "unknown option") {
				// tar doesn't support --no-same-devices (BusyBox/BSD tar on older hosts).
				// Exit code 2 means warnings only (e.g. mknod EPERM) — extraction succeeded.
				out2, exitCode2, err2 := f.runner.Execute("tar", "-xzf", remoteTar, "-C", dstImage)
				if err2 != nil && exitCode2 != 2 {
					return bosherr.WrapErrorf(err2, "Extracting stemcell rootfs on remote: %s", out2)
				}
			} else if exitCode != 2 {
				return bosherr.WrapErrorf(err, "Extracting stemcell rootfs on remote: %s", out)
			}
		}
```

Note: the old code used `exec.Command` and `cmd.ProcessState.ExitCode()`. The runner returns the exit code directly as the second return value `(string, int, error)`. The new code uses that directly.

Also remove the now-unused `"os/exec"` import if `exec.Command` is no longer used anywhere in the `dir` case. Check whether `exec` is still used by other cases in the same file:

```bash
grep -n "exec\." src/bosh-libvirt-cpi/stemcell/factory.go
```

The `ext4` case still uses `exec.Command` for local operations (dd, mkfs.ext4, mount, cp, umount). Keep the import.

- [ ] **Step 5: Run the new tests to verify they pass**

```bash
cd src/bosh-libvirt-cpi
go test ./stemcell/... -run "dir format" -v 2>&1 | tail -20
```

Expected: both new tests pass.

- [ ] **Step 6: Run full test suite**

```bash
cd src/bosh-libvirt-cpi
go test ./... 2>&1 | tail -20
```

Expected: no failures.

- [ ] **Step 7: Commit**

```bash
cd /Users/I539231/Projects/Workspaces/Upstream/bosh-libvirt-cpi-release
git add src/bosh-libvirt-cpi/stemcell/factory.go \
        src/bosh-libvirt-cpi/stemcell/factory_test.go \
        src/bosh-libvirt-cpi/driver/fakes/fake_runner.go
git commit -m "fix(cpi): extract dir stemcell on remote libvirt host to fix LXC create_vm cp failure"
```

---

### Task 3: Push and verify CI

- [ ] **Step 1: Push the branch**

```bash
cd /Users/I539231/Projects/Workspaces/Upstream/bosh-libvirt-cpi-release
git push
```

- [ ] **Step 2: Confirm CI passes**

Watch the GitHub Actions run at:
`https://github.com/ZPascal/bosh-libvirt-cpi-release/actions`

Both the QEMU E2E and LXC E2E jobs must pass:
- QEMU: `Deploy smoke test deployment (QEMU)` completes without kernel panic
- LXC: `Deploy smoke test deployment (LXC)` completes without `cp: cannot stat '.../image.dir/.'`
