# Director Startup Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the LXC director crash (`Permission denied @ blobstore-config`) by pre-creating `/var/vcap/data/<svc>/tmp` with vcap ownership, and add structured `console_log()` coverage plus a CI grep-filter extension to surface the QEMU nats root cause.

**Architecture:** Two targeted edits to `vm/factory.go` (one per monit-stub path) and one line change to `tests.yml`. The LXC fix inserts a pre-create block after the chown walk in the async director thread. The QEMU diagnostic fix hardens `console_log()` and adds call sites in the synchronous `start_svc` path, then widens the CI grep filter.

**Tech Stack:** Go, Python3 (embedded in init scripts), GitHub Actions YAML

---

### Context for all tasks

- Implementation file: `src/bosh-libvirt-cpi/vm/factory.go`
- CI workflow: `.github/workflows/tests.yml`
- Run tests: `cd src/bosh-libvirt-cpi && go test -mod=vendor ./vm/... -v 2>&1 | tail -20`
- Run build: `cd src/bosh-libvirt-cpi && go build -mod=vendor ./...`

There are **two separate monit-stub code blocks** in `factory.go` — one for LXC (the "dir" format, starts around line 320) and one for QEMU ext4 (the "ext4" format, starts around line 791). They share the same Python logic but are completely independent Go string literals. The LXC path does NOT have a `console_log()` function (it logs to files inside the container); the QEMU path does.

Use grep to find exact line numbers before editing:
```bash
grep -n "chown_root in \['/var/vcap/data/'+svc" src/bosh-libvirt-cpi/vm/factory.go
```
This returns two matches. The first is in the LXC async director thread; the second is in the QEMU async director thread.

---

### Task 1: LXC path — pre-create `data/<svc>/tmp` and `sys/log/<svc>` with vcap ownership

**Files:**
- Modify: `src/bosh-libvirt-cpi/vm/factory.go` — LXC async director thread

**Background:** The LXC director crash is:
```
Permission denied @ rb_sysopen - /var/vcap/data/director/tmp/blobstore-config
```
The Ruby davcli blobstore client creates `/var/vcap/data/director/tmp/` lazily at Puma startup — after the chown walk. Because the process runs as vcap (uid 1000), it can't chown the dir itself. Fix: pre-create the dirs with vcap ownership before Popen.

- [ ] **Step 1: Locate the LXC chown walk in the async director thread**

```bash
grep -n "chown_root in \['/var/vcap/data/'+svc" src/bosh-libvirt-cpi/vm/factory.go
```

Note the **first** line number returned — call it `LINE_A`. The block to insert after ends at the line containing `"            except: pass\n"` that closes the inner `for f in filenames` loop, right before:
```go
"      bpmyml = '/var/vcap/jobs/' + svc + '/config/bpm.yml'\n" +
```

- [ ] **Step 2: Insert the pre-create block after the LXC chown walk**

Find this exact string in `factory.go` (it appears in the **LXC** async thread — use the first occurrence):

```go
			"            for f in filenames:\n" +
			"              try: os.chown(os.path.join(dirpath, f), 1000, 1000)\n" +
			"              except: pass\n" +
			"      bpmyml = '/var/vcap/jobs/' + svc + '/config/bpm.yml'\n" +
			"      if not os.path.exists(bpmyml): return\n" +
```

Replace it with:

```go
			"            for f in filenames:\n" +
			"              try: os.chown(os.path.join(dirpath, f), 1000, 1000)\n" +
			"              except: pass\n" +
			"      # Pre-create runtime-write dirs that services create lazily (e.g. blobstore-config).\n" +
			"      for _rtdir in ['/var/vcap/data/'+svc+'/tmp', '/var/vcap/data/'+svc+'/run', '/var/vcap/sys/log/'+svc, '/var/vcap/sys/run/'+svc]:\n" +
			"        os.makedirs(_rtdir, exist_ok=True)\n" +
			"        try: os.chown(_rtdir, 1000, 1000)\n" +
			"        except: pass\n" +
			"      bpmyml = '/var/vcap/jobs/' + svc + '/config/bpm.yml'\n" +
			"      if not os.path.exists(bpmyml): return\n" +
```

**Important:** There are TWO occurrences of `"      bpmyml = '/var/vcap/jobs/' + svc + '/config/bpm.yml'\n" +` in the file — one in the LXC async thread and one in the QEMU async thread. You are editing the **first** one (LXC). The surrounding context (`"      if not os.path.exists(bpmyml): return\n"` follows immediately) confirms it's the right one.

- [ ] **Step 3: Build**

```bash
cd src/bosh-libvirt-cpi && go build -mod=vendor ./...
```
Expected: clean (no output).

