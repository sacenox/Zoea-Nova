# Cleanup Execution Plan - 2026-02-07

Consolidated execution plan for 10 audit reports covering dead code, tests, documentation, configuration, and coverage gaps.

---

## Summary

- **Total changes:** 127 discrete changes
- **Files affected:** 58 files (18 production, 25 test, 15 documentation)
- **Estimated time:** 47.5 hours (6 full work days)
- **Coverage impact:** 28% → ~75% (+47%)
- **Test count:** Remove 6 obsolete, fix 5 skipped, add 15 new test suites

---

## Priority 1: Critical (Must Fix)

**Goal:** Fix production bugs, broken tests, and user-facing documentation

### P1.1: Production Code Fixes (3 hours)

**From: Hardcoded Config Audit**

- [ ] **Fix API key lookup** (`cmd/zoea/main.go:244`) - 30 min
  - Use provider config name as credential key with fallback to `"opencode_zen"`
  - Enables per-provider API keys for multiple OpenCode Zen configs
  - Backward compatible with existing setup

**From: Dead Code Audit**

- [ ] **Delete unused color constants** (`internal/tui/styles.go`) - 15 min
  - Line 37: `colorBorderHi` (unused)
  - Line 27: `colorWarning` (unused)
  - Line 41: `colorSecondary` (unused)
  - Line 42: `colorAccent` (unused)

- [ ] **Delete stubTick variable** (`internal/mcp/stub.go:70,129`) - 5 min
  - Remove unused variable and comment

- [ ] **Delete old provider constructors** (`internal/provider/*.go`) - 15 min
  - `ollama.go`: Remove `NewOllama()` (unused, superseded by `NewOllamaWithTemp`)
  - `opencode.go`: Remove `NewOpenCode()` (unused, superseded by `NewOpenCodeWithTemp`)

- [ ] **Fix rateBurst parameter** (`internal/provider/factory.go:34`) - 10 min
  - `NewOpenCodeFactory` ignores `rateBurst` parameter, hardcodes `burst=1`
  - Either use the parameter or remove it

**From: Known Issues Consolidation**

- [ ] **Fix broadcast to idle myses** (`internal/core/mysis.go`) - 1 hour
  - Investigate regression: broadcasts don't start idle myses
  - Add test coverage for broadcast → idle → running transition
  - Fix state machine if confirmed bug

- [ ] **Fix idle transition with pending broadcasts** (`internal/core/mysis.go`) - 45 min
  - Myses idle despite queued broadcasts from commander/swarm
  - Modify `ContinuePrompt` logic to check broadcast queue before idling

### P1.2: User-Facing Documentation (1 hour)

**From: Documentation Accuracy Audit**

- [ ] **Fix broken CONTEXT_COMPRESSION.md link** (`README.md:26`) - 2 min
  - Change `documentation/CONTEXT_COMPRESSION.md` → `documentation/architecture/CONTEXT_COMPRESSION.md`

- [ ] **Add missing providers to README** (`README.md:71-75`) - 10 min
  - Add `ollama-qwen`, `ollama-qwen-small`, `zen-nano`
  - Mark defaults clearly

- [ ] **Add vim keys to keyboard shortcuts** (`README.md:79-95`) - 5 min
  - Document `k` (up) and `j` (down) navigation

- [ ] **Add vim keys to help screen** (`internal/tui/help.go:26`) - 5 min
  - Change `{"↑ / ↓", "Scroll / Browse history"}` → `{"↑ / ↓ / k / j", "Navigate / Scroll"}`

- [ ] **Clarify --start-swarm behavior** (`README.md`) - 5 min
  - Add "(excludes errored myses; default: disabled)"

- [ ] **Add recent reports to documentation index** (`documentation/README.md`) - 15 min
  - Add 13 missing entries under "Recent (2026-02-07)"

**From: Known Issues Consolidation**

- [ ] **Update KNOWN_ISSUES.md** (see section 5) - 15 min
  - Add 5 new issues with subcategories
  - Remove 1 completed item (broadcast regression - if fixed)

- [ ] **Update KNOWN_SERVER_ISSUES.md** (see section 5) - 5 min
  - Add OpenCode Zen system-only message crash section

