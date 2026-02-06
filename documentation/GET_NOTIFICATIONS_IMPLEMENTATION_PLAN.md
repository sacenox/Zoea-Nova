# get_notifications Implementation Plan

**Date:** 2026-02-05  
**Status:** Tests written, ready for implementation

---

## Root Cause Summary

### The Problem

Zoea Nova's TUI always shows "T0" because myses never receive current tick information from the SpaceMolt server.

### Why This Happens

**SpaceMolt provides tick information differently based on connection type:**

1. **WebSocket clients** - Receive `state_update` messages every tick with `"tick": <number>` in payload
2. **MCP/HTTP clients** - Must call `get_notifications` tool to poll for updates (added in v0.35.0)

**Zoea Nova uses MCP** but myses have **NEVER called `get_notifications`**:
- Database evidence: 0 calls to `get_notifications` in entire history
- Database evidence: 41 tool results with `arrival_tick` (from travel commands)
- Database evidence: 0 tool results with `current_tick` or top-level `tick`

### What We Found in the Database

**Tick data EXISTS but in the wrong format:**
- `arrival_tick` values: 40711 → 41708 (range of ~1000 ticks)
- Time span: ~3 hours of gameplay
- Estimated tick rate: ~5-6 ticks per minute (~10 seconds per tick)

**Standard tool responses do NOT include current_tick:**
- `mine` → No tick
- `travel` → Only `arrival_tick` (future tick when you arrive)
- `get_status` → No tick
- `get_system` → No tick
- `get_ship` → No tick

**Only `get_notifications` includes current_tick:**
```json
{
  "tick": 41708,
  "notifications": [
    {"type": "chat", "channel": "local", "sender": "player1", "content": "Hi"},
    {"type": "combat", "attacker": "pirate", "damage": 25}
  ]
}
```

---

## API Documentation Summary

### get_notifications Tool

**Purpose:** Poll for queued game events (chat, combat, trade, faction, friend, forum, tip, system)

**Parameters:**
```json
{
  "limit": 50,                    // Optional: max notifications to return (default 50)
  "types": ["chat", "combat"],    // Optional: filter by event types
  "clear": true                   // Optional: remove from queue (default true)
}
```

**Response:**
```json
{
  "tick": 41708,                  // ← CURRENT TICK (this is what we need!)
  "notifications": [
    {"type": "chat", "channel": "local", "sender": "player1", "content": "Hello!"},
    {"type": "combat", "attacker": "pirate", "damage": 50}
  ]
}
```

**Notification Types:**
- `chat` - Messages from other players
- `combat` - Attacks, damage, scans, police
- `trade` - Trade offers, completions, cancellations
- `faction` - Invites, war declarations, member changes
- `friend` - Friend requests, online/offline status
- `forum` - Forum updates (reserved for future)
- `tip` - Gameplay tips from server
- `system` - Server announcements, misc events

**Queue Behavior:**
- Events queue up to 100 per session
- Oldest events dropped when queue fills
- `clear: false` peeks without removing
- `clear: true` (default) removes from queue

### When to Poll (from skill.md)

> - **After each action** - Check if anything happened while you acted
> - **When idle** - Poll every 30-60 seconds during downtime
> - **Before important decisions** - Make sure you're not under attack!

---

## Test Coverage

### Tests Written ✅

**File:** `internal/core/notifications_test.go` (398 lines)

**Test Functions:**
1. `TestGetNotificationsTickExtraction` - 6 subtests
   - Tick at top level
   - current_tick variant
   - Empty notifications with tick
   - Multiple events with tick
   - Nested in data wrapper
   - No tick field

2. `TestNotificationParsing` - 4 subtests
   - Chat notification
   - Multiple notification types
   - Empty notifications
   - No tick field

3. `TestGetNotificationsIntegration` - 1 test
   - Complete flow: get_notifications → tick extraction → AggregateTick

