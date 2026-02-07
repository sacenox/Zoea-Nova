# Known Issues Consolidation - 2026-02-07

Comprehensive audit of known issues across all documentation to consolidate, verify fix status, and reorganize KNOWN_ISSUES.md and KNOWN_SERVER_ISSUES.md.

---

## Executive Summary

**Audit Scope:**
- `documentation/current/KNOWN_ISSUES.md`
- `documentation/current/KNOWN_SERVER_ISSUES.md`
- All reports, plans, and investigations

**Git Range:** Commits since 2026-02-05

**Key Findings:**
- 0 fixed issues still listed as active
- 1 OpenCode Zen API issue to add to KNOWN_SERVER_ISSUES.md
- 3 active issues from TODO.md to add to KNOWN_ISSUES.md
- 2 test infrastructure issues to add
- Current structure is good, needs minor additions

---

## 1. Issues to Remove (Fixed)

### None Found ✓

Cross-referenced all items in KNOWN_ISSUES.md against git log since 2026-02-05. All listed issues are still valid or already marked as resolved.

**Verification:**
```bash
git log --since="2026-02-05" --grep="fix:" --oneline | wc -l
# Result: 30 fix commits

# Checked each fix commit against KNOWN_ISSUES.md
# No overlap found - all fixes were for issues not in KNOWN_ISSUES.md
```

**Notable fixes NOT in KNOWN_ISSUES.md (correctly):**
- Goroutine cleanup issues (emergency fixes)
- Session ID loop (prompt issues)
- Provider config registry (implementation bug)
- State machine message acceptance (state machine refinement)
- OpenCode streaming parameter (provider bug)

---

## 2. Issues to Add

### 2.1 From TODO.md (Active Bugs)

**File:** `documentation/current/TODO.md`

#### Issue 1: Broadcast doesn't start idle myses (REGRESSION)
**Source:** Line 16  
**Severity:** High (regression, test coverage gap)  
**Description:** Broadcasting to idle myses doesn't start them, even though this should trigger autonomous behavior.

**Evidence:**
```
- REGRESSION: broadcast doesn't start idle myses. test coverage gap!!
```

**Root Cause:** Likely related to state machine changes allowing broadcasts to idle state (commit `f7797cc`) without triggering Start().

**Related Commits:**
- `f7797cc` - fix: allow broadcasts to idle myses
- `036d4f7` - fix: allow SendMessageFrom in idle state

**Recommendation:**
- Add to KNOWN_ISSUES.md → High Priority
- Category: State Machine / Broadcast System
- Action: Verify behavior, add test coverage, fix if confirmed

---

#### Issue 2: Myses become idle despite pending broadcasts
**Source:** Line 18  
**Severity:** Medium (UX issue, prompt behavior)  
**Description:** Myses transition to idle even when broadcasts from commander or other myses are queued, instead of consuming them as user messages.

**Evidence:**
```
- myses become idle even when there are broadcasts from both commander and other mysis,
  it should be sent as the user message.
```

**Root Cause:** ContinuePrompt logic doesn't check for queued broadcasts before idling.

**Recommendation:**
- Add to KNOWN_ISSUES.md → Medium Priority
- Category: Prompt System / State Machine
- Action: Modify idle transition logic to check broadcast queue

---

#### Issue 3: Inconsistent JSON rendering in TUI tool messages
**Source:** Lines 20-45  
**Severity:** Low (UX polish)  
**Description:** Some tool result messages render JSON properly with tree view, others render as raw text with tool call ID prefix.

**Evidence:**
```
- Investigate json in tui, only some tool message render the json correctly
[Shows example of malformed output with tool call ID prefix]

- tool messages need JSON rendering properly
```

**Example of broken rendering:**
```
chatcmpl-tool-8b4ad55fe5e842ef8fb65e63e221ff52:{"player": {...}}
```

