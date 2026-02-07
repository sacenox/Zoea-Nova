# Network Event Guarantees

**Version:** 1.0  
**Created:** 2026-02-07  
**Status:** Design Document

## Purpose

Define the contract for network activity events (`EventNetworkLLM`, `EventNetworkMCP`, `EventNetworkIdle`) to ensure the TUI network indicator correctly reflects Mysis activity.

## Problem Statement

The network indicator shows idle despite running Myses actively making LLM/MCP calls. Investigation revealed:

1. **Symptom:** Network indicator shows "⬦ IDLE" when 4 Myses are running
2. **Expected:** Indicator should show "⬥ LLM" or "⬥ MCP" when any Mysis is making network calls
3. **Root Cause:** Mismatched event publishing - `EventNetworkMCP` published without corresponding `EventNetworkIdle`

## Current Event Publishing (Broken)

### LLM Call Flow
```
1. Publish EventNetworkLLM
2. Call provider.ChatWithTools()
3. If error:
   - Publish EventNetworkIdle
   - Return error
4. If success with tool calls:
   - For each tool:
     - Publish EventNetworkMCP
     - Execute tool
     - If tool error: Publish EventNetworkIdle, return
     - If tool success: NO EVENT (BUG!)
   - Continue loop (back to step 1)
5. If success without tool calls:
   - Publish EventNetworkIdle
   - Return success
```

### The Bug

**Line 607-653** (`internal/core/mysis.go`):
```go
for _, tc := range response.ToolCalls {
    // Signal MCP activity
    a.bus.Publish(Event{Type: EventNetworkMCP, ...})  // +1 to counter
    
    result, execErr := a.executeToolCall(ctx, mcpProxy, tc)
    
    // BUG: No EventNetworkIdle published here on success!
    
    if execErr != nil && isToolTimeout(execErr) {
        a.bus.Publish(Event{Type: EventNetworkIdle, ...})  // -1 on error
        return
    }
    
    if execErr != nil && isToolRetryExhausted(execErr) {
        a.bus.Publish(Event{Type: EventNetworkIdle, ...})  // -1 on error
        return
    }
}
// Loop continues without publishing idle event
continue
```

**Result:** Counter increments on every MCP call but never decrements on success, causing it to grow unbounded.

## Truth Table: Network Events

| Scenario | Event Published | Counter Change | When |
|----------|----------------|----------------|------|
| LLM call starts | `EventNetworkLLM` | +1 | Before `provider.ChatWithTools()` |
| LLM call fails | `EventNetworkIdle` | -1 | After provider error |
| LLM returns (no tools) | `EventNetworkIdle` | -1 | After storing response |
| LLM returns (with tools) | *(none)* | 0 | Before tool execution loop |
| MCP tool call starts | `EventNetworkMCP` | +1 | Before `executeToolCall()` |
| MCP tool succeeds | **MISSING** | **0 (BUG!)** | After tool execution |
| MCP tool timeout | `EventNetworkIdle` | -1 | After timeout error |
| MCP tool retry exhausted | `EventNetworkIdle` | -1 | After retry error |
| Max iterations exceeded | `EventNetworkIdle` | -1 | After loop exit |

## Correct Event Publishing (Fixed)

### Guarantee: Every Start Event Has Matching End Event

**Rule 1:** Every `EventNetworkLLM` must have exactly one `EventNetworkIdle`  
**Rule 2:** Every `EventNetworkMCP` must have exactly one `EventNetworkIdle`  
**Rule 3:** Events must be published in the same goroutine (no race conditions)

### Fixed LLM Call Flow
```
1. Publish EventNetworkLLM                    // +1
2. Call provider.ChatWithTools()
3. If error:
   - Publish EventNetworkIdle                 // -1 (matches step 1)
   - Return error
4. If success with tool calls:
   - For each tool:
     - Publish EventNetworkMCP                // +1
     - Execute tool
     - Publish EventNetworkIdle               // -1 (matches EventNetworkMCP)
     - If tool error: return
   - Continue loop (back to step 1)
5. If success without tool calls:
   - Publish EventNetworkIdle                 // -1 (matches step 1)
   - Return success
```

## Truth Table: Fixed Network Events

| Scenario | Event Published | Counter Change | When |
|----------|----------------|----------------|------|
| LLM call starts | `EventNetworkLLM` | +1 | Before `provider.ChatWithTools()` |
| LLM call fails | `EventNetworkIdle` | -1 | After provider error |
| LLM returns (no tools) | `EventNetworkIdle` | -1 | After storing response |
| LLM returns (with tools) | *(none)* | 0 | Before tool execution loop |
| MCP tool call starts | `EventNetworkMCP` | +1 | Before `executeToolCall()` |
| **MCP tool succeeds** | **`EventNetworkIdle`** | **-1** | **After tool execution** |
| MCP tool timeout | `EventNetworkIdle` | -1 | After timeout error |
| MCP tool retry exhausted | `EventNetworkIdle` | -1 | After retry error |
| Max iterations exceeded | `EventNetworkIdle` | -1 | After loop exit |

## Implementation Plan

### Phase 1: Add Missing EventNetworkIdle

**File:** `internal/core/mysis.go`  
**Location:** Line 622 (after `a.setActivity(ActivityStateIdle, time.Time{})`)

**Change:**
```go
// Clear MCP activity state after call completes
a.setActivity(ActivityStateIdle, time.Time{})

// Signal MCP activity complete (ADD THIS)
a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})

a.updateActivityFromToolResult(result, execErr)
```

### Phase 2: Add Counter Tracking in TUI

**File:** `internal/tui/app.go`

**Add field:**
```go
activeNetworkOps  int // Count of active network operations
```

**Update event handlers:**
```go
case core.EventNetworkLLM:
    m.activeNetworkOps++
    m.netIndicator.SetActivity(NetActivityLLM)

case core.EventNetworkMCP:
    m.activeNetworkOps++
    m.netIndicator.SetActivity(NetActivityMCP)

case core.EventNetworkIdle:
    m.activeNetworkOps--
    if m.activeNetworkOps < 0 {
        m.activeNetworkOps = 0 // Safety
    }
    if m.activeNetworkOps == 0 {
        m.netIndicator.SetActivity(NetActivityIdle)
    }
    m.refreshTick()
```

### Phase 3: Add Test Coverage

**Test 1:** Verify EventNetworkIdle published after successful MCP call  
**Test 2:** Verify counter tracks multiple concurrent operations  
**Test 3:** Verify indicator stays active when operations overlap  
**Test 4:** Verify counter doesn't go negative

## Verification Checklist

- [ ] Every `EventNetworkLLM` has matching `EventNetworkIdle`
- [ ] Every `EventNetworkMCP` has matching `EventNetworkIdle`
- [ ] Counter increments on LLM/MCP events
- [ ] Counter decrements on idle events
- [ ] Indicator shows idle only when counter == 0
- [ ] No race conditions (all events in same goroutine)
- [ ] Tests verify autonomous activity tracking
- [ ] Manual testing with 4 running Myses shows active indicator

## Code Locations

- Event publishing: `internal/core/mysis.go:520-690`
- Event handling: `internal/tui/app.go:782-801`
- Network indicator: `internal/tui/netindicator.go`
- Event types: `internal/core/types.go:43-45`

## Migration Notes

**Breaking Change:** No  
**Database Migration:** No  
**Config Change:** No  
**Backward Compatible:** Yes (only adds events, doesn't remove)

---

**Version:** 1.0  
**Last Updated:** 2026-02-07
