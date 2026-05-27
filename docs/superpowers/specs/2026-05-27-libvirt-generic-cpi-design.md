# bosh-libvirt-cpi: Generic Libvirt CPI Driver Design

**Date:** 2026-05-27  
**Status:** Approved for implementation

## Goals

1. Replace the VBoxManage CLI-based driver with the libvirt Go API (`libvirt.org/go/libvirt`) as the unified hypervisor layer.
2. Support multiple backends (VirtualBox, LXC, QEMU/KVM) via libvirt connection URIs — no new abstraction layer needed beyond what libvirt already provides.
3. Increase unit test coverage to 90%, with integration tests gated behind a `//go:build integration` tag.

## Architecture Overview

Three concerns are kept separate:

- **Driver layer** (`driver/`): `LibvirtDriver` replaces `ExecDriver`. Holds a `*libvirt.Connect` and a `DomainBuilder`. Implements the `Driver` interface with libvirt-semantic methods.
- **Domain building** (`driver/domains/`): `DomainBuilder` interface produces libvirt XML domain definitions. Three implementations: `VBoxDomainBuilder`, `LXCDomainBuilder`, `QEMUDomainBuilder`.
- **Config** (`cpi/factory_options.go`): `BinPath` replaced by `BackendURI` (e.g. `"vbox:///session"`, `"lxc:///"`, `"qemu:///system"`). Factory parses the URI scheme to select the right `DomainBuilder`.

Everything above `driver/` — `vm/`, `stemcell/`, `disk/`, `cpi/` — retains its interface shape. Only the concrete implementations it receives change.

## Driver Package Changes

### Deleted
- `driver/exec.go` — `ExecDriver`, all VBoxManage regex patterns, exit-code 126 handling, `IsMissingVMErr` string matching.

### `driver/interfaces.go` — revised `Driver` interface

The `Driver` interface becomes libvirt-semantic rather than a generic shell executor:

```go
type Driver interface {
    // Domain lifecycle
    DefineDomain(xml string) error
    StartDomain(id string) error
    ShutdownDomain(id string) error
    DestroyDomain(id string) error
    RebootDomain(id string) error
    LookupDomain(id string) (Domain, error)

    // Domain config
    UpdateDomainMemory(id string, memoryMB int) error
    UpdateDomainCPUs(id string, cpus int) error

    // Storage
    CreateStorageVol(poolName, volName string, sizeMB int) (string, error)
    DeleteStorageVol(poolName, volName string) error

    // Error helpers
    IsMissingDomainErr(err error) bool
}

type Domain interface {
    GetName() (string, error)
    GetState() (int, int, error) // state, reason — avoids leaking libvirt pkg into interface
    IsActive() (bool, error)
}
```

`Execute`/`ExecuteComplex` are removed. `Runner` stays unchanged (file operations: `Upload`, `Put`, `Get`, `Execute` for shell commands on the target host).

### New `driver/libvirt_driver.go`

```go
type LibvirtDriver struct {
    conn       LibvirtConn   // interface wrapping *libvirt.Connect — swappable in tests
    domBuilder DomainBuilder
    logger     boshlog.Logger
}

func NewLibvirtDriver(uri string, builder DomainBuilder, logger boshlog.Logger) (LibvirtDriver, error)
```

`IsMissingDomainErr` checks `libvirt.IsNotFound(err)`.

### New `driver/libvirt_conn.go`

```go
type LibvirtConn interface {
    DomainDefineXML(xml string) (libvirt.Domain, error)
    LookupDomainByName(id string) (libvirt.Domain, error)
    StoragePoolLookupByName(name string) (libvirt.StoragePool, error)
    // ... other methods used by LibvirtDriver
}
```

Wrapping `*libvirt.Connect` behind this interface makes `LibvirtDriver` fully unit-testable without a real daemon.

### New `driver/domain_builder.go`

```go
type DomainDiskPaths struct {
    RootDisk      string
    EphemeralDisk string
}

type DomainBuilder interface {
    BuildDomain(id string, props VMDomainProps, disks DomainDiskPaths) (string, error)
    BuildStemcellDomain(id string, imagePath string) (string, error)
    DiskImageFormat() string   // "vmdk", "raw", "qcow2"
    StorageController() string // "ide", "sata", "virtio", "lxc"
}
```

