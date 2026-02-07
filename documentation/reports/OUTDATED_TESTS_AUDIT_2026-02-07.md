# Outdated Tests Audit - 2026-02-07

Comprehensive audit of test logic that verifies removed or changed functionality.

## Executive Summary

**Audit Scope:** All `*_test.go` files  
**Reference Period:** Commits since 2026-02-06  
**Tests Audited:** 67 test files  
**Issues Found:** 13 categories of outdated/obsolete test logic

---

## Key Findings

### 1. Already Removed/Skipped (Good State)

These tests are already marked as obsolete with `t.Skip()`:

#### `internal/core/mysis_test.go`
- **Line 349:** `TestContextCompressionRespectsWindow` - **OBSOLETE**
  - Tests old compaction strategy
  - Replaced by loop-based composition (TestLoopContextSlice)
  - **Recommendation:** DELETE (not just skip)

- **Line 388:** `TestPriorityBasedPromptSelection` - **OBSOLETE**
  - Tests old compaction strategy  
  - Replaced by loop-based composition (TestContextPromptSourcePriority)
  - **Recommendation:** DELETE (not just skip)

- **Line 492:** `TestMemoryCompactionSnapshotRules` - **OBSOLETE**
  - Tests old snapshot compaction
  - Loop composition doesn't use compaction
  - **Recommendation:** DELETE (not just skip)

- **Line 932:** `TestMemoryCompactionNonSnapshot` - **OBSOLETE**
  - Tests old non-snapshot compaction
  - Loop composition doesn't use compaction
  - **Recommendation:** DELETE (not just skip)

**What changed:**  
Commit `7cf375c` (Phase 2: Implement turn-aware context composition) replaced compaction with turn-aware composition. The `getContextMemories()` function no longer uses compaction/snapshot strategies.

**Recommendation:** **DELETE** all four skipped tests entirely. They document dead code paths.

---

### 2. Skipped TUI Integration Tests (Test Setup Issue)

#### `internal/tui/integration_test.go`

- **Line 321-324:** `TestIntegration_NewMysisInput` - **SKIPPED**
  - Skip reason: "Test environment setup issue - provider config not available"
  - What changed: Commit `bd4d6e6` added interactive provider selection requiring config
  - **Recommendation:** **FIX** test setup to load config, then unskip

- **Line 770-773:** `TestIntegration_CreateAndStartMysis` - **SKIPPED**
  - Skip reason: "Test environment setup issue - provider config not available"
  - What changed: Commit `068a5a6` removed DefaultConfig(), config now required
  - **Recommendation:** **FIX** test setup to load config, then unskip

**What changed:**  
- Commit `068a5a6` removed `DefaultConfig()` - config file now required
- Commit `bd4d6e6` added interactive provider selection using config
- Tests need to load config from inline TOML or temp file

**Recommendation:** **UPDATE** both tests to create temp config files using `setupTUITest()` helper pattern from `tui_test.go:32-56`.

---

### 3. Removed Test Comment (Already Fixed)

#### `internal/tui/integration_test.go`
- **Line 955:** `TestIntegration_AutoScrollBehavior` - **REMOVED**
  - Comment documents removal
  - Auto-scroll functionality removed per TODO item
  - **Recommendation:** No action (comment is appropriate)

---

### 4. Config Test Comment (Already Fixed)

#### `internal/config/config_test.go`
- **Line 11:** `// TestDefaultConfig removed - config file is now required`
  - Commit `068a5a6` removed `DefaultConfig()` function
  - Test properly removed
  - **Recommendation:** No action (comment is appropriate)

---

### 5. Skipped State Machine Tests (Legitimate)

#### `internal/core/state_machine_test.go`

- **Line 156:** Stress test skipped in short mode - **OK**
- **Line 260:** `TestStateTransition_Running_To_Idle` - **SKIPPED**
  - Skip reason: "Hangs during cleanup - goroutine not exiting after idle transition"
  - **Recommendation:** **INVESTIGATE** - This indicates a potential goroutine leak
  - Not obsolete, but documents a real bug

- **Line 466:** Rapid cycle test skipped in short mode - **OK**

**Recommendation:** Keep skip for line 260, but add to KNOWN_ISSUES.md if not already documented.

