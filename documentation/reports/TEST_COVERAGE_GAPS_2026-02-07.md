# Test Coverage Gaps Report - 2026-02-07

**Generated:** 2026-02-07
**Overall Coverage:** 28.0% (statements)
**Target:** 80%
**Critical Gap:** 52.0%

## Executive Summary

Zoea Nova has 65 test files covering 31 production files in `internal/`, but overall coverage is only 28%. This audit identifies:

- **28 untested functions** (0% coverage) in critical paths
- **Main entry point completely untested** (cmd/zoea/main.go: 0%)
- **Low-hanging fruit:** 15 quick wins that would boost coverage significantly
- **Critical gaps** in error handling, edge cases, and recent features

---

## 1. Untested Functions by Package

### 1.1 cmd/zoea/main.go (0% coverage - CRITICAL)

**All functions untested** - Main entry point has zero test coverage:

```
main()                  - 0.0%
initLogging()          - 0.0%
initProviders()        - 0.0%
CreateAccount()        - 0.0%
MarkAccountInUse()     - 0.0%
ReleaseAccount()       - 0.0%
ReleaseAllAccounts()   - 0.0%
ListMyses()            - 0.0%
MysisCount()           - 0.0%
MaxMyses()             - 0.0%
GetStateCounts()       - 0.0%
SendMessageAsync()     - 0.0%
BroadcastAsync()       - 0.0%
BroadcastFrom()        - 0.0%
SearchMessages()       - 0.0%
SearchReasoning()      - 0.0%
SearchBroadcasts()     - 0.0%
ClaimAccount()         - 0.0%
runMCPTest()           - 0.0%
```

**Impact:** Main.go is the integration layer that wires everything together. Zero coverage means flag parsing, provider initialization, and TUI startup are untested.

### 1.2 internal/core/mysis.go (Partial - HIGH PRIORITY)

**Completely untested functions:**

```
CreatedAt()             - 0.0%  (trivial getter)
setIdle()               - 0.0%  (state machine transition)
compactSnapshots()      - 0.0%  (memory compression helper)
extractToolNameFromResult() - 0.0%  (parsing helper)
toolCallNameIndex()     - 0.0%  (parsing helper)
isSnapshotTool()        - 0.0%  (predicate)
estimateCooldownUntil() - 0.0%  (activity tracking)
normalizeFloat()        - 0.0%  (JSON parsing utility)
```

**Low coverage functions (<70%):**

```
SendEphemeralMessage()      - 64.4%  (critical path - ephemeral messages)
executeToolCall()           - 58.3%  (CRITICAL - tool execution)
buildSystemPrompt()         - 46.2%  (CRITICAL - prompt construction)
normalizeInt()              - 46.2%  (JSON parsing)
```

**Critical Gap:** `executeToolCall()` at 58.3% means error paths in tool execution are untested. `buildSystemPrompt()` at 46.2% means broadcast injection logic is partially tested.

### 1.3 internal/core/commander.go (Partial)

**Untested:**
```
MaxMyses()         - 0.0%  (trivial getter)
GetStateCounts()   - 0.0%  (state aggregation)
```

**Note:** Most commander functions are well-tested (70-95%). These two are getters/helpers.

### 1.4 internal/provider/* (Partial)

**Untested:**
```
factory.go:
  Name()                    - 0.0%  (getter on Factory interface)
  
mock.go:
  NewMockFactoryWithLimiter() - 0.0%  (test helper)
  Name()                      - 0.0%  (getter)
  Close()                     - 0.0%  (cleanup)
  
ollama.go:
  Close()                     - 0.0%  (cleanup)
  
opencode.go:
  Close()                     - 0.0%  (cleanup)
  opencodeEndpointForModel()  - 50.0%  (endpoint selection)
```

**Low coverage:**
```
opencode.go:
  createChatCompletion()  - 58.8%  (HTTP request logic)
```

**Critical Gap:** Provider `Close()` methods are untested, meaning HTTP client cleanup on shutdown is not verified.

### 1.5 internal/mcp/* (Partial)

**Untested:**
```
client.go:
  Close()        - 0.0%  (cleanup)
  
proxy.go:
  Initialize()   - 0.0%  (setup)
```