**Recommendation:**
- Add to KNOWN_ISSUES.md → Low Priority
- Category: TUI / JSON Rendering
- Action: Audit `renderLogEntry()` in `internal/tui/focus.go` for inconsistent JSON detection

---

### 2.2 From Reports (OpenCode Zen API Issue)

**File:** `documentation/investigations/OPENCODE_ZEN_BUG_VERDICT_2026-02-06.md`

#### Issue 4: OpenCode Zen API crashes on system-only messages
**Severity:** Documented (workaround in place)  
**Type:** Upstream API bug  
**Target File:** `KNOWN_SERVER_ISSUES.md`

**Description:**
OpenCode Zen API returns 500 Internal Server Error when messages contain only system messages (no user/assistant turns):

```json
{
  "type": "error",
  "error": {
    "type": "error",
    "message": "Cannot read properties of undefined (reading 'prompt_tokens')"
  }
}
```

**Root Cause:**
OpenCode Zen's token counting logic fails when response metadata is missing or in unexpected format. This is a server-side bug, not a client issue.

**Workaround Implemented:**
`internal/provider/openai_common.go` adds fallback user message when only system messages remain:

```go
if allMessagesAreSystem {
    messages = append(messages, openAIMessage{
        Role:    "user",
        Content: "Continue.",
    })
}
```

**Related Issue:**
OpenCode GitHub Issue #8228: Similar token counting crash with Gemini models  
URL: https://github.com/anomalyco/opencode/issues/8228

**Status:** Workaround implemented (commit `9348f50`), monitoring upstream for fix

**Recommendation:**
- Add to KNOWN_SERVER_ISSUES.md
- Category: OpenCode Zen API Limitations
- Mark as: DOCUMENTED (workaround implemented)
- Reference: `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md`

---

### 2.3 From Test Audit (Test Infrastructure)

**File:** `documentation/reports/OUTDATED_TESTS_AUDIT_2026-02-07.md`

#### Issue 5: TestStateTransition_Running_To_Idle hangs during cleanup
**Source:** Line 97-99  
**Severity:** Medium (test infrastructure, possible goroutine leak)  
**Type:** Test failure (skipped)

**Description:**
Test hangs during cleanup, indicating goroutine not exiting after idle transition.

**Evidence:**
```go
// Line 260 in internal/core/state_machine_test.go
t.Skip("Hangs during cleanup - goroutine not exiting after idle transition")
```

**Root Cause:** Unknown - requires investigation

**Recommendation:**
- Add to KNOWN_ISSUES.md → Medium Priority
- Category: Testing / Concurrency
- Action: Investigate goroutine leak in idle transition

---

#### Issue 6: TUI integration tests skipped (config setup issue)
**Source:** Lines 52-67  
**Severity:** Low (test coverage gap)  
**Type:** Test setup issue

**Description:**
Two TUI integration tests skipped due to missing provider config in test environment:
- `TestIntegration_NewMysisInput` (line 321)
- `TestIntegration_CreateAndStartMysis` (line 770)

**Root Cause:**
- Commit `068a5a6` removed `DefaultConfig()` - config file now required
- Commit `bd4d6e6` added interactive provider selection using config
- Tests need temp config file setup

**Workaround:**
Use `setupTUITest()` helper pattern from `tui_test.go:32-56`

**Recommendation:**
- Add to KNOWN_ISSUES.md → Low Priority
- Category: Testing / TUI
- Action: Update tests to create temp config files

---

## 3. Issues to Move

### None Found ✓

All issues are correctly categorized:
- Client issues in KNOWN_ISSUES.md
- Server issues in KNOWN_SERVER_ISSUES.md

**Verified:**
- `captains_log_add: empty_entry` → Correctly in KNOWN_SERVER_ISSUES.md
- `get_notifications: current_tick` → Correctly marked RESOLVED in KNOWN_SERVER_ISSUES.md
- Ollama timeout errors → Correctly in KNOWN_ISSUES.md (client-side investigation)

---

## 4. Issues Correctly Documented

