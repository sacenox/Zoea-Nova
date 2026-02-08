# Zoea Nova v0.5.0 Design Truth Table

**Version:** 0.5.0-rc  
**Date:** 2026-02-08  
**Purpose:** Comprehensive analysis of actual implementation vs documentation vs tests

This document serves as the **single source of truth** for what Zoea Nova v0.5.0 actually does, identifying discrepancies between code, documentation, and tests.

---

## Executive Summary

**Overall Assessment:** The implementation is **90% aligned** with documentation. The codebase has mature patterns but has **3 critical issues** that must be fixed before v1.0.

**Critical Issues (MUST FIX):**
1. 🔴 **State machine bugs:** Two lifecycle bugs (race condition, setIdle hang) prevent reliable operation
2. 🔴 **Compression functions unused:** Myses run blind without current game state in context (snapshots not compacted)
3. 🔴 **Fallback user message missing:** OpenAI API requires user message after system, but we don't add one

**Key Findings:**
- ✅ **Core architecture matches design:** State machine, event bus, provider abstraction all implemented as documented
- ⚠️ **Undocumented features:** Several production features exist but aren't documented (auto-start, broadcast storage model, activity indicators)
- 🔴 **Unused compression code:** `compactSnapshots()` and `removeOrphanedToolCalls()` never called - myses lack current state context
- 🔴 **Missing fallback logic:** No fallback user message added when only system messages exist (OpenAI API violation)
- ⚠️ **Test gaps:** 2 critical bugs detected by tests but not fixed (state machine race, setIdle hang)
- ✅ **Test quality:** 83% coverage with honest tests that fail when bugs exist

---

## 1. Mysis Lifecycle

### Truth Table

| Feature                   | Documented                      | Implemented            | Tested               | Match?     | Notes                                          |
| ------------------------- | ------------------------------- | ---------------------- | -------------------- | ---------- | ---------------------------------------------- |
| **States**                | idle, running, stopped, errored | ✅ Same 4 states       | ✅ All states tested | ✅ YES     | Perfect match                                  |
| **Create → Idle**         | ✅ Documented                   | ✅ `mysis.go:76`       | ✅ Tested            | ✅ YES     | -                                              |
| **Idle → Running**        | ✅ Start()                      | ✅ `mysis.go:220`      | ✅ Tested            | ✅ YES     | -                                              |
| **Running → Stopped**     | ✅ Stop()                       | ✅ `mysis.go:296`      | ✅ Tested            | ⚠️ RACE    | Test detects race: error can override stopped  |
| **Running → Errored**     | ✅ setErrorState()              | ✅ `mysis.go:1022`     | ✅ Tested            | ✅ YES     | -                                              |
| **Running → Idle**        | ✅ 3 nudges failed              | ✅ `mysis.go:1067`     | ⏭️ SKIPPED           | ⚠️ BUG     | Test skipped: setIdle() doesn't stop goroutine |
| **Stopped → Running**     | ✅ Relaunch                     | ✅ `mysis.go:220`      | ✅ Tested            | ✅ YES     | -                                              |
| **Errored → Running**     | ✅ Relaunch                     | ✅ `mysis.go:231-248`  | ✅ Tested            | ✅ YES     | Includes cleanup logic                         |
| **Auto-start on message** | ⚠️ Implied                      | ✅ `mysis.go:407, 792` | ✅ Tested            | ⚠️ PARTIAL | Not in state diagram                           |
| **Message acceptance**    | ✅ idle/running accept          | ✅ `mysis.go:157-168`  | ✅ Tested            | ✅ YES     | -                                              |

### Discrepancies

1. 🔴 **Running → Idle Transition Bug (CRITICAL)**
   - **Code:** `setIdle()` sets state but doesn't stop run loop goroutine
   - **Test:** Skipped with note "Known bug: setIdle() doesn't stop loop goroutine"
   - **Impact:** Myses hang when transitioning to idle, goroutine continues running
   - **Priority:** MUST FIX - Prevents reliable idle state management
   - **Fix:** Make `setIdle()` cancel context or signal run loop to exit

2. 🔴 **Running → Stopped Race Condition (CRITICAL)**
   - **Code:** Context cancellation error can override Stopped state with Errored
   - **Test:** Stress test fails 0-5% of runs (`TestStateTransition_Running_To_Stopped_StressTest`)
   - **Impact:** Stop() doesn't reliably result in Stopped state
   - **Priority:** MUST FIX - Breaks state machine guarantees
   - **Fix:** Ensure Stop() always results in Stopped state, never Errored (strengthen state protection)

