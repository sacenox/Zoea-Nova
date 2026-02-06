# LLM Thinking State Investigation Report

**Date:** 2026-02-06  
**Issue:** Missing LLM thinking state in commander view (dashboard)  
**Status:** ✅ INVESTIGATION COMPLETE

---

## Executive Summary

**Problem:** The dashboard shows mysis state (idle/running/stopped/errored) but does NOT show when a mysis is actively waiting for an LLM response (thinking).

**Current Behavior:**
- Dashboard shows animated spinner for ALL "running" myses
- No visual distinction between "running but idle" vs "running and thinking"
- Status bar shows global LLM activity (`⬥ LLM` indicator)
- Per-mysis thinking state is NOT displayed

**Root Cause:** The mysis model tracks activity state internally (`ActivityState`) but this information is NOT exposed to the TUI for display.

---

## Investigation Findings

### 1. Mysis State Machine

**File:** `documentation/MYSIS_STATE_MACHINE.md`, `internal/core/types.go`

**States (MysisState):**
- `idle` - Created or loaded but not running
- `running` - Processing messages and tool calls
- `stopped` - Explicitly stopped by user action
- `errored` - Entered due to error

**Key Finding:** There is NO separate "thinking" state in the state machine. Thinking is an **activity** within the "running" state.

---

### 2. Activity State Tracking

**File:** `internal/core/types.go:18-27`, `internal/core/mysis.go:42`

**Activity States (ActivityState):**
```go
ActivityStateIdle      ActivityState = "idle"       // Not doing anything in-game
ActivityStateTraveling ActivityState = "traveling"  // Ship is traveling
ActivityStateMining    ActivityState = "mining"     // Mining resources
ActivityStateInCombat  ActivityState = "in_combat"  // In combat
ActivityStateCooldown  ActivityState = "cooldown"   // Waiting for cooldown
```

**Mysis struct fields:**
```go
activityState          ActivityState  // Current in-game activity
activityUntil          time.Time      // When activity completes
```

**Key Finding:** `ActivityState` tracks **in-game** activities (traveling, mining, combat, cooldown) but does NOT track **LLM thinking**.

---

### 3. Event System

**File:** `internal/core/types.go:29-45`

**Network Events:**
```go
EventNetworkLLM         EventType = "network_llm"   // LLM request started/finished
EventNetworkMCP         EventType = "network_mcp"   // MCP request started/finished
EventNetworkIdle        EventType = "network_idle"  // Network activity finished
```

**Event Publishing:**
- `EventNetworkLLM` published at `internal/core/mysis.go:415-420` **before LLM call**
- `EventNetworkIdle` published at `internal/core/mysis.go:449` **after LLM call completes/fails**

**Event Handling in TUI:**
- `internal/tui/app.go:700-710` - Updates global status bar net indicator
- Sets `NetActivityLLM` or `NetActivityMCP` based on event type
- Sets `NetActivityIdle` when all activity completes

**Key Finding:** Events track **global** network activity for the status bar, but NOT per-mysis thinking state.

---

### 4. Dashboard Rendering

**File:** `internal/tui/dashboard.go:156-247`

**Function:** `renderMysisLine()`

**State Indicators (line 158-175):**
```go
switch m.State {
case "running":
    stateIndicator = spinnerView  // Animated spinner for ALL running myses
case "idle":
    stateIndicator = "◦"
case "stopped":
    stateIndicator = "◌"
case "errored":
    stateIndicator = "✖"
}
```

**MysisInfo struct (passed to render function):**
```go
type MysisInfo struct {
    ID              string
    Name            string
    State           string          // "idle", "running", "stopped", "errored"
    Provider        string
    AccountUsername string
    CreatedAt       time.Time
    LastError       string
    LastMessage     string
    LastMessageAt   time.Time
}
```

**Key Finding:** `MysisInfo` does NOT include `ActivityState` or any "thinking" flag. The dashboard cannot distinguish between:
- Running and idle (waiting for nudge/message)
- Running and thinking (waiting for LLM response)
- Running and executing tools (waiting for MCP response)