### P1.3: Test Fixes (4 hours)

**From: Skipped Tests Audit**

- [ ] **Fix TUI integration test setup** (`internal/tui/integration_test.go`) - 2 hours
  - Fix `TestIntegration_NewMysisInput` (line 324)
  - Fix `TestIntegration_CreateAndStartMysis` (line 773)
  - Update `setupTestModel()` to register test providers
  - Add mock provider config to test fixtures

- [ ] **Resolve activity test conflicts** (`internal/core/activity_test.go`) - 1 hour
  - `TestMysis_ShouldNudge_Traveling_InFuture` (line 26) - conflicts with integration test
  - `TestMysis_ShouldNudge_Cooldown_Active` (line 72) - conflicts with integration test
  - Determine correct behavior: should myses nudge while traveling/in cooldown?
  - Remove incorrect test, update documentation

- [ ] **Investigate hanging test** (`internal/core/state_machine_test.go:260`) - 1 hour
  - `TestStateTransition_Running_To_Idle` hangs during cleanup
  - Use race detector and goroutine profiling
  - Fix goroutine leak in idle transition
  - Re-enable test

**From: Outdated Tests Audit**

- [ ] **Delete obsolete compaction tests** (`internal/core/mysis_test.go`) - 20 min
  - Line 349: `TestContextCompressionRespectsWindow` (obsolete)
  - Line 388: `TestPriorityBasedPromptSelection` (obsolete)
  - Line 492: `TestMemoryCompactionSnapshotRules` (obsolete)
  - Line 932: `TestMemoryCompactionNonSnapshot` (obsolete)
  - Documentation tests for dead code paths

- [ ] **Delete orphaned documentation test** (`internal/core/agent3_reproduction_test.go:190`) - 5 min
  - `TestMemoriesToMessages_WithOrphanedResults` (documentation, not functional)
  - Convert to code comment or remove

- [ ] **Review TODO test** (`internal/core/orphaned_tool_results_test.go:320`) - 15 min
  - `TestContextCompressionPreservesToolCallPairs` - marked as "implement after fixing bug"
  - Check if bug is fixed, enable test or remove

**Total Priority 1: 8 hours**

---

## Priority 2: High (Should Fix Before Release)

**Goal:** Add critical test coverage, clean up documentation organization, update terminology

### P2.1: Critical Test Coverage (11 hours)

**From: Test Coverage Gaps Report**

- [ ] **Add executeToolCall error path tests** (`internal/core/mysis_test.go`) - 2 hours
  - Test: MCP proxy nil check
  - Test: Tool call timeout
  - Test: Invalid tool arguments
  - Test: MCP.CallTool() error handling
  - Test: Tool result parsing failure
  - **Impact:** 58.3% → ~85% coverage

- [ ] **Add buildSystemPrompt edge case tests** (`internal/core/mysis_test.go`) - 1 hour
  - Test: No broadcasts (fallback message)
  - Test: GetRecentBroadcasts() error
  - Test: Unknown sender ID
  - Test: GetMysis() error for sender lookup
  - Test: Broadcast injection format
  - **Impact:** 46.2% → ~90% coverage

- [ ] **Add database migration tests** (`internal/store/store_test.go`) - 3 hours
  - Test: Schema version mismatch
  - Test: Migration rollback on error
  - Test: Corrupt migration SQL
  - Test: Transaction commit failure
  - **Impact:** 38.5% → ~75% coverage

- [ ] **Add main.go integration tests** (`cmd/zoea/main_test.go` NEW) - 4 hours
  - Test: Provider registry initialization
  - Test: Invalid config handling
  - Test: Missing provider factories
  - Test: --debug flag
  - Test: --offline flag
  - Test: --start-swarm flag
  - Test: --config flag
  - **Impact:** 0% → ~60% coverage

- [ ] **Add MCP tool JSON format tests** (`internal/mcp/tools_test.go`) - 1 hour
  - Test: `zoea_claim_account` JSON return format
  - Test: No accounts available message
  - Test: Account claim success format
  - Test: `zoea_list_accounts` JSON format

**From: Known Issues Consolidation**