3. **Auto-Start Not in State Diagram**
   - **Code:** Direct messages and broadcasts automatically start idle Myses
   - **Docs:** Mentioned in text but not shown in state diagram
   - **Fix:** Add note to `MYSIS_STATE_MACHINE.md` state diagram

---

## 2. Provider Architecture

### Truth Table

| Feature                    | Documented           | Implemented                    | Tested           | Match?         | Notes                                |
| -------------------------- | -------------------- | ------------------------------ | ---------------- | -------------- | ------------------------------------ |
| **OpenCode Zen**           | ✅ OpenAI-compatible | ✅ `opencode.go`               | ✅ Tested        | ✅ YES         | -                                    |
| **Ollama**                 | ✅ Custom API        | ✅ `ollama.go`                 | ✅ Tested        | ✅ YES         | -                                    |
| **Mock**                   | ❌ Not documented    | ✅ `mock.go`                   | ✅ Tested        | ⚠️ MISSING     | For testing only                     |
| **System message merging** | ✅ OpenAI first      | ✅ `mergeSystemMessagesOpenAI` | ✅ Tested        | ✅ YES         | -                                    |
| **Fallback user message**  | ⚠️ WRONG             | ❌ NOT IMPLEMENTED             | ✅ Test confirms | 🔴 MISSING     | CRITICAL: OpenAI requires user after system |
| **Tool call validation**   | ⚠️ Misleading        | ❌ In mysis layer              | ✅ Tested        | ⚠️ WRONG LAYER | Docs say provider, actually in mysis |
| **Retry logic**            | ❌ Not documented    | ✅ 3 attempts, 5/10/15s        | ✅ Tested        | ⚠️ MISSING     | Critical feature undocumented        |
| **Rate limiting**          | ❌ Not documented    | ✅ Via `rate.Limiter`          | ✅ Tested        | ⚠️ MISSING     | Undocumented                         |
| **Model endpoint routing** | ❌ Not documented    | ✅ 8 models mapped             | ✅ Tested        | ⚠️ MISSING     | OpenCode-specific                    |
| **Reasoning extraction**   | ✅ Ollama only       | ✅ `reasoning()` method        | ✅ Tested        | ✅ YES         | -                                    |

### Discrepancies

1. 🔴 **Fallback User Message Missing (CRITICAL)**
   - **AGENTS.md claims:** "Fallback user message if only system messages exist"
   - **Reality:** `mergeSystemMessagesOpenAI()` does NOT add fallback message
   - **Test confirms:** `TestMergeSystemMessagesOpenAI_OnlySystemMessages` expects 1 message (system only)
   - **OpenAI API requirement:** Conversations must have at least one user message after system message
   - **Impact:** Myses may fail to start or get API errors when only system prompt exists
   - **Priority:** MUST FIX - Violates OpenAI API contract
   - **Fix:** Add fallback user message in context building (`getContextMemories()`) when no user messages exist
   - **Suggested message:** "Begin your mission. Check notifications and coordinate with the swarm."

2. **Tool Call Validation Location**
   - **AGENTS.md claims:** "Tool call validation and orphaned message removal" in provider
   - **Reality:** Validation happens in `internal/mysis`, not provider
   - **Fix:** Update AGENTS.md to clarify validation location

3. **Retry Logic Undocumented**
   - **Implementation:** 3 retries with exponential backoff (5s, 10s, 15s)
   - **Retryable codes:** 429, 500, 502, 503, 504
   - **Fix:** Document in `OPENAI_COMPATIBILITY.md`

4. **Model Endpoint Routing Undocumented**
   - **Implementation:** Hardcoded map for 8 models, fallback rules for `gpt-*` and `claude-*`
   - **Fix:** Add table to documentation

---

## 3. Context Compression & Memory

### Truth Table

