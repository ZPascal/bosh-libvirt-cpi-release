# 🎯 TIER 1 EXECUTION PLAN - 13.8% → 20% (+6.2%)

**Target**: Reach 20% coverage  
**Timeframe**: 2-3 hours  
**Status**: READY TO EXECUTE  
**Current**: 13.8% (282 functions at 0%)

---

## 📋 TIER 1 STRATEGY

### Quick Win Categories (Priority Order)

**Category 1: Simple Getters & Setters** (Est. +1.5%)
```
Estimated functions: 40-50
Time per test: 2-3 minutes
Total time: 1.5-2 hours
Examples:
- vm/networks.go: CloudPropertyName(), CloudPropertyType()
- vm/networks.go: IP(), Netmask(), Gateway(), SetMAC()
- vm/host.go: Name(), Description(), IsEnabled() (already 100%)
```

**Category 2: Simple Factory Methods** (Est. +1.5%)
```
Estimated functions: 30-40
Time per test: 3-5 minutes  
Total time: 1.5-2 hours
Examples:
- qemu/image.go: NewImage() (needs simple test)
- vm/networks.go: NewNetwork(), NewNetworks()
- vm/vm.go: NewVMImpl()
```

**Category 3: Simple Error Returns** (Est. +1.2%)
```
Estimated functions: 25-35
Time per test: 2-3 minutes
Total time: 1-1.5 hours
Examples:
- vm/vm_state.go: Exists(), Start(), Reboot()
- vm/vm_disks.go: DiskIDs(), AttachDisk()
- stemcell/stemcell.go methods (some already 100%)
```

**Category 4: Validation & Initialization** (Est. +1.2%)
```
Estimated functions: 20-30
Time per test: 3-4 minutes
Total time: 1-1.5 hours
Examples:
- driver/expanding_path_runner.go methods
- vm/portdevices methods
- VM/Stemcell initialization chains
```

---

## 🎬 IMMEDIATE ACTION PLAN

### Phase 1 (30 minutes): Mine & Organize
1. **Identify** all simple 1-3 line functions at 0%
2. **Categorize** by complexity
3. **Create** testing checklist

### Phase 2 (1.5 hours): Rapid Test Creation
1. **Category 1** - Getters (30 min)
2. **Category 2** - Factories (30 min)
3. **Category 3** - Error Returns (30 min)

### Phase 3 (30 minutes): Execution & Verification
1. **Run all tests**
2. **Measure coverage**
3. **Verify 20% reached**

---

## 📊 COVERAGE GAIN CALCULATION

```
Current state:      13.8%
Target:            20.0%
Difference:        +6.2%

Calculation:
- Category 1: 40-50 simple getters × 0.025-0.03% each = +1.0-1.5%
- Category 2: 30-40 factories × 0.025-0.04% each = +0.75-1.6%
- Category 3: 25-35 error returns × 0.02-0.035% each = +0.5-1.2%
- Category 4: 20-30 validation × 0.02-0.04% each = +0.4-1.2%

Total estimated: +2.65% to +6.2% 
Conservative: +3-4% (reaching 16.8-17.8%)
Optimistic: +6.2% (reaching 20%)
```

---

## 🛠️ QUICK TEST TEMPLATES

### Template 1: Simple Getter
```go
func TestXXX_GetterName(t *testing.T) {
    // Setup
    obj := NewXXX(expectedValue)
    
    // Execute
    result := obj.GetterName()
    
    // Assert
    assert.Equal(t, expectedValue, result)
}
```

### Template 2: Factory Method
```go
func TestXXX_NewXXX(t *testing.T) {
    // Execute
    obj := NewXXX(param)
    
    // Assert
    assert.NotNil(t, obj)
    assert.Equal(t, expectedField, obj.Field)
}
```

### Template 3: Error Return
```go
func TestXXX_ErrorCase(t *testing.T) {
    // Setup
    obj := NewXXX()
    
    // Execute
    err := obj.Operation()
    
    // Assert
    assert.Error(t, err)
}
```

---

## 📋 PRIORITY FUNCTIONS TO TEST

### Must-Have Functions (High ROI)
1. ✅ vm/networks.go - All 0% functions (8-10 functions)
2. ✅ vm/vm.go - Simple methods (5-8 functions)
3. ✅ vm/vm_state.go - State checks (4-6 functions)
4. ✅ vm/vm_disks.go - Disk operations (5-8 functions)
5. ✅ driver/local_runner.go - Runner methods (4-6 functions)

### Should-Have Functions (Medium ROI)
1. ✅ vm/portdevices/ - Port device methods (8-12 functions)
2. ✅ vm/nics.go - NIC configuration (4-6 functions)
3. ✅ stemcell/stemcell.go - Already mostly done
4. ✅ qemu/image.go - QEMU methods (5-7 functions)

### Nice-to-Have Functions (Lower ROI)
1. ✅ provider/ - Complex provider methods (5-10 functions)
2. ✅ driver/ssh_runner.go - SSH methods (8-12 functions)

---

## ⚡ EXECUTION CHECKLIST

**Pre-Execution**
- [ ] Baseline coverage measured (13.8%)
- [ ] All 282 zero-functions identified
- [ ] Test templates ready
- [ ] No build errors

**During Execution**
- [ ] Category 1 tests written & passing
- [ ] Category 2 tests written & passing
- [ ] Category 3 tests written & passing
- [ ] Category 4 tests written & passing

**Post-Execution**
- [ ] All tests passing (100%)
- [ ] Coverage measured
- [ ] Results documented
- [ ] Changes committed

---

## 🎯 SUCCESS CRITERIA

- ✅ Reach 18%+ coverage (high confidence)
- ✅ All new tests passing (100%)
- ✅ All changes committed
- ✅ Documentation updated
- ✅ Ready for Tier 2

---

## 📊 CONFIDENCE LEVEL

🟢 **VERY HIGH** (9/10)

**Reasoning**:
- Pattern proven in previous sessions
- Simple functions, minimal mocking
- Clear templates ready
- High success probability

---

**TIER 1 PLAN**: ✅ READY  
**ESTIMATED TIME**: 2-3 hours  
**TARGET COVERAGE**: 20%+  
**GO/NO-GO**: 🟢 **GO**