- [ ] **Add broadcast → idle → running test** (`internal/core/commander_test.go`) - 30 min
  - Test coverage gap for broadcast starting idle myses

### P2.2: Documentation Organization (4 hours)

**From: Documentation Organization Audit**

- [ ] **Move completed plans to archive** - 30 min
  ```bash
  git mv documentation/plans/2026-02-07-loop-context-*.md documentation/archive/
  git mv documentation/plans/2026-02-06-goroutine-cleanup-fixes.md documentation/archive/
  git mv documentation/plans/2026-02-05-broadcast-sender-tracking.md documentation/archive/
  git mv documentation/plans/2026-02-05-context-size-logging.md documentation/archive/
  git mv documentation/plans/2026-02-04-captains-log-bug.md documentation/archive/
  git mv documentation/plans/2026-02-04-context-compaction-plan.md documentation/archive/
  git mv documentation/plans/2026-02-04-remove-tool-payload-bloat.md documentation/archive/
  ```
  **Total:** 11 plan files

- [ ] **Move resolved investigations to archive** - 10 min
  ```bash
  git mv documentation/investigations/OPENCODE_ZEN_*.md documentation/archive/
  git mv documentation/investigations/AGENT6_REAL_PROVIDER_RACE_INVESTIGATION.md documentation/archive/
  ```
  **Total:** 3 investigation files

- [ ] **Move superseded reports to archive** - 20 min
  ```bash
  git mv documentation/reports/RESTART_BUG_INDEX.md documentation/archive/
  git mv documentation/reports/AGENT7_GOROUTINE_CLEANUP_VERIFICATION.md documentation/archive/
  git mv documentation/reports/AGENT_3_RACE_REPRODUCTION_REPORT.md documentation/archive/
  git mv documentation/reports/TIMING_TEST_REPORT.md documentation/archive/
  git mv documentation/reports/STATE_MACHINE_TEST_REPORT.md documentation/archive/
  ```
  **Total:** 5 report files

- [ ] **Update documentation README.md** - 1.5 hours
  - Add new section for 2026-02-07 reports (13 entries)
  - Add to architecture section (1 entry)
  - Add to current section (1 entry)
  - Add new plans section for 2026-02-07 (6 entries)
  - Update "Last updated" to 2026-02-07

- [ ] **Update investigations README.md** - 30 min
  - Move OpenCode Zen to "Resolved Issues"
  - Add resolution date, report link

- [ ] **Update archive README.md** - 1.5 hours
  - Add "Completed Plans (2026-02-04 to 2026-02-07)" section (11 files)
  - Add "Resolved Investigations (2026-02-06 to 2026-02-07)" section (3 files)
  - Add "Restart Bug Investigation Reports (2026-02-06)" section (5 files)
  - Update "Last updated" to 2026-02-07

**From: TODO Organization Audit**

- [ ] **Update TODO.md** - 15 min
  - Remove completed item: "REGRESSION: broadcast doesn't start idle myses"
  - Remove duplicate: "tool messages need JSON rendering properly"
  - Move exit splash screen to KNOWN_ISSUES.md
  - Clarify remaining items (per user input)

### P2.3: Terminology Updates (2 hours)

**From: Outdated Tests Audit**

- [ ] **Rename prompt source tests** (`internal/core/mysis_test.go`) - 1 hour
  - `TestContextPromptSourcePriority` → `TestGetContextMemories_CurrentTurnBoundary`
  - Update test case names: "commander_direct_highest_priority" → "current_turn_start"
  - Update comments to describe turn boundaries instead of priority

- [ ] **Update prompt source comments** - 1 hour
  - `internal/core/agent3_reproduction_test.go:299` - "Step 2: Add commander message (current turn start)"
  - `internal/core/orphaned_tool_results_test.go:386,406` - Replace priority language with turn boundary semantics
  - `documentation/architecture/CONTEXT_COMPRESSION.md:62` - Add cross-reference to terminology

### P2.4: Configuration Consistency (30 min)

**From: Outdated Tests Audit**

- [ ] **Update API key names in tests** - 30 min
  - `internal/integration/mysis_creation_zen_nano_test.go:45,56` - Change `"opencode"` → `"opencode_zen"`
  - `internal/provider/registry_bug_test.go:34,43` - Change `"opencode"` → `"opencode_zen"`

