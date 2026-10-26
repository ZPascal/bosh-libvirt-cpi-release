# VBox Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add VirtualBox (via libvirt's `vbox:///session` driver) as a first-class backend by wiring the existing `VBoxDomainBuilder` to a config-drive provisioner and extending CI.

**Architecture:** Add `ConfigDisk string` to `DomainDiskPaths`; `VBoxDomainBuilder` attaches it as a third IDE cdrom when non-empty. A new `provisionVBoxConfigDrive()` in `vm/factory.go` builds an ISO9660 agent-env image and writes it to the VM store, then passes its path as `ConfigDisk`. CI workflows gain a `vbox` backend branch mirroring the existing `lxc` pattern.

**Tech Stack:** Go 1.21+, Ginkgo/Gomega test framework, libvirt Go bindings (`libvirt.org/go/libvirt`), GitHub Actions.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `src/bosh-libvirt-cpi/driver/domain_builder.go` | Modify | Add `ConfigDisk string` to `DomainDiskPaths` |
| `src/bosh-libvirt-cpi/driver/domains/vbox.go` | Modify | Attach cdrom disk when `ConfigDisk` non-empty |
| `src/bosh-libvirt-cpi/driver/domains/vbox_test.go` | Modify | Add cdrom/no-cdrom/escape test cases |
| `src/bosh-libvirt-cpi/vm/factory.go` | Modify | Add `provisionVBoxConfigDrive()`, wire into `Create()` switch |
| `src/bosh-libvirt-cpi/vm/factory_test.go` | Modify | Add vmdk provisioner unit tests |
| `src/bosh-libvirt-cpi/vm/vm_integration_test.go` | Modify | Add VBox Create integration test |
| `.github/workflows/e2e.yml` | Modify | Add `vbox` backend branch |
| `.github/workflows/setup-bosh-director.yml` | Modify | Add `vbox` backend branch |
| `.github/workflows/tests.yml` | Modify | Add `integration-tests-vbox` and `e2e-vbox` jobs |
| `manifests/cloud-config-vbox.yml` | Create | Cloud config for VBox subnet |
| `manifests/ops/vbox-local.yml` | Create | Ops file (placeholder, mirrors `lxc-local.yml`) |
| `manifests/ops/vbox-mac.yml` | Create | Fixed MAC ops file for CI |
| `manifests/ops/vbox-director-ssh.yml` | Create | SSH inject ops file for deployed CPI |

---

## Task 1: Add `ConfigDisk` to `DomainDiskPaths`

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/domain_builder.go`

- [ ] **Step 1: Open the file and locate `DomainDiskPaths`**

  `src/bosh-libvirt-cpi/driver/domain_builder.go` lines 18-21:
  ```go
  type DomainDiskPaths struct {
      RootDisk      string
      EphemeralDisk string
  }
  ```

- [ ] **Step 2: Add the `ConfigDisk` field**

  Replace the struct with:
  ```go
  // DomainDiskPaths holds the paths to disk images for a VM domain.
  type DomainDiskPaths struct {
      RootDisk      string
      EphemeralDisk string
      // ConfigDisk is an optional ISO image path attached as a read-only cdrom.
      // When empty, builders omit the config drive disk entirely.
      ConfigDisk string
  }
  ```

- [ ] **Step 3: Verify the project still compiles (zero-value is backward-compatible)**

  ```bash
  cd src/bosh-libvirt-cpi && go build -mod=vendor ./...
  ```
  Expected: no output, exit 0.

- [ ] **Step 4: Commit**

  ```bash
  git add src/bosh-libvirt-cpi/driver/domain_builder.go
  git commit -m "feat: add ConfigDisk field to DomainDiskPaths for optional cdrom attachment"
  ```

---

## Task 2: Attach config drive in `VBoxDomainBuilder`

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/domains/vbox.go`
- Test: `src/bosh-libvirt-cpi/driver/domains/vbox_test.go`