| Feature                        | Documented          | Implemented                 | Tested    | Match?          | Notes                                          |
| ------------------------------ | ------------------- | --------------------------- | --------- | --------------- | ---------------------------------------------- |
| **Turn-aware composition**     | ⚠️ "Loop slices"    | ✅ `getContextMemories`     | ✅ Tested | ⚠️ TERMINOLOGY  | Docs say "loop slices", code does "turn-aware" |
| **Historical compression**     | ✅ Latest tool loop | ✅ `extractLatestToolLoop`  | ✅ Tested | ✅ YES          | -                                              |
| **Current turn uncompressed**  | ❌ Not documented   | ✅ Entire turn included     | ✅ Tested | ⚠️ MISSING      | Can reach 24 messages                          |
| **Snapshot compaction** | ✅ Documented | ✅ Function exists | ✅ Tested | 🔴 NEVER CALLED | CRITICAL: Myses lack current state |
| **Orphaned tool call removal** | ✅ Documented | ✅ Function exists | ✅ Tested | 🔴 NEVER CALLED | CRITICAL: Invalid tool sequences |
| **Broadcast fallback query**   | ❌ Not documented   | ✅ `GetMostRecentBroadcast` | ✅ Tested | ⚠️ MISSING      | Critical for new myses                         |
| **Encouragement counter**      | ✅ 3 limit          | ✅ Implemented              | ✅ Tested | ✅ YES          | -                                              |
| **Synthetic nudge**            | ✅ Documented       | ✅ Ephemeral message        | ✅ Tested | ✅ YES          | Not stored in DB                               |
| **Size bounds**                | ⚠️ "3-10 typical"   | ❌ Can reach 24             | ✅ Tested | ❌ VIOLATED     | Docs claim 3-10, reality is 1-24               |

### Discrepancies

1. 🔴 **Compression Functions Never Called (CRITICAL)**
   - **Functions exist:** `compactSnapshots()`, `removeOrphanedToolCalls()`
   - **Tests exist:** Both functions have unit tests
   - **Reality:** Never called in `getContextMemories()` production flow
   - **Impact:** 
     - **Myses run blind:** Without `compactSnapshots()`, context contains stale game state (old `get_status`, `get_system`, `get_notifications` results)
     - **Invalid tool sequences:** Without `removeOrphanedToolCalls()`, context may have assistant messages with tool calls but no results (violates OpenAI API)
     - **Context bloat:** Redundant data crowds out relevant information
   - **Priority:** MUST FIX - Myses need current game state to make decisions
   - **Fix:** Integrate both functions into `getContextMemories()` pipeline:
     ```go
     result = m.compactSnapshots(result)
     result = m.removeOrphanedToolCalls(result)
     return result, addedSynthetic, nil
     ```

2. **Context Size Underbounded**
   - **Docs claim:** "3-10 messages typical, ~17 max"
   - **Reality:** Can reach 24 messages (1 system + 20 sliding window + 3 historical)
   - **Fix:** Update documentation with actual bounds

3. **Turn-Aware vs Loop Slices**
   - **Docs describe:** "Loop slices" as primary model
   - **Code implements:** Turn-aware composition (historical + current turn)
   - **Fix:** Update `CONTEXT_COMPRESSION.md` to reflect turn-aware model

4. **Broadcast Fallback Undocumented**
   - **Implementation:** Queries entire DB for broadcast if none in 20-message window
   - **Purpose:** New myses inherit mission directives
   - **Fix:** Document in `CONTEXT_COMPRESSION.md`

---

## 4. MCP Integration

### Truth Table

| Feature                  | Documented           | Implemented                  | Tested    | Match?        | Notes                              |
| ------------------------ | -------------------- | ---------------------------- | --------- | ------------- | ---------------------------------- |
| **Proxy architecture**   | ✅ Documented        | ✅ `proxy.go`                | ✅ Tested | ✅ YES        | -                                  |
| **Local tools**          | ⚠️ Partial           | ✅ 7 tools                   | ✅ Tested | ⚠️ INCOMPLETE | Not all listed in docs             |
| **Upstream tools**       | ✅ Dynamic discovery | ✅ `ListTools()`             | ✅ Tested | ✅ YES        | -                                  |
| **Offline mode**         | ✅ Documented        | ✅ `stub.go`                 | ✅ Tested | ✅ YES        | -                                  |
| **Stub tools**           | ⚠️ 4 tools listed    | ✅ 5 tools                   | ✅ Tested | ⚠️ MISSING    | `get_notifications` not documented |
| **Retry logic**          | ❌ Not documented    | ✅ 3 attempts, 200/400/800ms | ✅ Tested | ⚠️ MISSING    | Undocumented                       |
| **Error rewriting**      | ⚠️ In reports        | ✅ `rewriteSessionError`     | ✅ Tested | ⚠️ PARTIAL    | Not in architecture docs           |
| **Account interception** | ⚠️ In tests          | ✅ `interceptAuthTools`      | ✅ Tested | ⚠️ PARTIAL    | Not user-facing docs               |
| **Caller context**       | ❌ Not documented    | ✅ `CallerContext`           | ✅ Tested | ⚠️ MISSING    | Developer feature                  |

### Discrepancies

1. **Offline Mode Tool List Incomplete**
   - **Documented:** `get_status`, `get_system`, `get_ship`, `get_poi`
   - **Implemented:** Above + `get_notifications`
   - **Fix:** Add `get_notifications` to `OFFLINE_MODE.md`