Implementations in `driver/domains/vbox.go`, `driver/domains/lxc.go`, `driver/domains/qemu.go`.

### `driver/retry.go` — kept unchanged

`Retrier`, `RetrierImpl`, `RetryableErrorImpl` stay. Retry logic is still useful for transient libvirt connection errors.

## Config Changes (`cpi/factory_options.go`)

```go
type FactoryOpts struct {
    // Connection
    BackendURI string   // replaces BinPath; e.g. "vbox:///session", "lxc:///", "qemu:///system"
    Host       string   // for SSH runner (remote file operations)
    Username   string
    PrivateKey string

    StoreDir string

    AutoEnableNetworks bool
    // StorageController field removed — each DomainBuilder encodes its own controller type

    Agent apiv1.AgentOptions
}
```

`Validate()` checks `BackendURI` is non-empty and is one of the known schemes. `StorageController` is removed from `FactoryOpts` — each `DomainBuilder` encodes its own controller type.

`cpi/factory.go` `New()` method:
1. Parses `BackendURI` scheme
2. Selects `DomainBuilder` based on scheme
3. Constructs `LibvirtDriver` with `libvirt.NewConnect(opts.BackendURI)`
4. Constructs `stemcell.Factory`, `disk.Factory`, `vm.Factory` as before

## VM, Stemcell, and Disk Layer Changes

### `vm/vm.go` (`VMImpl`)

| Old (VBoxManage) | New (libvirt) |
|---|---|
| `driver.Execute("modifyvm", ...)` | `driver.UpdateDomainMemory/CPUs(...)` |
| `driver.Execute("unregistervm", "--delete")` | `driver.DestroyDomain(id)` |
| `driver.Execute("controlvm", id, "reset")` | `driver.RebootDomain(id)` |
| `driver.Execute("showvminfo", ...)` | `driver.LookupDomain(id)` |
| `driver.Execute("controlvm", id, "acpipowerbutton")` | `driver.ShutdownDomain(id)` |

Shared folder support is VirtualBox-specific and is removed. `SetProps` is simplified to memory/CPU only — backend-specific props go into `DomainBuilder`.

### `vm/portdevices/`

The `PortDevices` abstraction (IDE/SATA/SCSI port numbering) is removed. Disk attachment logic moves into `DomainBuilder` XML generation. The `vm/portdevices/` package is deleted.

### `stemcell/stemcell.go` (`StemcellImpl`)

| Old | New |
|---|---|
| `driver.Execute("snapshot", id, "take", snapshotName)` | `driver.DefineDomain(builder.BuildStemcellDomain(id, imagePath))` |
| `driver.Execute("showvminfo", ...)` | `driver.LookupDomain(id)` |
| `driver.Execute("unregistervm", "--delete")` + rm path | `driver.DestroyDomain(id)` + `runner.Execute("rm", "-rf", path)` |

`SnapshotName()` is removed — libvirt cloning does not use VirtualBox snapshots.

### `stemcell/factory.go`

- `importOVF()` removed — libvirt does not import OVF
- `switchRootDiskToIDEController()` / `switchRootDiskToSATAController()` removed
- New flow: decompress stemcell tarball → extract disk image → call `driver.DefineDomain(builder.BuildStemcellDomain(id, diskImagePath))`
- Storage controller type comes from `builder.StorageController()` not from `FactoryOpts`

### `disk/disk.go` (`DiskImpl`)

- `VMDKPath()` → `ImagePath()` returning path to a raw/qcow2 image
- `DiskImpl.Exists()` checks via libvirt storage vol lookup rather than `ls`
- `DiskImpl.Delete()` calls `driver.DeleteStorageVol(...)` then `runner.Execute("rm", "-rf", path)` as fallback

### `disk/factory.go`

- `Create()` calls `driver.CreateStorageVol(poolName, id, sizeMB)` instead of `VBoxManage createmedium`

## Testing Strategy

### Fakes (unit tests, no libvirt daemon required)

| Package | Fake |
|---|---|
| `driver/fakes/` | `FakeDriver`, `FakeDomainBuilder`, `FakeLibvirtConn`, `FakeRunner`, `FakeDomain` |
| `stemcell/fakes/` | `FakeStemcell`, `FakeImporter`, `FakeFinder` |
| `vm/fakes/` | `FakeVM`, `FakeCreator`, `FakeFinder` |
| `disk/fakes/` | `FakeDisk`, `FakeCreator`, `FakeFinder` |