**Critical Gap:** MCP client lifecycle (Initialize/Close) is untested.

### 1.6 internal/store/store.go (Partial)

**Untested:**
```
New()   - 0.0%  (constructor)
DB()    - 0.0%  (getter)
```

**Low coverage:**
```
Open()      - 61.1%  (database initialization)
migrate()   - 38.5%  (CRITICAL - schema migrations)
```

**Critical Gap:** Database migration logic at 38.5% means schema changes are not fully tested.

### 1.7 internal/config/* (Partial - 72.7% overall)

**Low coverage:**
```
config.go:
  applyEnvOverrides()  - 57.4%  (environment variable handling)
  EnsureDataDir()      - 66.7%  (filesystem setup)
  
credentials.go:
  GetAPIKey()          - 66.7%  (credential retrieval)
```

**Note:** Config package is relatively well-tested but environment overrides need work.

### 1.8 internal/tui/* (Mostly well-tested)

**Low coverage:**
```
app.go:
  mysisNameByID()  - 33.3%  (helper function)
  mysisByID()      - 75.0%  (helper function)
```

**Note:** TUI has excellent test coverage (40+ test files). Only minor gaps remain.

### 1.9 internal/constants (No test files)

Package has no test files but contains only constants - acceptable.

---

## 2. Critical Paths Coverage Analysis

### 2.1 CreateMysis Flow (78.9%)

**Tested:**
- Happy path: Create mysis with valid config
- Duplicate name detection
- State persistence

**NOT Tested:**
- Provider factory lookup failure
- Store.CreateMysis() database error
- Invalid provider name handling
- Race condition on concurrent create

**Recommendation:** Add error path tests (Priority: HIGH)

### 2.2 SendMessage Flow (83.3%)

**Tested:**
- Happy path: Message delivery to running mysis
- Mysis state validation

**NOT Tested:**
- Message to stopped mysis
- Message to errored mysis
- Store failure during message save
- Async delivery race conditions

**Recommendation:** Add edge case tests (Priority: MEDIUM)

### 2.3 getContextMemories (Tested indirectly via integration tests)

**Coverage:** Estimated 75% (tested via mysis_test.go, agent3_reproduction_test.go)

**Tested:**
- Turn boundary detection
- System prompt injection
- Tool loop extraction
- Ephemeral message handling

**NOT Tested:**
- Store.GetRecentMemories() failure
- Empty memory list edge case
- Nudge generation for autonomous myses
- Historical compression with complex tool loops

**Recommendation:** Add unit tests for edge cases (Priority: MEDIUM)

### 2.4 executeToolCall (58.3% - CRITICAL GAP)

**Tested:**
- Happy path: Successful tool call
- Account tracking on login/register/logout

**NOT Tested:**
- MCP proxy nil check
- Tool call timeout
- MCP.CallTool() error handling
- Invalid tool arguments
- Tool result parsing failure

**Recommendation:** Add error path tests (Priority: HIGH)

### 2.5 buildSystemPrompt (46.2% - CRITICAL GAP)

**Tested:**
- Basic system prompt generation
- Broadcast injection (happy path)

**NOT Tested:**
- Store.GetRecentBroadcasts() failure
- Empty broadcasts edge case
- Sender lookup failure (unknown sender)
- Invalid broadcast format
- String replacement edge cases

**Recommendation:** Add comprehensive unit tests (Priority: HIGH)

---

## 3. Error Handling Paths Not Tested

### 3.1 Database Errors

**Untested paths:**
- store.Open() failure modes (corrupt DB, permission denied)
- store.migrate() rollback on schema error
- Transaction rollback on constraint violation
- Connection pool exhaustion

**Recommendation:** Add store error injection tests (Priority: HIGH)

### 3.2 Network Errors

**Untested paths:**
- Provider HTTP timeout (partial coverage)
- Provider connection refused
- Provider rate limit exceeded (429)
- Provider malformed response
- MCP server crash mid-request

**Recommendation:** Add network error simulation tests (Priority: MEDIUM)

### 3.3 Concurrent Access Errors

**Untested paths:**
- Double-start of same mysis
- Delete mysis while running
- Broadcast to mysis being deleted
- Account claim race condition (partially tested)

