# LXC Integration Test Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full end-to-end LXC integration path — URI-aware Go integration test helpers, local-mode manifests, a minimal deployment manifest, a colleague config, and a new `integration-tests-lxc` CI job alongside the renamed `integration-tests-qemu` job.

**Architecture:** The domain builder selection is lifted out of hardcoded test setup into URI-driven helpers that mirror `cpi/factory.go`. A new ops file strips SSH fields from the existing `lxc-cpi.yml` for local CI use. The GitHub Actions workflow grows one new job and renames the existing one.

**Tech Stack:** Go 1.24, Ginkgo/Gomega, libvirt-go, BOSH CLI, GitHub Actions

---

## File Map

| File | Action |
|------|--------|
| `src/bosh-libvirt-cpi/driver/integration_helpers_test.go` | **Create** — `domBuilderFromEnv()` + `testDomainXML()` helpers |
| `src/bosh-libvirt-cpi/driver/libvirt_driver_integration_test.go` | **Modify** — use helpers instead of hardcoded QEMU builder + XML |
| `src/bosh-libvirt-cpi/stemcell/integration_helpers_test.go` | **Create** — `domBuilderFromEnv()` helper |
| `src/bosh-libvirt-cpi/stemcell/stemcell_integration_test.go` | **Modify** — replace `QEMUDomainBuilder{}` with `domBuilderFromEnv()` |
| `src/bosh-libvirt-cpi/vm/integration_helpers_test.go` | **Create** — `domBuilderFromEnv()` helper |
| `src/bosh-libvirt-cpi/vm/vm_integration_test.go` | **Modify** — replace `QEMUDomainBuilder{}` with `domBuilderFromEnv()` |
| `manifests/ops/lxc-local.yml` | **Create** — ops file removing SSH fields for local CI use |
| `manifests/lxc-deployment.yml` | **Create** — minimal nats-only deployment for smoke test |
| `config/cpi-lxc-local.json` | **Create** — colleague handoff config |
| `.github/workflows/tests.yml` | **Modify** — rename existing job; add LXC job |

---

## Task 1: Create driver integration helpers

**Files:**
- Create: `src/bosh-libvirt-cpi/driver/integration_helpers_test.go`

- [ ] **Step 1: Create the helpers file**

```go
//go:build integration

package driver_test

import (
	"fmt"
	"net/url"
	"os"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

func domBuilderFromEnv() driver.DomainBuilder {
	uri := os.Getenv("LIBVIRT_URI")
	u, _ := url.Parse(uri)
	switch u.Scheme {
	case "lxc":
		return domains.LXCDomainBuilder{}
	case "vbox":
		return domains.VBoxDomainBuilder{}
	default:
		return domains.QEMUDomainBuilder{}
	}
}

func testDomainXML(name string) string {
	uri := os.Getenv("LIBVIRT_URI")
	u, _ := url.Parse(uri)
	if u.Scheme == "lxc" {
		return fmt.Sprintf(`<domain type='lxc'>
  <name>%s</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type>exe</type><init>/sbin/init</init></os>
  <devices><emulator>/usr/lib/libvirt/libvirt_lxc</emulator></devices>
</domain>`, name)
	}
	return fmt.Sprintf(`<domain type='kvm'>
  <name>%s</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
</domain>`, name)
}
```

Save to `src/bosh-libvirt-cpi/driver/integration_helpers_test.go`.

- [ ] **Step 2: Verify it compiles**

```bash
cd src/bosh-libvirt-cpi
go build -tags integration ./driver/...
```

Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/integration_helpers_test.go
git commit -m "test(driver): add URI-driven domain builder and XML helpers for integration tests"
```

---

## Task 2: Update driver integration test to use helpers

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/libvirt_driver_integration_test.go`

The existing test has three places to update:
1. `BeforeEach`: `domains.QEMUDomainBuilder{}` → `domBuilderFromEnv()`
2. `DefineDomain / LookupDomain / DestroyDomain` `It` block: inline XML literal → `testDomainXML("bosh-integration-test")`
3. `UpdateDomainMemory / UpdateDomainCPUs` `const updateTestXML` → `testDomainXML("bosh-integration-update-test")`

- [ ] **Step 1: Replace the `BeforeEach` builder**

In `src/bosh-libvirt-cpi/driver/libvirt_driver_integration_test.go`, change line 39:

Old:
```go
		d = driver.NewLibvirtDriver(libvirtConn, domains.QEMUDomainBuilder{}, logger)
```

New:
```go
		d = driver.NewLibvirtDriver(libvirtConn, domBuilderFromEnv(), logger)
```

- [ ] **Step 2: Remove the hardcoded XML in the define/lookup/destroy test**

Old (lines 50–56):
```go
			xml := `<domain type='kvm'>
  <name>bosh-integration-test</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