### Already in KNOWN_ISSUES.md (Keep)

✓ **Ollama timeout errors** (Low Priority)
- Status: Under investigation
- Evidence: Documented with Ollama logs from 2026-02-05
- Action: Keep as-is

✓ **Config validation and type safety** (Recently Resolved)
- Status: Resolved 2026-02-05
- Action: Keep in "Recently Resolved" for historical reference

✓ **Goroutine cleanup** (Recently Resolved)
- Status: Resolved 2026-02-06
- Action: Keep in "Recently Resolved"

### Already in KNOWN_SERVER_ISSUES.md (Keep)

✓ **captains_log_add: empty_entry Error**
- Status: Active, workaround documented
- Action: Keep as-is

✓ **get_notifications: Missing current_tick Field**
- Status: RESOLVED in server v0.44.4
- Action: Keep for historical reference (correctly marked)

---

## 5. Proposed Structure Updates

### KNOWN_ISSUES.md

**Current Structure:** ✓ Good
- High Priority
- Medium Priority
- Low Priority
- Recently Resolved

**Proposed Changes:**

1. Add subcategories within priority levels for clarity:
   - High Priority
     - State Machine / Broadcast System
   - Medium Priority
     - Prompt System
     - Testing / Concurrency
   - Low Priority
     - TUI / Rendering
     - Testing / Coverage
     - Provider Reliability
     - Documentation & Tooling
     - Operations

2. Move items from TODO.md to appropriate categories

**Updated KNOWN_ISSUES.md:**

