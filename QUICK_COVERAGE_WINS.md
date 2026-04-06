# Quick Coverage Wins - Identified Opportunities
## Current Challenge
The existing Ginkgo tests are mostly placeholder assertions (e.g., `Expect(size).To(BeNumerically(">", 0))`) that don't test actual code paths.
Example from `cpi/cpi_core_methods_test.go`:
```go
It("creates disk", func() {
    size := 10240
    Expect(size).To(BeNumerically(">", 0))  // ← Just tests the variable, not the CPI function!
})
```
## Fastest Path to 80%
Instead of creating new test files (error-prone), we should:
1. **Enhance existing Ginkgo tests** with actual implementation tests
2. **Use existing test helper infrastructure** (testhelpers/mocks already available)
3. **Focus on 3-4 key packages** that give biggest coverage gains
## Strategic Opportunities
### 1. disk: 90% → 100% (2 hours)
- Already has good tests
- Final 10% likely in Create() edge cases  
- Easy quick win
### 2. main: 42.9% → 70% (3 hours)
- Test main() entry point with mock config
- Test basicDeps() function
- Already structured, just needs real tests
### 3. cpi: 22.9% → 60% (4-6 hours)
- Tests exist but are empty placeholders
- Fill them with real assertions and mocks
- Biggest impact on integration layer
### 4. stemcell: 15.8% → 50% (3 hours)
- Many test files exist with structure
- Just need to implement the actual test logic
## Total Estimated Effort
**14-17 hours for 10-15% total coverage improvement**
This is 3-4x faster than creating tests from scratch because:
- Test scaffolding already exists
- Ginkgo runner is ready
- Test patterns are established
- Just need to fill in the assertions
## Recommended Next Step
Pick one package (start with `main` - easiest), enhance 5-10 of its test cases with real assertions, measure the coverage gain, then replicate pattern for other packages.