---

### 5. Data Flow Gap Analysis

**Current Data Flow:**
```
┌─────────────────────────────────────────────────────┐
│ Mysis (internal/core/mysis.go)                      │
│ - state: MysisState (idle/running/stopped/errored) │
│ - activityState: ActivityState (in-game activities)│
│ - turnMu: Mutex (LLM call in progress)             │
└──────────────────┬──────────────────────────────────┘
                   │ Commander.ListMyses()
                   │ Returns []*Mysis
                   ▼
┌─────────────────────────────────────────────────────┐
│ Commander (internal/core/commander.go)              │
│ - ListMyses() → []*Mysis                           │
└──────────────────┬──────────────────────────────────┘
                   │
                   │ TUI calls m.State(), m.Name(), etc.
                   ▼
┌─────────────────────────────────────────────────────┐
│ MysisInfoFromCore (internal/tui/dashboard.go:250)  │
│ Converts *Mysis → MysisInfo                        │
│ - Copies State, Name, Provider, etc.               │
│ - Does NOT copy ActivityState or LLM status        │
└──────────────────┬──────────────────────────────────┘
                   │
                   │ MysisInfo passed to render
                   ▼
┌─────────────────────────────────────────────────────┐
│ renderMysisLine (internal/tui/dashboard.go:156)    │
│ - Shows spinner for "running" state                │
│ - Cannot distinguish thinking vs idle              │
└─────────────────────────────────────────────────────┘
```

**Missing Links:**
1. ❌ Mysis does NOT expose "IsThinking()" or "CurrentActivity()" accessor
2. ❌ MysisInfo does NOT include LLM thinking flag
3. ❌ Dashboard render does NOT have data to show thinking state

---

## Comparison with Status Bar

**Status Bar (working correctly):**
- Shows `⬥ LLM` indicator when ANY mysis is calling LLM
- Uses `NetIndicator` component (`internal/tui/netindicator.go`)
- Updated via `EventNetworkLLM` events
- Global indicator (not per-mysis)

**Dashboard (missing per-mysis state):**
- Shows spinner for ALL running myses equally
- Cannot show which specific mysis is thinking
- No access to thinking state data

---

## Implementation Options

### Option A: Add ActivityState to MysisInfo (Minimal) ⭐ RECOMMENDED

**Description:** Expose the existing `ActivityState` field to the TUI.

**Changes Required:**

1. **Add accessor to Mysis** (`internal/core/mysis.go`):
```go
// ActivityState returns the mysis current in-game activity.
func (m *Mysis) ActivityState() ActivityState {
    a := m
    a.mu.RLock()
    defer a.mu.RUnlock()
    return a.activityState
}
```

2. **Update MysisInfo struct** (`internal/tui/dashboard.go`):
```go
type MysisInfo struct {
    ID              string
    Name            string
    State           string
    Activity        string  // NEW: "idle", "traveling", "mining", etc.
    Provider        string
    AccountUsername string
    CreatedAt       time.Time
    LastError       string
    LastMessage     string
    LastMessageAt   time.Time
}
```

3. **Update MysisInfoFromCore** (`internal/tui/dashboard.go:250`):
```go
func MysisInfoFromCore(m *core.Mysis) MysisInfo {
    return MysisInfo{
        ID:              m.ID(),
        Name:            m.Name(),
        State:           string(m.State()),
        Activity:        string(m.ActivityState()),  // NEW
        Provider:        m.ProviderName(),
        AccountUsername: m.CurrentAccountUsername(),
        CreatedAt:       m.CreatedAt(),
        LastError:       formatCoreError(m.LastError()),
    }
}
```

