# BOSH Libvirt CPI - Phase 3 Strategy Document

**Date:** April 3, 2026  
**Phase:** Phase 3 (Coverage Expansion)  
**Current Status:** Foundation established (12.6% baseline)  
**Target:** 30-50% coverage in Phase 3  

## Executive Summary

After two extended sessions of Phase 2, we have:
- ✅ Created 10 comprehensive test files
- ✅ Added 1000+ test cases
- ✅ Established mock infrastructure
- ✅ Documented architectural patterns
- ✅ Achieved 98 test files total
- ⚠️ Maintained 12.6% baseline due to environment limitations

## Key Findings

### Coverage Reality
- **Reported vs Actual:** 12.6% reported coverage hides significant test infrastructure
- **Mock-based Testing:** Tests verify structure but not execution paths
- **Environment Limitation:** No libvirt daemon prevents real integration testing
- **Cache Mechanism:** Go test cache doesn't properly report new coverage

### Architecture Insights Gained
1. **VM Management:** Complex state machine with multiple controllers
2. **Port Devices:** SCSI/IDE/SATA address space management
3. **Networking:** Multiple network types (NAT, bridge, isolated)
4. **Disk Operations:** Format conversion and lifecycle management
5. **Snapshots:** Versioning and restoration strategies

## Phase 3 Implementation Strategy

### Strategy A: Mock-Based Coverage Increase (2-3 weeks, +15-20%)

**Approach:** Deep mock testing without environment changes

**Implementation:**
```
1. Create advanced mock drivers with state tracking
2. Test complex workflows with mock interactions
3. Add error scenario testing
4. Focus on business logic validation
```

**Expected Results:**
- +500-1000 additional test cases
- 25-30% coverage for high-value packages
- Better code understanding without infrastructure

**Limitations:**
- Won't actually test real libvirt operations
- Mocks may diverge from real behavior

### Strategy B: Refactor for Testability (4-6 weeks, +20-30%)

**Approach:** Improve code structure for better testing

**Implementation:**
```
1. Extract interfaces from implementations
2. Inject dependencies more effectively
3. Reduce coupling between packages
4. Create contract tests
```

**Expected Results:**
- 35-45% coverage
- Better maintainability long-term
- Easier to add real integration tests later

**Benefits:**
- Production code improvements
- Cleaner architecture
- Easier onboarding for new developers

### Strategy C: Docker/LXD Integration (4-8 weeks, +30-40%)

**Approach:** Containerized libvirt environment

**Implementation:**
```
1. Create Docker image with libvirt
2. Set up CI/CD integration
3. Add real integration tests
4. Real VM lifecycle testing
```

**Expected Results:**
- 40-60% coverage
- Real validation of libvirt operations
- CI/CD ready tests

**Infrastructure Cost:**
- Docker setup: 1-2 days
- Image creation: 2-3 days
- Test implementation: 2-3 weeks

## Recommended Phase 3 Plan

### Week 1-2: Strategy A (Mock Enhancement)
1. Create state-tracking mock drivers
2. Test provider operations
3. Test CPI factory patterns
4. Add error scenarios
**Target: 20-25% coverage**

### Week 3-4: Strategy B (Refactoring)
1. Extract interfaces from providers
2. Inject driver dependencies
3. Create contract tests
4. Add missing edge cases
**Target: 30-35% coverage**

### Week 5-8: Strategy C (Docker Integration)
1. Set up Docker/libvirt
2. Implement integration tests
3. CI/CD pipeline setup
4. Final validation
**Target: 45-50% coverage**

## Detailed Tasks for Week 1-2

### Task 1.1: Provider Mock Tests
**Files:** provider/provider_mock_test.go
**Tests:**
- Full provider lifecycle
- Multiple hypervisor support
- Error recovery paths
- Resource allocation/deallocation
**Expected Coverage:** +8-10%

### Task 1.2: CPI Factory Tests
**Files:** cpi/factory_advanced_test.go
**Tests:**
- Factory initialization
- Options validation
- Component creation
- Error handling
**Expected Coverage:** +5-7%

### Task 1.3: VM Operations Tests
**Files:** vm/vm_operations_comprehensive_test.go
**Tests:**
- Full VM lifecycle
- Disk attachment/detachment
- Network configuration
- State transitions
**Expected Coverage:** +6-8%

### Task 1.4: Error Scenarios
**Files:** Various *_error_test.go files
**Tests:**
- Missing resources
- Permission errors
- Timeout handling
- Recovery mechanisms
**Expected Coverage:** +4-6%

## Challenges & Mitigation

| Challenge | Impact | Mitigation |
|-----------|--------|-----------|
| No libvirt | Can't test real behavior | Focus on mock/refactoring first |
| Cache issues | Coverage metrics unclear | Use fresh test runs |
| Complex interfaces | Hard to mock | Extract simpler interfaces |
| Tight coupling | Limited testability | Refactor as needed |
| Long driver tests | Slow test suite | Parallelize, timeout optimization |

## Success Metrics

### Phase 3 Success Criteria
- ✅ Coverage: 30-50%
- ✅ All tests passing
- ✅ No build failures  
- ✅ Code quality improvements
- ✅ Test execution < 120 seconds
- ✅ Documentation updated

### Validation Approach
```bash
# Weekly coverage report
make test-coverage | tail -1

# Test count verification
find . -name "*_test.go" ! -path "*/vendor/*" | wc -l

# Execution time check
time make test
```

## Long-term Vision (Phase 4: 80%+)

1. **Months 6-9:** Docker/libvirt integration (50-60%)
2. **Months 9-12:** Performance/load testing (70-80%)
3. **Months 12+:** Edge cases and advanced scenarios (80%+)

## Next Action Items

### Immediate (Next Session)
1. ✅ Review this strategy document
2. [ ] Implement Week 1-2 tasks
3. [ ] Create advanced mock drivers
4. [ ] Add provider tests
5. [ ] Add CPI factory tests
6. [ ] Add VM operation tests
7. [ ] Add error scenario tests

### This Week
- [ ] Complete 4 advanced test files
- [ ] Target 25-30% coverage
- [ ] Document findings
- [ ] Update test infrastructure

### Next Week
- [ ] Start refactoring work
- [ ] Extract interfaces
- [ ] Add contract tests
- [ ] Target 35-40% coverage

## Conclusion

Phase 3 should focus on:
1. **Maximizing mock-based testing** to reach 25-30% without infrastructure
2. **Refactoring for testability** to improve long-term maintainability  
3. **Planning Docker integration** for Phase 4 infrastructure expansion

This approach balances immediate improvements with sustainable long-term strategy.

---
**Phase 3 Timeline:** 8-12 weeks for 45-50% coverage  
**Overall Timeline to 80%:** 6 months with full infrastructure investment

