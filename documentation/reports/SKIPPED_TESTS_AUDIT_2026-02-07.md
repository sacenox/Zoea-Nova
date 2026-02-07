# Skipped Tests Audit

**Date:** 2026-02-07  
**Auditor:** Agent (Automated)  
**Total Skipped Tests:** 18

---

## Executive Summary

Found 18 skipped tests across the codebase. Classification:

- **6 tests**: Obsolete (safe to remove)
- **4 tests**: Environmental (skip is correct)
- **3 tests**: Documentation (skip is intentional)
- **2 tests**: Conflicting behavior (needs resolution)
- **2 tests**: Test fixture issues (should be fixed)
- **1 test**: Known hang bug (needs investigation)

**Recommendations:**
- Remove 6 obsolete tests (cleanup)
- Fix 2 TUI integration tests (test environment)
- Resolve 2 activity test conflicts (behavior clarification)
- Investigate 1 hanging test (state machine bug)

---

## 1. Obsolete Tests (Remove)

### 1.1 TestMysisContextMemoryLimit
**Location:** `internal/core/mysis_test.go:348`  
**Skip Reason:** "Obsolete: Tests old compaction strategy. Replaced by loop-based composition (TestLoopContextSlice)."  
**Context:** Tests deprecated context compression strategy  
**Recommendation:** **REMOVE**  
**Effort:** 5 minutes  
**Justification:** Explicitly marked obsolete. Replacement test exists.

### 1.2 TestMysisContextMemoryWithRecentSystemPrompt
**Location:** `internal/core/mysis_test.go:387`  
**Skip Reason:** "Obsolete: Tests old compaction strategy. Replaced by loop-based composition (TestContextPromptSourcePriority)."  
**Context:** Tests deprecated system prompt handling  
**Recommendation:** **REMOVE**  
**Effort:** 5 minutes  
**Justification:** Explicitly marked obsolete. Replacement test exists.

### 1.3 TestMysisContextCompaction
**Location:** `internal/core/mysis_test.go:491`  
**Skip Reason:** "Obsolete: Tests old snapshot compaction. Loop composition doesn't use compaction."  
**Context:** Tests removed snapshot compaction feature  
**Recommendation:** **REMOVE**  
**Effort:** 5 minutes  
**Justification:** Tests feature that no longer exists in codebase.

### 1.4 TestMysisContextCompactionNonSnapshot
**Location:** `internal/core/mysis_test.go:931`  
**Skip Reason:** "Obsolete: Tests old non-snapshot compaction. Loop composition doesn't use compaction."  
**Context:** Tests removed non-snapshot compaction  
**Recommendation:** **REMOVE**  
**Effort:** 5 minutes  
**Justification:** Tests feature that no longer exists in codebase.

### 1.5 TestMemoriesToMessages_WithOrphanedResults
**Location:** `internal/core/agent3_reproduction_test.go:190`  
**Skip Reason:** "This test documents that memoriesToMessages is a pure converter - orphan removal happens before it"  
**Context:** Documents behavior that's now tested elsewhere  
**Recommendation:** **REMOVE** (or convert to comment)  
**Effort:** 10 minutes  
**Justification:** This is documentation, not a functional test. The behavior is covered by other tests. Consider converting to code comments instead.

### 1.6 TestContextCompressionPreservesToolCallPairs
**Location:** `internal/core/orphaned_tool_results_test.go:320`  
**Skip Reason:** "This test documents the DESIRED behavior - implement after fixing the bug"  
**Context:** Written to document a bug that needs fixing  
**Recommendation:** **REMOVE or ENABLE**  
**Effort:** 30 minutes (if enabling)  
**Justification:** Check if the bug mentioned has been fixed. If yes, enable test. If no, track in KNOWN_ISSUES.md and remove test.

---

## 2. Environmental Dependencies (Keep Skipped)

