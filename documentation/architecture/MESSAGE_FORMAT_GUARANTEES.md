# Message Format Guarantees

## Purpose

This document defines how Zoea Nova ensures every LLM request has proper OpenAI-compatible message format: `[system][user][assistant?][tool?]...`

The message format guarantee is the **single source of truth** for when and how synthetic messages are added to conversation history.

---

## Core Principle

**One place, one time:** Synthetic messages are added **only** in `getContextMemories()`, nowhere else.

No other code adds or removes messages from context:
- Provider layer converts types but does not modify message lists
- Mysis loop sends context as-is from `getContextMemories()`
- Commander broadcasts and direct messages go to database, never injected mid-flight

---

## The Encouragement System

When no real user message exists in the database, `getContextMemories()` adds a synthetic encouragement message to satisfy OpenAI format requirements.

### Encouragement Counter

**Purpose:** Track consecutive synthetic messages to detect idle myses.

**Behavior:**
- Counter increments when synthetic message is added
- Counter resets to 0 when real user message (broadcast/direct) exists
- After 3 consecutive synthetic messages, mysis transitions to idle

### Truth Table

| State | User Msg in DB? | Action | Counter | Next State |
|-------|-----------------|--------|---------|------------|
| Running | Yes (broadcast) | Use real message | Reset to 0 | Running |
| Running | Yes (direct) | Use real message | Reset to 0 | Running |
| Running | No | Add synthetic | +1 (now 1) | Running |
| Running | No | Add synthetic | +1 (now 2) | Running |
| Running | No | Add synthetic | +1 (now 3) | **Idle** |
| Idle | Yes (broadcast) | Auto-start | Reset to 0 | Running |
| Idle | Yes (direct) | Start via API | Reset to 0 | Running |
| Idle | No | — | — | Idle |

---

## Message Sources

### Real User Messages (Reset Counter)

1. **Commander Broadcast** - User types `b` in TUI and sends message to all myses
2. **Direct Message** - User types `m` in TUI and targets specific mysis

All real user messages:
- Are stored in database with `role=user` and `source=broadcast|direct`
- Reset encouragement counter to 0
- Start idle myses automatically (broadcasts) or via API call (direct)

**Note:** Myses cannot broadcast to each other. Only the Commander (via TUI) can send broadcast messages to the swarm.

### Synthetic Messages (Increment Counter)

**Source:** `getContextMemories()` line 1461-1478

**When added:** No real user message found in `findLastUserPromptIndex()`

**Content:** `"Continue your mission. Check notifications and coordinate with the swarm."`

**Format:**
```go
&store.Memory{
    Role:      store.MemoryRoleUser,
    Source:    store.MemorySourceSystem,
    Content:   nudgeContent,
    SenderID:  "",
    CreatedAt: time.Now(),
}
```

**Not persisted to database** - exists only in context slice sent to LLM.

---

## Mysis Lifecycle

### Creation

```
Commander creates mysis via TUI
    ↓
Mysis starts in idle state
    ↓
No messages in database
    ↓
Waits for user message to start
```

### Running (No Broadcasts)

```
Mysis.Start() called
    ↓
Loop iteration 1:
  getContextMemories() → No user message → Add synthetic → Counter = 1
    ↓
LLM responds, tools execute
    ↓
Loop iteration 2:
  getContextMemories() → Still no user message → Add synthetic → Counter = 2
    ↓
LLM responds, tools execute
    ↓
Loop iteration 3:
  getContextMemories() → Still no user message → Add synthetic → Counter = 3
    ↓
Counter >= 3 → Transition to idle
    ↓
Waits for real user message
```

### Running (With Broadcasts)

