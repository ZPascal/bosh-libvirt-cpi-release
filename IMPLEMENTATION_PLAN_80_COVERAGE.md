# 80% Coverage Achievement Action Plan - bosh-libvirt-cpi

## Executive Summary
This document outlines a comprehensive, achievable plan to reach 80% test coverage in each package folder of the bosh-libvirt-cpi project. Current baseline: 11.7% total coverage, with disk package already at 97.9%.

## Current State Analysis

### Packages Achieving/Approaching Target
- ✅ **disk** (97.9%): Needs final 2.1% for 100%
- ⚠️ **stemcell** (40.0%): Halfway to target, solid foundation

### Packages Requiring Major Work
- **driver** (15.9%): Needs +64.1%
- **provider** (13.5%): Needs +66.5%
- **vm** (13.1%): Needs +66.9%
- **qemu** (28.6%): Needs +51.4%
- **cpi** (28.1%): Needs +51.9%
- **main** (33.3%): Needs +46.7%

## Prioritized Implementation Strategy

### TIER 1: Quick Wins (Week 1) - Packages ≥25% Coverage

#### 1.1 Disk Package: 97.9% → 100% (PRIORITY: HIGHEST)
**Effort**: 2-3 hours
**Expected Gain**: +2.1% (nearly done)

**Missing Coverage Areas**:
- Disk.Create error paths (edge case with invalid size)
- Factory.Create with low disk space scenarios
- Concurrent access patterns

**Test additions needed** (~5 tests):
- `TestDisk_Create_InvalidSize`
- `TestDisk_Create_FullDisk`
- `TestFactory_Create_DiskSpace`
- `TestDisk_Concurrent_Delete`
- `TestDisk_Path_Normalization`

**File**: `disk/disk_edge_cases_test.go`

---

#### 1.2 Stemcell Package: 40% → 65% (PRIORITY: HIGH)
**Effort**: 6-8 hours
**Expected Gain**: +25%

**Missing Coverage Areas**:
- Factory.ImportFromPath (main entry point)
- Stemcell lifecycle (Create, Delete, Exists)
- Snapshot operations
- Error scenarios (invalid OVF, corrupted files)

**Test additions needed** (~20 tests):
- Factory tests:
  - `TestFactory_ImportFromPath_ValidOVF`
  - `TestFactory_ImportFromPath_InvalidOVF`
  - `TestFactory_ImportFromPath_Network`
  - `TestFactory_ImportFromPath_Permissions`
  
- Stemcell tests:
  - `TestStemcell_Create_Success`
  - `TestStemcell_Delete_Success`
  - `TestStemcell_Exists_True/False`
  - `TestStemcell_Snapshot_Create`
  - `TestStemcell_ConvertDiskFormat`

**File**: `stemcell/factory_integration_test.go` and `stemcell/stemcell_lifecycle_test.go`

---

#### 1.3 Main Package: 33.3% → 60% (PRIORITY: HIGH)
**Effort**: 4-5 hours
**Expected Gain**: +26.7%

**Missing Coverage Areas**:
- Main.basicDeps (dependency injection)
- Config parsing edge cases
- CPI plugin initialization
- Error handling in main entry

**Test additions needed** (~12 tests):
- `TestMain_BasicDeps_Complete`
- `TestConfig_Parse_Valid`
- `TestConfig_Parse_Invalid`
- `TestCPI_Initialize_Success`
- `TestCPI_Initialize_Failure`

**File**: `main/main_integration_test.go`

---

### TIER 2: Foundation Building (Week 2-3) - Packages 15-25% Coverage

#### 2.1 CPI Package: 28.1% → 75% (PRIORITY: HIGH)
**Effort**: 10-12 hours
**Expected Gain**: +46.9%

**Missing Coverage Areas**:
- High-level methods: CreateDisk, DeleteDisk, AttachDisk, DetachDisk
- VM operations: CreateVM, DeleteVM, RebootVM
- Stemcell operations: CreateStemcell, DeleteStemcell
- Error propagation and wrapping

**Test additions needed** (~25 tests in Ginkgo format):
```ginkgo
Describe("Disks operations", func() {
  Context("CreateDisk", func() {
    It("creates disk successfully")
    It("handles creator error")
    It("validates disk size")
  })
  Context("DeleteDisk", func() {
    It("deletes existing disk")
    It("handles not found error")
  })
  ...
})
```

**File**: `cpi/disk_operations_test.go`, `cpi/vm_operations_test.go`, `cpi/stemcell_operations_test.go`

---

#### 2.2 QEMU Package: 28.6% → 70% (PRIORITY: MEDIUM)
**Effort**: 8-10 hours
**Expected Gain**: +41.4%

**Missing Coverage Areas**:
- Image.Create (all formats)
- Image.Convert (format conversions)
- Image.Info (parsing output)
- Image.Resize (size validation)
- Error scenarios (qemu-img not found)

**Test additions needed** (~15 tests):
- `TestImage_Create_QCOW2`
- `TestImage_Create_InvalidSize`
- `TestImage_Convert_RAW_to_QCOW2`
- `TestImage_Info_Parse`
- `TestImage_Resize_Success`
- `TestImage_Resize_TooSmall`

**File**: `qemu/image_operations_test.go`

---

#### 2.3 Provider Package: 13.5% → 65% (PRIORITY: MEDIUM-HIGH)
**Effort**: 12-14 hours
**Expected Gain**: +51.5%

**Missing Coverage Areas**:
- LibvirtProvider initialization
- Domain XML creation
- Network operations
- VM lifecycle methods (Start, Stop, Reboot)
- Error handling

**Test additions needed** (~30 tests):
- Provider factory
- Connection URI generation
- Domain XML generation
- Network creation/deletion
- VM operations
- Snapshot operations