- [ ] **Step 4: Run tests**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./vm/... -v 2>&1 | tail -20
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add src/bosh-libvirt-cpi/vm/factory.go
git commit -m "fix(lxc): pre-create data/<svc>/tmp and sys dirs with vcap ownership before starting director"
```

---

### Task 2: QEMU path — same pre-create fix in the QEMU async director thread

**Files:**
- Modify: `src/bosh-libvirt-cpi/vm/factory.go` — QEMU async director thread

**Background:** The QEMU path has the identical async director thread with the identical chown walk. Apply the same pre-create fix so both paths are consistent and the QEMU director won't hit the same blobstore-config error if/when it reaches that point.

- [ ] **Step 1: Locate the QEMU chown walk anchor**

```bash
grep -n "chown_root in \['/var/vcap/data/'+svc" src/bosh-libvirt-cpi/vm/factory.go
```

Note the **second** line number returned — this is the QEMU async director thread.

- [ ] **Step 2: Insert the pre-create block after the QEMU async chown walk**

Find this exact string in `factory.go` (second occurrence — use context to confirm it's the QEMU path, which has `console_log('postgres ready, starting '+svc)` a few lines above):

```go
			"            for f in filenames:\n" +
			"              try: os.chown(os.path.join(dirpath, f), 1000, 1000)\n" +
			"              except: pass\n" +
			"      bpmyml = '/var/vcap/jobs/' + svc + '/config/bpm.yml'\n" +
			"      if not os.path.exists(bpmyml): return\n" +
			"      try:\n" +
			"        cfg = load_bpm(bpmyml)\n" +
			"        procs = cfg.get('processes',[])\n" +
```

Replace it with:

```go
			"            for f in filenames:\n" +
			"              try: os.chown(os.path.join(dirpath, f), 1000, 1000)\n" +
			"              except: pass\n" +
			"      # Pre-create runtime-write dirs that services create lazily (e.g. blobstore-config).\n" +
			"      for _rtdir in ['/var/vcap/data/'+svc+'/tmp', '/var/vcap/data/'+svc+'/run', '/var/vcap/sys/log/'+svc, '/var/vcap/sys/run/'+svc]:\n" +
			"        os.makedirs(_rtdir, exist_ok=True)\n" +
			"        try: os.chown(_rtdir, 1000, 1000)\n" +
			"        except: pass\n" +
			"      bpmyml = '/var/vcap/jobs/' + svc + '/config/bpm.yml'\n" +
			"      if not os.path.exists(bpmyml): return\n" +
			"      try:\n" +
			"        cfg = load_bpm(bpmyml)\n" +
			"        procs = cfg.get('processes',[])\n" +
```

- [ ] **Step 3: Build and test**

```bash
cd src/bosh-libvirt-cpi && go build -mod=vendor ./... && go test -mod=vendor ./vm/... -v 2>&1 | tail -20
```
Expected: clean build, all PASS.

- [ ] **Step 4: Commit**

```bash
git add src/bosh-libvirt-cpi/vm/factory.go
git commit -m "fix(ext4): pre-create data/<svc>/tmp and sys dirs with vcap ownership before starting director"
```

---

### Task 3: QEMU path — harden `console_log()` and add missing call sites in synchronous `start_svc`

**Files:**
- Modify: `src/bosh-libvirt-cpi/vm/factory.go` — QEMU monit stub `console_log` definition and synchronous `start_svc` path

**Background:** `console_log()` currently silently swallows errors (`except: pass`). If `/dev/console` is not writable in the QEMU guest, all `[monit]` messages are lost. Also, the synchronous `start_svc` path (for nats, postgres, blobstore, health_monitor) has no `console_log()` calls at all — so we never see whether `start_svc('nats')` was called or whether it failed.

- [ ] **Step 1: Harden `console_log()` with a fallback file**

Find this exact string in `factory.go` (it only appears once, in the QEMU path):

```go
			"def console_log(msg):\n" +
			"  try: open('/dev/console','a').write('[monit] '+msg+'\\n')\n" +
			"  except: pass\n" +
```

Replace it with:

```go
			"def console_log(msg):\n" +
			"  line = '[monit] '+msg+'\\n'\n" +
			"  try: open('/dev/console','a').write(line)\n" +
			"  except:\n" +
			"    try: open('/tmp/monit-console.log','a').write(line)\n" +
			"    except: pass\n" +
```

- [ ] **Step 2: Add `console_log()` calls in the synchronous `start_svc` path**

The synchronous path (for nats, postgres, blobstore, etc.) starts after the `threading.Thread(...).start(); return` that ends the async director block. Find this exact string in `factory.go` (it appears once in the QEMU path, just after `"    return\n"`):

```go
			"  log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')\n" +
			"  # Run pre-start script as root so it can create required directories\n" +
			"  pre_start = '/var/vcap/jobs/' + svc + '/bin/pre-start'\n" +
			"  if os.path.exists(pre_start):\n" +
			"    try:\n" +
			"      r = subprocess.run([pre_start], capture_output=True, timeout=120)\n" +
			"      log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\\n'); log.flush()\n" +
			"    except Exception as e: log.write('pre-start failed: '+str(e)+'\\n'); log.flush()\n" +