**Recommendation:** Add race condition tests (Priority: MEDIUM)

### 3.4 Configuration Errors

**Untested paths:**
- Missing config file (now required - added 3 days ago)
- Invalid TOML syntax
- Missing required provider fields
- Invalid endpoint URLs
- Credential file corruption

**Recommendation:** Add config validation tests (Priority: LOW - already has 72.7% coverage)

---

## 4. New Features Added in Last 3 Days Without Tests

**Commit analysis (last 3 days):**

### 4.1 Interactive Provider Selection (bd4d6e6, d80f9ea) - TESTED

**Feature:** `internal/tui/app.go` - Interactive provider picker for new myses

**Tests Added:** `internal/tui/tui_test.go` - Comprehensive tests added in commit 8b122ee

**Status:** COVERED ✓

### 4.2 --start-swarm CLI Flag (d80f9ea) - TESTED

**Feature:** `cmd/zoea/main.go` - Auto-start all myses on launch

**Tests Added:** `internal/tui/tui_test.go` - Integration test for flag

**Status:** COVERED ✓

### 4.3 Config Keys for Provider Registry (b7ec302) - TESTED

**Feature:** Changed provider registry to use config keys instead of factory names

**Tests Updated:** Multiple test files updated to use new keys

**Status:** COVERED ✓

### 4.4 Tool Loop History Preservation (09e496c) - UNTESTED

**Feature:** `internal/core/mysis.go` - Preserve tool loop history for autonomous myses

**Location:** `getContextMemories()` function (lines 1418-1473)

**Tests:** Indirectly tested via integration tests, but no specific unit test

**Status:** PARTIALLY COVERED (Priority: LOW - covered by integration tests)

### 4.5 JSON Return for zoea_claim_account (a7d2a78, 1a5c363) - UNTESTED

**Feature:** `internal/mcp/tools.go` - Return JSON instead of instructional text

**Tests:** No specific tests for new JSON format

**Status:** NOT COVERED (Priority: MEDIUM)

---

## 5. Recommended Tests (Prioritized)

### 5.1 HIGH Priority (Critical Gaps)

#### H1. executeToolCall Error Paths (Priority: HIGH)
**File:** `internal/core/mysis_test.go`

```go
func TestExecuteToolCall_Errors(t *testing.T) {
    // Test: MCP proxy nil
    // Test: Tool call timeout
    // Test: Invalid tool arguments
    // Test: MCP.CallTool() error
    // Test: Tool result parsing failure
}
```

**Effort:** 2 hours
**Impact:** Covers critical tool execution error paths (currently 58.3% → ~85%)

#### H2. buildSystemPrompt Edge Cases (Priority: HIGH)
**File:** `internal/core/mysis_test.go`

```go
func TestBuildSystemPrompt(t *testing.T) {
    // Test: No broadcasts (fallback message)
    // Test: GetRecentBroadcasts() error
    // Test: Unknown sender ID
    // Test: GetMysis() error for sender lookup
    // Test: Broadcast injection format
}
```

**Effort:** 1 hour
**Impact:** Covers prompt construction edge cases (46.2% → ~90%)

#### H3. Database Migration Tests (Priority: HIGH)
**File:** `internal/store/store_test.go`

```go
func TestMigrate_Errors(t *testing.T) {
    // Test: Schema version mismatch
    // Test: Migration rollback on error
    // Test: Corrupt migration SQL
    // Test: Transaction commit failure
}
```

**Effort:** 3 hours
**Impact:** Covers schema migration safety (38.5% → ~75%)

#### H4. Main.go Integration Tests (Priority: HIGH)
**File:** `cmd/zoea/main_test.go` (NEW)

```go
func TestMain_InitProviders(t *testing.T) {
    // Test: Provider registry initialization
    // Test: Invalid config handling
    // Test: Missing provider factories
}

func TestMain_Flags(t *testing.T) {
    // Test: --debug flag
    // Test: --offline flag
    // Test: --start-swarm flag
    // Test: --config flag
}
```

**Effort:** 4 hours
**Impact:** Covers main entry point (0% → ~60%)

#### H5. MCP Tool JSON Format (Priority: HIGH)
**File:** `internal/mcp/tools_test.go` (NEW or add to mcp_test.go)