- [ ] **Step 1: Write three failing tests**

  Open `src/bosh-libvirt-cpi/driver/domains/vbox_test.go`. Add inside the `Describe("VBoxDomainBuilder"` / `Describe("BuildDomain"` block, after the existing `It` cases:

  ```go
  It("omits cdrom disk when ConfigDisk is empty", func() {
      xml, err := builder.BuildDomain("vm-vbox-nocd", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
          driver.DomainDiskPaths{RootDisk: "/r.vmdk", EphemeralDisk: "/e.vmdk"})
      Expect(err).To(BeNil())
      Expect(xml).ToNot(ContainSubstring("cdrom"))
      Expect(xml).ToNot(ContainSubstring("sdc"))
  })

  It("includes cdrom disk when ConfigDisk is set", func() {
      xml, err := builder.BuildDomain("vm-vbox-cd", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
          driver.DomainDiskPaths{RootDisk: "/r.vmdk", EphemeralDisk: "/e.vmdk", ConfigDisk: "/vms/vm-1/env.iso"})
      Expect(err).To(BeNil())
      Expect(xml).To(ContainSubstring("device='cdrom'"))
      Expect(xml).To(ContainSubstring("/vms/vm-1/env.iso"))
      Expect(xml).To(ContainSubstring("dev='sdc'"))
      Expect(xml).To(ContainSubstring("<readonly/>"))
      var v interface{}
      Expect(xml2.NewDecoder(strings.NewReader(xml)).Decode(&v)).To(Succeed())
  })

  It("escapes XML special characters in ConfigDisk path", func() {
      result, err := builder.BuildDomain("vm-vbox-esc", driver.VMDomainProps{CPUs: 1, MemoryMB: 512},
          driver.DomainDiskPaths{RootDisk: "/r.vmdk", EphemeralDisk: "/e.vmdk", ConfigDisk: "/path&env.iso"})
      Expect(err).To(BeNil())
      Expect(result).To(ContainSubstring("/path&amp;env.iso"))
      var v interface{}
      Expect(xml2.NewDecoder(strings.NewReader(result)).Decode(&v)).To(Succeed())
  })
  ```

  **Important:** The existing test file imports `"encoding/xml"` — but the test uses it as `xml.NewDecoder`. Since `xml` is already imported, use an alias at the top of the test. Check the existing import block in `vbox_test.go`:

  ```go
  import (
      "encoding/xml"
      "strings"
      ...
  )
  ```

  The existing tests already use `xml.NewDecoder` directly. Add a local alias in only the new tests if needed, or just use `xml.NewDecoder` — the import is already present.

- [ ] **Step 2: Run the new tests and confirm they fail**

  ```bash
  cd src/bosh-libvirt-cpi && go test -mod=vendor ./driver/domains/... -v -run "VBoxDomainBuilder"
  ```
  Expected: the three new `It` cases FAIL (cdrom not present yet).

- [ ] **Step 3: Implement config drive in `VBoxDomainBuilder.BuildDomain`**

  Open `src/bosh-libvirt-cpi/driver/domains/vbox.go`. Replace the `BuildDomain` method body so it conditionally appends the cdrom disk:

  ```go
  func (b VBoxDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
      network := props.Network
      if network == "" {
          network = "default"
      }
      configDiskXML := ""
      if disks.ConfigDisk != "" {
          configDiskXML = fmt.Sprintf(`
      <disk type='file' device='cdrom'>
        <source file='%s'/>
        <target dev='sdc' bus='ide'/>
        <readonly/>
      </disk>`, xmlEscape(disks.ConfigDisk))
      }
      xml := fmt.Sprintf(`<domain type='vbox'>
    <name>%s</name>
    <memory unit='KiB'>%d</memory>
    <vcpu>%d</vcpu>
    <os><type>hvm</type></os>
    <devices>
      <disk type='file' device='disk'>
        <source file='%s'/>
        <target dev='sda' bus='ide'/>
      </disk>
      <disk type='file' device='disk'>
        <source file='%s'/>
        <target dev='sdb' bus='ide'/>
      </disk>%s
      <interface type='network'>
        <source network='%s'/>
      </interface>
    </devices>
  </domain>`, xmlEscape(id), props.MemoryMB*1024, props.CPUs,
          xmlEscape(disks.RootDisk), xmlEscape(disks.EphemeralDisk),
          configDiskXML, xmlEscape(network))
      return xml, nil
  }
  ```