---

### 6. Skipped Activity Tests (Test Conflicts)

#### `internal/core/activity_test.go`
- **Line 26:** `TestMysis_ActivityState_Traveling` - **SKIPPED**
  - Skip reason: "Conflicts with TestMysis_ActivityStateTransitions/traveling_future_no_nudge"
  - **Recommendation:** **REVIEW** - Consolidate or fix conflict

- **Line 72:** `TestMysis_ActivityState_Cooldown` - **SKIPPED**
  - Skip reason: "Conflicts with TestMysis_ActivityStateTransitions/cooldown_future_no_nudge"
  - **Recommendation:** **REVIEW** - Consolidate or fix conflict

**Recommendation:** **REVIEW** - Either consolidate into a single test or fix the conflict.

---

### 7. Tests Using Hardcoded Provider Names (Need Review)

The following tests hardcode `"ollama"` or `"opencode_zen"` provider names. After commit `b7ec302` (use config keys for provider registry), we should use config-defined keys.

#### `internal/core/commander_test.go`
- **Lines 31, 39, 157, 170:** Hardcoded `"ollama"` provider
- **What changed:** Commit `b7ec302` changed registry to use config keys (e.g., `"ollama-llama"`, `"zen-nano"`) instead of factory names
- **Recommendation:** **REVIEW** - Tests still pass because they mock the registry. Consider updating to use realistic config keys for clarity.

#### `internal/tui/tui_test.go`
- **Lines 40-41, 48, 52-53, 169-170, 199, 252-253, 279, 516, 536, 615, 639:** Hardcoded `"ollama"` and `"opencode_zen"`
- **What changed:** Same as above
- **Recommendation:** **REVIEW** - Update to use realistic config keys (`"zen-nano"`, `"ollama-llama"`)