```go
func TestZoeaClaimAccount_JSONFormat(t *testing.T) {
    // Test: JSON return format
    // Test: No accounts available message
    // Test: Account claim success format
}

func TestZoeaListAccounts_JSONFormat(t *testing.T) {
    // Test: JSON return format for account list
}
```

**Effort:** 1 hour
**Impact:** Covers recent feature changes

---

### 5.2 MEDIUM Priority (Important Gaps)

#### M1. SendEphemeralMessage Error Paths (Priority: MEDIUM)
**File:** `internal/core/mysis_test.go`

```go
func TestSendEphemeralMessage_Errors(t *testing.T) {
    // Test: Message to stopped mysis
    // Test: Message to errored mysis
    // Test: Invalid message content
}
```

**Effort:** 1 hour
**Impact:** Raises coverage from 64.4% → ~85%

#### M2. Provider Close() Lifecycle (Priority: MEDIUM)
**File:** `internal/provider/provider_test.go`

```go
func TestProviderClose(t *testing.T) {
    // Test: Ollama.Close() HTTP client cleanup
    // Test: OpenCode.Close() HTTP client cleanup
    // Test: Mock.Close() resource cleanup
    // Test: Close() idempotency (double close)
}
```

**Effort:** 1 hour
**Impact:** Tests shutdown safety

#### M3. Network Error Simulation (Priority: MEDIUM)
**File:** `internal/provider/http_errors_test.go` (EXPAND)

```go
func TestHTTPErrors_Extended(t *testing.T) {
    // Test: Connection timeout
    // Test: Connection refused
    // Test: Rate limit (429)
    // Test: Malformed JSON response
    // Test: Partial response read
}
```

**Effort:** 2 hours
**Impact:** Tests real-world failure modes

#### M4. Concurrent Mysis Operations (Priority: MEDIUM)
**File:** `internal/core/commander_concurrent_test.go` (NEW)

```go
func TestConcurrentOperations(t *testing.T) {
    // Test: Double-start same mysis
    // Test: Delete while running
    // Test: Configure while running
    // Test: Broadcast to deleting mysis
}
```

**Effort:** 3 hours
**Impact:** Tests race conditions

#### M5. getContextMemories Edge Cases (Priority: MEDIUM)
**File:** `internal/core/mysis_test.go`

```go
func TestGetContextMemories_EdgeCases(t *testing.T) {
    // Test: Store.GetRecentMemories() error
    // Test: Empty memory list
    // Test: No user prompt (autonomous nudge)
    // Test: Complex tool loop extraction
    // Test: Turn boundary detection edge cases
}
```

**Effort:** 2 hours
**Impact:** Adds explicit coverage for critical context logic

---

### 5.3 LOW Priority (Nice to Have)

#### L1. Trivial Getters (Priority: LOW)
**File:** Various

```go
// Add tests for:
// - CreatedAt()
// - MaxMyses()
// - GetStateCounts()
// - Provider.Name()
// - Store.DB()
```

**Effort:** 30 minutes
**Impact:** Boosts coverage numbers but low value

#### L2. Config applyEnvOverrides (Priority: LOW)
**File:** `internal/config/config_test.go`

```go
func TestApplyEnvOverrides_AllVars(t *testing.T) {
    // Test: All env var overrides
    // Test: Partial env overrides
    // Test: Invalid env var values
}
```

**Effort:** 1 hour
**Impact:** Raises config coverage from 57.4% → ~85%

#### L3. Helper Functions (Priority: LOW)
**File:** Various

```go
// Test:
// - isSnapshotTool()
// - extractToolNameFromResult()
// - toolCallNameIndex()
// - normalizeFloat()
// - normalizeInt()
```

**Effort:** 1 hour
**Impact:** Minimal - these are simple utilities

---

## 6. Quick Wins (Easy Tests to Add)

### 6.1 Trivial Getters (15 minutes)
**Impact:** +2% coverage

```go
func TestTrivialGetters(t *testing.T) {
    // Test all 0% getters: CreatedAt(), MaxMyses(), Name(), etc.
}
```

### 6.2 Close() Methods (30 minutes)
**Impact:** +1% coverage