- [ ] **Step 4: Run all vbox domain tests and confirm they pass**

  ```bash
  cd src/bosh-libvirt-cpi && go test -mod=vendor ./driver/domains/... -v -run "VBoxDomainBuilder"
  ```
  Expected: all cases PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add src/bosh-libvirt-cpi/driver/domains/vbox.go \
          src/bosh-libvirt-cpi/driver/domains/vbox_test.go
  git commit -m "feat: attach ISO config drive as third IDE cdrom in VBoxDomainBuilder"
  ```

---

## Task 3: Add `provisionVBoxConfigDrive()` to `vm/factory.go`

**Files:**
- Modify: `src/bosh-libvirt-cpi/vm/factory.go`
- Test: `src/bosh-libvirt-cpi/vm/factory_test.go`

- [ ] **Step 1: Write the failing unit test**

  Open `src/bosh-libvirt-cpi/vm/factory_test.go`. Find the `Describe("Create"` block. Add a new `Context` block for the vmdk provisioner, after the existing `ext4` context:

  ```go
  Context("with vmdk (VBox) domain builder", func() {
      var vboxBuilder *driverfakes.FakeDomainBuilder

      BeforeEach(func() {
          vboxBuilder = &driverfakes.FakeDomainBuilder{
              BuildDomainXML:        "<domain/>",
              DiskImageFormatResult: "vmdk",
          }
          factory = vm.NewFactory(
              vm.FactoryOpts{DirPath: filepath.Join(tmpDir, "vms")},
              vmUUIDGen,
              drv,
              runner,
              vboxBuilder,
              diskFactory,
              apiv1.AgentOptions{Mbus: "nats://nats:nats-password@127.0.0.1:4222"},
              apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2}),
              logger,
          )
          stemcell.ImagePathResult = "/stemcells/sc-1/image.vmdk"
      })

      It("writes env.iso to the VM store", func() {
          _, err := factory.Create(
              apiv1.NewAgentID("agent-1"),
              stemcell,
              cloudProps,
              apiv1.Networks{},
              apiv1.NewVMEnv(nil),
          )
          Expect(err).ToNot(HaveOccurred())

          isoPath := filepath.Join(tmpDir, "vms", "vm-uuid-vm-1", "env.iso")
          isoBytes, readErr := os.ReadFile(isoPath)
          Expect(readErr).ToNot(HaveOccurred())
          Expect(isoBytes).ToNot(BeEmpty())
      })

      It("passes ConfigDisk pointing to env.iso to BuildDomain", func() {
          _, err := factory.Create(
              apiv1.NewAgentID("agent-1"),
              stemcell,
              cloudProps,
              apiv1.Networks{},
              apiv1.NewVMEnv(nil),
          )
          Expect(err).ToNot(HaveOccurred())

          Expect(vboxBuilder.LastBuildDomainDisks.ConfigDisk).To(Equal(
              filepath.Join(tmpDir, "vms", "vm-uuid-vm-1", "env.iso"),
          ))
      })

      It("passes stemcell and ephemeral disk paths to BuildDomain", func() {
          _, err := factory.Create(
              apiv1.NewAgentID("agent-1"),
              stemcell,
              cloudProps,
              apiv1.Networks{},
              apiv1.NewVMEnv(nil),
          )
          Expect(err).ToNot(HaveOccurred())

          Expect(vboxBuilder.LastBuildDomainDisks.RootDisk).To(Equal("/stemcells/sc-1/image.vmdk"))
          Expect(vboxBuilder.LastBuildDomainDisks.EphemeralDisk).ToNot(BeEmpty())
      })
  })
  ```

  **Note:** The test references `vboxBuilder.LastBuildDomainDisks` — you must add this field to `FakeDomainBuilder` in the next step.

- [ ] **Step 2: Add `LastBuildDomainDisks` to `FakeDomainBuilder`**

  Open `src/bosh-libvirt-cpi/driver/fakes/fake_domain_builder.go`. Add the field and update `BuildDomain` to record the call:

  ```go
  package fakes

  import "bosh-libvirt-cpi/driver"

  type FakeDomainBuilder struct {
      BuildDomainXML string
      BuildDomainErr error

      BuildStemcellDomainXML string
      BuildStemcellDomainErr error

      DiskImageFormatResult string

      LastBuildDomainDisks driver.DomainDiskPaths
  }

  var _ driver.DomainBuilder = &FakeDomainBuilder{}

  func (b *FakeDomainBuilder) BuildDomain(id string, props driver.VMDomainProps, disks driver.DomainDiskPaths) (string, error) {
      b.LastBuildDomainDisks = disks
      return b.BuildDomainXML, b.BuildDomainErr
  }

  func (b *FakeDomainBuilder) BuildStemcellDomain(id string, imagePath string) (string, error) {
      return b.BuildStemcellDomainXML, b.BuildStemcellDomainErr
  }

  func (b *FakeDomainBuilder) DiskImageFormat() string { return b.DiskImageFormatResult }
  ```

- [ ] **Step 3: Run the new tests and confirm they fail**

  ```bash
  cd src/bosh-libvirt-cpi && go test -mod=vendor ./vm/... -v -run "vmdk"
  ```
  Expected: compile error or FAIL — `provisionVBoxConfigDrive` not yet implemented.

- [ ] **Step 4: Implement `provisionVBoxConfigDrive` in `vm/factory.go`**

  Add the new method after `provisionExt4Rootfs` (around line 893). Also wire it into the `Create()` switch:

  **New method** (add after the closing `}` of `provisionExt4Rootfs`):
  ```go
  // provisionVBoxConfigDrive builds an ISO9660 agent-env image and writes it
  // to the VM store as env.iso for VBox (vmdk) VMs. The ISO is attached as a
  // read-only cdrom by VBoxDomainBuilder. On error the caller (Create) cleans up.
  func (f Factory) provisionVBoxConfigDrive(
      vm VMImpl,
      stemcell bstem.Stemcell,
      ephemeralDisk bdisk.Disk,
      initialAgentEnv apiv1.AgentEnv,
  ) (driver.DomainDiskPaths, error) {
      envBytes, err := initialAgentEnv.AsBytes()
      if err != nil {
          return driver.DomainDiskPaths{}, bosherr.WrapError(err, "Marshalling agent env for VBox config drive")
      }

      agentEnvBytes := f.injectMbusCert(addBlobstoreToEnv(envBytes))

      isoBytes, err := ISO9660{FileName: "ENV", Contents: agentEnvBytes}.Bytes()
      if err != nil {
          return driver.DomainDiskPaths{}, bosherr.WrapError(err, "Building ISO9660 config drive")
      }

      if err := vm.store.Put("env.iso", isoBytes); err != nil {
          return driver.DomainDiskPaths{}, bosherr.WrapError(err, "Writing env.iso to VM store")
      }

      return driver.DomainDiskPaths{
          RootDisk:      stemcell.ImagePath(),
          EphemeralDisk: ephemeralDisk.ImagePath(),
          ConfigDisk:    vm.store.Path("env.iso"),
      }, nil
  }
  ```

  **Wire into `Create()`** — find the switch block (around line 228):
  ```go
  switch f.domBuilder.DiskImageFormat() {
  case "dir":
      disks, err = f.provisionLXCRootfs(vmID, stemcell, ephemeralDisk, initialAgentEnv, vmProps)
      if err != nil {
          f.cleanUpPartialCreate(vm)
          return nil, err
      }
  case "ext4":
      disks, err = f.provisionExt4Rootfs(vmID, stemcell, ephemeralDisk, initialAgentEnv)
      if err != nil {
          f.cleanUpPartialCreate(vm)
          return nil, err
      }
  case "vmdk":
      disks, err = f.provisionVBoxConfigDrive(vm, stemcell, ephemeralDisk, initialAgentEnv)
      if err != nil {
          f.cleanUpPartialCreate(vm)
          return nil, err
      }
  }
  ```

- [ ] **Step 5: Run the new tests and confirm they pass**

  ```bash
  cd src/bosh-libvirt-cpi && go test -mod=vendor ./vm/... -v -run "vmdk"
  ```
  Expected: all three new cases PASS.

- [ ] **Step 6: Run the full unit test suite to catch regressions**

  ```bash
  cd src/bosh-libvirt-cpi && make test
  ```
  Expected: all tests PASS.

- [ ] **Step 7: Commit**

  ```bash
  git add src/bosh-libvirt-cpi/vm/factory.go \
          src/bosh-libvirt-cpi/vm/factory_test.go \
          src/bosh-libvirt-cpi/driver/fakes/fake_domain_builder.go
  git commit -m "feat: add VBox config drive provisioner (provisionVBoxConfigDrive)"
  ```

---

## Task 4: VBox integration test

**Files:**
- Modify: `src/bosh-libvirt-cpi/vm/vm_integration_test.go`

The integration tests are guarded by `//go:build integration` and run only when a real libvirt connection is available. Add a VBox-backend `Create` test mirroring the existing QEMU pattern.