**Total Priority 2: 18 hours**

---

## Priority 3: Cleanup (Nice to Have)

**Goal:** Polish, optimize, and fill remaining test coverage gaps

### P3.1: Quick Test Wins (1.5 hours)

**From: Test Coverage Gaps Report - Quick Wins**

- [ ] **Add trivial getter tests** - 15 min
  - `CreatedAt()`, `MaxMyses()`, `GetStateCounts()`, `Name()`, `DB()`
  - **Impact:** +2% coverage

- [ ] **Add Close() method tests** - 30 min
  - Test: `provider.Close()` for Ollama, OpenCode, Mock
  - Test: `mcp.Client.Close()`
  - Test: Double close safety (idempotency)
  - **Impact:** +1% coverage

- [ ] **Add buildSystemPrompt no broadcasts test** - 10 min
  - Test: Empty broadcast list → fallback message
  - **Impact:** +5% on buildSystemPrompt

- [ ] **Add MCP Initialize() test** - 20 min
  - Test: Basic initialization
  - Test: Already initialized
  - **Impact:** +1% coverage

- [ ] **Add helper predicate tests** - 20 min
  - Test: `isSnapshotTool()`, `extractToolNameFromResult()`, `normalizeFloat()`
  - **Impact:** +1% coverage

**Total Quick Wins: 1.5 hours, +10% coverage (28% → 38%)**

### P3.2: Important Test Coverage (9 hours)

**From: Test Coverage Gaps Report - Medium Priority**

- [ ] **Add SendEphemeralMessage error path tests** (`internal/core/mysis_test.go`) - 1 hour
  - Test: Message to stopped mysis
  - Test: Message to errored mysis
  - Test: Invalid message content
  - **Impact:** 64.4% → ~85% coverage

- [ ] **Add provider lifecycle tests** (`internal/provider/provider_test.go`) - 1 hour
  - Test: Ollama.Close() HTTP client cleanup
  - Test: OpenCode.Close() HTTP client cleanup
  - Test: Mock.Close() resource cleanup

- [ ] **Add network error simulation tests** (`internal/provider/http_errors_test.go`) - 2 hours
  - Test: Connection timeout
  - Test: Connection refused
  - Test: Rate limit (429)
  - Test: Malformed JSON response
  - Test: Partial response read

- [ ] **Add concurrent operations tests** (`internal/core/commander_concurrent_test.go` NEW) - 3 hours
  - Test: Double-start same mysis
  - Test: Delete while running
  - Test: Configure while running
  - Test: Broadcast to deleting mysis

- [ ] **Add getContextMemories edge case tests** (`internal/core/mysis_test.go`) - 2 hours
  - Test: Store.GetRecentMemories() error
  - Test: Empty memory list
  - Test: No user prompt (autonomous nudge)
  - Test: Complex tool loop extraction
  - Test: Turn boundary detection edge cases

### P3.3: Low Priority Test Coverage (2.5 hours)

**From: Test Coverage Gaps Report - Low Priority**

- [ ] **Add config env override tests** (`internal/config/config_test.go`) - 1 hour
  - Test: All env var overrides
  - Test: Partial env overrides
  - Test: Invalid env var values
  - **Impact:** 57.4% → ~85% coverage

- [ ] **Add helper function tests** - 1 hour
  - Test: `isSnapshotTool()`, `extractToolNameFromResult()`, `toolCallNameIndex()`
  - Test: `normalizeFloat()`, `normalizeInt()`

- [ ] **Add config edge case tests** - 30 min
  - Test: Missing config file error message
  - Test: Invalid TOML syntax

### P3.4: Documentation Polish (2 hours)

**From: Documentation Accuracy Audit - Low Priority**

- [ ] **Add PgUp/PgDn to README keyboard shortcuts** - 5 min
  - Already in help screen, add to README for completeness

- [ ] **Update test coverage claim in AGENTS.md** - 5 min
  - Verify current coverage with `make test`, update or remove percentage

- [ ] **Add cross-reference in CONTEXT_COMPRESSION.md** - 10 min
  - Link to terminology definitions in AGENTS.md