4. **Update renderMysisLine** (`internal/tui/dashboard.go:156`):
```go
// For running myses, show activity-specific indicator
if m.State == "running" {
    switch m.Activity {
    case "traveling":
        stateIndicator = travelingStyle.Render("→")  // Arrow for traveling
    case "mining":
        stateIndicator = miningStyle.Render("⛏")    // Pickaxe for mining
    case "in_combat":
        stateIndicator = combatStyle.Render("⚔")    // Sword for combat
    case "cooldown":
        stateIndicator = cooldownStyle.Render("⏳")  // Hourglass for cooldown
    default:
        stateIndicator = spinnerView  // Default spinner for idle/other
    }
} else if isLoading {
    stateIndicator = spinnerView
} else {
    // ... existing idle/stopped/errored indicators
}
```

**Pros:**
- ✅ Uses existing data structure (ActivityState)
- ✅ Minimal code changes (4 files, ~30 lines)
- ✅ Shows rich in-game activity (not just LLM)
- ✅ No new event handling needed

**Cons:**
- ⚠️ ActivityState does NOT include LLM thinking (only in-game activities)
- ⚠️ Would need to add ActivityStateLLMCall to track thinking separately

---

### Option B: Add IsThinking Flag (LLM-Specific)

**Description:** Add a separate flag specifically for LLM thinking state.

**Changes Required:**

1. **Add field to Mysis** (`internal/core/mysis.go:42`):
```go
type Mysis struct {
    // ... existing fields
    isThinking bool  // NEW: true when waiting for LLM response
}
```

2. **Set flag during LLM call** (`internal/core/mysis.go:415`):
```go
// Before LLM call
a.mu.Lock()
a.isThinking = true
a.mu.Unlock()

// After LLM call (line 448)
a.mu.Lock()
a.isThinking = false
a.mu.Unlock()
```

3. **Add accessor**:
```go
func (m *Mysis) IsThinking() bool {
    a := m
    a.mu.RLock()
    defer a.mu.RUnlock()
    return a.isThinking
}
```

4. **Update MysisInfo and dashboard** (similar to Option A):
```go
type MysisInfo struct {
    // ... existing fields
    IsThinking bool  // NEW
}

// In renderMysisLine:
if m.IsThinking {
    stateIndicator = thinkingStyle.Render("💭")  // Thinking bubble
} else if m.State == "running" {
    stateIndicator = spinnerView
}
```

**Pros:**
- ✅ Explicit LLM thinking state
- ✅ Easy to understand and maintain
- ✅ Simple boolean flag

**Cons:**
- ❌ Duplicates effort (ActivityState exists for similar purpose)
- ❌ Doesn't show MCP/tool activity separately
- ❌ Requires more synchronization (lock/unlock during LLM calls)

---

### Option C: Use EventNetworkLLM to Track Per-Mysis State (Event-Driven)

**Description:** Track which mysis is thinking in the TUI by listening to `EventNetworkLLM` events.

**Changes Required:**

1. **Add tracking map in TUI Model** (`internal/tui/app.go`):
```go
type Model struct {
    // ... existing fields
    thinkingMyses map[string]bool  // NEW: map[mysisID]isThinking
}
```

2. **Update event handler** (`internal/tui/app.go:700`):
```go
case core.EventNetworkLLM:
    m.netIndicator.SetActivity(NetActivityLLM)
    if event.MysisID != "" {
        if m.thinkingMyses == nil {
            m.thinkingMyses = make(map[string]bool)
        }
        m.thinkingMyses[event.MysisID] = true  // Mark as thinking
    }

case core.EventNetworkIdle:
    if event.MysisID != "" {
        delete(m.thinkingMyses, event.MysisID)  // No longer thinking
    }
    if len(m.thinkingMyses) == 0 {
        m.netIndicator.SetActivity(NetActivityIdle)
    }
```

3. **Check map in dashboard** (`internal/tui/dashboard.go:156`):
```go
// Pass thinkingMyses map to RenderDashboard
func RenderDashboard(..., thinkingMyses map[string]bool) string {
    // ...
}

// In renderMysisLine:
if thinkingMyses[m.ID] {
    stateIndicator = thinkingStyle.Render("💭")
} else if m.State == "running" {
    stateIndicator = spinnerView
}
```