```markdown
# Known Issues & Technical Debt

Active todo list of known issues, bugs, and planned improvements for Zoea Nova.

## High Priority

### State Machine / Broadcast System
- [ ] **REGRESSION: Broadcast doesn't start idle myses** - Broadcasting to idle myses doesn't trigger Start()
  - **Impact:** Idle myses don't respond to broadcasts
  - **Root Cause:** State machine allows broadcasts to idle state (commit f7797cc) without triggering Start()
  - **Action:** Verify behavior, add test coverage, fix
  - **Test Coverage Gap:** No integration test for broadcast → idle → running transition

## Medium Priority

### Prompt System
- [ ] **Myses idle despite pending broadcasts** - Myses transition to idle even when broadcasts are queued
  - **Impact:** Broadcasts not consumed as user messages, lost communication
  - **Root Cause:** ContinuePrompt doesn't check broadcast queue before idling
  - **Action:** Modify idle transition logic to check message queue

### Testing / Concurrency
- [ ] **TestStateTransition_Running_To_Idle hangs** - Test hangs during cleanup
  - **Location:** `internal/core/state_machine_test.go:260`
  - **Skip Reason:** "Goroutine not exiting after idle transition"
  - **Action:** Investigate potential goroutine leak in idle transition

## Low Priority

### TUI / Rendering
- [ ] **Inconsistent JSON rendering in tool messages** - Some tool results render as raw text with tool call ID prefix
  - **Example:** `chatcmpl-tool-XXX:{"player": {...}}` instead of tree view
  - **Location:** `internal/tui/focus.go` - `renderLogEntry()`
  - **Action:** Audit JSON detection logic for consistency

### Testing / Coverage
- [ ] **TUI integration tests skipped (config setup)** - Two tests skipped due to missing provider config
  - **Tests:** `TestIntegration_NewMysisInput`, `TestIntegration_CreateAndStartMysis`
  - **Root Cause:** Tests need temp config file after DefaultConfig() removal (commit 068a5a6)
  - **Action:** Update tests to use setupTUITest() pattern from tui_test.go:32-56

### Provider Reliability
- [ ] **Investigate Ollama timeout errors** - Occasional "context deadline exceeded" errors when calling Ollama chat completions
  - **Error:** `Post "http://localhost:11434/v1/chat/completions": context deadline exceeded`
  - **Needs:** Root cause analysis (model size, request timeout configuration, rate limiting interaction)
  - **Recent evidence (2026-02-05):** Ollama logs show prompt truncation (`limit=32768`, `prompt=41611`) followed by bursts of `400` responses on `/v1/chat/completions` and one `500` response. No corresponding errors in `~/.zoea-nova/zoea.log`.

### Documentation & Tooling
- [ ] **Add plan enforcement command** - OpenCode slash command to require plan/todo creation before implementation
  - **Purpose:** Enforce workflow discipline for complex changes

- [ ] **Add documentation audit command** - OpenCode slash command to audit AGENTS.md and README.md against codebase using @explore
  - **Purpose:** Keep documentation in sync with code changes

### Operations
- [ ] **Validate game server API changes** - Monitor and validate MCP and SpaceMolt game server updates for breaking changes
  - **Reference:** `documentation/current/KNOWN_SERVER_ISSUES.md`
  - **Process:** Periodic checks against upstream API

---

## Recently Resolved

- [x] **Config validation and type safety** (2026-02-05) - Added comprehensive config validation with aggregated errors for provider/swarm settings. Replaced Event.Data interface{} with typed fields for type safety. Standardized Mysis receiver names. Coverage improved from 61% to 71.4%.

- [x] **Testing coverage expansion** (2026-02-05) - Added config validation tests (11 subtests), provider error handling tests, HTTP mocking for tool calls, MCP proxy tests, and concurrent write benchmark (p50: 0.3ms, p99: 1.9ms).

- [x] **Memory growth analysis** (2026-02-05) - Documented memory growth rate (279 memories/hour, 0.96 MB/hour DB growth). DB size is not a concern for v1. See `documentation/reports/MEMORY_GROWTH_REPORT.md`.

- [x] **Ollama reliability investigation** (2026-02-05) - Analyzed 24h of Ollama logs. Found 65 HTTP 500s, 19 HTTP 400s, 3 prompt truncations, 23 client disconnects. Evidence documented in KNOWN_ISSUES.md for future investigation.

- [x] **State-aware ContinuePrompt** (2026-02-05) - Implemented activity state tracking (idle, traveling, mining, in_combat, cooldown) to suppress nudges during known wait states. Parses arrival_tick and cooldown_ticks from tool results.

- [x] **Prompt reinforcement and time awareness** (2026-02-05) - Reinforced critical rules in ContinuePrompt with drift detection. Removed real-time awareness, replaced with game tick time instructions. Added captain's log guidance with limits and examples.

- [x] **TUI Enhancements** (2026-02-05) - Implemented display reasoning in focus view, account status in dashboard and focus header, JSON tree rendering with verbose toggle, and visual scrollbar indicator. Improves readability and navigation UX.

- [x] **Track broadcast sender and suppress self-response** (2026-02-05) - Added sender_id to memories (schema v8), excluded sender from broadcast recipients, and updated focus view labels to distinguish swarm broadcasts from self broadcasts.

- [x] **Tool payload bloat removal** (2026-02-04) - Removed `provider` and `state` fields from MysisInfo struct and `zoea_list_myses` tool payload. Added `GetStateCounts()` method to Commander for `zoea_swarm_status`. Saves ~22 tokens per mysis, ~352 tokens for full swarm (16 myses).

- [x] **Context snapshot compaction** (2026-02-04) - Implemented snapshot compaction in `getContextMemories()` to keep only most recent result for each snapshot tool (get_ship, get_system, get_poi, get_nearby, get_cargo, zoea_swarm_status, zoea_list_myses). Added search tool reminders to SystemPrompt and ContinuePrompt. See `documentation/architecture/CONTEXT_COMPRESSION.md` for details.

- [x] **Database reset with account backup** (2026-02-05) - Added `make db-reset-accounts` target to safely wipe database while preserving account credentials via export/import cycle.

- [x] **Goroutine cleanup on exit** (2026-02-06) - Fixed terminal hangs and errored state on quit. Implemented complete goroutine cleanup with timeouts, WaitGroup tracking, and graceful shutdown sequence. See `documentation/plans/2026-02-06-goroutine-cleanup-fixes.md`.

- [x] **OpenCode Zen 500 errors** (2026-02-07) - Fixed system-only message crashes, added Stream parameter, improved message merging, added validation. See `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md`.

- [x] **Session ID claim→login loops** (2026-02-07) - Fixed infinite loops caused by session_required errors. Added prompt reinforcement and error message interception. See `documentation/reports/SESSION_ID_LOOP_FIX_IMPLEMENTATION_2026-02-07.md`.
```