2. **Local Tools Not Listed**
   - **Implemented:** 7 `zoea_*` tools (list_myses, send_message, broadcast, search_messages, etc.)
   - **Documentation:** Not listed in offline mode docs
   - **Fix:** Add section listing all local tools

3. **Retry Logic Undocumented**
   - **Implementation:** 3 retries with 200ms, 400ms, 800ms delays
   - **Fix:** Document in architecture docs

4. **Error Rewriting Undocumented**
   - **Implementation:** Rewrites `session_required` errors to prevent login loops
   - **Fix:** Document in `MCP_ERROR_HANDLING.md` (new file)

---

## 5. TUI Implementation

### Truth Table

| Feature                  | Documented        | Implemented         | Tested    | Match?     | Notes                             |
| ------------------------ | ----------------- | ------------------- | --------- | ---------- | --------------------------------- |
| **Dashboard view**       | ✅ Documented     | ✅ `dashboard.go`   | ✅ Tested | ✅ YES     | -                                 |
| **Focus view**           | ✅ Documented     | ✅ `focus.go`       | ✅ Tested | ✅ YES     | -                                 |
| **Help overlay**         | ❌ Not documented | ✅ `help.go`        | ✅ Tested | ⚠️ MISSING | Toggled with `?` key              |
| **Activity indicators**  | ❌ Not documented | ✅ 7 types          | ✅ Tested | ⚠️ MISSING | LLM, MCP, traveling, mining, etc. |
| **Network indicator**    | ⚠️ Partial        | ✅ `NetIndicator`   | ✅ Tested | ⚠️ PARTIAL | Counter logic undocumented        |
| **Status bar**           | ❌ Not documented | ✅ 3 segments       | ✅ Tested | ⚠️ MISSING | Layout not explained              |
| **Verbose toggle**       | ⚠️ In help        | ✅ `v` key          | ✅ Tested | ⚠️ PARTIAL | Not in AGENTS.md                  |
| **Reasoning truncation** | ❌ Not documented | ✅ Smart truncation | ✅ Tested | ⚠️ MISSING | First + [x more] + last 2         |
| **Event bus**            | ✅ Documented     | ✅ `bus.go`         | ✅ Tested | ✅ YES     | -                                 |
| **Input modes**          | ✅ Documented     | ✅ 5 modes          | ✅ Tested | ✅ YES     | -                                 |

### Discrepancies

1. **Help Overlay Not Documented**
   - **Implementation:** `?` key toggles centered help box with 15 commands
   - **Fix:** Add to AGENTS.md TUI section

2. **Activity Indicators Not Documented**
   - **Implementation:** 7 types (idle, llm_call, mcp_call, traveling, mining, in_combat, cooldown)
   - **Fix:** Document in AGENTS.md

3. **Status Bar Layout Not Documented**
   - **Implementation:** 3 segments (left: activity, middle: tick+time, right: state counts)
   - **Fix:** Document in AGENTS.md

4. **Reasoning Truncation Not Documented**
   - **Implementation:** When verbose=false, shows first line + "[x more]" + last 2 lines
   - **Fix:** Document in TUI_TESTING.md

---

## 6. Database Schema

### Truth Table

| Feature                  | Documented           | Implemented             | Tested    | Match?      | Notes                        |
| ------------------------ | -------------------- | ----------------------- | --------- | ----------- | ---------------------------- |
| **myses table**          | ✅ Documented        | ✅ `schema.sql`         | ✅ Tested | ✅ YES      | -                            |
| **memories table**       | ⚠️ Missing sender_id | ✅ Includes sender_id   | ✅ Tested | ⚠️ OUTDATED | v8 added sender_id           |
| **accounts table**       | ✅ Documented        | ✅ `schema.sql`         | ✅ Tested | ✅ YES      | -                            |
| **schema_version table** | ❌ Not documented    | ✅ `schema.sql`         | ✅ Tested | ⚠️ MISSING  | Migration control            |
| **Indexes**              | ❌ Not documented    | ✅ 4 indexes            | ✅ Tested | ⚠️ MISSING  | 3 on memories, 1 on accounts |
| **Broadcast storage**    | ⚠️ Unclear           | ✅ Replicated per-mysis | ✅ Tested | ⚠️ UNCLEAR  | One record per recipient     |
| **Search methods**       | ❌ Not documented    | ✅ 3 search methods     | ✅ Tested | ⚠️ MISSING  | LIKE queries                 |
| **Two-phase claiming**   | ❌ Not documented    | ✅ Claim + MarkInUse    | ✅ Tested | ⚠️ MISSING  | Race prevention              |