```
Commander sends broadcast "Explore the universe!"
    ↓
Stored in database with role=user, source=broadcast
    ↓
Mysis.Start() called (or auto-started if idle)
    ↓
Loop iteration 1:
  getContextMemories() → Finds broadcast → Use it → Counter = 0
    ↓
LLM responds: "I will navigate to node_alpha..."
    ↓
Loop iteration 2:
  getContextMemories() → Broadcast still most recent → Use it → Counter = 0
    ↓
LLM responds: "[TOOL_CALLS]navigate:..."
    ↓
Loop continues until:
  - New broadcast arrives (resets counter)
  - Direct message arrives (resets counter)
  - 3 consecutive iterations with no user message (goes idle)
```

### Idle Recovery

```
Mysis idle (counter = 3, no user messages)
    ↓
Commander sends broadcast "Mine iron ore"
    ↓
Broadcast stored in database
    ↓
Mysis auto-starts (QueueBroadcast triggers Start())
    ↓
Loop iteration 1:
  getContextMemories() → Finds new broadcast → Counter = 0
    ↓
Mysis active again
```

---

## Code Locations

### Primary Implementation

**`internal/core/mysis.go:getContextMemories()`** (lines 1169-1268)
- Checks for real user messages via `findLastUserPromptIndex()`
- **Sliding Window Protection:** If no user message found in recent 20 messages, checks for broadcast in full DB
- Adds synthetic message only if no broadcast exists anywhere
- **This is the only place synthetic messages are added**

**`internal/store/memories.go:GetMostRecentBroadcast()`** (lines 233-286)
- Retrieves most recent broadcast for a mysis from full database
- **Global fallback:** If no broadcast for this mysis, returns most recent broadcast from any mysis
- Used to prevent idle state when broadcasts exist outside sliding window
- Ensures new myses inherit current swarm mission

### Counter Management

**Increment:** In `getContextMemories()` when synthetic message added (no broadcast exists)
```go
// After adding synthetic message
a.mu.Lock()
a.encouragementCount++
count := a.encouragementCount
a.mu.Unlock()

if count >= 3 {
    // Transition to idle
}
```

**Reset:** When real user message arrives OR when broadcast found in DB
```go
// In QueueBroadcast (line 719) and SendMessageFrom (line 392)
a.mu.Lock()
a.encouragementCount = 0
a.mu.Unlock()

// Also in getContextMemories when broadcast found outside sliding window (line 1211)
a.mu.Lock()
a.encouragementCount = 0
a.mu.Unlock()
```

### Obsolete Code (To Be Removed)

**`internal/provider/openai_common.go`:**
- Lines 166-173: "Begin." fallback - **Remove**
- Lines 175-184: "Continue." fallback - **Remove**

**`internal/core/mysis.go`:**
- Lines 1676-1727: Ticker-based nudge system - **Remove**
- Field `nudgeCh chan struct{}` - **Remove**
- Method `buildContinuePrompt()` with escalation - **Remove**

---

## OpenAI Format Requirements

The OpenAI Chat Completions API requires:
1. **System messages first** - Handled by provider message conversion
2. **At least one user message** - Guaranteed by encouragement system
3. **Alternating user/assistant** - Natural conversation flow
4. **Tool results follow tool calls** - Handled by tool loop logic

The encouragement system ensures requirement #2. The other requirements are satisfied by normal conversation flow and provider message conversion.

---

## Sliding Window Protection

### The Problem

Context is limited to the most recent 20 messages (`MaxContextMessages = 20`). When a mysis processes a broadcast and generates 20+ messages (assistant responses, tool calls, tool results), the original broadcast is pushed out of the sliding window.

