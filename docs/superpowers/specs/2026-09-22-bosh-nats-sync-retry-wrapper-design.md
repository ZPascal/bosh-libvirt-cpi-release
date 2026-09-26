# bosh_nats_sync Retry Wrapper

**Date:** 2026-09-22  
**Status:** Approved

## Problem

Both LXC and QEMU CI runs fail with `Authorization Violation` when `bosh deploy` tries to connect an agent to NATS. The root cause is that `bosh_nats_sync` crashes immediately on every startup attempt, so `auth.json` is never populated with per-agent NATS credentials.

### Crash sequence

1. Our monit stub starts nats-server, then starts `bosh_nats_sync` as root.
2. `bosh_nats_sync` (`bosh-nats-sync` Ruby binary) initializes its logger, then immediately calls `UsersSync.reload_nats_server_config` — which runs `nats-server --signal reload=<pidfile>` to send SIGHUP to the running nats-server.
3. If nats-server is not yet ready to receive that signal (common at startup), the command exits non-zero.
4. `reload_nats_server_config` raises an unhandled exception → process exits before the Rufus scheduler ever starts.
5. The scheduler never fires, so `auth.json` never gets real NATS user entries.
6. Agents get `Authorization Violation` when connecting.

The crash happens every startup, not just occasionally — nats-server is always racing with the signal call because both processes are launched within milliseconds of each other by our monit stub.

### Why previous fixes didn't reach this

Earlier patches fixed prerequisite issues (log dir creation, root privilege for the process) that caused earlier crashes. Once those were resolved, this startup-signal crash became the failure point.

## Design

### Wrapper approach

At `create_vm` time, intercept the `bosh_nats_sync` executable in the director VM's rootfs before the container starts:

1. Rename `/var/vcap/jobs/nats/bin/bosh_nats_sync` → `bosh_nats_sync.real`
2. Write a shell wrapper at `/var/vcap/jobs/nats/bin/bosh_nats_sync`

The wrapper does two things:

**Phase 1 — wait for nats-server:** Poll TCP port 4222 on 127.0.0.1 for up to 60 seconds. Only proceed once nats-server is accepting connections. This ensures the initial `reload_nats_server_config` signal call succeeds.

**Phase 2 — retry loop:** Run the real binary in an infinite loop. If it exits within 10 seconds of starting (crash-on-startup), wait 5 seconds then retry. If it ran longer (at least one successful sync cycle), restart immediately without delay.

The loop is intentionally infinite — bosh-agent stops the service by killing the monit-registered process (the wrapper), which also kills the real binary child via `start_new_session=False` (same session). The wrapper exits when killed.

```sh
#!/bin/sh
# Wait for nats-server port 4222 before first start so --signal reload succeeds
for i in $(seq 1 60); do
  nc -z 127.0.0.1 4222 2>/dev/null && break
  sleep 1
done
# Retry loop: bosh_nats_sync crashes if nats-server signal fails; restart it
while true; do
  _start=$(date +%s)
  /var/vcap/jobs/nats/bin/bosh_nats_sync.real "$@"
  _rc=$?
  _elapsed=$(( $(date +%s) - _start ))
  echo "$(date): bosh_nats_sync exited rc=$_rc after ${_elapsed}s, restarting..." \
    >> /var/vcap/bosh/log/monit-nats.log
  [ "$_elapsed" -lt 10 ] && sleep 5
done
```

### Install pattern

Follows the same pattern as the existing `cpi.real` wrapper installed in `create_vm`:

- Check if `bosh_nats_sync` exists and `bosh_nats_sync.real` does not (idempotent)
- Rename original → `.real`
- Write wrapper script with mode 0755

This runs unconditionally for any VM that has a nats job installed in its rootfs, which is only the director VM.

### Scope

- **LXC path:** `create_vm` for LXC director VMs (already has cpi.real wrapper install logic nearby)
- **ext4/QEMU path:** `create_vm` for ext4 director VMs (same location in the ext4 branch)
- No changes to manifests, tests.yml diagnostics, or the bosh_nats_sync binary itself

## Files Changed

| File | Change |
|------|--------|
| `src/bosh-libvirt-cpi/vm/factory.go` | Add `installNatsSyncWrapper(vmRootfs string)` helper; call it from both the LXC and ext4 `create_vm` code paths after the rootfs is set up |

## Success Criteria

- `bosh_nats_sync` eventually starts successfully (wrapper retries until nats-server is ready)
- `auth.json` contains per-agent NATS user entries when `bosh deploy` runs
- No `Authorization Violation` error
- The wrapper is idempotent: re-running `create_vm` on the same rootfs doesn't break it