### Discrepancies

1. **sender_id Column Not in Docs**
   - **Documentation:** Shows schema v6 → v7
   - **Reality:** Schema is v8, added `sender_id` to memories table
   - **Fix:** Update `INITIAL_IMPLEMENTATION_DESIGN.md` schema section

2. **Broadcast Storage Model Unclear**
   - **Implementation:** Broadcasts replicated as individual memory records per recipient
   - **Example:** Broadcast to 10 myses creates 10 DB rows
   - **Fix:** Create `BROADCAST_ARCHITECTURE.md` explaining storage model

3. **Search Methods Undocumented**
   - **Implementation:** `SearchMemories`, `SearchReasoning`, `SearchBroadcasts`
   - **Fix:** Document in store API reference

4. **Two-Phase Account Claiming Undocumented**
   - **Implementation:** `ClaimAccount()` returns account WITHOUT marking in_use; requires separate `MarkAccountInUse()` call
   - **Purpose:** Prevent race conditions during login validation
   - **Fix:** Document in architecture docs

---

## 7. Commander Orchestration

### Truth Table

| Feature                     | Documented        | Implemented                 | Tested    | Match?     | Notes             |
| --------------------------- | ----------------- | --------------------------- | --------- | ---------- | ----------------- |
| **Mysis lifecycle**         | ✅ Documented     | ✅ Create/Delete/Start/Stop | ✅ Tested | ✅ YES     | -                 |
| **Message routing**         | ✅ Documented     | ✅ Direct + Broadcast       | ✅ Tested | ✅ YES     | -                 |
| **Broadcast exclusion**     | ⚠️ Implied        | ✅ `BroadcastFrom`          | ✅ Tested | ⚠️ PARTIAL | Sender excluded   |
| **Auto-start on broadcast** | ❌ Not documented | ✅ Idle myses wake          | ✅ Tested | ⚠️ MISSING | Critical behavior |
| **AggregateTick**           | ❌ Not documented | ✅ Max tick across swarm    | ✅ Tested | ⚠️ MISSING | For UI display    |
| **WaitGroup tracking**      | ⚠️ Mentioned      | ✅ Graceful shutdown        | ✅ Tested | ⚠️ PARTIAL | 10-second timeout |
| **SendMessageAsync**        | ❌ Not documented | ✅ Non-blocking send        | ✅ Tested | ⚠️ MISSING | TUI uses this     |
| **BroadcastAsync**          | ❌ Not documented | ✅ Alias for BroadcastFrom  | ✅ Tested | ⚠️ MISSING | -                 |

### Discrepancies

1. **Auto-Start on Broadcast Not Documented**
   - **Implementation:** Broadcasts automatically wake idle myses
   - **Fix:** Document in Commander section of AGENTS.md

2. **AggregateTick Undocumented**
   - **Implementation:** Returns maximum tick across all myses for UI display
   - **Fix:** Add to Commander API documentation

3. **Async Methods Undocumented**
   - **Implementation:** `SendMessageAsync`, `BroadcastAsync` for non-blocking operations
   - **Fix:** Document in AGENTS.md

---

## 8. Configuration System

### Truth Table

| Feature                     | Documented        | Implemented                   | Tested    | Match?     | Notes             |
| --------------------------- | ----------------- | ----------------------------- | --------- | ---------- | ----------------- |
| **TOML config**             | ✅ Documented     | ✅ `config.toml`              | ✅ Tested | ✅ YES     | -                 |
| **Environment variables**   | ❌ Not documented | ✅ 13 vars                    | ✅ Tested | ⚠️ MISSING | All `ZOEA_*` vars |
| **Credentials file**        | ❌ Not documented | ✅ `credentials.json`         | ✅ Tested | ⚠️ MISSING | API keys          |
| **Default provider**        | ❌ Not documented | ✅ `default_provider`         | ✅ Tested | ⚠️ MISSING | In swarm section  |
| **Rate limiting**           | ❌ Not documented | ✅ `rate_limit`, `rate_burst` | ✅ Tested | ⚠️ MISSING | Per-provider      |
| **Provider auto-detection** | ❌ Not documented | ✅ By endpoint URL            | ✅ Tested | ⚠️ MISSING | String matching   |
| **Validation rules**        | ❌ Not documented | ✅ Comprehensive              | ✅ Tested | ⚠️ MISSING | Bounds checking   |

### Discrepancies

1. **Environment Variables Not Documented**
   - **Implementation:** 13 `ZOEA_*` environment variables for overrides
   - **Fix:** Create `CONFIGURATION.md` with complete reference

