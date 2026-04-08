# ⚡ TIER 1 FAST EXECUTION - REVISED STRATEGY

**Status**: READY TO EXECUTE  
**Current Coverage**: 13.8%  
**Target**: 20% (+6.2%)  
**Approach**: Focus on Partially-Covered Functions (50-99%)

---

## 🎯 NEW STRATEGY: "Push to 100%" Approach

Instead of testing 0% functions (require complex mocking), we **enhance partially-tested functions to 100%** for faster gains.

### Functions at 50-99% Coverage (Quick Wins)

```
disk/factory.go:43 - Create() @ 76.9%     → Target 100%
driver/exec.go:41 - ExecuteComplex() @ 84.0% → Target 100%  
driver/retry.go:52 - AttemptsWithDelay() @ 88.9% → Target 100%
provider/libvirt_provider.go:32 - NewLibvirtProvider() @ 71.4% → Target 100%
```

**Opportunity**: These functions need only 1-3 additional test cases to reach 100%!

---

## 📊 COVERAGE MATH (REVISED)

```
Current: 13.8% (42 functions at 100%)
Target: 20.0%

If we push 15-20 partially-covered functions from 50-99% to 100%:
- disk/factory Create() from 76.9% → 100% = +0.23%
- ExecuteComplex from 84.0% → 100% = +0.16%
- AttemptsWithDelay from 88.9% → 100% = +0.11%
- Plus 12-17 more similar functions...

Total potential: +3-4% (reaching 16.8-17.8%)
Then with additional simple functions: +2-3% more
Conservative total: 16.8% → 20%+ achievable
```

---

## 🚀 IMMEDIATE ACTION: 15-Min Quick Audit

**Find all functions at 50-99% coverage**:
```bash
go tool cover -func=coverage.out | awk -F'%' '$1 ~ /[5-9][0-9]\.[0-9]/'
```

**These are our priority targets** - low hanging fruit!

---

## 💡 ALTERNATIVE TIER 1 STRATEGY

### **Option A**: Enhance Existing Tests (Recommended)
- Time: 1-2 hours
- Approach: Add 1-2 test cases to each partial-coverage function
- ROI: High (proven functions)
- Confidence: Very High

### **Option B**: Leverage Ginkgo Tests (Already Available!)
- Time: 30 minutes
- Approach: Some existing Ginkgo tests might not be counted
- ROI: Potential 2-3% quick gain
- Confidence: Medium

### **Option C**: Add Driver/Disk Tests  
- Time: 1 hour
- Approach: Test disk operations and driver functions
- ROI: Medium (complex orchestration)
- Confidence: Medium

---

## ✅ RECOMMENDED IMMEDIATE PLAN

**Phase 1 (10 min)**: Identify all 50-99% functions
**Phase 2 (30 min)**: Add 1 targeted test per function
**Phase 3 (30 min)**: Run & measure coverage
**Total**: ~1 hour for +2-3% gain

---

## 📋 NEXT STEPS

1. ✅ **List all partial-coverage functions** (50-99%)
2. ✅ **Prioritize by ease of testing**
3. ✅ **Create 1-2 additional test cases per function**
4. ✅ **Push to 100% coverage**
5. ✅ **Measure results**

---

**This approach is more pragmatic and achieves faster results!**