**Example:**
1. Broadcast stored: "Explore the universe!" (message #1)
2. Mysis processes it, generates 24 messages (8 iterations × 3 messages each)
3. Total: 25 messages in database
4. `GetRecentMemories(20)` returns only messages #6-25
5. Broadcast (message #1) is excluded
6. `findLastUserPromptIndex` returns -1 (no user message found)
7. Synthetic encouragement added, counter increments
8. After 3 iterations: mysis incorrectly goes idle despite having mission directive

### The Solution

When `findLastUserPromptIndex` returns -1 (no user message in sliding window), `getContextMemories` performs a secondary check:

1. Query database for most recent broadcast: `GetMostRecentBroadcast(mysisID)`
2. If broadcast exists (even outside window), include it in context
3. Reset encouragement counter to 0
4. Mysis continues running with mission directive

**Code Location:** `internal/core/mysis.go:1203-1225`

**Global Fallback:** `GetMostRecentBroadcast` has two-tier lookup:
1. First, search for broadcasts sent to this specific mysis
2. If none found, search for most recent Commander broadcast in entire system (global swarm mission)
3. This ensures new myses created after a Commander broadcast inherit the current mission

**Benefits:**
- Ensures autonomous operation: myses stay running as long as broadcasts exist
- New myses inherit swarm mission: no idle state for late joiners
- Minimal token cost: adds 1 broadcast message to context
- Maintains truth table guarantees: broadcasts always keep myses running

**Test Coverage:** 
- `TestBroadcastSlidingWindowBug` (internal/core/mysis_test.go:2397)
- `TestNewMysisInheritsGlobalBroadcast` (internal/core/mysis_test.go:2507)

---

## Design Rationale

### Why One Place?

**Clarity:** Anyone debugging message format issues knows to check `getContextMemories()` only.

**Correctness:** No risk of multiple code paths adding conflicting synthetic messages.

**Testability:** Mock database content, call `getContextMemories()`, verify output.

### Why Not Provider Layer?

The provider layer converts message types (`store.Memory` → `openai.ChatCompletionMessage`). It should not modify message lists.

Adding synthetic messages in the provider layer would:
- Hide the logic far from where context is built
- Make it harder to track encouragement counter
- Duplicate logic across multiple providers (OpenCode, Ollama, etc.)

### Why Not Ticker?

A ticker nudges every 30 seconds regardless of database state. This creates synthetic messages even when real user messages exist.

The ticker-based approach:
- Adds unnecessary messages to conversation history
- Increments counter based on time, not actual message presence
- Conflicts with database-driven context building

---

## Migration Path

**Current state:** Ticker-based nudges + provider-layer fallbacks + context-layer synthetic messages

**Target state:** Context-layer synthetic messages only

**Steps:**
1. Document this architecture (this file)
2. Remove ticker goroutine and `nudgeCh` from `mysis.go`
3. Remove "Begin." and "Continue." fallbacks from `openai_common.go`
4. Rename `nudgeFailCount` to `encouragementCount` for clarity
5. Move counter increment/reset to `getContextMemories()` and message handlers
6. Update tests to verify new behavior

---

## Testing Strategy

### Unit Tests

**Test:** `getContextMemories()` with no user messages
- **Verify:** Synthetic message added
- **Verify:** Counter increments

**Test:** `getContextMemories()` with broadcast in DB
- **Verify:** Real message used
- **Verify:** Counter resets to 0

**Test:** 3 consecutive calls with no user messages
- **Verify:** Counter reaches 3
- **Verify:** Mysis transitions to idle

### Integration Tests

**Test:** Create mysis, start without broadcasts
- **Verify:** Runs 3 iterations
- **Verify:** Goes idle

**Test:** Create mysis, send broadcast, start
- **Verify:** Uses broadcast message
- **Verify:** Counter stays at 0
- **Verify:** Continues running

**Test:** Idle mysis receives broadcast
- **Verify:** Auto-starts
- **Verify:** Counter resets

---

## Summary

- **One source of truth:** `getContextMemories()` adds synthetic messages
- **Counter tracks encouragements:** 3 consecutive = idle
- **Real messages reset counter:** Broadcasts and directs keep myses running
- **No ticker, no provider fallbacks:** Obsolete patterns removed
- **Database-driven:** Context built from persisted messages only

This design ensures every LLM request has proper OpenAI format while keeping myses idle when no real user messages exist.

---

Last updated: 2026-02-07