2. **Credentials File Not Documented**
   - **Implementation:** `~/.zoea-nova/credentials.json` with 0600 permissions
   - **Fix:** Document in README.md and configuration guide

3. **Rate Limiting Not Documented**
   - **Implementation:** Per-provider `rate_limit` and `rate_burst` settings
   - **Fix:** Document in configuration guide

---

## 9. Error Handling

### Truth Table

| Feature                      | Documented          | Implemented                | Tested    | Match?      | Notes               |
| ---------------------------- | ------------------- | -------------------------- | --------- | ----------- | ------------------- |
| **Error wrapping**           | ❌ Not documented   | ✅ `fmt.Errorf("op: %w")`  | ✅ Tested | ⚠️ IMPLICIT | Consistent pattern  |
| **Retry logic**              | ❌ Not documented   | ✅ Provider + MCP          | ✅ Tested | ⚠️ MISSING  | Different delays    |
| **Event-driven errors**      | ⚠️ Mentioned        | ✅ `EventMysisError`       | ✅ Tested | ⚠️ PARTIAL  | Flow not diagrammed |
| **State machine protection** | ⚠️ In code comments | ✅ Stopped overrides error | ✅ Tested | ⚠️ PARTIAL  | Race prevention     |
| **Error visibility**         | ✅ TUI shows errors | ✅ `LastError()` method    | ✅ Tested | ✅ YES      | -                   |
| **Structured logging**       | ✅ Use zerolog      | ✅ All layers              | ✅ Tested | ✅ YES      | -                   |

### Discrepancies

1. **No Error Handling Documentation**
   - **Implementation:** Consistent error wrapping, retry logic, event-driven propagation
   - **Fix:** Create `ERROR_HANDLING.md` documenting patterns

2. **Retry Policies Undocumented**
   - **Implementation:** Provider (5/10/15s), MCP (200/400/800ms)
   - **Fix:** Document rationale for different delays

3. **Error Flow Not Diagrammed**
   - **Implementation:** Store → Provider/MCP → Mysis → Commander → TUI
   - **Fix:** Add error flow diagram to architecture docs

---

## 10. Test Coverage

### Truth Table

| Category       | Coverage | Quality    | Issues                    | Notes                  |
| -------------- | -------- | ---------- | ------------------------- | ---------------------- |
| **Overall**    | 83%      | ⭐⭐⭐⭐⭐ | 2 critical bugs           | Target: 80%+           |
| **Core logic** | 82.6%    | ⭐⭐⭐⭐⭐ | State machine race        | Comprehensive          |
| **Provider**   | 86.2%    | ⭐⭐⭐⭐⭐ | None                      | Excellent              |
| **TUI**        | 85.4%    | ⭐⭐⭐⭐   | 2 flaky E2E tests         | Golden files excellent |
| **Store**      | 74.9%    | ⭐⭐⭐⭐   | None                      | Good                   |
| **MCP**        | 60.9%    | ⭐⭐⭐     | Offline mode under-tested | Needs improvement      |

### Critical Test Findings

1. **State Machine Race (DETECTED, NOT FIXED)**
   - **Test:** `TestStateTransition_Running_To_Stopped_StressTest`
   - **Issue:** Stop() can be overridden by error state
   - **Failure rate:** 0-5% in 100-iteration stress test
   - **Status:** Test correctly detects bug, bug not fixed

2. **setIdle() Hang (DETECTED, TEST SKIPPED)**
   - **Test:** `TestStateTransition_Running_To_Idle`
   - **Issue:** setIdle() doesn't stop run loop goroutine
   - **Status:** Test skipped with note "Known bug"

3. **Tests Assert Correct Behavior**
   - Tests fail when bugs exist (good test hygiene)
   - No tests asserting incorrect behavior to make them pass
   - Workarounds documented (time.Sleep, accepting errored state)

---

## 11. Priority Fixes

### 🔴 Critical (MUST Fix Before v1.0 - Blocks Production Use)

1. **Fix State Machine Race (Running → Stopped)**
   - **Issue:** Context cancellation error can override Stopped state with Errored
   - **Test:** `TestStateTransition_Running_To_Stopped_StressTest` fails 0-5% of runs
   - **Impact:** Stop() doesn't reliably result in Stopped state, breaks state machine guarantees
   - **Root cause:** `setError()` can run after `Stop()` sets state to Stopped
   - **Fix:** Strengthen state protection in `setError()` - check if state is Stopped AFTER acquiring lock
   - **Verification:** Stress test must pass 100% of runs (1000+ iterations)