**Pros:**
- ✅ No changes to core package
- ✅ Event-driven (already happening)
- ✅ Can track LLM and MCP separately

**Cons:**
- ❌ More complex (map management)
- ❌ Potential race conditions (event arrives before/after render)
- ❌ State lives in TUI, not in source of truth (core)

---

### Option D: Extend ActivityState to Include LLM/MCP (Comprehensive) ⭐⭐ BEST

**Description:** Extend `ActivityState` to track LLM and MCP activity in addition to in-game activities.

**Changes Required:**

1. **Add constants** (`internal/core/types.go:22-27`):
```go
const (
    ActivityStateIdle      ActivityState = "idle"
    ActivityStateTraveling ActivityState = "traveling"
    ActivityStateMining    ActivityState = "mining"
    ActivityStateInCombat  ActivityState = "in_combat"
    ActivityStateCooldown  ActivityState = "cooldown"
    ActivityStateLLMCall   ActivityState = "llm_call"   // NEW: Waiting for LLM
    ActivityStateMCPCall   ActivityState = "mcp_call"   // NEW: Waiting for MCP
)
```

2. **Set activity during calls** (`internal/core/mysis.go`):
```go
// Before LLM call (line 415):
a.setActivity(ActivityStateLLMCall, time.Time{})

// After LLM call (line 448):
a.setActivity(ActivityStateIdle, time.Time{})

// Before MCP call (in tool execution):
a.setActivity(ActivityStateMCPCall, time.Time{})

// After MCP call:
a.setActivity(ActivityStateIdle, time.Time{})
```

3. **Add accessor and update MysisInfo** (same as Option A)

4. **Update renderMysisLine** (`internal/tui/dashboard.go:156`):
```go
if m.State == "running" {
    switch m.Activity {
    case "llm_call":
        stateIndicator = thinkingStyle.Render("🧠")  // Brain for LLM
    case "mcp_call":
        stateIndicator = mcpStyle.Render("🎮")       // Game controller for MCP
    case "traveling":
        stateIndicator = travelingStyle.Render("→")
    case "mining":
        stateIndicator = miningStyle.Render("⛏")
    case "in_combat":
        stateIndicator = combatStyle.Render("⚔")
    case "cooldown":
        stateIndicator = cooldownStyle.Render("⏳")
    default:
        stateIndicator = spinnerView  // Default spinner
    }
}
```

**Pros:**
- ✅ Unified activity tracking (in-game + network)
- ✅ Single source of truth (ActivityState)
- ✅ Rich visual feedback (LLM, MCP, traveling, mining, etc.)
- ✅ Reuses existing setActivity/clearActivity infrastructure

**Cons:**
- ⚠️ Semantic confusion (ActivityState was originally for in-game activities)
- ⚠️ More locations to update (LLM call sites, MCP call sites)

---

## Recommended Approach

**Recommendation: Option D (Extend ActivityState)** ⭐⭐

**Rationale:**
1. **Unified Model:** ActivityState becomes the single source of truth for "what is the mysis doing right now"
2. **Rich Feedback:** Can show LLM, MCP, traveling, mining, combat, cooldown all with distinct indicators
3. **Existing Infrastructure:** Reuses `setActivity()` and `clearActivity()` helpers
4. **Event-Driven:** Already has activity state tracking, just needs expansion
5. **Scalable:** Easy to add more activities in the future (e.g., docking, trading, etc.)

**Alternative: Option A (Basic ActivityState Exposure)**
If we want a minimal change, Option A exposes the current ActivityState without adding LLM/MCP. This would show in-game activities but NOT thinking state. This could be a **Phase 1** step before implementing Option D.

---

## Implementation Plan

### Phase 1: Expose Existing ActivityState (Minimal)

1. Add `ActivityState()` accessor to Mysis
2. Add `Activity` field to MysisInfo
3. Update MysisInfoFromCore to copy activity
4. Update renderMysisLine to show activity indicators (traveling, mining, combat, cooldown)