### 2.1 TestZenNanoWithProductionConfig
**Location:** `internal/integration/zen_nano_failure_test.go:27`  
**Skip Reason:** "Skipping test - no credentials file (expected in CI)"  
**Context:** Integration test requiring production credentials  
**Recommendation:** **KEEP SKIP**  
**Effort:** N/A  
**Justification:** Correct behavior. Test requires credentials that aren't available in CI. Skip logic is appropriate.

### 2.2 TestStopWithRealOllamaProvider
**Location:** `internal/core/mysis_real_provider_test.go:26`  
**Skip Reason:** "Ollama not available"  
**Context:** Tests real Ollama HTTP provider timing  
**Recommendation:** **KEEP SKIP**  
**Effort:** N/A  
**Justification:** Correct behavior. Test requires local Ollama server. Skip logic is appropriate.

### 2.3 TestStopDuringRealLLMCall
**Location:** `internal/core/mysis_real_provider_test.go:86`  
**Skip Reason:** "Ollama not available"  
**Context:** Tests stopping during real LLM HTTP call  
**Recommendation:** **KEEP SKIP**  
**Effort:** N/A  
**Justification:** Correct behavior. Test requires local Ollama server. Skip logic is appropriate.

### 2.4 TestOpenCodeProvider_HandlesMinimalRequest
**Location:** `internal/provider/opencode_integration_test.go:90`  
**Skip Reason:** "Requires mock HTTP server - implement when debugging OpenCode issues"  
**Context:** Placeholder for future debugging test  
**Recommendation:** **KEEP SKIP** (or remove)  
**Effort:** 2 hours (if implementing)  
**Justification:** This is a TODO test. Either implement it when needed or remove as dead code. Current skip is acceptable.

---

## 3. Documentation/Stress Tests (Keep Skipped)

### 3.1 TestStateTransition_Running_To_Stopped_StressTest
**Location:** `internal/core/state_machine_test.go:155`  
**Skip Reason:** "skipping stress test in short mode"  
**Context:** 100-iteration race condition stress test  
**Recommendation:** **KEEP SKIP**  
**Effort:** N/A  
**Justification:** Correct use of `testing.Short()`. Test runs in full mode (`go test` without `-short`). This is proper Go convention.

### 3.2 TestStateTransition_RapidStartStop
**Location:** `internal/core/state_machine_test.go:465`  
**Skip Reason:** "skipping rapid cycle test in short mode"  
**Context:** 10-cycle rapid start/stop test  
**Recommendation:** **KEEP SKIP**  
**Effort:** N/A  
**Justification:** Correct use of `testing.Short()`. Test runs in full mode. This is proper Go convention.

### 3.3 TestCharacterRenderingMatrix
**Location:** `internal/tui/unicode_test.go:382`  
**Skip Reason:** "Skipping visual reference chart in short mode"  
**Context:** Visual Unicode character reference  
**Recommendation:** **KEEP SKIP**  
**Effort:** N/A  
**Justification:** Correct use of `testing.Short()`. This is documentation/reference, not a functional test. Skip is appropriate.

---

## 4. Test Conflicts (Needs Resolution)

### 4.1 TestMysis_ShouldNudge_Traveling_InFuture
**Location:** `internal/core/activity_test.go:26`  
**Skip Reason:** "Conflicts with TestMysis_ActivityStateTransitions/traveling_future_no_nudge - needs review"  
**Context:** Tests traveling state nudge behavior  
**Recommendation:** **FIX - Resolve Conflict**  
**Effort:** 1 hour  
**Analysis:**
- This test expects `shouldNudge()=true` while traveling (future activityUntil)
- `TestMysis_ActivityStateTransitions/traveling_future_no_nudge` expects `shouldNudge()=false`
- These are contradictory requirements

**Action Required:**
1. Review actual `shouldNudge()` implementation
2. Determine correct behavior: should myses nudge while traveling or not?
3. Remove incorrect test and update documentation
4. Ensure activity state machine documentation reflects decision