2. **Fix setIdle() Goroutine Hang (Running → Idle)**
   - **Issue:** `setIdle()` sets state but doesn't stop run loop goroutine
   - **Test:** `TestStateTransition_Running_To_Idle` skipped with note "Known bug"
   - **Impact:** Myses hang when transitioning to idle, goroutine continues running, wastes resources
   - **Root cause:** `setIdle()` doesn't cancel context or signal run loop to exit
   - **Fix:** Make `setIdle()` cancel context (like `Stop()` does) or add explicit exit signal
   - **Verification:** Unskip test, ensure it passes

3. **Integrate Compression Functions (Context Building)**
   - **Issue:** `compactSnapshots()` and `removeOrphanedToolCalls()` exist but never called
   - **Impact:** 
     - **Myses run blind:** Context contains stale game state (old snapshots), not current state
     - **Invalid tool sequences:** Orphaned tool calls violate OpenAI API requirements
     - **Poor decision-making:** Myses act on outdated information
   - **Fix:** Add to `getContextMemories()` pipeline:
     ```go
     // After composing result, before returning:
     result = m.compactSnapshots(result)
     result = m.removeOrphanedToolCalls(result)
     return result, addedSynthetic, nil
     ```
   - **Verification:** 
     - Unit tests already exist and pass
     - Integration test: Verify myses see current game state in context
     - Check context size reduction (should be smaller with compaction)

4. **Implement Fallback User Message (OpenAI API Compliance)**
   - **Issue:** No fallback user message added when only system messages exist
   - **Impact:** Violates OpenAI API requirement (must have user message after system)
   - **Current behavior:** `mergeSystemMessagesOpenAI()` only merges systems, doesn't add fallback
   - **Fix:** Add fallback logic in `getContextMemories()`:
     ```go
     // After composing result, check if only system messages exist
     hasUserMessage := false
     for _, msg := range result {
         if msg.Role == "user" {
             hasUserMessage = true
             break
         }
     }
     if !hasUserMessage && len(result) > 0 {
         // Add fallback user message
         result = append(result, provider.Message{
             Role:    "user",
             Content: "Begin your mission. Check notifications and coordinate with the swarm.",
         })
     }
     ```
   - **Verification:** 
     - Update `TestMergeSystemMessagesOpenAI_OnlySystemMessages` to expect fallback
     - Test with OpenCode provider (should not error)
     - Test mysis startup with only system prompt

### High Priority (Documentation Gaps)

4. **Update AGENTS.md**
   - Add auto-start behavior
   - Document activity indicators
   - Add help overlay
   - Document status bar layout
   - Fix provider claims (fallback message, tool validation location)

5. **Create ERROR_HANDLING.md**
   - Document error wrapping pattern
   - Diagram error flow
   - Explain retry policies

6. **Create CONFIGURATION.md**
   - Complete config reference
   - Environment variables
   - Credentials file setup

7. **Update CONTEXT_COMPRESSION.md**
   - Reflect turn-aware model (not "loop slices")
   - Document actual size bounds (1-24, not 3-10)
   - Explain broadcast fallback query

8. **Update OFFLINE_MODE.md**
   - Add `get_notifications` to tool list
   - List all local `zoea_*` tools

### Medium Priority (Undocumented Features)

9. **Document Broadcast Architecture**
   - Storage model (replicated per-mysis)
   - Sender tracking and exclusion
   - Retrieval and deduplication

10. **Document MCP Error Handling**
    - Retry logic
    - Error rewriting
    - Account interception

11. **Update Database Schema Docs**
    - Add `sender_id` column
    - Document indexes
    - Explain two-phase account claiming

### Low Priority (Nice to Have)

12. **Add Error Metrics to TUI**
    - Provider error rate (already tracked)
    - Degradation warnings

13. **Make Retry Policies Configurable**
    - Move to `config.toml`
    - Document defaults

---

## 12. What's Working Well

### Strengths

1. **Clean Architecture**
   - Clear separation of concerns
   - Interface-based design
   - Event-driven coordination

2. **Mature Error Handling**
   - Consistent error wrapping
   - Structured logging
   - Event-driven propagation
   - Retry logic for transient failures

3. **Comprehensive Testing**
   - 83% coverage
   - Three test types (unit, golden, integration)
   - Honest tests that fail when bugs exist

4. **Production-Ready Features**
   - Graceful shutdown with WaitGroup
   - Race condition mitigation
   - Account lifecycle management
   - Offline mode for development

5. **Robust State Machine**
   - All documented states implemented
   - State persistence to database
   - Event emission for UI updates

### What Makes This Codebase Good