- [ ] **Step 1: Read the existing QEMU integration test for shape**

  Open `src/bosh-libvirt-cpi/vm/vm_integration_test.go` and find the `Context("with qemu backend"` block.

- [ ] **Step 2: Add the VBox integration test block**

  Add a new `Context` block after the QEMU block:

  ```go
  Context("with vbox backend", func() {
      It("creates a VM with a config drive ISO in the store", func() {
          if os.Getenv("LIBVIRT_URI") != "vbox:///session" {
              Skip("LIBVIRT_URI != vbox:///session — skipping VBox integration test")
          }

          conn, err := libvirt.NewConnect("vbox:///session")
          Expect(err).ToNot(HaveOccurred())
          defer conn.Close()

          libvirtConn := driver.NewLibvirtConnImpl(conn)
          domBuilder := domains.VBoxDomainBuilder{}
          drv := driver.NewLibvirtDriver(libvirtConn, domBuilder, logger)

          storeDir, mkErr := os.MkdirTemp("", "vbox-vm-integration")
          Expect(mkErr).ToNot(HaveOccurred())
          defer os.RemoveAll(storeDir)

          uuidGen := boshuuid.NewGenerator()
          runner := driver.NewLocalRunner(boshsys.NewOsFileSystem(logger), boshcmd.NewExecCmdRunner(logger), logger)
          diskFactory := bdisk.NewFactory(filepath.Join(storeDir, "disks"), uuidGen, drv, runner, logger)

          factory := vm.NewFactory(
              vm.FactoryOpts{DirPath: filepath.Join(storeDir, "vms")},
              uuidGen,
              drv,
              runner,
              domBuilder,
              diskFactory,
              apiv1.AgentOptions{Mbus: "nats://nats:nats-password@127.0.0.1:4222"},
              apiv1.NewStemcellAPIVersion(&stubCallContext{version: 2}),
              logger,
          )

          sc := stemcellfakes.NewFakeStemcell("sc-vbox-1")
          sc.ImagePathResult = os.Getenv("VBOX_STEMCELL_VMDK")
          if sc.ImagePathResult == "" {
              Skip("VBOX_STEMCELL_VMDK not set — skipping VBox integration test")
          }

          v, createErr := factory.Create(
              apiv1.NewAgentID("agent-vbox-1"),
              sc,
              apiv1.CloudPropsImpl{RawMessage: json.RawMessage(`{"memory":512,"cpus":1,"ephemeral_disk":1024}`)},
              apiv1.Networks{},
              apiv1.NewVMEnv(nil),
          )
          Expect(createErr).ToNot(HaveOccurred())
          defer v.Delete()

          isoPath := filepath.Join(storeDir, "vms", v.ID().AsString(), "env.iso")
          isoBytes, readErr := os.ReadFile(isoPath)
          Expect(readErr).ToNot(HaveOccurred())
          Expect(isoBytes).ToNot(BeEmpty())
      })
  })
  ```