4. `TestNotificationResponseFormats` - 4 subtests
   - Standard format with events
   - Empty notifications
   - current_tick variant
   - With metadata

5. `TestNotificationEventTypes` - 1 test
   - Verifies all 7 notification types can be parsed

6. `TestNotificationPollingFrequency` - 1 test
   - Tracks polling intervals
   - Verifies tick duration calculation

7. `TestNotificationFiltering` - 6 subtests
   - Default all types
   - Limit parameter
   - Filter by type
   - Multiple type filters
   - Peek without clearing
   - Combined filters

8. `TestTickExtractionFromRealServerData` - 4 subtests
   - Travel response (has arrival_tick, NOT current_tick)
   - Mine response (no tick)
   - get_status response (no tick)
   - get_notifications response (HAS current_tick) ✅

9. `TestCalculateCurrentTickFromArrivalTick` - 3 subtests
   - Simple calculation (arrival_tick - ticks)
   - Multi-tick travel
   - Zero ticks

**Total Test Cases:** 30 subtests  
**All Tests:** ✅ PASSING

---

## Implementation Plan

### Phase 1: Add get_notifications to Mysis Turn Loop

**Location:** `internal/core/mysis.go` in the `turn()` method

**After line 492** (after tool execution loop completes), add:

```go
// Poll for notifications to get tick updates and game events
notificationResult, notifErr := a.executeToolCall(ctx, mcpProxy, provider.ToolCall{
    ID:        fmt.Sprintf("notification_poll_%d", time.Now().Unix()),
    Name:      "get_notifications",
    Arguments: json.RawMessage(`{}`),
})

// Update tick and activity state from notifications
if notifErr == nil && notificationResult != nil {
    a.updateActivityFromToolResult(notificationResult, nil)
    
    // TODO: Process notification events (chat, combat, trade, etc.)
    // For now, we only care about tick extraction
}
```

**Why this location:**
- After all tool calls have been executed
- Before the turn completes
- Ensures tick is updated every turn
- Non-blocking (part of normal turn flow)

### Phase 2: Update System Prompt

**Location:** `internal/core/prompts.go`

**Add to system prompt:**

```markdown
## Notifications

Use get_notifications to stay aware of game events:
- Call after each action to check for chat, combat, trade, and other events
- Poll every 30-60 seconds when idle
- Respond to chat messages from other players
- React to combat alerts and trade offers
```

**Add to continue prompt:**

```markdown
CRITICAL REMINDERS:
- Call get_notifications after actions to check for events and get current tick
```

### Phase 3: Handle Notification Events (Future Enhancement)

**Not required for tick display**, but useful for mysis awareness:

```go
// Parse notification events
payload, ok := parseToolResultPayload(notificationResult)
if ok {
    if notifications, ok := payload["notifications"].([]interface{}); ok {
        for _, notif := range notifications {
            notifMap, ok := notif.(map[string]interface{})
            if !ok {
                continue
            }
            
            eventType, _ := notifMap["type"].(string)
            
            switch eventType {
            case "chat":
                // Handle chat message
                // TODO: Add to memory? Respond?
            case "combat":
                // Handle combat alert
                // TODO: Defensive action?
            case "trade":
                // Handle trade offer
                // TODO: Accept/decline?
            case "tip":
                // Handle gameplay tip
                // TODO: Log to captain's log?
            }
        }
    }
}
```

### Phase 4: Offline Mode Stub

**Location:** `internal/mcp/stub.go`

**Add get_notifications to stub:**

```go
case "get_notifications":
    return &ToolResult{
        Content: []ContentBlock{
            {
                Type: "text",
                Text: `{
                    "tick": 42,
                    "notifications": []
                }`,
            },
        },
    }, nil
```

---

## Expected Behavior After Implementation

### Before Fix
- TUI shows: `T0 ⬡ [HH:MM]`
- Myses never call `get_notifications`
- `lastServerTick` remains 0 for all myses
- `AggregateTick()` returns 0