**Effort:** 1-2 hours  
**Benefit:** Shows in-game activities in dashboard  
**Gap:** Does NOT show LLM thinking yet

### Phase 2: Add LLM/MCP Activities (Complete Solution)

5. Add `ActivityStateLLMCall` and `ActivityStateMCPCall` constants
6. Call `setActivity(ActivityStateLLMCall)` before LLM calls
7. Call `setActivity(ActivityStateMCPCall)` before MCP calls
8. Call `clearActivity()` or `setActivity(ActivityStateIdle)` after calls
9. Add thinking/MCP indicators to renderMysisLine

**Effort:** 2-3 hours  
**Benefit:** Complete per-mysis activity visualization  
**Result:** Resolves TODO item completely

---

## Visual Mockup

### Current Dashboard (Before Fix)

```
╔══════════════════════════════════════════════════════════════════════╗
║ ⠋ mysis-1    running  [ollama] @account1 │ 14:30:05 Mining ore...   ║
║ ⠋ mysis-2    running  [ollama] @account2 │ 14:29:12 Traveling...    ║
║ ◦ mysis-3    idle     [zen] (no account) │                          ║
╚══════════════════════════════════════════════════════════════════════╝
```
**Problem:** Both running myses show the same spinner. Can't tell which is thinking.

### After Fix (Phase 2 Complete)

```
╔══════════════════════════════════════════════════════════════════════╗
║ 🧠 mysis-1    running  [ollama] @account1 │ 14:30:05 Mining ore...   ║
║ → mysis-2    running  [ollama] @account2 │ 14:29:12 Traveling...    ║
║ ◦ mysis-3    idle     [zen] (no account) │                          ║
╚══════════════════════════════════════════════════════════════════════╝
```
**Solution:** 
- 🧠 = LLM thinking (ActivityStateLLMCall)
- → = Traveling (ActivityStateTraveling)
- ◦ = Idle (ActivityStateIdle)

**Alternative Icons:**
- LLM: 🧠 (brain), 💭 (thought bubble), 🤔 (thinking face)
- MCP: 🎮 (game controller), 🌐 (globe), ⚙️ (gear)
- Traveling: → (arrow), 🚀 (rocket), ✈️ (plane)
- Mining: ⛏ (pickaxe), 💎 (diamond), ⚒️ (hammer)
- Combat: ⚔️ (swords), 🗡️ (dagger), 💥 (explosion)
- Cooldown: ⏳ (hourglass), ⏱️ (stopwatch), 🕐 (clock)

**Recommendation:** Use simple Unicode characters that work in all terminals:
- LLM: `⋯` (ellipsis) or `⚡` (lightning)
- MCP: `⚙` (gear) or `↻` (refresh)
- Traveling: `→` (arrow)
- Mining: `⛏` (pickaxe)
- Combat: `⚔` (crossed swords)
- Cooldown: `⏳` (hourglass)

---

## Testing Plan

### Unit Tests

1. **Test ActivityState accessor:**
```go
func TestMysisActivityState(t *testing.T) {
    m := &Mysis{activityState: ActivityStateTraveling}
    if m.ActivityState() != ActivityStateTraveling {
        t.Errorf("expected traveling, got %v", m.ActivityState())
    }
}
```

2. **Test setActivity for LLM/MCP:**
```go
func TestMysisSetActivityLLM(t *testing.T) {
    m := &Mysis{activityState: ActivityStateIdle}
    m.setActivity(ActivityStateLLMCall, time.Time{})
    if m.activityState != ActivityStateLLMCall {
        t.Errorf("expected llm_call, got %v", m.activityState)
    }
}
```

3. **Test MysisInfo conversion:**
```go
func TestMysisInfoFromCore_Activity(t *testing.T) {
    m := &Mysis{activityState: ActivityStateLLMCall}
    info := MysisInfoFromCore(m)
    if info.Activity != "llm_call" {
        t.Errorf("expected llm_call, got %v", info.Activity)
    }
}
```

### Integration Tests

1. **Test dashboard renders activity indicators:**
```bash
go test ./internal/tui -run TestDashboard -v
```