- [ ] **Step 3: Run unit tests to confirm the file compiles**

  ```bash
  cd src/bosh-libvirt-cpi && go build -mod=vendor -tags integration ./vm/...
  ```
  Expected: exit 0.

- [ ] **Step 4: Commit**

  ```bash
  git add src/bosh-libvirt-cpi/vm/vm_integration_test.go
  git commit -m "test: add VBox backend integration test for Create with config drive"
  ```

---

## Task 5: Add VBox ops files and cloud config manifest

**Files:**
- Create: `manifests/ops/vbox-local.yml`
- Create: `manifests/ops/vbox-mac.yml`
- Create: `manifests/ops/vbox-director-ssh.yml`
- Create: `manifests/cloud-config-vbox.yml`

- [ ] **Step 1: Create `manifests/ops/vbox-local.yml`**

  ```yaml
  # Placeholder: no modifications needed for local VBox create-env.
  # SSH credentials remain in both cloud_provider and instance_groups so
  # both the bootstrap CPI (running on this host) and the deployed director's
  # CPI can reach libvirtd via the SSH tunnel on 192.168.122.1.
  []
  ```

- [ ] **Step 2: Create `manifests/ops/vbox-mac.yml`**

  ```yaml
  ---
  # Sets a fixed MAC address on the VBox Director VM for CI use.
  # The DHCP reservation pins 192.168.1.8 to this MAC.
  - type: replace
    path: /resource_pools/name=vms/cloud_properties/mac?
    value: "52:54:00:ab:cd:12"
  ```

- [ ] **Step 3: Create `manifests/ops/vbox-director-ssh.yml`**

  ```yaml
  ---
  # Adds SSH credentials for the director-deployed CPI so it can SSH-tunnel
  # back to the host's libvirt socket from inside the director VBox VM.
  - type: replace
    path: /instance_groups/name=bosh/properties/libvirt_cpi/host?
    value: ((libvirt_host))
  - type: replace
    path: /instance_groups/name=bosh/properties/libvirt_cpi/username?
    value: ((libvirt_username))
  - type: replace
    path: /instance_groups/name=bosh/properties/libvirt_cpi/private_key?
    value: ((libvirt_private_key))
  - type: replace
    path: /instance_groups/name=bosh/properties/libvirt_cpi/host_key?
    value: ((libvirt_host_key))
  - type: replace
    path: /cloud_provider/properties/inject_backend_uri?
    value: vbox:///session
  - type: replace
    path: /cloud_provider/properties/inject_host?
    value: ((libvirt_host))
  - type: replace
    path: /cloud_provider/properties/inject_username?
    value: ((libvirt_username))
  - type: replace
    path: /cloud_provider/properties/inject_private_key?
    value: ((libvirt_private_key))
  - type: replace
    path: /cloud_provider/properties/inject_host_key?
    value: ((libvirt_host_key))
  ```