**File**: `provider/libvirt_provider_operations_test.go`

---

### TIER 3: Completion (Week 4-5) - Packages <15% Coverage

#### 3.1 Driver Package: 15.9% → 85% (PRIORITY: HIGH - Complex)
**Effort**: 14-16 hours
**Expected Gain**: +69.1%

**Missing Coverage Areas**:
- SSH Runner: Execute, Upload, Put, Get methods
- Local Runner: All methods
- Expanding Path Runner: Path expansion logic
- Retry mechanism with exponential backoff
- Error handling and timeouts

**Test additions needed** (~40 tests):
- SSH Runner tests (12 tests)
- Local Runner tests (10 tests)
- Path expansion tests (5 tests)
- Retry logic tests (13 tests)

**File**: `driver/ssh_runner_comprehensive_test.go`, `driver/local_runner_test.go`, `driver/retry_comprehensive_test.go`

---

#### 3.2 VM Package: 13.1% → 80% (PRIORITY: HIGH - Complex)
**Effort**: 16-18 hours
**Expected Gain**: +66.9%

**Missing Coverage Areas**:
- VM lifecycle: Start, Stop, Reboot, Halt, Exists, State
- NIC configuration
- Disk attachment/detachment
- Metadata management
- Port device allocation

**Test additions needed** (~50 tests):
- VM lifecycle tests (20 tests)
- NIC configuration tests (10 tests)
- Disk management tests (15 tests)
- State machine tests (5 tests)

**File**: `vm/vm_lifecycle_comprehensive_test.go`, `vm/vm_disks_test.go`, `vm/vm_nics_test.go`

---

## Implementation Approach

### Pattern: Mock-Based Unit Testing

For packages with external dependencies (SSH, libvirt), use:

```go
// Example pattern for all new tests
type MockExecutor struct {
  ExecuteFn func(cmd string, args ...string) (string, int, error)
}

func (m *MockExecutor) Execute(cmd string, args ...string) (string, int, error) {
  return m.ExecuteFn(cmd, args...)
}

func TestMyFunction_Success(t *testing.T) {
  mock := &MockExecutor{
    ExecuteFn: func(cmd string, args ...string) (string, int, error) {
      return "output", 0, nil
    },
  }
  
  result := myFunction(mock)
  assert.NoError(t, result)
}
```

### Testing Framework
- **Standard**: Use `testing.T` with `testify/assert` for simplicity
- **Ginkgo**: For already-Ginkgo-based packages (CPI, integration)
- **Mocks**: testify/mock or simple function mocks
- **Fixtures**: Use `t.TempDir()` for file operations

### Coverage Measurement
```bash
# Per-package measurement
go test -v ./stemcell -coverprofile=stemcell.out
go tool cover -func=stemcell.out | grep "total"

# Total measurement
go test -v ./... -coverprofile=total.out -timeout=60s
go tool cover -func=total.out | tail -1

# HTML report
go tool cover -html=total.out -o report.html
```

---

## Risk Mitigation

### High-Risk Areas
1. **SSH/Remote Operations**: Test with mocks; live SSH testing is fragile
2. **Libvirt Integration**: Use Docker container for isolated testing
3. **File System Operations**: Use `t.TempDir()` and cleanup
4. **Concurrent Operations**: Use sync.Mutex mocks for concurrency

### Contingency Plans
- If external tool (qemu-img, virsh) not found: Mock at command level
- If libvirt not available: Create minimal test doubles
- If performance issues: Use table-driven tests for bulk scenarios

---

## Success Metrics

### Immediate Goals (This Week)
- [ ] disk: 97.9% → 100% (+2.1%)
- [ ] stemcell: 40% → 65% (+25%)
- [ ] main: 33.3% → 60% (+26.7%)
- **Subtotal Improvement**: +53.8%

### Mid-Term Goals (2 Weeks)
- [ ] cpi: 28.1% → 75% (+46.9%)
- [ ] qemu: 28.6% → 70% (+41.4%)
- [ ] provider: 13.5% → 65% (+51.5%)
- **Total Improvement**: +139.8%

### Final Goals (4 Weeks)
- [ ] driver: 15.9% → 85% (+69.1%)
- [ ] vm: 13.1% → 80% (+66.9%)
- **Project Total**: 11.7% → 50%+ target

---

## Estimated Timeline

| Phase | Packages | Hours | Target Coverage | Target Date |
|-------|----------|-------|-----------------|-------------|
| **Phase 1** | disk, stemcell, main | 12-16 | 60-70% avg | Week 1 |
| **Phase 2** | cpi, qemu, provider | 30-36 | 70%+ avg | Week 3 |
| **Phase 3** | driver, vm | 30-34 | 80%+ avg | Week 5 |
| **Validation** | All packages | 4-8 | 80%+ each | Week 6 |

**Total Estimated Effort**: 76-94 hours (~2-2.5 weeks intensive)

---

## Next Steps

1. **Immediate Action**: Create `disk/disk_edge_cases_test.go` (2 hours)
2. **Week 1**: Complete all Tier 1 packages (12-16 hours)
3. **Weekly Reviews**: Measure and adjust based on actual progress
4. **Parallel Work**: Tier 2 packages can start after first 2 Tier 1 packages done
5. **Final Sprint**: Tier 3 packages in weeks 4-5

---

## Deliverables

- ✅ Comprehensive test coverage reports (weekly)
- ✅ New test files for each package (as per File list above)
- ✅ Coverage trend analysis
- ✅ Documentation of test patterns for future developers
- ✅ Final coverage report with 80%+ achievement validation

---

## Sign-Off
**Plan Created**: April 4, 2026  
**Status**: READY FOR IMPLEMENTATION  
**Confidence Level**: HIGH (based on existing test infrastructure and clear coverage gaps)