### 4.2 TestMysis_ShouldNudge_Cooldown_Active
**Location:** `internal/core/activity_test.go:72`  
**Skip Reason:** "Conflicts with TestMysis_ActivityStateTransitions/cooldown_future_no_nudge - needs review"  
**Context:** Tests cooldown state nudge behavior  
**Recommendation:** **FIX - Resolve Conflict**  
**Effort:** 1 hour  
**Analysis:**
- This test expects `shouldNudge()=true` during active cooldown
- `TestMysis_ActivityStateTransitions/cooldown_future_no_nudge` expects `shouldNudge()=false`
- These are contradictory requirements

**Action Required:**
1. Review actual `shouldNudge()` implementation
2. Determine correct behavior: should myses nudge during cooldown or not?
3. Remove incorrect test and update documentation
4. Ensure activity state machine documentation reflects decision

---

## 5. Test Environment Issues (Should Fix)

### 5.1 TestIntegration_NewMysisInput
**Location:** `internal/tui/integration_test.go:324`  
**Skip Reason:** "Test environment setup issue - provider config not available"  
**Context:** Integration test for TUI 'n' key (new mysis)  
**Recommendation:** **FIX TEST ENVIRONMENT**  
**Effort:** 2 hours  
**Analysis:**
- Test fails with: "Error: provider config not found: opencode_zen"
- This is NOT a production bug (skip comment confirms)
- Test setup doesn't properly initialize provider registry

**Action Required:**
1. Update `setupTestModel()` to register test providers
2. Add mock provider config to test fixtures
3. Verify test passes after fix
4. Remove skip statement

### 5.2 TestIntegration_CreateAndStartMysis
**Location:** `internal/tui/integration_test.go:773`  
**Skip Reason:** "Test environment setup issue - provider config not available"  
**Context:** Integration test for full mysis lifecycle  
**Recommendation:** **FIX TEST ENVIRONMENT**  
**Effort:** 2 hours (same fix as 5.1)  
**Analysis:**
- Same root cause as TestIntegration_NewMysisInput
- Test setup doesn't properly initialize provider registry

**Action Required:**
1. Apply same fix as 5.1 (update setupTestModel)
2. Both tests likely fixed by single change
3. Remove skip statements after verification

---

## 6. Known Bug (Needs Investigation)

### 6.1 TestStateTransition_Running_To_Idle
**Location:** `internal/core/state_machine_test.go:260`  
**Skip Reason:** "Hangs during cleanup - goroutine not exiting after idle transition. Needs investigation."  
**Context:** Tests running → idle transition (nudge breaker)  
**Recommendation:** **INVESTIGATE AND FIX**  
**Effort:** 4-8 hours  
**Priority:** HIGH  

**Analysis:**
- Test hangs during cleanup, indicating goroutine leak
- Likely bug in idle transition logic (goroutine not receiving stop signal)
- This could indicate production bug (goroutines not cleaning up after idle)

**Action Required:**
1. Run test with `-race` detector and extended timeout
2. Use goroutine profiling to identify stuck goroutine
3. Review `setIdle()` implementation for cleanup logic
4. Add goroutine leak detection to test
5. Fix underlying bug
6. Re-enable test
7. Add similar test for other transitions to catch regression

**Investigation Steps:**
```bash
# Run with race detector and timeout
go test -v -timeout 30s -race ./internal/core -run TestStateTransition_Running_To_Idle

# Use goroutine profiling
go test -v -cpuprofile=cpu.prof -memprofile=mem.prof ./internal/core -run TestStateTransition_Running_To_Idle
```

---

## Summary Table

