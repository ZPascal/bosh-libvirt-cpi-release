# E2E CI Backends

The end-to-end CI (`.github/workflows/tests.yml`) exercises the CPI by bootstrapping a
real bosh-lite BOSH Director and running a smoke deployment. The E2E flow is implemented
as a reusable workflow (`.github/workflows/e2e.yml`) invoked once per backend via a matrix.

Both backends below are **supported and run on every push/PR to `main`**.

| Backend | libvirt URI | Director IP | Manifest set | Status |
|---------|-------------|-------------|--------------|--------|
| QEMU/KVM | `qemu:///system` | `192.168.122.10` | `manifests/qemu-cpi.yml` + `manifests/ops/qemu-*.yml` | Supported |
| LXC | `lxc:///` | `192.168.122.11` | `manifests/lxc-cpi.yml` + `manifests/ops/lxc-*.yml` | Supported |

## QEMU/KVM

Full-VM bosh-lite via `qemu:///system`. The director and compilation/deployment VMs are
QEMU guests booted by direct kernel boot (`manifests/ops/qemu-kernel-boot.yml`) using the
host kernel. The host reserves `192.168.122.10` for the director via a libvirt DHCP host
entry. Because guests boot from ext4 images, the CPI clears the `needs_recovery`
superblock bit before start; see the deploy diagnostics in
`.github/scripts/e2e-debug-qemu.sh`.

## LXC

Container-based bosh-lite via `lxc:///`. Requires host tuning applied in the workflow:
cgroup v2 delegation, unprivileged userns, `kernel.shmmax`/`shmall` for postgres, and
disabling libvirtd idle shutdown (`shutdown_timeout = 0`) so veth interfaces are not torn
down when CPI clients disconnect. The director runs at `192.168.122.11`.

## Running a single backend

The reusable workflow accepts a `backend` input (`qemu` or `lxc`). To call it from another
workflow:

```yaml
jobs:
  e2e-qemu-only:
    uses: ./.github/workflows/e2e.yml
    with:
      backend: qemu
```

Failure diagnostics for each backend live in `.github/scripts/e2e-debug-<backend>.sh`.