</domain>`

			err := d.DefineDomain(xml)
```

New:
```go
			err := d.DefineDomain(testDomainXML("bosh-integration-test"))
```

- [ ] **Step 3: Replace the const XML in the update test**

Old (lines 82–88):
```go
		const updateTestXML = `<domain type='kvm'>
  <name>bosh-integration-update-test</name>
  <memory unit='KiB'>65536</memory>
  <vcpu>1</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
</domain>`

		BeforeEach(func() {
			_ = d.DestroyDomain("bosh-integration-update-test")
			Expect(d.DefineDomain(updateTestXML)).To(Succeed())
```

New (remove the const, call helper inline):
```go
		BeforeEach(func() {
			_ = d.DestroyDomain("bosh-integration-update-test")
			Expect(d.DefineDomain(testDomainXML("bosh-integration-update-test"))).To(Succeed())
```

Also remove the `AfterEach` reference to `updateTestXML` — it only uses the domain name string, which stays unchanged.

- [ ] **Step 4: Verify compilation**

```bash
cd src/bosh-libvirt-cpi
go build -tags integration ./driver/...
```

Expected: no output, exit 0.

- [ ] **Step 5: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/libvirt_driver_integration_test.go
git commit -m "test(driver): use domBuilderFromEnv and testDomainXML in integration test"
```

---

## Task 3: Create stemcell integration helper and update test

**Files:**
- Create: `src/bosh-libvirt-cpi/stemcell/integration_helpers_test.go`
- Modify: `src/bosh-libvirt-cpi/stemcell/stemcell_integration_test.go`

- [ ] **Step 1: Create the helpers file**

```go
//go:build integration

package stemcell_test

