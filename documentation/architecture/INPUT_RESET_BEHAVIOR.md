# Input Reset Behavior

**Version:** 1.0  
**Last updated:** 2026-02-07

## Purpose

This document defines the truth table for input box reset behavior across all input modes in the Zoea Nova TUI.

## Problem Statement

When sending a direct message (press 'm', type message, press Enter), the input box shows the typed message text frozen during the entire async operation (tool calling + LLM reply), instead of showing the "Sending message..." animation. The expectation is that the input box should reset immediately after pressing Enter and display the sending indicator.

## Expected Behavior

When a user sends a message (broadcast or direct), the input box should:
1. Clear immediately after pressing Enter
2. Show a "Sending..." indicator while the async operation completes
3. Return to the inactive placeholder state after completion

## Truth Table: Input State During Message Send

| Event | `m.input.Mode()` | `m.input.Value()` | `m.sending` | `ViewAlways` Output |
|-------|------------------|-------------------|-------------|---------------------|
| **Before Enter** | `InputModeMessage` | `"hello"` | `false` | Input field showing `"hello"` |
| **Enter pressed** | `InputModeMessage` ❌ | `"hello"` ❌ | `true` | Input field showing `"hello"` ❌ |
| **During async** | `InputModeMessage` ❌ | `"hello"` ❌ | `true` | Input field showing `"hello"` ❌ |
| **After async** | `InputModeNone` ✅ | `""` ✅ | `false` | Placeholder ✅ |

**Expected state after Enter:**
| Event | `m.input.Mode()` | `m.input.Value()` | `m.sending` | `ViewAlways` Output |
|-------|------------------|-------------------|-------------|---------------------|
| **Enter pressed** | `InputModeNone` ✅ | `""` ✅ | `true` | `"⬡ Sending message..."` ✅ |
| **During async** | `InputModeNone` ✅ | `""` ✅ | `true` | `"⬡ Sending message..."` ✅ |
| **After async** | `InputModeNone` ✅ | `""` ✅ | `false` | Placeholder ✅ |

## Root Cause Analysis

### ViewAlways Rendering Priority (input.go:185-202)

The `ViewAlways` function has three rendering modes with **strict priority order**:

```go
func (m InputModel) ViewAlways(width int, sending bool, sendingLabel, spinnerView string) string {
    // Priority 1: Active input mode (HIGHEST)
    if m.mode != InputModeNone {
        return inputStyle.Width(width - 2).Render(m.textInput.View())
    }
    
    // Priority 2: Sending indicator
    if sending {
        indicator := fmt.Sprintf("%s %s", spinnerView, sendingLabel)
        return inputStyle.Width(width - 2).Render(indicator)
    }
    
    // Priority 3: Placeholder (LOWEST)
    placeholder := dimmedStyle.Render("Press 'm' to message, 'b' to broadcast...")
    return inputStyle.Width(width - 2).Render(placeholder)
}
```

**Critical Issue:** ViewAlways checks `m.mode != InputModeNone` FIRST. If the input mode is still active (not reset), it ALWAYS shows the input field, regardless of the `sending` flag.

### Message Send Flow (app.go:648-659)

```go
case InputModeMessage:
    m.input.AddToHistory(value)
    targetID := m.input.TargetID()
    m.loadingSet[targetID] = true
    m.netIndicator.SetActivity(NetActivityLLM)
    m.sending = true                    // Set sending flag
    m.sendingMode = InputModeMessage    // Set mode for label
    cmd = m.sendMessageAsync(targetID, value)
    if m.view == ViewFocus {
        m.loadMysisLogs()
    }
    // NOTE: m.input.Reset() is NOT called here!
    // Input mode remains InputModeMessage
    // Input value remains in text field
```

**The Bug:** Input is NOT reset when Enter is pressed. The input mode remains `InputModeMessage` and the value remains in the text field.

### Why "Sending message..." Never Shows

1. User presses Enter
2. `m.sending = true` is set
3. `m.sendingMode = InputModeMessage` is set
4. **BUT `m.input.Reset()` is NOT called**
5. Input mode remains `InputModeMessage` (not `InputModeNone`)
6. `ViewAlways` checks `m.mode != InputModeNone` → **TRUE**
7. Returns early with input field (line 193)
8. Never reaches the `if sending` check (line 195)
9. User sees frozen input with their typed text