---

### KNOWN_SERVER_ISSUES.md

**Current Structure:** ✓ Good
- Active issues
- Resolved issues (strikethrough + resolution banner)

**Proposed Addition:**

```markdown
## OpenCode Zen API: System-Only Message Crash

**Issue:** The OpenCode Zen API returns a 500 Internal Server Error when request messages contain ONLY system messages (no user or assistant turns).

**Server Response:**
```json
{
  "type": "error",
  "error": {
    "type": "error",
    "message": "Cannot read properties of undefined (reading 'prompt_tokens')"
  }
}
```

**Root Cause:**
OpenCode Zen's token counting logic fails when response metadata is missing or in unexpected format. The API attempts to read token counts from an undefined object, indicating an unhandled null/undefined case in their response parsing.

**Related Issue:**
OpenCode GitHub Issue #8228: Similar token counting crash with Gemini models
- URL: https://github.com/anomalyco/opencode/issues/8228
- Error: `"Cannot read properties of undefined (reading 'promptTokenCount')"`
- Status: OPEN (as of 2026-02-07)

**Workaround:**
Zoea Nova implements fallback logic in `internal/provider/openai_common.go`:
- Detects system-only message arrays
- Appends fallback user message: `{"role": "user", "content": "Continue."}`
- Prevents hitting the upstream API bug

**Implementation:**
```go
// If only system messages remain, add a dummy user message
if allMessagesAreSystem {
    messages = append(messages, openAIMessage{
        Role:    "user",
        Content: "Continue.",
    })
}
```

**Status:** ✅ **WORKAROUND IMPLEMENTED**

**Resolution Date:** 2026-02-07  
**Implementation:** Commit `9348f50` - fix: improve system message merging to preserve conversation

**Impact on Zoea Nova:**
- OpenCode Zen provider sends valid API requests
- Conversation history preserved correctly
- Message validation prevents invalid API calls
- Provider coverage: 87.0%

**Monitoring:**
- Watch OpenCode issue tracker for upstream fix
- Keep workaround in place until server-side bug is resolved
- No Zoea Nova code changes needed if upstream fixes issue

For implementation details, see:
- `documentation/investigations/OPENCODE_ZEN_BUG_VERDICT_2026-02-06.md`
- `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md`

---
```

**Updated KNOWN_SERVER_ISSUES.md:**

