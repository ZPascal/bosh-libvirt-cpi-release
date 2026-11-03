# CI/CD Workflows

> The E2E sections below describe the repository's own test pipeline (contributors). The [Setup BOSH Director workflow](#setup-bosh-director-workflow-setup-bosh-directoryml) is meant for reuse from other repositories. To install the CPI on your own host, see [INSTALL.md](INSTALL.md).

## Overview

Tests run on every push and PR to `main`. The E2E suite spins up a real BOSH director on the CI runner, deploys a smoke test, and tears everything down. Two backends are tested in parallel: QEMU/KVM and LXC.

```
tests.yml
├── unit-tests
├── build
├── e2e (matrix: qemu, lxc)  →  e2e.yml
├── integration-tests-qemu
├── integration-tests-lxc
├── code-quality
└── security-scan
```

---

## E2E workflow (`e2e.yml`)

Everything runs in a single job on one runner. The BOSH director is a VM inside the runner's libvirt network (`192.168.122.x`), so setup, tests, and teardown all have to be on the same machine.

**What it does, in order:**

1. Install libvirt and the backend (QEMU or LXC)
2. Generate an SSH key so the director VM can reach libvirt on the host
3. Build the CPI release tarball and download the stemcell
4. Boot the BOSH director with `bosh create-env` (runs in background, polled until healthy)
5. Wait for the director API and task worker to be ready
6. Upload the stemcell and a BOSH release to the director
7. Deploy a smoke test deployment
8. Verify the deployment is reachable
9. Delete the director (`bosh delete-env`) - runs even if earlier steps failed

**Inputs:**

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `backend` | yes | - | `qemu` or `lxc` |
| `bosh_cli_version` | no | `v7.10.8` | BOSH CLI version to install |

**Calling it from another workflow:**

```yaml
jobs:
  e2e:
    uses: ./.github/workflows/e2e.yml
    with:
      backend: qemu
```

---

## Backends

### QEMU/KVM

Director and deployment VMs are full QEMU guests. The host kernel is copied to `/tmp/vmlinuz` and used for direct-kernel boot. Director IP is `192.168.122.10`.

### LXC

Director runs as an LXC container. Requires a few host tweaks the workflow applies automatically: unprivileged user namespaces, `kernel.shmmax` for postgres, and `shutdown_timeout = 0` in libvirtd so idle shutdown doesn't tear down the container's network interface. Director IP is `192.168.122.11`.

| | QEMU | LXC |
|---|---|---|
| Director IP | `192.168.122.10` | `192.168.122.11` |
| libvirt URI | `qemu:///system` | `lxc:///` |
| Main manifest | `manifests/qemu-cpi.yml` | `manifests/lxc-cpi.yml` |

---

## Setup BOSH Director composite action (`.github/actions/setup-bosh-director`)

Steps 1-5 of the E2E workflow (host setup, libvirt install, SSH key, release build, `bosh create-env`, wait for API and worker) live in a composite action. `e2e.yml` and `setup-bosh-director.yml` both use it, so they cannot drift apart. Because a composite action runs as steps inside the caller's job, the Director stays reachable by later steps on the same runner.

It expects the CPI repository to be checked out at the workspace root and exports `DIRECTOR_IP`, `BOSH_ENV_ALIAS`, `DIRECTOR_NAME`, `KERNEL_PATH` (QEMU) and `LIBVIRT_HOST_KEY` to later steps via `GITHUB_ENV`. Use it directly from another repository to run your own steps against the Director in the same job:

```yaml
steps:
  - uses: actions/checkout@v7
    with:
      repository: ZPascal/bosh-libvirt-cpi-release
      ref: main
  - uses: ./.github/actions/setup-bosh-director
    with:
      backend: qemu
  - run: bosh -e "$BOSH_ENV_ALIAS" deployments --client admin --client-secret admin-password
```

---

## Setup BOSH Director workflow (`setup-bosh-director.yml`)

A thin reusable-workflow wrapper around the composite action above. It only handles director bootstrap and emits credentials as job outputs. Intended for callers that run their own persistent runner and want to manage the director lifecycle separately from their test steps.

It can be called from any repository: it checks out the CPI repository itself rather than the caller's.

```yaml
uses: ZPascal/bosh-libvirt-cpi-release/.github/workflows/setup-bosh-director.yml@main
```

**Inputs:**

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `backend` | yes | - | `qemu` or `lxc` |
| `bosh_cli_version` | no | `v7.10.8` | BOSH CLI version to install |
| `cpi_repository` | no | `ZPascal/bosh-libvirt-cpi-release` | Repo providing the release, manifests, scripts |
| `cpi_ref` | no | `main` | Branch, tag or SHA to build the CPI from |

**Important:** this workflow does not work with `e2e.yml` - they both assume the director is reachable from the same runner. `e2e.yml` runs everything in one job by design.

**Outputs:**

| Output | Description |
|--------|-------------|
| `director-url` | `https://<ip>:25555` |
| `director-ca-cert` | Base64-encoded PEM CA cert |
| `bosh-client` | `admin` |
| `bosh-client-secret` | Admin password (masked in logs) |
| `bosh-env-alias` | BOSH env alias (`qemu-ci` or `lxc-ci`) |
| `bosh-state-b64` | Base64-encoded `bosh-state.json` (needed for teardown) |
| `bosh-vars-b64` | Base64-encoded `bosh-vars.yml` (needed for teardown) |
| `libvirt-private-key-b64` | Base64-encoded SSH private key (needed for teardown) |

**Example:**

```yaml
jobs:
  setup:
    uses: ./.github/workflows/setup-bosh-director.yml
    with:
      backend: lxc

  my-tests:
    needs: setup
    runs-on: my-persistent-runner  # must be the same machine as setup
    steps:
      - name: Decode CA cert
        run: echo "${{ needs.setup.outputs.director-ca-cert }}" | base64 -d > /tmp/ca.crt
      - name: Run tests
        env:
          BOSH_ENVIRONMENT: ${{ needs.setup.outputs.director-url }}
          BOSH_CA_CERT: /tmp/ca.crt
          BOSH_CLIENT: ${{ needs.setup.outputs.bosh-client }}
          BOSH_CLIENT_SECRET: ${{ needs.setup.outputs.bosh-client-secret }}
        run: # your test commands here
```

---

## Failure diagnostics

Each backend has a diagnostics script that collects logs when a step fails:

- `.github/scripts/e2e-debug-qemu.sh <stage>`
- `.github/scripts/e2e-debug-lxc.sh <stage>`

Stages: `wait`, `upload`, `deploy`, `verify`, `worker`.
