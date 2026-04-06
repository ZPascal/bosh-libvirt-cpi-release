# 🎯 MASTER EXECUTION CHECKLIST - 80% Coverage Project
**Project**: bosh-libvirt-cpi  
**Goal**: 80% coverage per package, 50%+ overall  
**Status**: READY FOR EXECUTION ✅  
**Last Updated**: April 4, 2026
---
## 📋 PRE-EXECUTION PHASE (COMPLETED ✅)
### Analysis & Planning
- [x] Measure baseline coverage (12.4%)
- [x] Identify coverage gaps per package
- [x] Create detailed implementation plan
- [x] Analyze existing test infrastructure (141 files)
- [x] Identify quick-win opportunities
### Strategy Development
- [x] Evaluate Ginkgo vs Go unit test effectiveness
- [x] Discover optimal pattern (Go unit tests)
- [x] Create implementation templates
- [x] Document lessons learned
### Deliverables
- [x] 8 planning documents created
- [x] Pattern example working (NewConfigFromPath = 100%)
- [x] Clear next steps documented
---
## 🚀 EXECUTION PHASE (STARTING SESSION 4)
### Session 4: Main Package (3-4 hours)
**Target**: 42.9% → 60%+
- [ ] Add Go unit test for basicDeps()
- [ ] Add Go unit test for Config.Validate()
- [ ] Add Go unit test for Config defaults
- [ ] Add Go unit test for error handling
- [ ] Add Go unit test for connection logic
- [ ] Add 5-10 more edge case tests
- [ ] Run: `go test -v ./main -coverprofile=main.out`
- [ ] Measure: `go tool cover -func=main.out | tail -1`
- [ ] Document results
**Success Criteria**: main coverage ≥ 60%
### Session 5: Disk Package (2-3 hours)
**Target**: 90% → 100%
- [ ] Identify missing 10% in disk functions
- [ ] Add tests for Create() error paths
- [ ] Add tests for edge cases
- [ ] Run coverage measurement
- [ ] Document results
**Success Criteria**: disk coverage = 100%
### Session 6: CPI Package (4-6 hours)
**Target**: 22.9% → 60%+
- [ ] Add tests for Disks wrapper methods
- [ ] Add tests for VMs wrapper methods
- [ ] Add tests for Stemcells wrapper methods
- [ ] Add tests for Factory methods
- [ ] Add tests for error scenarios
- [ ] Run coverage measurement
- [ ] Document results
**Success Criteria**: cpi coverage ≥ 60%
### Session 7: QEMU Package (3-4 hours)
**Target**: 9.4% → 50%
- [ ] Add tests for Image.Create()
- [ ] Add tests for Image.Convert()
- [ ] Add tests for Image.Info()
- [ ] Add tests for error handling
- [ ] Run coverage measurement
- [ ] Document results
**Success Criteria**: qemu coverage ≥ 50%
### Session 8: Provider Package (3-4 hours)
**Target**: 7.9% → 50%
- [ ] Add tests for LibvirtProvider initialization
- [ ] Add tests for VM operations
- [ ] Add tests for network operations
- [ ] Add tests for error handling
- [ ] Run coverage measurement
- [ ] Document results
**Success Criteria**: provider coverage ≥ 50%
### Session 9-10: VM & Driver Packages (6-8 hours)
**Target**: 4.7% → 80%, 15.9% → 80%
- [ ] Add VM lifecycle tests
- [ ] Add VM disk management tests
- [ ] Add Driver SSH runner tests
- [ ] Add Driver retry logic tests
- [ ] Run coverage measurements
- [ ] Document results
**Success Criteria**: vm ≥ 80%, driver ≥ 80%
### Session 11: Final Validation (2-3 hours)
**Target**: Verify all packages ≥ 80%
- [ ] Run full suite coverage measurement
- [ ] Verify each package meets target
- [ ] Run all tests to confirm pass
- [ ] Generate final report
- [ ] Document completion
**Success Criteria**: All packages ≥ 80%, project ≥ 50%
---
## 📊 COVERAGE TARGETS
| Package | Current | Target | Status |
|---------|---------|--------|--------|
| disk | 90.0% | 80% | ✅ Almost done |
| main | 42.9% | 80% | ⏳ Session 4-5 |
| cpi | 22.9% | 80% | ⏳ Session 6 |
| stemcell | 15.8% | 80% | ⏳ Session 7 |
| qemu | 9.4% | 80% | ⏳ Session 7 |
| provider | 7.9% | 80% | ⏳ Session 8 |
| driver | 15.9% | 80% | ⏳ Session 9 |
| vm | 4.7% | 80% | ⏳ Session 10 |
| **TOTAL** | **12.4%** | **50%+** | ⏳ Session 11 |
---
## 🛠️ TOOLS & COMMANDS
### Measure Coverage
```bash
cd src/bosh-libvirt-cpi
# Overall coverage
go test -v ./... -coverprofile=coverage.out -timeout=60s
go tool cover -func=coverage.out | tail -1
# Package-specific coverage
go test -v ./main -coverprofile=main.out
go tool cover -func=main.out | tail -1
# Find uncovered functions
go tool cover -func=main.out | grep "0.0%"
# Generate HTML report
go tool cover -html=main.out -o report.html
```
### Run Tests
```bash
# Specific package
go test -v ./main -timeout=30s
# With coverage
go test -v ./main -coverprofile=main.out -timeout=30s
# Specific test
go test -v ./main -run TestNewConfigFromPath_Success
```
---
## 📝 TEMPLATE FOR EACH SESSION
### Pre-Session
- [ ] Read: SESSION_3_LESSONS_LEARNED.md
- [ ] Review: config_real_test.go (pattern)
- [ ] Plan: Which 10-15 tests to add
### During Session
- [ ] Create new tests following pattern
- [ ] Run: `go test -v ./PACKAGE -coverprofile=test.out`
- [ ] Measure: `go tool cover -func=test.out | tail -1`
- [ ] Document: What tests added, result
### Post-Session
- [ ] Update: Main README with progress
- [ ] Commit: New tests to repo
- [ ] Plan: Next session package
---
## 🎯 PATTERN TO FOLLOW
```go
// File: PACKAGE/NEW_FEATURE_test.go
package PACKAGE_test
import (
    "testing"
    "github.com/stretchr/testify/assert"
    PACKAGE "bosh-libvirt-cpi/PACKAGE"
)
// TestFunctionName_Success tests happy path
func TestFunctionName_Success(t *testing.T) {
    // 1. SETUP: Create test data
    input := "test-value"
    // 2. EXECUTE: Call production function
    result, err := PACKAGE.FunctionName(input)
    // 3. ASSERT: Verify behavior
    assert.NoError(t, err)
    assert.Equal(t, "expected", result)
}
// TestFunctionName_Error tests error path
func TestFunctionName_Error(t *testing.T) {
    // Setup
    input := ""
    // Execute
    _, err := PACKAGE.FunctionName(input)
    // Assert
    assert.Error(t, err)
}
```
---
## ✅ DAILY STANDUP TEMPLATE
**Date**: _____  
**Package**: _____  
**Session**: _____  
### Completed Today
- [ ] Test 1: ______
- [ ] Test 2: ______
- [ ] Test 3: ______
- [ ] Measurement taken
### Results
- Coverage before: ____%
- Coverage after: ____%
- Tests added: ___
- Tests passing: ___
### Blockers
- [ ] None
- [ ] Issue: _____
### Next Session Plan
- Package: _____
- Goal: ____%
- Estimated tests: ___
---
## 🚀 QUICK START (Session 4)
1. **Read**: SESSION_3_LESSONS_LEARNED.md (5 min)
2. **Review**: main/config_real_test.go (10 min)
3. **Create**: 5 new Go unit tests for main (60 min)
4. **Test**: `go test -v ./main -coverprofile=main.out` (5 min)
5. **Measure**: `go tool cover -func=main.out | tail -1` (1 min)
6. **Document**: Results in session notes (5 min)
**Total Time**: 1.5 hours for first session
---
## 📞 KEY DOCUMENTS TO REFERENCE
| When | Read |
|------|------|
| Starting session | SESSION_3_LESSONS_LEARNED.md |
| Writing tests | main/config_real_test.go |
| Measuring coverage | Coverage commands section above |
| Getting stuck | README_COVERAGE_PLAN.md |
| Full details | IMPLEMENTATION_PLAN_80_COVERAGE.md |
---
## 🎓 SUCCESS METRICS
- [x] Strategy clear and documented
- [x] Pattern proven with examples
- [x] Timeline realistic (50-70 hours)
- [x] Each session has clear goals
- [ ] Session 4: main package 60%+
- [ ] Session 11: All packages 80%+
---
## 🏁 COMPLETION CRITERIA
- [x] Planning complete
- [ ] Session 4-5: 15-20% total coverage
- [ ] Session 6-8: 30-40% total coverage
- [ ] Session 9-11: 50%+ total coverage
- [ ] All packages: 80%+ coverage
- [ ] All tests passing
- [ ] Full project documented
---
## 📌 IMPORTANT REMINDERS
1. **Use Go unit tests** - Not Ginkgo for coverage
2. **Follow the pattern** - Setup → Execute → Assert
3. **Measure after each session** - Track progress
4. **Keep tests simple** - Focus on coverage, not complexity
5. **Commit regularly** - Push changes to repo
6. **Document learnings** - Update session notes
---
## 🎯 FINAL VISION
```
Today (Apr 4):    Analysis complete ✅ (12.4% coverage)
Week 1 (Sessions 4-5):    Main & Disk done (15-20% coverage)
Week 2 (Sessions 6-8):    CPI & Others (30-40% coverage)
Week 3 (Sessions 9-11):   Complete (50%+ coverage, 80%+ per package)
```
---
**Status**: READY FOR EXECUTION ✅  
**Confidence**: HIGH ✅  
**Go**: YES ✅
🚀 **Let's reach 80% coverage!**