| Test | Location | Status | Recommendation | Effort |
|------|----------|--------|----------------|--------|
| TestMysisContextMemoryLimit | mysis_test.go:348 | Obsolete | Remove | 5m |
| TestMysisContextMemoryWithRecentSystemPrompt | mysis_test.go:387 | Obsolete | Remove | 5m |
| TestMysisContextCompaction | mysis_test.go:491 | Obsolete | Remove | 5m |
| TestMysisContextCompactionNonSnapshot | mysis_test.go:931 | Obsolete | Remove | 5m |
| TestMemoriesToMessages_WithOrphanedResults | agent3_reproduction_test.go:190 | Documentation | Remove/Comment | 10m |
| TestContextCompressionPreservesToolCallPairs | orphaned_tool_results_test.go:320 | TODO | Remove or Enable | 30m |
| TestZenNanoWithProductionConfig | zen_nano_failure_test.go:27 | Environmental | Keep | N/A |
| TestStopWithRealOllamaProvider | mysis_real_provider_test.go:26 | Environmental | Keep | N/A |
| TestStopDuringRealLLMCall | mysis_real_provider_test.go:86 | Environmental | Keep | N/A |
| TestOpenCodeProvider_HandlesMinimalRequest | opencode_integration_test.go:90 | TODO | Keep or Remove | 2h |
| TestStateTransition_Running_To_Stopped_StressTest | state_machine_test.go:155 | Stress | Keep | N/A |
| TestStateTransition_RapidStartStop | state_machine_test.go:465 | Stress | Keep | N/A |
| TestCharacterRenderingMatrix | unicode_test.go:382 | Documentation | Keep | N/A |
| TestMysis_ShouldNudge_Traveling_InFuture | activity_test.go:26 | Conflict | Fix | 1h |
| TestMysis_ShouldNudge_Cooldown_Active | activity_test.go:72 | Conflict | Fix | 1h |
| TestIntegration_NewMysisInput | integration_test.go:324 | Test Env | Fix | 2h |
| TestIntegration_CreateAndStartMysis | integration_test.go:773 | Test Env | Fix | 2h |
| TestStateTransition_Running_To_Idle | state_machine_test.go:260 | Bug | Investigate | 4-8h |

---

## Action Plan

### Phase 1: Cleanup (30 minutes)
1. Remove 6 obsolete tests
2. Update test file comments
3. Run `make test` to verify no regressions

### Phase 2: Fix Test Environment (4 hours)
1. Fix `setupTestModel()` in `internal/tui/integration_test.go`
2. Add mock provider registration to test fixtures
3. Enable 2 TUI integration tests
4. Verify tests pass

### Phase 3: Resolve Conflicts (2 hours)
1. Review `shouldNudge()` implementation
2. Determine correct behavior for traveling/cooldown states
3. Remove incorrect tests
4. Update activity state machine documentation

### Phase 4: Investigate Hang (8 hours)
1. Debug `TestStateTransition_Running_To_Idle` hang
2. Use race detector and goroutine profiling
3. Fix goroutine leak in idle transition
4. Add goroutine leak detection
5. Re-enable test

### Total Effort: ~14.5 hours

---

## Notes

### Testing Best Practices Observed

**Good:**
- Proper use of `testing.Short()` for stress tests
- Environmental checks before expensive integration tests
- Clear skip messages with rationale

**Needs Improvement:**
- Some skipped tests should be removed (obsolete code)
- Test conflicts indicate unclear behavior specification
- Test environment setup needs better provider mocking

### Recommendations for Future

1. **Delete obsolete tests immediately** - Don't leave them skipped
2. **Document test conflicts in KNOWN_ISSUES.md** - Track until resolved
3. **Add goroutine leak detection** - Prevent future hangs
4. **Improve test fixtures** - Better provider mocking for integration tests
5. **CI verification** - Ensure `go test -short` passes, `go test` (full) runs periodically

---

## Related Documentation

- `documentation/architecture/MYSIS_STATE_MACHINE.md` - State transitions (needs update after conflict resolution)
- `documentation/current/KNOWN_ISSUES.md` - Track hanging test as known issue
- `documentation/guides/TUI_TESTING.md` - Testing guidelines

---

**End of Audit**