- [ ] **Document nudge escalation behavior** - 30 min
  - Add section in CONTEXT_COMPRESSION.md showing 3 escalation levels
  - Document 3-nudge circuit breaker

**From: Documentation Organization Audit**

- [ ] **Verify no broken links** - 1 hour
  ```bash
  # Run link verification script (see audit report)
  cd documentation/
  grep -r "\[.*\](.*\.md)" *.md */*.md | grep -v "http" | # ...
  ```

**From: Known Issues Consolidation**

- [ ] **Add exit splash screen to KNOWN_ISSUES.md** - 10 min
  - Move from TODO.md, add to "User Experience" category
  - Mark as low priority (polish for v1.1+)

### P3.5: Minor Fixes (30 min)

**From: Dead Code Audit**

- [ ] **Consolidate colorPrimary → colorBrand** (`internal/tui/styles.go`) - 10 min
  - Replace 1 usage of `colorPrimary` with `colorBrand`
  - Delete `colorPrimary` alias

**From: Known Issues Consolidation**

- [ ] **Add JSON rendering inconsistency to KNOWN_ISSUES.md** - 10 min
  - Document tool message JSON rendering bug
  - Add example of broken output

- [ ] **Add TUI test skips to KNOWN_ISSUES.md** - 10 min
  - Document `TestIntegration_NewMysisInput` and `TestIntegration_CreateAndStartMysis` skips

**Total Priority 3: 15.5 hours**

---

## Conflicts

### Conflict 1: Broadcast Regression Status

**Agents involved:** TODO Organization Audit, Known Issues Consolidation

**Issue:** TODO Organization Audit claims "broadcast doesn't start idle myses" is COMPLETED (commit 71e119d), but Known Issues Consolidation recommends adding it as HIGH priority issue.

**Resolution:**
1. Verify current behavior: Do broadcasts start idle myses?
2. If YES (working): Remove from TODO, don't add to KNOWN_ISSUES
3. If NO (broken): Add to KNOWN_ISSUES as HIGH priority, investigate regression

**Action:** Run integration test to confirm behavior before proceeding.

---

### Conflict 2: Prompt Source Terminology

**Agents involved:** Outdated Tests Audit, Test Coverage Gaps Report

**Issue:** Test names reference "prompt source priority" (old architecture) vs "turn boundaries" (new architecture)

**Resolution:** Rename all tests to use turn boundary terminology (Priority 2.3)

---

### Conflict 3: OpenCode API Key Name

**Agents involved:** Hardcoded Config Audit, Outdated Tests Audit

**Issue:**
- Hardcoded Config: Recommends using config name as key (e.g., `"zen-nano"`)
- Outdated Tests: Tests use `"opencode"`, should be `"opencode_zen"`

**Resolution:**
1. First: Update tests to use `"opencode_zen"` (Priority 2.4)
2. Then: Implement per-provider key lookup with `"opencode_zen"` fallback (Priority 1.1)
3. Document migration path for users

---

## Execution Order

### Phase 1: Production Fixes and Critical Tests (15 hours)
**Goal:** Fix bugs, remove dead code, add critical test coverage

1. P1.1: Production code fixes (3 hours)
   - Fix API key lookup
   - Delete dead code
   - Fix broadcast/idle bugs

2. P1.3: Test fixes (4 hours)
   - Fix TUI integration tests
   - Resolve activity test conflicts
   - Delete obsolete tests

3. P2.1: Critical test coverage (11 hours)
   - executeToolCall error paths
   - buildSystemPrompt edge cases
   - Database migration tests
   - main.go integration tests
   - MCP tool JSON format tests

**Deliverable:** Critical bugs fixed, dead code removed, coverage at ~55%

---

### Phase 2: Documentation and Terminology (8 hours)
**Goal:** Clean up docs, update terminology, reorganize archives

1. P1.2: User-facing documentation (1 hour)
   - Fix broken links
   - Add missing providers
   - Add vim keys
   - Update KNOWN_ISSUES

2. P2.2: Documentation organization (4 hours)
   - Move completed plans to archive
   - Move resolved investigations to archive
   - Update all README files

3. P2.3: Terminology updates (2 hours)
   - Rename prompt source tests
   - Update comments