### When Reset Finally Happens (app.go:259-274)

```go
case sendMessageResult:
    delete(m.loadingSet, msg.mysisID)
    if msg.err != nil {
        m.err = msg.err
    }
    if len(m.loadingSet) == 0 {
        m.netIndicator.SetActivity(NetActivityIdle)
        m.sending = false
        m.sendingMode = InputModeNone
        m.input.Reset()  // ← Reset ONLY after async completes
    }
```

Reset is only called AFTER the async operation completes (which can take 5-10 seconds for LLM + tool calls).

## Fix Requirements

### Primary Fix: Reset Input Immediately After Enter

**Location:** `internal/tui/app.go:648-659` (handleInputKey, InputModeMessage case)

**Change:**
```go
case InputModeMessage:
    m.input.AddToHistory(value)
    targetID := m.input.TargetID()  // ← Save targetID BEFORE reset
    m.input.Reset()                 // ← ADD THIS LINE
    m.loadingSet[targetID] = true
    m.netIndicator.SetActivity(NetActivityLLM)
    m.sending = true
    m.sendingMode = InputModeMessage
    cmd = m.sendMessageAsync(targetID, value)
    if m.view == ViewFocus {
        m.loadMysisLogs()
    }
```

**Important:** Must save `targetID` before calling `Reset()` because `Reset()` clears `m.targetID` (input.go:208).

### Same Fix for Broadcast

**Location:** `internal/tui/app.go:628-646` (handleInputKey, InputModeBroadcast case)

**Change:**
```go
case InputModeBroadcast:
    if value == "" {
        m.input.Reset()
        return m, nil
    }
    m.input.AddToHistory(value)
    m.input.Reset()  // ← ADD THIS LINE
    // Mark all running myses as loading
    myses := m.commander.ListMyses()
    for _, mysis := range myses {
        if mysis.State() == core.MysisStateRunning {
            m.loadingSet[mysis.ID()] = true
        }
    }
    m.netIndicator.SetActivity(NetActivityLLM)
    m.sending = true
    m.sendingMode = InputModeBroadcast
    cmd = m.broadcastAsync(value)
```

### Test Coverage

1. Direct message with non-empty value → input clears immediately, shows "Sending message..."
2. Broadcast with non-empty value → input clears immediately, shows "Broadcasting..."
3. Multiple sequential messages → input clears between each send

## Test Scenarios

### Scenario 1: Send non-empty direct message
**Given:** User is in dashboard, mysis selected  
**When:** Press 'm', type "hello", press Enter  
**Then:** 
- Input box clears immediately (mode → None, value → "")
- "⬡ Sending message..." indicator shows with animated spinner
- Message is sent to mysis
- Input returns to inactive placeholder after completion

### Scenario 2: Send non-empty broadcast
**Given:** User is in dashboard  
**When:** Press 'b', type "hello", press Enter  
**Then:** 
- Input box clears immediately (mode → None, value → "")
- "⬡ Broadcasting..." indicator shows with animated spinner
- Message is broadcast to all myses
- Input returns to inactive placeholder after completion

### Scenario 3: Send multiple messages sequentially
**Given:** User is in dashboard, mysis selected  
**When:** 
1. Press 'm', type "message 1", press Enter
2. Wait for completion
3. Press 'm', type "message 2", press Enter
**Then:** 
- First message: input clears, shows "Sending message...", completes
- Second message: input clears, shows "Sending message...", completes
- No text from previous message visible at any point

## Implementation Plan

1. **Fix direct message reset** - Add `m.input.Reset()` after saving targetID in InputModeMessage case
2. **Fix broadcast reset** - Add `m.input.Reset()` after history in InputModeBroadcast case
3. **Update existing tests** - Modify `input_reset_test.go` to verify "Sending..." indicator shows
4. **Run tests** - Verify all scenarios pass
5. **Manual testing** - Verify visual behavior matches expectations

## References

- `internal/tui/app.go:611-743` - `handleInputKey` function
- `internal/tui/app.go:259-274` - `sendMessageResult` handler
- `internal/tui/app.go:276-287` - `broadcastResult` handler
- `internal/tui/input.go:204-212` - `Reset` method