- [ ] **Step 4: Create `manifests/cloud-config-vbox.yml`**

  The VBox manifest (`vbox-cpi.yml`) uses the `192.168.1.0/24` subnet with static IP `192.168.1.8`:

  ```yaml
  azs:
  - name: z1
    cloud_properties: {}

  vm_types:
  - name: default
    cloud_properties:
      memory: 2048
      cpus: 2
      ephemeral_disk: 10240

  disk_types:
  - name: default
    disk_size: 5120
    cloud_properties: {}

  networks:
  - name: default
    type: manual
    subnets:
    - range: 192.168.1.0/24
      gateway: 192.168.1.1
      dns: [8.8.8.8]
      az: z1
      static: [192.168.1.20]
      reserved: [192.168.1.0, 192.168.1.1, 192.168.1.2-192.168.1.19, 192.168.1.30-192.168.1.254]
      cloud_properties: {}

  compilation:
    workers: 1
    reuse_compilation_vms: true
    az: z1
    vm_type: default
    network: default
  ```

- [ ] **Step 5: Commit**

  ```bash
  git add manifests/ops/vbox-local.yml \
          manifests/ops/vbox-mac.yml \
          manifests/ops/vbox-director-ssh.yml \
          manifests/cloud-config-vbox.yml
  git commit -m "feat: add VBox ops files and cloud config manifest"
  ```

---

## Task 6: Extend CI workflows with VBox backend

**Files:**
- Modify: `.github/workflows/e2e.yml`
- Modify: `.github/workflows/setup-bosh-director.yml`
- Modify: `.github/workflows/tests.yml`

### 6a: `e2e.yml`

- [ ] **Step 1: Update the `description` of the `backend` input and the validation step**

  Find:
  ```yaml
        description: 'Virtualization backend: qemu or lxc'
  ```
  Replace with:
  ```yaml
        description: 'Virtualization backend: qemu, lxc, or vbox'
  ```

  Find:
  ```yaml
          qemu|lxc) echo "backend=${{ inputs.backend }}";;
          *) echo "ERROR: unsupported backend '${{ inputs.backend }}' (expected qemu or lxc)"; exit 1;;
  ```
  Replace with:
  ```yaml
          qemu|lxc|vbox) echo "backend=${{ inputs.backend }}";;
          *) echo "ERROR: unsupported backend '${{ inputs.backend }}' (expected qemu, lxc, or vbox)"; exit 1;;
  ```

- [ ] **Step 2: Add VBox env vars to the "Export backend-specific environment" step**

  Find the `else` branch that sets `lxc-ci` env vars:
  ```yaml
          else
            echo "DIRECTOR_IP=192.168.122.11"
            echo "DIRECTOR_MAC=52:54:00:ab:cd:11"
            echo "BOSH_ENV_ALIAS=lxc-ci"
            echo "DIRECTOR_NAME=bosh-lxc-ci"
            echo "DEPLOYMENT_NAME=lxc-test-deployment"
          fi
  ```
  Replace with:
  ```yaml
          elif [ "${{ inputs.backend }}" = "lxc" ]; then
            echo "DIRECTOR_IP=192.168.122.11"
            echo "DIRECTOR_MAC=52:54:00:ab:cd:11"
            echo "BOSH_ENV_ALIAS=lxc-ci"
            echo "DIRECTOR_NAME=bosh-lxc-ci"
            echo "DEPLOYMENT_NAME=lxc-test-deployment"
          else
            echo "DIRECTOR_IP=192.168.1.8"
            echo "DIRECTOR_MAC=52:54:00:ab:cd:12"
            echo "BOSH_ENV_ALIAS=vbox-ci"
            echo "DIRECTOR_NAME=bosh-vbox-ci"
            echo "DEPLOYMENT_NAME=vbox-test-deployment"
          fi
  ```

- [ ] **Step 3: Add "Install VBox + libvirt" step after the LXC install step**

  Add after the `- name: Install LXC + libvirt` block:
  ```yaml
      - name: Install VBox + libvirt
        if: inputs.backend == 'vbox'
        run: |
          sudo apt-get update
          sudo apt-get install -y virtualbox libvirt-daemon-system libvirt-daemon-driver-vbox libvirt-dev
          sudo systemctl start libvirtd
          sudo usermod -aG libvirt "$USER"
          sudo virsh net-start default || true
          sudo virsh net-autostart default || true
  ```