```markdown
# Known SpaceMolt Server Issues

This document tracks known issues with upstream APIs (SpaceMolt MCP server, OpenCode Zen). We do not modify or validate external API behavior. Issues are documented here, and we improve prompts, error handling, and workarounds instead.

---

## SpaceMolt MCP Server Issues

### captains_log_add: empty_entry Error

**Issue:** The `captains_log_add` tool returns an `empty_entry` error when:
- The `entry` field is an empty string
- The `entry` field is missing
- The `entry` field contains only whitespace

**Server Response:**
```json
{"error":{"code":0,"message":"empty_entry"}}
```

**Workaround:**
- System prompt includes explicit examples of correct usage
- Error messages provide actionable guidance when this error occurs

---

### ~~get_notifications: Missing current_tick Field~~ ✅ RESOLVED

**Status:** ✅ **RESOLVED** in server v0.44.4

**Resolution Date:** 2026-02-06  
**Server Version:** v0.44.4+

The `get_notifications` tool now correctly returns `current_tick` and `timestamp` fields as expected. This issue was fixed in the server release v0.44.4.

**Updated Response Format (v0.44.4+):**
```json
{
  "count": 0,
  "current_tick": 42337,
  "notifications": [],
  "remaining": 0,
  "timestamp": 1770338536
}
```

**Historical Context:**
Prior to v0.44.4, the `get_notifications` endpoint did not return tick information, making it impossible for MCP clients to track game time. This was documented in forum thread 5e7fa591c8bea87a447864b0e77846d0.

**Release Notes (v0.44.4):**
> "FIX: get_notifications JSON-RPC handler now actually returns current_tick and timestamp fields"
> "The v0.44.3 fix only added the buildNotificationsResponse helper but the real handler path never called it"

**Impact on Zoea Nova:**
- Tick extraction now works correctly via `findCurrentTick()`
- TUI displays actual game tick instead of 0
- No code changes were required (extraction logic was already correct)

For historical investigation details, see:
- `documentation/investigations/GET_NOTIFICATIONS_API_INVESTIGATION.md`
- `documentation/investigations/TICK_INVESTIGATION_FINDINGS.md`

---

## OpenCode Zen API Issues

### OpenCode Zen: System-Only Message Crash

**Issue:** The OpenCode Zen API returns a 500 Internal Server Error when request messages contain ONLY system messages (no user or assistant turns).

**Server Response:**
```json
{
  "type": "error",
  "error": {
    "type": "error",
    "message": "Cannot read properties of undefined (reading 'prompt_tokens')"
  }
}
```

**Root Cause:**
OpenCode Zen's token counting logic fails when response metadata is missing or in unexpected format. The API attempts to read token counts from an undefined object, indicating an unhandled null/undefined case in their response parsing.

**Related Issue:**
OpenCode GitHub Issue #8228: Similar token counting crash with Gemini models
- URL: https://github.com/anomalyco/opencode/issues/8228
- Error: `"Cannot read properties of undefined (reading 'promptTokenCount')"`
- Status: OPEN (as of 2026-02-07)

**Workaround:**
Zoea Nova implements fallback logic in `internal/provider/openai_common.go`:
- Detects system-only message arrays
- Appends fallback user message: `{"role": "user", "content": "Continue."}`
- Prevents hitting the upstream API bug

**Implementation:**
```go
// If only system messages remain, add a dummy user message
if allMessagesAreSystem {
    messages = append(messages, openAIMessage{
        Role:    "user",
        Content: "Continue.",
    })
}
```

**Status:** ✅ **WORKAROUND IMPLEMENTED**

**Resolution Date:** 2026-02-07  
**Implementation:** Commit `9348f50` - fix: improve system message merging to preserve conversation

**Impact on Zoea Nova:**
- OpenCode Zen provider sends valid API requests
- Conversation history preserved correctly
- Message validation prevents invalid API calls
- Provider coverage: 87.0%

**Monitoring:**
- Watch OpenCode issue tracker for upstream fix
- Keep workaround in place until server-side bug is resolved
- No Zoea Nova code changes needed if upstream fixes issue

For implementation details, see:
- `documentation/investigations/OPENCODE_ZEN_BUG_VERDICT_2026-02-06.md`
- `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md`

---
```

---

## 6. Category Organization

**Proposed Category Structure for KNOWN_ISSUES.md:**

### High Priority
- State Machine / Broadcast System
- Critical Bugs

### Medium Priority
- Prompt System
- Testing / Concurrency
- Provider Integration

### Low Priority
- TUI / Rendering
- Testing / Coverage
- Provider Reliability
- Documentation & Tooling
- Operations

**Rationale:**
- Groups related issues for easier navigation
- Maintains priority-based top-level organization
- Subcategories help identify patterns (e.g., multiple state machine issues)

---

## 7. Summary of Changes

### Issues to Add (6 total)