### After Fix
- TUI shows: `T41708 ⬡ [HH:MM]` (actual game tick)
- Myses call `get_notifications` after each turn
- `lastServerTick` updates every turn
- `AggregateTick()` returns max tick across swarm
- Myses receive game events (chat, combat, trade, etc.)

---

## Verification Steps

### 1. Run Tests
```bash
go test ./internal/core -run TestNotification -v
go test ./internal/core -run TestTickExtraction -v
```

### 2. Test Offline Mode
```bash
make build
./bin/zoea --offline
# Create a mysis, send a message
# Observe tick updates to T42 in status bar
```

### 3. Test with Real Server
```bash
./bin/zoea
# Create a mysis, login to game
# Observe tick updates to actual game tick (41700+)
# Check logs for [TICK DEBUG] messages
```

### 4. Verify Logs
```bash
tail -f ~/.zoea-nova/zoea.log | grep "TICK DEBUG"
```

Expected log output:
```
[TICK DEBUG] Parsed tool result payload
[TICK DEBUG] Extracted current_tick from payload tick_found=true tick=41708
[TICK DEBUG] Updated mysis lastServerTick tick=41708
[TICK DEBUG] AggregateTick called mysis_count=3 max_tick=41708
```

---

## Alternative Approaches (Not Recommended)

### Option 1: Calculate from arrival_tick

**Idea:** Use `arrival_tick - ticks` from travel responses

**Pros:**
- No additional API calls
- Works with existing data

**Cons:**
- Only works when traveling
- Inaccurate (assumes travel started immediately)
- Doesn't work for idle myses
- Doesn't provide game events

**Verdict:** ❌ Not recommended. Use `get_notifications` instead.

### Option 2: Estimate from timestamps

**Idea:** Track real-world time and estimate tick progression

**Pros:**
- No API calls needed

**Cons:**
- Highly inaccurate (tick rate varies)
- Drifts over time
- No way to sync with server
- Doesn't provide game events

**Verdict:** ❌ Not recommended. Use `get_notifications` instead.

---

## References

- **SpaceMolt Skill Guide:** https://www.spacemolt.com/skill.md
- **SpaceMolt API Docs:** https://www.spacemolt.com/api.md
- **MCP Endpoint:** https://game.spacemolt.com/mcp
- **Server Version:** v0.43.6 (as of 2026-02-05)

---

## Next Steps

1. ✅ Write comprehensive tests (COMPLETE)
2. ⏳ Implement get_notifications polling in mysis turn loop
3. ⏳ Update system prompt with notification guidance
4. ⏳ Add get_notifications to offline stub
5. ⏳ Verify tick display works in UI
6. ⏳ Remove debug logging after verification
7. ⏳ Update TICK_INVESTIGATION_FINDINGS.md with final resolution

---

## Test Results Summary

**File:** `internal/core/notifications_test.go`  
**Total Tests:** 9 test functions, 30 subtests  
**Status:** ✅ ALL PASSING

**Coverage:**
- ✅ Tick extraction from get_notifications responses
- ✅ Multiple response format variants
- ✅ Notification event type parsing
- ✅ Polling frequency tracking
- ✅ API parameter validation
- ✅ Real server data format verification
- ✅ Integration with AggregateTick

**Key Findings from Tests:**
1. ✅ Existing tick extraction logic (`findCurrentTick`) works correctly with get_notifications responses
2. ✅ Both `tick` and `current_tick` field names are supported
3. ✅ Nested tick fields are extracted correctly
4. ✅ Empty notification arrays still include tick
5. ✅ Multiple notification events can be parsed
6. ✅ Tick duration calculation works with polling intervals
7. ✅ Standard tool responses (mine, travel, get_status) correctly return 0 tick

**No code changes needed for tick extraction** - the existing logic already handles get_notifications responses correctly. We only need to:
1. Call the tool
2. Update prompts
3. Add to offline stub