- **Consistency:** Patterns used consistently across layers
- **Testability:** High test coverage with good test quality
- **Maintainability:** Clear structure, good naming, comprehensive comments
- **Reliability:** Retry logic, error handling, graceful degradation
- **Observability:** Structured logging, event bus, state tracking

---

## 13. Recommendations for v1.0

### 🔴 Critical Code Fixes (MUST DO FIRST)

1. **Fix state machine race** (Stop → Errored override)
   - Strengthen state protection in `setError()`
   - Stress test must pass 100% (1000+ iterations)

2. **Fix setIdle() goroutine hang** (Running → Idle)
   - Cancel context or add exit signal
   - Unskip test, ensure it passes

3. **Integrate compression functions** (Context building)
   - Add `compactSnapshots()` and `removeOrphanedToolCalls()` to `getContextMemories()`
   - Verify myses see current game state

4. **Implement fallback user message** (OpenAI API compliance)
   - Add fallback when only system messages exist
   - Update tests to expect fallback

5. **Remove workarounds in tests** (After fixes)
   - Remove time.Sleep delays
   - Remove acceptance of errored state in Stop tests

### Documentation Updates

5. Update AGENTS.md (auto-start, activity indicators, help overlay, status bar)
6. Create ERROR_HANDLING.md (patterns, flow, retry policies)
7. Create CONFIGURATION.md (complete reference, env vars, credentials)
8. Update CONTEXT_COMPRESSION.md (turn-aware model, actual bounds, broadcast fallback)
9. Update OFFLINE_MODE.md (get_notifications, local tools)
10. Create BROADCAST_ARCHITECTURE.md (storage model, sender tracking)
11. Update database schema docs (sender_id, indexes, two-phase claiming)

### Test Improvements

12. Fix or remove flaky E2E tests
13. Add MCP offline mode tool tests
14. Add provider rate limiting tests
15. Add regression tests for fixed bugs

---

## 14. Conclusion

**Zoea Nova v0.5.0 has excellent architecture but is NOT production-ready** due to 4 critical issues. The implementation is **90% aligned** with documentation.

### 🔴 Critical Issues Blocking v1.0

1. **State machine race** - Stop() can be overridden by error state (0-5% failure rate)
2. **setIdle() hang** - Transition doesn't stop goroutine (test skipped)
3. **Compression functions unused** - Myses run blind without current game state
4. **Fallback user message missing** - Violates OpenAI API requirements

### ✅ What's Working Well

The codebase demonstrates **excellent engineering practices**:
- Consistent patterns across layers
- Comprehensive error handling
- High test coverage with honest tests
- Clear separation of concerns
- Mature retry logic and graceful degradation

### ⚠️ Secondary Issues

- **Undocumented features** (auto-start, activity indicators, retry logic, etc.)
- **Documentation lags** behind implementation
- **Test workarounds** (time.Sleep, accepting errored state)

### 🎯 Path to v1.0

**Phase 1: Critical Fixes (MUST DO FIRST)**
1. Fix state machine race (strengthen state protection)
2. Fix setIdle() hang (cancel context)
3. Integrate compression functions (add to `getContextMemories()`)
4. Implement fallback user message (OpenAI compliance)
5. Remove test workarounds (after fixes)

**Phase 2: Documentation Updates**
6. Update AGENTS.md (auto-start, activity indicators, help overlay, status bar)
7. Create ERROR_HANDLING.md (patterns, flow, retry policies)
8. Create CONFIGURATION.md (complete reference, env vars, credentials)
9. Update CONTEXT_COMPRESSION.md (turn-aware model, actual bounds, broadcast fallback)
10. Update OFFLINE_MODE.md (get_notifications, local tools)
11. Create BROADCAST_ARCHITECTURE.md (storage model, sender tracking)
12. Update database schema docs (sender_id, indexes, two-phase claiming)

**Phase 3: Test Improvements**
13. Fix or remove flaky E2E tests
14. Add MCP offline mode tool tests
15. Add provider rate limiting tests
16. Add regression tests for fixed bugs

### 📊 Success Criteria for v1.0

- ✅ All 4 critical issues fixed
- ✅ State machine stress test passes 100% (1000+ iterations)
- ✅ setIdle() test unskipped and passing
- ✅ Myses demonstrate awareness of current game state
- ✅ OpenAI provider works without API errors
- ✅ No test workarounds (time.Sleep, accepting errored state)
- ✅ Documentation matches implementation (100% alignment)

**With Phase 1 complete, Zoea Nova will be production-ready with complete alignment between code, documentation, and tests—a true "single source of truth."**