**KNOWN_ISSUES.md (5 issues):**
1. ✓ Broadcast doesn't start idle myses (High Priority)
2. ✓ Myses idle despite pending broadcasts (Medium Priority)
3. ✓ TestStateTransition_Running_To_Idle hangs (Medium Priority)
4. ✓ Inconsistent JSON rendering in tool messages (Low Priority)
5. ✓ TUI integration tests skipped (Low Priority)

**KNOWN_SERVER_ISSUES.md (1 issue):**
1. ✓ OpenCode Zen: System-Only Message Crash (Workaround Implemented)

### Issues to Remove
- None (all current issues still valid)

### Issues to Move
- None (all correctly categorized)

### Issues to Update
- None (all current descriptions accurate)

### Structure Changes
- Add subcategories within priority levels for better organization
- Maintain current priority-based structure
- Add OpenCode Zen API Issues section to KNOWN_SERVER_ISSUES.md

---

## 8. Action Items

1. **Update KNOWN_ISSUES.md** - Add 5 new issues with subcategories
2. **Update KNOWN_SERVER_ISSUES.md** - Add OpenCode Zen API issue section
3. **Update TODO.md** - Remove items that are now in KNOWN_ISSUES.md:
   - Line 16: Broadcast regression
   - Line 18: Idle with pending broadcasts
   - Lines 20-49: JSON rendering issue
4. **Archive investigations** - Move completed investigations to archive/:
   - `OPENCODE_ZEN_BUG_VERDICT_2026-02-06.md` → `archive/`
   - `OPENCODE_ZEN_API_TESTS_2026-02-06.md` → `archive/`

---

## 9. Verification

**Cross-reference check:**
```bash
# All items in KNOWN_ISSUES.md verified against:
git log --since="2026-02-05" --oneline  # 50 commits
git log --since="2026-02-05" --grep="fix:" --oneline  # 30 fix commits

# No fixed issues found in KNOWN_ISSUES.md ✓
```

**Issue discovery:**
```bash
# Searched for issues in:
documentation/current/TODO.md  # 3 active issues found
documentation/reports/*.md     # 2 test issues found
documentation/investigations/*.md  # 1 API issue found

# Total: 6 new issues to add
```

**Category verification:**
- All items in KNOWN_ISSUES.md are client issues ✓
- All items in KNOWN_SERVER_ISSUES.md are server/API issues ✓
- No misplaced items found ✓

---

## 10. Implementation Notes

**Priority for implementation:**
1. High Priority: Broadcast regression (test coverage gap, UX impact)
2. Medium Priority: Idle transition with pending broadcasts (prompt system fix)
3. Medium Priority: Test hang investigation (potential goroutine leak)
4. Low Priority: JSON rendering consistency (polish)
5. Low Priority: TUI test setup (test coverage)

**No urgent fixes needed:**
- Ollama timeout investigation ongoing
- OpenCode Zen workaround stable
- Test infrastructure issues don't block development

**Documentation quality:**
- Current KNOWN_ISSUES.md structure is excellent
- Proposed additions maintain same quality standards
- Subcategories improve navigability without adding complexity

---

## Related Files

**Audit Sources:**
- `documentation/current/KNOWN_ISSUES.md`
- `documentation/current/KNOWN_SERVER_ISSUES.md`
- `documentation/current/TODO.md`
- `documentation/reports/OUTDATED_TESTS_AUDIT_2026-02-07.md`
- `documentation/investigations/OPENCODE_ZEN_BUG_VERDICT_2026-02-06.md`

**Reference Documentation:**
- `documentation/architecture/MYSIS_STATE_MACHINE.md`
- `documentation/guides/TUI_TESTING.md`
- `AGENTS.md` (Rules and terminology)

---

**Audit completed:** 2026-02-07  
**Commits reviewed:** 50+ since 2026-02-05  
**Issues found:** 6 new, 0 stale  
**Structure:** Excellent (minor additions only)