#### `internal/store/store_test.go`
- **Lines 43, 85, 89, 120, 198, 225, 252:** Hardcoded `"ollama"` and `"opencode_zen"`
- **Recommendation:** **REVIEW** - These are store-level tests, hardcoded names are fine (store doesn't validate provider names)

**Recommendation:** **LOW PRIORITY UPDATE** - Tests work correctly but should use realistic provider keys for documentation purposes.

---

### 8. Provider Factory Tests (Potentially Outdated)

#### `internal/provider/factory_test.go`
- **All tests:** Use `NewOllamaFactory()` and `NewOpenCodeFactory()` constructors
- **What changed:** Commit `b7ec302` introduced dynamic provider registration using config keys
- **Status:** Tests verify factory behavior (rate limiter sharing), still valid
- **Recommendation:** **KEEP** - Tests factory internals, not config-level registration

---

### 9. Account Locking Tests (Need Review)

#### `internal/store/store_test.go`
- **Lines 366-497:** `TestClaimAccount_*` suite (5 tests)
  - What changed: Commit `7b77396` removed account locking from `ClaimAccount()`
  - Tests verify `ClaimAccount()` does NOT lock accounts (correct behavior)
  - **Recommendation:** **KEEP** - Tests verify the NEW correct behavior

#### `internal/integration/login_flow_test.go`
- **Lines 12-236:** `TestLoginFlowIntegration` and `TestLoginFlowIntegration_MultipleMyses`
  - Tests verify accounts are NOT locked until login succeeds
  - **Recommendation:** **KEEP** - Tests verify post-refactor behavior

#### `internal/core/mysis_account_release_test.go`
- **Lines 50-80:** Tests account locking/unlocking lifecycle
  - Tests verify accounts ARE locked after successful login (via `setCurrentAccount()`)
  - **Recommendation:** **KEEP** - Tests verify the correct lock-on-success behavior

**Recommendation:** **KEEP ALL** - These tests verify the NEW behavior after account locking refactor.

---

### 10. MCP Tool Tests (Removed Tool)

#### `internal/mcp/mcp_test.go`
- **Status:** No references to `zoea_swarm_status` found (good)
- **What changed:** Commit `2ee4189` removed `zoea_swarm_status` tool
- **Commits:**
  - `157c1f4`: Removed zoea_swarm_status from mcp test
  - `e4e0fd2`: Fixed variable declaration after removal
- **Recommendation:** **NO ACTION** - Already cleaned up

---

### 11. Prompt Source Selection Tests (Architecture Change)

#### `internal/core/mysis_test.go`
- **Lines 588-800:** `TestContextPromptSourcePriority` and related tests
  - What changed: Commit `9c1f41c` removed `selectPromptSource()` function
  - Tests now verify turn-aware composition (find last user prompt index)
  - Test names/comments reference old "prompt source priority" concept
  - **Recommendation:** **UPDATE** - Rename test to reflect turn boundaries, not priority

**What changed:**  
The `selectPromptSource()` function implemented priority-based prompt selection (commander direct → commander broadcast → swarm broadcast → nudge). Commit `7cf375c` replaced this with turn-aware composition using `findLastUserPromptIndex()`. The concept of "prompt source priority" no longer exists.

**Recommendation:** **REFACTOR** test names and comments:
- `TestContextPromptSourcePriority` → `TestGetContextMemories_CurrentTurnBoundary`
- Update comments to describe turn boundaries instead of priority

---

### 12. Context Composition Tests (Architecture Change)

#### `internal/core/mysis_test.go`
- **Lines 641-800:** Context composition tests
  - Test table includes cases like `"commander_direct_highest_priority"`, `"commander_broadcast_when_no_direct"`, `"swarm_broadcast_when_no_commander_messages"`
  - **What changed:** Priority-based selection replaced with turn-boundary detection
  - **Recommendation:** **UPDATE** - Rename test cases to reflect turn semantics

#### `internal/core/agent3_reproduction_test.go`
- **Line 299:** Comment "Step 2: Add commander message (prompt source)"
- **Recommendation:** **UPDATE** - Change to "Step 2: Add commander message (current turn start)"

#### `internal/core/orphaned_tool_results_test.go`
- **Line 386:** Comment "- Selected prompt source (commander direct → last commander broadcast → last swarm broadcast → nudge)"
- **Line 406:** Comment "Step 2: Add commander direct message (prompt source)"
- **Recommendation:** **UPDATE** - Replace with turn boundary semantics

**Recommendation:** **UPDATE** comments and test case names to use turn-boundary language instead of priority language.

---

### 13. OpenCode Zen API Key Tests (Minor Inconsistency)

#### `internal/integration/mysis_creation_zen_nano_test.go`
- **Line 45:** `creds.SetAPIKey("opencode", "test-api-key-12345")`
- **Line 56:** `apiKey := creds.GetAPIKey("opencode")`
- **What changed:** Commit `31e5b3f` changed API key name to `"opencode_zen"`
- **Status:** Test uses old key name
- **Recommendation:** **UPDATE** to `"opencode_zen"` for consistency

#### `internal/provider/registry_bug_test.go`
- **Line 34:** `creds.SetAPIKey("opencode", "test-api-key")`
- **Line 43:** `apiKey := creds.GetAPIKey("opencode")`
- **Recommendation:** **UPDATE** to `"opencode_zen"`

**Recommendation:** **UPDATE** both tests to use `"opencode_zen"` as the API key name (matches commit `31e5b3f`).

---

## Summary Table

| Category | Files Affected | Status | Priority | Recommendation |
|----------|---------------|--------|----------|----------------|
| Obsolete compaction tests | `mysis_test.go` | Skipped | **HIGH** | **DELETE** 4 tests |
| Skipped TUI integration | `integration_test.go` | Skipped | **HIGH** | **FIX** test setup |
| Hardcoded provider names | `commander_test.go`, `tui_test.go` | Passing | **LOW** | Update for clarity |
| Prompt source terminology | `mysis_test.go`, `agent3_reproduction_test.go`, `orphaned_tool_results_test.go` | Passing | **MEDIUM** | Rename/refactor |
| API key name inconsistency | `mysis_creation_zen_nano_test.go`, `registry_bug_test.go` | Passing | **MEDIUM** | Update key names |
| State machine hang | `state_machine_test.go` | Skipped | **MEDIUM** | Investigate bug |
| Activity test conflicts | `activity_test.go` | Skipped | **MEDIUM** | Review conflicts |

---

## Detailed Recommendations

### Immediate Actions (HIGH Priority)

1. **DELETE obsolete tests** (`internal/core/mysis_test.go`):
   - Remove `TestContextCompressionRespectsWindow` (line 349)
   - Remove `TestPriorityBasedPromptSelection` (line 388)
   - Remove `TestMemoryCompactionSnapshotRules` (line 492)
   - Remove `TestMemoryCompactionNonSnapshot` (line 932)

2. **FIX TUI integration test setup** (`internal/tui/integration_test.go`):
   - Add config loading to `TestIntegration_NewMysisInput`
   - Add config loading to `TestIntegration_CreateAndStartMysis`
   - Use pattern from `setupTUITest()` helper

### Medium Priority

3. **UPDATE terminology** (prompt source → turn boundary):
   - Rename `TestContextPromptSourcePriority` → `TestGetContextMemories_CurrentTurnBoundary`
   - Update test case names in table tests
   - Update comments in `agent3_reproduction_test.go` and `orphaned_tool_results_test.go`

4. **FIX API key names**:
   - `mysis_creation_zen_nano_test.go`: Change `"opencode"` → `"opencode_zen"`
   - `registry_bug_test.go`: Change `"opencode"` → `"opencode_zen"`

5. **INVESTIGATE state machine hang** (`state_machine_test.go:260`):
   - Test documents goroutine leak during idle transition
   - Add to KNOWN_ISSUES.md if not already documented

6. **REVIEW activity test conflicts** (`activity_test.go`):
   - Consolidate or fix conflicting tests

### Low Priority

7. **UPDATE hardcoded provider names** (optional):
   - `commander_test.go`: Use realistic config keys
   - `tui_test.go`: Use realistic config keys
   - Improves test documentation, not critical

---

## Tests That Are Correct (No Changes Needed)

### Account Management
- All `TestClaimAccount_*` tests in `store_test.go` (verify NEW behavior)
- `TestLoginFlowIntegration` tests (verify NEW behavior)
- `TestMysis_AccountLifecycle` tests (verify lock-on-success)

### Provider Factory
- All factory tests in `factory_test.go` (test internals, not config)

### Removed Functionality
- MCP tool tests (already cleaned up)
- DefaultConfig test (properly removed with comment)
- Auto-scroll test (properly removed with comment)

---

## Commits Referenced

| Commit | Date | Description |
|--------|------|-------------|
| `2ee4189` | 2026-02-07 | Remove zoea_swarm_status tool |
| `9c1f41c` | 2026-02-07 | Remove selectPromptSource function |
| `7cf375c` | 2026-02-07 | Implement turn-aware context composition |
| `7b77396` | 2026-02-07 | Remove account locking from ClaimAccount() |
| `068a5a6` | 2026-02-07 | Remove DefaultConfig and applyProviderDefaults |
| `bd4d6e6` | 2026-02-07 | Add interactive provider selection |
| `b7ec302` | 2026-02-07 | Use config keys for provider registry |
| `31e5b3f` | 2026-02-07 | Use correct API key name (opencode_zen) |
| `3152600` | 2026-02-07 | Skip failing TUI integration tests |

---

## Verification Commands

```bash
# Find all skipped tests
rg "t\.Skip" --type go

# Find obsolete compaction references
rg "compaction|snapshot" --type go internal/core/mysis_test.go

# Find prompt source references
rg "prompt.?source|selectPromptSource" --type go

# Find hardcoded provider names
rg '"ollama"|"opencode_zen"' --type go internal/core/commander_test.go

# Run all tests to verify passing state
make test
```

---

## Conclusion

**Total Issues:** 13 categories  
**High Priority:** 2 (delete obsolete tests, fix TUI setup)  
**Medium Priority:** 4 (terminology, API keys, state machine, activity conflicts)  
**Low Priority:** 1 (hardcoded names)  

**Overall Status:** The codebase is in good shape. Most outdated tests are already skipped with clear comments. The main work is:
1. Deleting obsolete skipped tests (dead code cleanup)
2. Fixing TUI integration test setup (test environment issue, not production bug)
3. Updating terminology from "prompt source priority" to "turn boundaries"

No critical production bugs discovered. All failing/skipped tests are documented with clear reasons.

---

**Audit Date:** 2026-02-07  
**Audited By:** Zoea Nova Development Team  
**Next Review:** After completing high-priority recommendations