- [ ] **Step 4: Add "Configure DHCP reservation for Director VM (VBox)" step**

  Add after the `- name: Configure DHCP reservation for Director VM (LXC)` block:
  ```yaml
      - name: Configure DHCP reservation for Director VM (VBox)
        if: inputs.backend == 'vbox'
        run: |
          sudo virsh net-update default add ip-dhcp-host \
            "<host mac='${DIRECTOR_MAC}' ip='${DIRECTOR_IP}'/>" \
            --live --config || true
          sudo arp -s "${DIRECTOR_IP}" "${DIRECTOR_MAC}" 2>/dev/null || true
  ```

- [ ] **Step 5: Add "Bootstrap BOSH Director (VBox)" step**

  Add after the `- name: Bootstrap BOSH Director (LXC)` block:
  ```yaml
      - name: Bootstrap BOSH Director (VBox)
        if: inputs.backend == 'vbox'
        shell: bash
        run: |
          sudo mkdir -p /tmp/bosh_libvirt_cpi/{stemcells,vms,disks}
          sudo chmod -R 755 /tmp/bosh_libvirt_cpi
          sudo chown -R root:root /tmp/bosh_libvirt_cpi
          rm -f /tmp/bosh-create-env.done /tmp/bosh-create-env.failed
          setsid bash -c '
            sudo bosh create-env manifests/vbox-cpi.yml \
              -o manifests/ops/vbox-local.yml \
              -o manifests/local-release.yml \
              -o manifests/ops/local-store-dir.yml \
              -o manifests/ops/vbox-mac.yml \
              -o manifests/ops/vbox-director-ssh.yml \
              -v director_name='"${DIRECTOR_NAME}"' \
              -v internal_ip='"${DIRECTOR_IP}"' \
              -v stemcell_url=file:///tmp/bosh-stemcell.tgz \
              -v libvirt_host=192.168.122.1 \
              -v libvirt_username=root \
              --var-file libvirt_private_key=/tmp/bosh-libvirt-ssh-key \
              -v "libvirt_host_key='"${LIBVIRT_HOST_KEY}"'" \
              --state /tmp/bosh-state.json \
              --vars-store /tmp/bosh-vars.yml \
              > /tmp/bosh-create-env.log 2>&1 \
              && touch /tmp/bosh-create-env.done \
              || touch /tmp/bosh-create-env.failed
          ' &
          disown $!
          echo "bosh create-env started in new session (PID $!)"
  ```

- [ ] **Step 6: Add "Upload stemcell and release (VBox)" step**

  Add after the `- name: Upload stemcell and release (LXC)` block:
  ```yaml
      - name: Upload stemcell and release (VBox)
        if: inputs.backend == 'vbox'
        shell: bash
        run: |
          timeout 120 bosh -e "${BOSH_ENV_ALIAS}" update-cloud-config manifests/cloud-config-vbox.yml \
            --client admin --client-secret admin-password \
            -n
          timeout 600 bosh -e "${BOSH_ENV_ALIAS}" upload-stemcell /tmp/bosh-stemcell.tgz \
            --client admin --client-secret admin-password || {
              bash .github/scripts/e2e-debug-${{ inputs.backend }}.sh upload || true
              exit 1
            }
          echo "Waiting for blobstore (port 25250)..."
          _blob_ok=0
          for _i in $(seq 1 80); do
            if nc -z "${DIRECTOR_IP}" 25250 2>/dev/null; then _blob_ok=1; break; fi
            sleep 3
          done
          if [ "$_blob_ok" != "1" ]; then
            echo "ERROR: blobstore port 25250 never opened after 240s"
            bash .github/scripts/e2e-debug-${{ inputs.backend }}.sh upload || true
            exit 1
          fi
          echo "Blobstore ready"
          timeout 300 bosh -e "${BOSH_ENV_ALIAS}" upload-release \
            https://bosh.io/d/github.com/cloudfoundry/bosh?v=283.1.3 \
            --sha1 91a350d377e9185fbc950a1168073eb1e3207eae \
            --client admin --client-secret admin-password
  ```

- [ ] **Step 7: Add "Clean up Director (VBox)" step**

  Add after the `- name: Clean up Director (LXC)` block:
  ```yaml
      - name: Clean up Director (VBox)
        if: always() && inputs.backend == 'vbox'
        run: |
          sudo bosh delete-env manifests/vbox-cpi.yml \
            -o manifests/ops/vbox-local.yml \
            -o manifests/local-release.yml \
            -o manifests/ops/local-store-dir.yml \
            -o manifests/ops/vbox-mac.yml \
            -o manifests/ops/vbox-director-ssh.yml \
            --state /tmp/bosh-state.json \
            --vars-store /tmp/bosh-vars.yml \
            -v director_name="${DIRECTOR_NAME}" \
            -v internal_ip="${DIRECTOR_IP}" \
            -v stemcell_url=file:///tmp/bosh-stemcell.tgz \
            -v libvirt_host=192.168.122.1 \
            -v libvirt_username=root \
            --var-file libvirt_private_key=/tmp/bosh-libvirt-ssh-key \
            -v "libvirt_host_key=${LIBVIRT_HOST_KEY}" || true
  ```