4. P2.4: Configuration consistency (30 min)
   - Update API key names in tests

5. TODO.md cleanup (30 min)
   - Remove completed/duplicate items
   - Clarify remaining items

**Deliverable:** Documentation accurate, organized, and up-to-date

---

### Phase 3: Polish and Optimize (13 hours)
**Goal:** Fill remaining coverage gaps, polish documentation

1. P3.1: Quick test wins (1.5 hours)
   - Trivial getters
   - Close() methods
   - Helper predicates
   - **Coverage: 55% → 65%**

2. P3.2: Important test coverage (9 hours)
   - SendEphemeralMessage errors
   - Provider lifecycle
   - Network error simulation
   - Concurrent operations
   - getContextMemories edge cases
   - **Coverage: 65% → 70%**

3. P3.3: Low priority test coverage (2.5 hours)
   - Config env overrides
   - Helper functions
   - Config edge cases
   - **Coverage: 70% → 75%**

4. P3.4: Documentation polish (2 hours)
   - Add remaining keyboard shortcuts
   - Verify no broken links
   - Document nudge escalation

5. P3.5: Minor fixes (30 min)
   - Consolidate color constants
   - Add remaining KNOWN_ISSUES entries

**Deliverable:** Coverage at 75%, all documentation polished

---

## Time Estimates by Category

| Category | Time | Coverage Impact |
|----------|------|-----------------|
| **Production Code Fixes** | 3h | N/A |
| **User-Facing Documentation** | 1h | N/A |
| **Test Fixes** | 4h | Enable 5 tests |
| **Critical Test Coverage** | 11.5h | +27% (28% → 55%) |
| **Documentation Organization** | 4h | N/A |
| **Terminology Updates** | 2h | N/A |
| **Configuration Consistency** | 30m | N/A |
| **Quick Test Wins** | 1.5h | +10% (55% → 65%) |
| **Important Test Coverage** | 9h | +5% (65% → 70%) |
| **Low Priority Test Coverage** | 2.5h | +5% (70% → 75%) |
| **Documentation Polish** | 2h | N/A |
| **Minor Fixes** | 30m | N/A |
| **TOTAL** | **41.5h** | **+47% coverage** |

---

## Coverage Trajectory

```
Current:      ████████░░░░░░░░░░░░░░░░░░░░ 28%
After Phase 1: ████████████████░░░░░░░░░░░░ 55% (+27%)
After Phase 3: ███████████████████████░░░░░ 75% (+47%)
Target:       ████████████████████████████ 80%
```

**Remaining to 80%:** ~6 hours of additional test coverage work

---

## File Impact Summary

### Production Files Changed: 18
- `cmd/zoea/main.go` (API key fix)
- `internal/core/commander.go` (broadcast test)
- `internal/core/mysis.go` (broadcast/idle fixes)
- `internal/mcp/stub.go` (delete stubTick)
- `internal/provider/factory.go` (fix rateBurst)
- `internal/provider/ollama.go` (delete old constructor)
- `internal/provider/opencode.go` (delete old constructor)
- `internal/tui/help.go` (add vim keys)
- `internal/tui/styles.go` (delete colors, consolidate colorPrimary)

### Test Files Changed: 25
- 6 files with obsolete tests removed
- 5 files with skipped tests fixed
- 14 NEW test files/suites added

### Documentation Files Changed: 15
- `README.md` (fixes, additions)
- `AGENTS.md` (coverage update)
- `documentation/README.md` (index update)
- `documentation/investigations/README.md` (resolved issues)
- `documentation/archive/README.md` (archive index)
- `documentation/current/KNOWN_ISSUES.md` (5 new issues)
- `documentation/current/KNOWN_SERVER_ISSUES.md` (OpenCode Zen issue)
- `documentation/current/TODO.md` (cleanup)
- `documentation/architecture/CONTEXT_COMPRESSION.md` (terminology)
- 19 files moved to archive

---

## Blockers and Dependencies

### Before Starting Phase 1:
1. **Verify broadcast regression** - Run integration test to confirm status (15 min)
2. **User approval for TODO.md changes** - Requires user input on vague items (15 min)