```

Replace it with:

```go
			"  log = open('/var/vcap/bosh/log/monit-'+svc+'.log','a')\n" +
			"  console_log('start_svc sync: '+svc)\n" +
			"  # Run pre-start script as root so it can create required directories\n" +
			"  pre_start = '/var/vcap/jobs/' + svc + '/bin/pre-start'\n" +
			"  if os.path.exists(pre_start):\n" +
			"    try:\n" +
			"      r = subprocess.run([pre_start], capture_output=True, timeout=120)\n" +
			"      log.write('pre-start rc='+str(r.returncode)+' '+r.stdout.decode()[:200]+r.stderr.decode()[:200]+'\\n'); log.flush()\n" +
			"      console_log('pre-start '+svc+' rc='+str(r.returncode))\n" +
			"    except Exception as e: log.write('pre-start failed: '+str(e)+'\\n'); log.flush()\n" +
```

- [ ] **Step 3: Add `console_log()` on bpm.yml launch failure in synchronous path**

Find this exact string in the QEMU synchronous path (appears once — just before the `ctl` fallback):

```go
			"    except Exception as e: log.write('bpm.yml start failed: '+str(e)+'\\n')\n" +
			"  ctl = '/var/vcap/jobs/' + svc + '/bin/ctl'\n" +
```

Replace it with:

```go
			"    except Exception as e: log.write('bpm.yml start failed: '+str(e)+'\\n'); console_log('start failed '+svc+': '+str(e))\n" +
			"  ctl = '/var/vcap/jobs/' + svc + '/bin/ctl'\n" +
```

- [ ] **Step 4: Build and test**

```bash
cd src/bosh-libvirt-cpi && go build -mod=vendor ./... && go test -mod=vendor ./vm/... -v 2>&1 | tail -20
```
Expected: clean build, all PASS.

- [ ] **Step 5: Commit**

```bash
git add src/bosh-libvirt-cpi/vm/factory.go
git commit -m "fix(ext4): harden console_log with fallback file; add console_log to sync start_svc path"
```

---

### Task 4: CI workflow — widen QEMU console log grep filter

**Files:**
- Modify: `.github/workflows/tests.yml` — QEMU diagnostic grep line

**Background:** The current filter only catches `[monit]`, `postgres`, `director`, `nats`, `Error`, `5432`, `25555`. After Task 3, the console log will emit `[monit] start_svc sync: nats`, `[monit] pre-start nats rc=0`, `[monit] started nats pid=X`, `[monit] start failed nats: ...`. These are already caught by `\[monit\]`. But also add `start_svc`, `pre-start`, `bpm\.yml`, `never ready` as extra anchors in case the `[monit]` prefix is stripped.

- [ ] **Step 1: Update the grep filter**

Find this exact line in `.github/workflows/tests.yml`:

```yaml
          sudo find /tmp -name 'bosh-vm-*-console.log' 2>/dev/null | xargs sudo cat 2>/dev/null | grep -i "\[monit\]\|postgres\|director\|nats\|Error\|5432\|25555" | tail -40 || true
```

Replace it with:

```yaml
          sudo find /tmp -name 'bosh-vm-*-console.log' 2>/dev/null | xargs sudo cat 2>/dev/null | grep -i "\[monit\]\|postgres\|director\|nats\|Error\|5432\|25555\|start_svc\|pre-start\|bpm\.yml\|never ready\|started.*pid" | tail -40 || true
```

- [ ] **Step 2: Also add a fallback log dump in QEMU diagnostics**

Find this exact block in `.github/workflows/tests.yml` (the block that dumps monit logs from a mounted rootfs):

```yaml
              echo "=== monit-postgres.log ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/monit-postgres.log 2>/dev/null | tail -30 || echo "(not found)"
```

Insert these two lines immediately before it (to also dump the fallback console log and monit-req.log):

```yaml
              echo "=== monit-console-fallback.log ===" && sudo cat /mnt/qemu-rootfs/tmp/monit-console.log 2>/dev/null | tail -30 || echo "(not found)"
              echo "=== monit-req.log ===" && sudo cat /mnt/qemu-rootfs/var/vcap/bosh/log/monit-req.log 2>/dev/null | tail -30 || echo "(not found)"
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/tests.yml
git commit -m "ci: widen QEMU console log grep filter; add fallback console log and monit-req.log dump"
```

---

## Self-Review

**Spec coverage:**
- ✓ Fix 1 (LXC blobstore-config): Task 1
- ✓ Fix 1 applied to QEMU async path too: Task 2
- ✓ Fix 2 `console_log()` hardening with fallback: Task 3 Step 1
- ✓ Fix 2 `console_log()` coverage in sync path: Task 3 Steps 2-3
- ✓ Fix 3 CI grep filter widened: Task 4 Step 1
- ✓ Fix 3 fallback log + monit-req.log dump in QEMU diagnostics: Task 4 Step 2

**Placeholder scan:** No TBDs, no vague steps. All code blocks are complete. All commands have expected output. ✓

**Type consistency:** No new types or functions introduced. All edits are string insertions into existing Go string literals. ✓