import (
	"net/url"
	"os"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

func domBuilderFromEnv() driver.DomainBuilder {
	uri := os.Getenv("LIBVIRT_URI")
	u, _ := url.Parse(uri)
	switch u.Scheme {
	case "lxc":
		return domains.LXCDomainBuilder{}
	case "vbox":
		return domains.VBoxDomainBuilder{}
	default:
		return domains.QEMUDomainBuilder{}
	}
}
```

Save to `src/bosh-libvirt-cpi/stemcell/integration_helpers_test.go`.

- [ ] **Step 2: Update stemcell_integration_test.go**

In `src/bosh-libvirt-cpi/stemcell/stemcell_integration_test.go`, change line 61:

Old:
```go
		domBuilder := domains.QEMUDomainBuilder{}
```

New:
```go
		domBuilder := domBuilderFromEnv()
```

- [ ] **Step 3: Verify compilation**

```bash
cd src/bosh-libvirt-cpi
go build -tags integration ./stemcell/...
```

Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add src/bosh-libvirt-cpi/stemcell/integration_helpers_test.go \
        src/bosh-libvirt-cpi/stemcell/stemcell_integration_test.go
git commit -m "test(stemcell): use domBuilderFromEnv in integration test"
```

---

## Task 4: Create vm integration helper and update test

**Files:**
- Create: `src/bosh-libvirt-cpi/vm/integration_helpers_test.go`
- Modify: `src/bosh-libvirt-cpi/vm/vm_integration_test.go`

- [ ] **Step 1: Create the helpers file**

```go
//go:build integration

package vm_test

import (
	"net/url"
	"os"

	"bosh-libvirt-cpi/driver"
	"bosh-libvirt-cpi/driver/domains"
)

func domBuilderFromEnv() driver.DomainBuilder {
	uri := os.Getenv("LIBVIRT_URI")
	u, _ := url.Parse(uri)
	switch u.Scheme {
	case "lxc":
		return domains.LXCDomainBuilder{}
	case "vbox":
		return domains.VBoxDomainBuilder{}
	default:
		return domains.QEMUDomainBuilder{}
	}
}
```

Save to `src/bosh-libvirt-cpi/vm/integration_helpers_test.go`.

- [ ] **Step 2: Update vm_integration_test.go**

In `src/bosh-libvirt-cpi/vm/vm_integration_test.go`, change line 57:

Old:
```go
		domBuilder := domains.QEMUDomainBuilder{}
```

New:
```go
		domBuilder := domBuilderFromEnv()
```

- [ ] **Step 3: Verify compilation**

```bash
cd src/bosh-libvirt-cpi
go build -tags integration ./vm/...
```

Expected: no output, exit 0.

- [ ] **Step 4: Run unit tests to confirm nothing regressed**

```bash
cd src/bosh-libvirt-cpi
go test ./...
```

Expected: all unit tests pass, exit 0.

- [ ] **Step 5: Commit**

```bash
git add src/bosh-libvirt-cpi/vm/integration_helpers_test.go \
        src/bosh-libvirt-cpi/vm/vm_integration_test.go
git commit -m "test(vm): use domBuilderFromEnv in integration test"
```

---

## Task 5: Add lxc-local ops file

**Files:**
- Create: `manifests/ops/lxc-local.yml`

This ops file strips the four SSH fields from both `instance_groups[name=bosh].properties.libvirt_cpi` and `cloud_provider.properties.libvirt_cpi`, making the manifest usable on a local runner with no secrets.

- [ ] **Step 1: Create the ops file**

```yaml
---
# Removes SSH connection fields from lxc-cpi.yml so bosh create-env
# can run locally (lxc:///) without any SSH secrets.
- type: remove
  path: /instance_groups/name=bosh/properties/libvirt_cpi/host?
- type: remove
  path: /instance_groups/name=bosh/properties/libvirt_cpi/username?
- type: remove
  path: /instance_groups/name=bosh/properties/libvirt_cpi/private_key?
- type: remove
  path: /instance_groups/name=bosh/properties/libvirt_cpi/host_key?
- type: remove
  path: /cloud_provider/properties/libvirt_cpi/host?
- type: remove
  path: /cloud_provider/properties/libvirt_cpi/username?
- type: remove
  path: /cloud_provider/properties/libvirt_cpi/private_key?
- type: remove
  path: /cloud_provider/properties/libvirt_cpi/host_key?
```

Note: the `?` suffix on each path means "remove if present" — the op is a no-op if the field is already absent, which makes this safe to apply on top of other ops files too.

Save to `manifests/ops/lxc-local.yml`.

- [ ] **Step 2: Commit**

```bash
git add manifests/ops/lxc-local.yml
git commit -m "feat(manifests): add lxc-local ops file to strip SSH fields for local CI use"
```

---

## Task 6: Add minimal deployment manifest

**Files:**
- Create: `manifests/lxc-deployment.yml`

This is the smoke-test deployment run after `bosh create-env`. It deploys a single `nats` instance — the smallest self-contained BOSH job.

- [ ] **Step 1: Create the manifest**

```yaml
---
# Minimal single-instance deployment for LXC integration smoke test.
# Deploys one nats instance to verify the Director can manage LXC containers.
name: lxc-test-deployment

releases:
- name: bosh
  version: latest

stemcells:
- alias: default
  os: ubuntu-jammy
  version: latest

update:
  canaries: 1
  max_in_flight: 1
  canary_watch_time: 1000-30000
  update_watch_time: 1000-30000

instance_groups:
- name: nats
  instances: 1
  jobs:
  - name: nats
    release: bosh
  vm_type: default
  stemcell: default
  networks:
  - name: default
```

Save to `manifests/lxc-deployment.yml`.

- [ ] **Step 2: Commit**

```bash
git add manifests/lxc-deployment.yml
git commit -m "feat(manifests): add minimal nats deployment manifest for LXC smoke test"
```

---

## Task 7: Add colleague config

**Files:**
- Create: `config/cpi-lxc-local.json`

- [ ] **Step 1: Create the config file**

```json
{
  "BackendURI": "lxc:///",
  "StoreDir": "~/.bosh_libvirt_cpi",
  "Agent": {
    "mbus": "https://mbus:mbus-password@0.0.0.0:6868",
    "ntp": ["0.pool.ntp.org", "1.pool.ntp.org"],
    "blobstore": {
      "provider": "local",
      "options": {
        "blobstore_path": "/var/vcap/micro_bosh/data/cache"
      }
    }
  }
}
```

Save to `config/cpi-lxc-local.json`.

**Prerequisites for the recipient (include in the handoff message):**
```bash
# Ubuntu 22.04 / 24.04
sudo apt-get install -y lxc libvirt-daemon-system libvirt-daemon-driver-lxc libvirt-dev
sudo systemctl enable --now libvirtd
sudo usermod -aG libvirt $USER
newgrp libvirt                     # activate group without re-login
virsh -c lxc:/// list --all        # should return empty list, not an error
```

- [ ] **Step 2: Commit**

```bash
git add config/cpi-lxc-local.json
git commit -m "feat(config): add cpi-lxc-local.json for local LXC use without SSH"
```

---

## Task 8: Update GitHub Actions workflow

**Files:**
- Modify: `.github/workflows/tests.yml`

Two changes:
1. Rename the `integration-tests` job id and display name to make it clear it targets QEMU.
2. Add a new `integration-tests-lxc` job that installs LXC, runs Go integration tests via `lxc:///`, then runs `bosh create-env` + `bosh deploy`.

- [ ] **Step 1: Rename the existing integration-tests job**

In `.github/workflows/tests.yml`, make the following two changes to the existing job:

Old (line 71):
```yaml
  integration-tests:
    name: Integration Tests
```

New:
```yaml
  integration-tests-qemu:
    name: Integration Tests (QEMU)
```

Also update the `code-quality` job's `needs` reference on line 119 if it references `integration-tests` — check with:
```bash
grep 'needs:' .github/workflows/tests.yml
```
The `code-quality` job needs `unit-tests`, not `integration-tests`, so no change needed there.

- [ ] **Step 2: Add the LXC integration job**

Append the following job to `.github/workflows/tests.yml`, after the `integration-tests-qemu` job block and before `code-quality`:

```yaml
  integration-tests-lxc:
    name: Integration Tests (LXC)
    runs-on: ubuntu-latest
    needs: build

    steps:
    - uses: actions/checkout@v7

    - name: Set up Go
      uses: actions/setup-go@v7
      with:
        go-version-file: src/bosh-libvirt-cpi/go.mod
        cache-dependency-path: src/bosh-libvirt-cpi/go.sum

    - name: Install LXC + libvirt
      run: |
        sudo apt-get update
        sudo apt-get install -y lxc libvirt-daemon-system libvirt-daemon-driver-lxc libvirt-dev
        sudo systemctl start libvirtd
        sudo usermod -aG libvirt $USER

    - name: Verify vendor directory
      working-directory: src/bosh-libvirt-cpi
      run: |
        if [ ! -d "vendor" ]; then
          echo "Vendor directory not found. Running go mod vendor..."
          go mod vendor
        fi

    - name: Run integration tests (LXC)
      working-directory: src/bosh-libvirt-cpi
      env:
        LIBVIRT_URI: lxc:///
      run: sudo -E go test -tags integration -mod=vendor ./...

    - name: Install BOSH CLI
      run: |
        curl -fsSL https://github.com/cloudfoundry/bosh-cli/releases/latest/download/bosh-cli-linux-amd64 \
          -o /usr/local/bin/bosh
        chmod +x /usr/local/bin/bosh
        bosh --version

    - name: Build CPI release tarball
      run: |
        sudo apt-get install -y ruby ruby-dev build-essential
        sudo gem install bosh-cli --no-document
        bosh create-release --force --tarball /tmp/bosh-libvirt-cpi.tgz

    - name: Download BOSH stemcell
      run: |
        curl -fsSL https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-jammy-go_agent \
          -o /tmp/bosh-stemcell.tgz

    - name: Bootstrap BOSH Director via LXC
      run: |
        INTERNAL_IP=$(hostname -I | awk '{print $1}')
        sudo bosh create-env manifests/lxc-cpi.yml \
          -o manifests/ops/lxc-local.yml \
          -o manifests/local-release.yml \
          -v director_name=bosh-lxc-ci \
          -v internal_ip="${INTERNAL_IP}" \
          -v stemcell_url=/tmp/bosh-stemcell.tgz \
          --state /tmp/bosh-state.json

    - name: Configure BOSH alias
      run: |
        INTERNAL_IP=$(hostname -I | awk '{print $1}')
        bosh alias-env lxc-ci \
          -e "${INTERNAL_IP}" \
          --ca-cert <(bosh int /tmp/bosh-state.json --path /current_manifest_sha)

    - name: Deploy smoke test deployment
      run: |
        bosh -e lxc-ci -d lxc-test-deployment deploy manifests/lxc-deployment.yml \
          -n \
          --fix

    - name: Clean up Director
      if: always()
      run: |
        sudo bosh delete-env manifests/lxc-cpi.yml \
          -o manifests/ops/lxc-local.yml \
          -o manifests/local-release.yml \
          --state /tmp/bosh-state.json \
          -v director_name=bosh-lxc-ci \
          -v stemcell_url=/tmp/bosh-stemcell.tgz || true
```

- [ ] **Step 3: Verify YAML is valid**

```bash
python3 -c "import yaml, sys; yaml.safe_load(open('.github/workflows/tests.yml'))" && echo "YAML OK"
```

Expected: `YAML OK`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/tests.yml
git commit -m "ci: rename integration-tests to integration-tests-qemu; add integration-tests-lxc job"
```

---

## Self-Review Notes

### Spec coverage check

| Spec requirement | Covered by |
|------------------|-----------|
| Rename existing job to QEMU | Task 8 Step 1 |
| Go tests run against `lxc:///` | Tasks 1–4 + Task 8 |
| URI-driven `domBuilderFromEnv()` in all 3 packages | Tasks 1, 3, 4 |
| `testDomainXML()` for driver package | Task 1 |
| `manifests/ops/lxc-local.yml` | Task 5 |
| `manifests/lxc-deployment.yml` | Task 6 |
| `config/cpi-lxc-local.json` with prerequisites | Task 7 |
| `bosh create-env` in CI | Task 8 Step 2 |
| `bosh deploy` in CI | Task 8 Step 2 |
| Cleanup step (always) | Task 8 Step 2 |

All spec requirements are covered. ✓