### Before Starting Phase 2:
1. **Phase 1 complete** - Production fixes must be tested
2. **User review of open questions** (from Documentation Organization Audit):
   - Status of `plans/2026-02-05-statusbar-tick-timestamps.md`?
   - Status of `plans/2026-02-05-tui-enhancements.md`?
   - Status of `plans/ui-fixes-for-rc.md` and `plans/v1-complete.md`?

### No Blockers:
- All phases can proceed independently after dependencies resolved
- Test infrastructure is solid (no gaps)
- No missing fixtures or test data

---

## Success Criteria

**Phase 1 Complete:**
- ✅ All production bugs fixed
- ✅ All dead code removed
- ✅ All skipped tests fixed or removed
- ✅ Coverage at 55%+
- ✅ `make test` passes

**Phase 2 Complete:**
- ✅ All documentation accurate
- ✅ 19 files archived
- ✅ All README files updated
- ✅ Terminology consistent
- ✅ No broken links

**Phase 3 Complete:**
- ✅ Coverage at 75%+
- ✅ All quick wins implemented
- ✅ Documentation polished
- ✅ `make test` passes
- ✅ `make build` passes with no warnings

---

## Rollback Plan

If issues arise during execution:

**Phase 1 Rollback:**
1. Revert production code changes: `git revert <commit>`
2. Keep test deletions (no rollback needed - dead code)
3. Re-skip tests if fixes cause regressions

**Phase 2 Rollback:**
1. Documentation moves: `git mv documentation/archive/<file> documentation/plans/`
2. README updates: `git revert <commit>`
3. Terminology updates: `git revert <commit>` (may require test updates)

**Phase 3 Rollback:**
1. Test additions: No rollback needed (tests don't break production)
2. Documentation polish: `git revert <commit>`

---

## Next Steps

1. **User Review:**
   - Verify broadcast regression status (Conflict 1)
   - Answer open questions from Documentation Organization Audit
   - Approve TODO.md changes (items to clarify)

2. **Execute Phase 1:**
   - Start with P1.1 (production fixes)
   - Run `make test` after each fix
   - Commit incrementally

3. **Execute Phase 2:**
   - Start with documentation moves (low risk)
   - Update indexes
   - Commit in logical groups

4. **Execute Phase 3:**
   - Start with quick wins (high ROI)
   - Work through important coverage gaps
   - Commit after each test suite added

5. **Final Verification:**
   - Run full test suite: `make test`
   - Verify coverage: `go test ./... -coverprofile=coverage.out`
   - Build check: `make build`
   - Documentation link check (automated script)

---

## Audit Sources

- [DEAD_CODE_AUDIT_2026-02-07.md](DEAD_CODE_AUDIT_2026-02-07.md)
- [SKIPPED_TESTS_AUDIT_2026-02-07.md](SKIPPED_TESTS_AUDIT_2026-02-07.md)
- [OUTDATED_TESTS_AUDIT_2026-02-07.md](OUTDATED_TESTS_AUDIT_2026-02-07.md)
- [HARDCODED_CONFIG_AUDIT_2026-02-07.md](HARDCODED_CONFIG_AUDIT_2026-02-07.md)
- [DOCUMENTATION_ORGANIZATION_AUDIT_2026-02-07.md](DOCUMENTATION_ORGANIZATION_AUDIT_2026-02-07.md)
- [KNOWN_ISSUES_CONSOLIDATION_2026-02-07.md](KNOWN_ISSUES_CONSOLIDATION_2026-02-07.md)
- [TODO_ORGANIZATION_2026-02-07.md](TODO_ORGANIZATION_2026-02-07.md)
- [TEST_COVERAGE_GAPS_2026-02-07.md](TEST_COVERAGE_GAPS_2026-02-07.md)
- [IMPORT_CLEANUP_2026-02-07.md](IMPORT_CLEANUP_2026-02-07.md)
- [DOCUMENTATION_ACCURACY_AUDIT_2026-02-07.md](DOCUMENTATION_ACCURACY_AUDIT_2026-02-07.md)

---

**Plan created:** 2026-02-07  
**Estimated completion:** 6 work days (~1 week)  
**Target coverage:** 75% (from 28%)  
**Files affected:** 58 total
