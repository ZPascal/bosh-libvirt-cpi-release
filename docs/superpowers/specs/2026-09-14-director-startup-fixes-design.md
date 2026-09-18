# Director Startup Fixes (LXC blobstore-config + QEMU diagnostics) Design

## Goal

Fix the LXC director crash at startup (`Permission denied @ blobstore-config`) and add structured console logging to the QEMU monit stub so the next QEMU failure reveals the root cause.

## Background

Two CI failures on `feat/lxc-integration-tests`:

**LXC**: Director process starts (Puma loads, DB connects, UUID found) then immediately crashes:
```
Permission denied @ rb_sysopen - /var/vcap/data/director/tmp/blobstore-config
```
The davcli blobstore client tries to create `blobstore-config` inside `/var/vcap/data/director/tmp/` on first use. That directory is created lazily by the Ruby gem at Puma startup — after the monit stub's chown walk has already run. The directory is therefore owned by root, not vcap (uid 1000), so the vcap-process open fails.

**QEMU**: NATS never starts; bosh-agent is at attempt #21852 in the unlimited NATS retry loop. The CI console log shows only `[DelayedAuditLogger] ERROR - Unix syslog delivery error` — none of the `[monit]` lines that the `console_log()` function should emit. The rootfs can't be mounted (VM still running), so `monit-nats.log` is inaccessible. The QEMU nats root cause is unknown.

## Architecture

### Fix 1 — LXC/QEMU: pre-create runtime dirs before starting director

In `vm/factory.go`, the `start_svc` async thread (for `director`, `worker_1`, `worker_2`, `worker_3`, `director_scheduler`) runs pre-start then chowns `/var/vcap/data/<svc>` recursively. Add an explicit pre-create step immediately after the chown walk that creates a fixed list of known runtime-write dirs and chowns each to vcap:

```
/var/vcap/data/<svc>/tmp
/var/vcap/data/<svc>/run
/var/vcap/sys/log/<svc>
/var/vcap/sys/run/<svc>
```

This closes the race: the dirs exist and are vcap-owned before `subprocess.Popen` is called, so no lazy-creation-as-root can happen.

Apply this in **both** the LXC path and the QEMU path — the async director thread is duplicated between the two paths. Use the code anchor `chown_root in ['/var/vcap/data/'+svc` to locate both blocks (grep for it in `factory.go`; the first occurrence is in the LXC async thread, the second in the QEMU async thread).

### Fix 2 — QEMU: structured console_log at key events

The existing `console_log()` function in the QEMU `bosh-init` writes to `/dev/console`. Make two changes:

1. **Robustness**: Wrap the `/dev/console` write in `try/except`; on failure, append to `/tmp/monit-console.log` as a fallback. This ensures the message is never silently lost.

2. **Coverage**: Add `console_log()` calls at these events:
   - `start_svc` entry: `[monit] start_svc: <svc>`
   - pre-start result: `[monit] pre-start <svc> rc=<rc>`
   - process launched: `[monit] started <svc> pid=<pid>` (already present in some paths — verify and add where missing)
   - bpm.yml parse or launch failure: `[monit] start failed <svc>: <err>`
   - postgres ready: `[monit] postgres ready, starting <svc>` (already present — verify)
   - postgres never ready: `[monit] postgres never ready for <svc>` (already present — verify)

The LXC path does not have `console_log()` (it uses container log files directly). No change needed there.

### Fix 3 — CI workflow: widen QEMU console log grep filter

In `.github/workflows/tests.yml`, the QEMU diagnostic step greps the console log with:
```
grep -i "\[monit\]\|postgres\|director\|nats\|Error\|5432\|25555"
```

Extend to also match: `start_svc`, `pre-start`, `bpm.yml`, `started.*pid`, `pg_isready`, `never ready`.

New pattern:
```
grep -i "\[monit\]\|postgres\|director\|nats\|Error\|5432\|25555\|start_svc\|pre-start\|bpm\.yml\|started.*pid\|pg_isready\|never ready"
```

## Files Changed

- `src/bosh-libvirt-cpi/vm/factory.go` — LXC async path: add pre-create dirs block; QEMU async path: add pre-create dirs block + `console_log()` hardening + new `console_log()` call sites
- `.github/workflows/tests.yml` — widen QEMU console log grep filter

## Non-goals

- Fixing the actual QEMU nats root cause (deferred until diagnostics surface it)
- Changing LXC curl wait logic (already fixed in `ffd25fb`)
- Refactoring the monit stub out of `factory.go`
- Any QEMU network/DHCP changes

## Success Criteria

- **LXC**: director comes up on port 25555; no `Permission denied @ blobstore-config` in logs
- **QEMU (pass case)**: director comes up on port 25555
- **QEMU (still-failing case)**: CI console log now shows `[monit] start_svc: nats`, `[monit] started nats pid=X` or `[monit] start failed nats: <error>` — giving us the root cause to fix next