2. **Test activity changes during LLM calls:**
```go
func TestMysisLLMCallSetsActivity(t *testing.T) {
    // Start mysis
    // Trigger LLM call
    // Check activityState == ActivityStateLLMCall
    // Wait for call to complete
    // Check activityState == ActivityStateIdle
}
```

### Manual Testing

1. Start zoea with `./bin/zoea`
2. Create 3 myses
3. Start all myses
4. Send broadcast to trigger LLM calls
5. Verify different indicators appear:
   - 🧠 or ⋯ when waiting for LLM
   - → when traveling
   - ⛏ when mining
   - ◦ when idle
6. Check status bar shows `⬥ LLM` when any mysis is thinking

---

## Code Locations Summary

| File | Lines | Purpose | Changes Needed |
|------|-------|---------|----------------|
| `internal/core/types.go` | 22-27 | ActivityState constants | Add LLMCall, MCPCall |
| `internal/core/mysis.go` | 42 | Mysis struct | Already has activityState |
| `internal/core/mysis.go` | 114-166 | Mysis accessors | Add ActivityState() |
| `internal/core/mysis.go` | 415-449 | LLM call site | Set/clear activity |
| `internal/tui/dashboard.go` | 30-44 | MysisInfo struct | Add Activity field |
| `internal/tui/dashboard.go` | 250-260 | MysisInfoFromCore | Copy activity |
| `internal/tui/dashboard.go` | 156-175 | renderMysisLine | Add activity switch |

---

## Risks and Mitigations

### Risk 1: ActivityState Lock Contention

**Issue:** Calling `setActivity()` during LLM call requires mutex lock.

**Mitigation:** 
- Use existing `a.mu` mutex (already protects activity state)
- Call setActivity() before and after LLM call (not during)
- No performance impact (lock held for nanoseconds)

### Risk 2: Activity Not Cleared on Error

**Issue:** If LLM call fails, activity might stay as "llm_call".

**Mitigation:**
- Wrap LLM calls with defer to ensure clearActivity:
```go
a.setActivity(ActivityStateLLMCall, time.Time{})
defer a.setActivity(ActivityStateIdle, time.Time{})

// LLM call
response, err = p.ChatWithTools(ctx, messages, tools)
```

### Risk 3: Visual Clutter with Too Many Icons

**Issue:** Too many different indicators may be confusing.

**Mitigation:**
- Start with minimal set: LLM (🧠), MCP (⚙), Idle (◦)
- Add in-game activities (traveling, mining, etc.) in Phase 2 if needed
- Use consistent icon theme (hexagonal, geometric, etc.)

---

## Conclusion

**Status:** ✅ Investigation complete, root cause identified, solution designed

**Key Findings:**
1. ❌ Dashboard does NOT show per-mysis thinking state
2. ✅ Mysis model tracks activity state internally (ActivityState)
3. ❌ ActivityState is NOT exposed to TUI
4. ✅ Event system tracks global LLM activity (status bar)
5. ❌ Per-mysis activity is NOT conveyed to dashboard

**Recommended Solution:** Option D - Extend ActivityState to include LLMCall and MCPCall

**Implementation Effort:**
- Phase 1 (Expose existing activity): 1-2 hours
- Phase 2 (Add LLM/MCP activity): 2-3 hours
- **Total:** 3-5 hours

**Benefits:**
- ✅ Rich per-mysis activity visualization
- ✅ Clear visual distinction between idle, thinking, traveling, mining, etc.
- ✅ Unified activity tracking model
- ✅ Scales to future activity types

**Next Steps:**
1. Review this investigation report
2. Approve recommended approach (Option D)
3. Implement Phase 1 (expose existing activity) - PR #1
4. Implement Phase 2 (add LLM/MCP activity) - PR #2
5. Update TODO.md to mark as complete

---

**Investigation By:** OpenCode Agent  
**Documentation:** `documentation/LLM_THINKING_STATE_INVESTIGATION.md`