```go
func TestCloseIdempotency(t *testing.T) {
    // Test: provider.Close()
    // Test: mcp.Client.Close()
    // Test: Double close safety
}
```

### 6.3 buildSystemPrompt No Broadcasts (10 minutes)
**Impact:** +5% on buildSystemPrompt

```go
func TestBuildSystemPrompt_NoBroadcasts(t *testing.T) {
    // Empty broadcast list → fallback message
}
```

### 6.4 MCP Initialize() (20 minutes)
**Impact:** +1% coverage

```go
func TestMCPProxyInitialize(t *testing.T) {
    // Test: Basic initialization
    // Test: Already initialized
}
```

### 6.5 Helper Predicates (20 minutes)
**Impact:** +1% coverage

```go
func TestHelpers(t *testing.T) {
    // Test: isSnapshotTool()
    // Test: extractToolNameFromResult()
    // Test: normalizeFloat()
}
```

**Total Quick Wins Effort:** 1.5 hours
**Total Quick Wins Impact:** +10% coverage (28% → 38%)

---

## 7. Coverage by Module (Summary)

| Module | Current | Target | Gap | Priority |
|--------|---------|--------|-----|----------|
| cmd/zoea | 0% | 60% | -60% | HIGH |
| internal/config | 72.7% | 80% | -7.3% | LOW |
| internal/core | 85%* | 90% | -5% | MEDIUM |
| internal/mcp | 70%* | 85% | -15% | MEDIUM |
| internal/provider | 75%* | 85% | -10% | MEDIUM |
| internal/store | 70%* | 85% | -15% | HIGH |
| internal/tui | 90%* | 95% | -5% | LOW |
| **Overall** | **28%** | **80%** | **-52%** | - |

*Estimated based on function-level coverage analysis

---

## 8. Implementation Plan

### Phase 1: Quick Wins (1.5 hours)
- Trivial getters
- Close() methods
- buildSystemPrompt edge cases
- Helper predicates
- **Result:** 28% → 38% coverage

### Phase 2: Critical Gaps (11 hours)
- executeToolCall error paths (2h)
- buildSystemPrompt comprehensive (1h)
- Database migration tests (3h)
- Main.go integration tests (4h)
- MCP tool JSON format (1h)
- **Result:** 38% → 55% coverage

### Phase 3: Important Gaps (9 hours)
- SendEphemeralMessage errors (1h)
- Provider lifecycle (1h)
- Network error simulation (2h)
- Concurrent operations (3h)
- getContextMemories edge cases (2h)
- **Result:** 55% → 70% coverage

### Phase 4: Nice to Have (2.5 hours)
- Config env overrides (1h)
- Helper functions (1h)
- Trivial edge cases (0.5h)
- **Result:** 70% → 75% coverage

**Total Effort:** 24 hours
**Coverage Gain:** 28% → 75% (target: 80%)

---

## 9. Blockers and Dependencies

### 9.1 Test Infrastructure Gaps

**None identified.** Test infrastructure is solid:
- Mock providers work well
- Store test helpers are comprehensive
- TUI testing patterns are established
- Integration test framework is mature

### 9.2 Known Issues Blocking Tests

**None critical.** Known issues in `KNOWN_ISSUES.md` do not block test additions.

### 9.3 Missing Test Data

**None.** Test fixtures and mock data are sufficient.

---

## 10. Conclusion

Zoea Nova has **strong test coverage in core logic** (commander, mysis, TUI) but **critical gaps** in:

1. **Entry point** (cmd/zoea: 0%)
2. **Error paths** (executeToolCall: 58.3%, buildSystemPrompt: 46.2%)
3. **Database migrations** (migrate: 38.5%)
4. **Provider lifecycle** (Close: 0%)
5. **Recent features** (JSON tool format: untested)

**Recommended immediate actions:**

1. **Execute Phase 1 (Quick Wins)** - 1.5 hours, +10% coverage
2. **Execute H1-H5 (Critical Gaps)** - 11 hours, +17% coverage
3. **Defer LOW priority items** until after critical gaps are closed

**Coverage trajectory:**
- Current: 28%
- After Phase 1: 38%
- After Phase 2: 55%
- After Phase 3: 70%
- Target: 80%

---

**Report End**