All fakes record calls and return configurable responses/errors.

### Unit test coverage targets (≥90%)

| Package | Key scenarios tested |
|---|---|
| `cpi/` | All CPI methods: success path, finder error, creator error, delete not-found |
| `cpi/factory_options.go` | All `Validate()` error branches |
| `vm/factory.go` | Create success, clone error, SetProps error, NIC config error, agent env error, ephemeral disk error |
| `vm/vm.go` | Delete, Reboot, Exists (found/not-found), SetMetadata, DiskIDs |
| `stemcell/factory.go` | ImportFromPath success, upload error, domain define error |
| `stemcell/stemcell.go` | Prepare, Exists, Delete (found/not-found) |
| `disk/factory.go` | Create success, create error |
| `disk/disk.go` | ImagePath, Exists, Delete |
| `driver/libvirt_driver.go` | Each method with fake conn: success + libvirt error + not-found error |
| `driver/domains/vbox.go` | XML output contains expected elements (memory, CPUs, disk path, controller type) |
| `driver/domains/lxc.go` | Same |
| `driver/domains/qemu.go` | Same |
| `driver/retry.go` | Retry on retryable error, give up after N attempts |
| `driver/local_runner.go` | HomeDir (existing test kept) |

### Integration tests (`//go:build integration`)

Live in `*_integration_test.go` files. Require `LIBVIRT_URI` env var.

Scenarios:
- `LibvirtDriver`: connect, define domain from XML, start, lookup, shutdown, destroy, undefine
- Storage vol: create, lookup, delete
- Full stemcell import → VM create → disk attach → VM delete flow (smoke test)

### Makefile targets

```makefile
test-unit:
    go test ./... -coverprofile=coverage.out

test-integration:
    go test -tags integration ./...

coverage:
    go tool cover -html=coverage.out

lint:
    golangci-lint run ./...
```

## File Map Summary

### Deleted
- `driver/exec.go`
- `vm/portdevices/` (entire package)

### New files
- `driver/libvirt_driver.go`
- `driver/libvirt_conn.go`
- `driver/domain_builder.go`
- `driver/domains/vbox.go`
- `driver/domains/lxc.go`
- `driver/domains/qemu.go`
- `driver/fakes/fake_driver.go`
- `driver/fakes/fake_domain_builder.go`
- `driver/fakes/fake_libvirt_conn.go`
- `driver/fakes/fake_runner.go`
- `driver/fakes/fake_domain.go`
- `stemcell/fakes/fake_stemcell.go`
- `stemcell/fakes/fake_importer.go`
- `stemcell/fakes/fake_finder.go`
- `vm/fakes/fake_vm.go`
- `vm/fakes/fake_creator.go`
- `vm/fakes/fake_finder.go`
- `disk/fakes/fake_disk.go`
- `disk/fakes/fake_creator.go`
- `disk/fakes/fake_finder.go`
- `cpi/*_test.go` (all CPI method tests)
- `vm/*_test.go` (vm factory and vm impl tests)
- `stemcell/*_test.go` (stemcell factory and impl tests)
- `disk/*_test.go` (disk factory and impl tests)
- `driver/libvirt_driver_test.go`
- `driver/domains/*_test.go`
- `*_integration_test.go` files per package
- `Makefile`

### Modified
- `driver/interfaces.go` — new `Driver` interface, `LibvirtConn` interface
- `driver/retry.go` — unchanged
- `driver/local_runner.go` — unchanged
- `driver/ssh_runner.go` — unchanged
- `driver/expanding_path_runner.go` — unchanged
- `cpi/factory.go` — use `LibvirtDriver`, select `DomainBuilder` by URI scheme
- `cpi/factory_options.go` — `BackendURI` replaces `BinPath`, remove `StorageController`
- `stemcell/stemcell.go` — libvirt calls
- `stemcell/factory.go` — remove OVF import, add domain define
- `stemcell/interfaces.go` — remove `SnapshotName()`
- `vm/vm.go` — libvirt calls
- `vm/vm_state.go` — libvirt shutdown
- `vm/factory.go` — remove portdevices dependency
- `disk/disk.go` — `ImagePath()` replaces `VMDKPath()`
- `disk/factory.go` — libvirt storage vol creation
- `go.mod` — fix libvirt version, add any new deps