### 6b: `setup-bosh-director.yml`

Apply the same changes as `e2e.yml`:

- [ ] **Step 8: Update `description`, validation case, env-var block, install step, DHCP reservation, bootstrap step, and emit step in `setup-bosh-director.yml`**

  - Change `description: 'Virtualization backend: qemu or lxc'` → `'Virtualization backend: qemu, lxc, or vbox'`
  - Change `qemu|lxc)` → `qemu|lxc|vbox)` and update error message
  - Change `else` in the env-var block to `elif ... lxc` / `else` for vbox (same vars as step 2 above)
  - Add `- name: Install VBox + libvirt` step (identical to step 3 above)
  - Add `- name: Configure DHCP reservation for VBox Director VM` step (identical to step 4 above)
  - Add `- name: Bootstrap BOSH Director` (vbox, `if: inputs.backend == 'vbox'`) step (identical to step 5 above)

### 6c: `tests.yml`

- [ ] **Step 9: Add `integration-tests-vbox` job to `tests.yml`**

  Add after the `integration-tests-lxc` job:
  ```yaml
    integration-tests-vbox:
      name: Integration Tests (VBox)
      runs-on: ubuntu-latest
      needs: build

      steps:
      - uses: actions/checkout@v7

      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version-file: src/bosh-libvirt-cpi/go.mod
          cache-dependency-path: src/bosh-libvirt-cpi/go.sum

      - name: Install VBox + libvirt
        run: |
          sudo apt-get update
          sudo apt-get install -y virtualbox libvirt-daemon-system libvirt-daemon-driver-vbox libvirt-dev
          sudo systemctl start libvirtd
          sudo usermod -aG libvirt $USER

      - name: Verify vendor directory
        working-directory: src/bosh-libvirt-cpi
        run: |
          if [ ! -d "vendor" ]; then
            echo "Vendor directory not found. Running go mod vendor..."
            go mod vendor
          fi

      - name: Run integration tests (VBox)
        working-directory: src/bosh-libvirt-cpi
        env:
          LIBVIRT_URI: vbox:///session
        run: sudo -E "$(which go)" test -tags integration -mod=vendor ./...

      - name: Generate coverage
        working-directory: src/bosh-libvirt-cpi
        run: sudo -E "$(which go)" test -coverprofile=coverage.out -mod=vendor ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v7
        with:
          files: src/bosh-libvirt-cpi/coverage.out
          flags: integration-vbox
  ```

- [ ] **Step 10: Add `vbox` to the E2E matrix in `tests.yml`**

  Find:
  ```yaml
      matrix:
        backend: [qemu, lxc]
  ```
  Replace with:
  ```yaml
      matrix:
        backend: [qemu, lxc, vbox]
  ```

- [ ] **Step 11: Commit all CI changes**

  ```bash
  git add .github/workflows/e2e.yml \
          .github/workflows/setup-bosh-director.yml \
          .github/workflows/tests.yml
  git commit -m "ci: add VBox backend to e2e, setup-bosh-director, and tests workflows"
  ```

---

## Task 7: Final verification

- [ ] **Step 1: Run the full unit test suite**

  ```bash
  cd src/bosh-libvirt-cpi && make test
  ```
  Expected: all tests PASS, no failures.

- [ ] **Step 2: Run `go vet` to catch any issues**

  ```bash
  cd src/bosh-libvirt-cpi && go vet -mod=vendor ./...
  ```
  Expected: no output, exit 0.

- [ ] **Step 3: Run `gofmt` check**

  ```bash
  cd src/bosh-libvirt-cpi && files=$(gofmt -l $(find . -name '*.go' -not -path './vendor/*')); if [ -n "$files" ]; then echo "Format issues: $files"; exit 1; fi
  ```
  Expected: no output, exit 0.

- [ ] **Step 4: Confirm domain builder interface is satisfied**

  ```bash
  cd src/bosh-libvirt-cpi && go build -mod=vendor ./driver/domains/...
  ```
  Expected: exit 0 (the `var _ driver.DomainBuilder = VBoxDomainBuilder{}` compile-time check still passes).
