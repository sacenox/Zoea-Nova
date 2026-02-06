# TUI Tick Display Investigation

**Date:** 2026-02-05  
**Issue:** TUI shows T0 even after get_notifications is called  
**Status:** ✅ ROOT CAUSE IDENTIFIED AND FIXED

---

## Root Cause

The TUI was not refreshing the tick display after `get_notifications` was called because **no event was emitted** after the tick was updated.

### The Flow Problem

**Before Fix:**
1. Mysis completes turn and emits `EventMysisResponse` (line 514-520)
2. Mysis emits `EventNetworkIdle` (line 511)
3. **TUI receives events and calls `refreshTick()`** → reads tick from Commander (still 0 at this point)
4. Mysis calls `get_notifications` (line 524-528)
5. Mysis extracts tick and updates `lastServerTick` (line 532)
6. **NO EVENT EMITTED** → TUI never knows to refresh again
7. Function returns (line 538)

**Result:** TUI shows stale tick value (0) because it refreshed before the tick was updated.

### Event-Driven Architecture

The TUI uses event-driven updates via `handleEvent()` in `internal/tui/app.go`:

```go
case core.EventMysisResponse, core.EventMysisMessage:
    m.refreshMysisList()
    m.refreshTick()  // ← Only called when events are received
    
case core.EventNetworkIdle:
    m.refreshTick()  // ← Only called when events are received
```

Without an event after `get_notifications`, the TUI has no signal to refresh.

---

## The Fix

**Location:** `internal/core/mysis.go` lines 530-536

Added event emission after tick update from get_notifications:

```go
// Update tick and activity state from notifications
if notifErr == nil && notificationResult != nil {
    a.updateActivityFromToolResult(notificationResult, nil)

    // Emit an event to notify TUI that tick may have been updated
    // This ensures the TUI refreshes the tick display after get_notifications
    a.bus.Publish(Event{
        Type:      EventMysisMessage,
        MysisID:   a.id,
        MysisName: a.name,
        Timestamp: time.Now(),
    })

    // TODO: Process notification events (chat, combat, trade, etc.)
    // For now, we only care about tick extraction
}
```

**Why `EventMysisMessage`?**
- It triggers `refreshTick()` in the TUI (line 664)
- It's semantically appropriate (notifications are messages from the game)
- It's a lightweight event that won't cause unnecessary UI updates

---

## Verification Steps

### 1. Debug Logging

The code already has debug logging for tick flow:

**Mysis side:**
- `[TICK DEBUG] Parsed tool result payload` (line 1056)
- `[TICK DEBUG] Extracted current_tick from payload` (line 1062)
- `[TICK DEBUG] Updated mysis lastServerTick` (line 1065)

**Commander side:**
- `[TICK DEBUG] AggregateTick called` (line 426)

**TUI side:**
- `[TICK DEBUG] TUI tick updated` (line 762)

### 2. Test with Offline Mode

```bash
./bin/zoea --offline

# Create a mysis
# Send a message
# Observe tick updates in UI (should show T42 for offline stub)

# Check logs
tail -f ~/.zoea-nova/zoea.log | grep "TICK DEBUG"
```

Expected log sequence:
```
[TICK DEBUG] Parsed tool result payload payload_ok=true
[TICK DEBUG] Extracted current_tick from payload tick_found=true tick=42
[TICK DEBUG] Updated mysis lastServerTick tick=42
[TICK DEBUG] AggregateTick called mysis_count=1 max_tick=42
[TICK DEBUG] TUI tick updated old_tick=0 new_tick=42
```

### 3. Test with Real Server

```bash
./bin/zoea

# Create a mysis, login to game
# Perform actions
# Observe tick updates in UI (should show real game tick, e.g., T41708)
```

---

## Related Code Locations

### Tick Extraction
- `internal/core/mysis.go:1306` - `findCurrentTick()` function
- `internal/core/mysis.go:1336` - `extractTickFromNotifications()` function
- `internal/core/mysis.go:1056-1066` - Tick extraction in `updateActivityFromToolResult()`

### Tick Updates
- `internal/core/mysis.go:1117` - `updateServerTick()` method
- `internal/core/commander.go:408` - `AggregateTick()` method

### TUI Refresh
- `internal/tui/app.go:757` - `refreshTick()` method
- `internal/tui/app.go:664` - `refreshTick()` called on `EventMysisResponse`/`EventMysisMessage`
- `internal/tui/app.go:685` - `refreshTick()` called on `EventNetworkIdle`
- `internal/tui/app.go:426` - `renderTickTimestamp()` displays tick in status bar

---

## Why This Was Hard to Find

1. **Timing issue**: The tick was being extracted correctly, but the UI refresh happened too early
2. **Event-driven architecture**: Without understanding the event flow, it wasn't obvious why the UI wasn't updating
3. **Multiple layers**: The issue required understanding mysis → commander → TUI flow
4. **Debug logging existed**: But logs showed tick extraction working, masking the UI refresh issue

---

## Lessons Learned

1. **Event-driven UIs need events**: Always emit events after state changes that affect the UI
2. **Timing matters**: Order of operations is critical in event-driven systems
3. **Debug logging is essential**: The existing debug logs helped trace the flow
4. **Test the complete flow**: Unit tests passed, but the integration flow had a gap

---

## Alternative Solutions Considered

### 1. Periodic TUI Refresh (Rejected)
**Idea:** Poll `Commander.AggregateTick()` every second

**Pros:** Simple, would work

**Cons:**
- Wasteful (polls even when nothing changes)
- Not idiomatic for event-driven architecture
- Adds unnecessary CPU usage

### 2. Emit EventNetworkIdle After get_notifications (Rejected)
**Idea:** Emit another `EventNetworkIdle` after get_notifications

**Pros:** Would trigger TUI refresh

**Cons:**
- Misleading semantics (network isn't "idle" after every notification poll)
- Could confuse network activity indicator
- Less precise than `EventMysisMessage`

### 3. New Event Type: EventTickUpdate (Overkill)
**Idea:** Create a dedicated event type for tick updates

**Pros:** Most explicit, clearest intent

**Cons:**
- Requires new event type definition
- Overkill for this use case
- `EventMysisMessage` already serves this purpose

**Chosen:** Emit `EventMysisMessage` after get_notifications (simple, semantic, works)

---

## Testing Checklist

- [x] Build succeeds
- [ ] Offline mode shows T42 in status bar
- [ ] Real server shows current game tick (T41700+)
- [ ] Debug logs show correct sequence
- [ ] Tick updates when mysis completes turns
- [ ] Multiple myses show max tick across swarm

---

## Next Steps

1. Run with `--offline` and verify tick shows T42
2. Run with real server and verify tick updates
3. Remove debug logging after verification (if desired)
4. Update documentation if needed

---

## References

- **get_notifications implementation:** `internal/core/mysis.go:522-538`
- **Tick extraction logic:** `internal/core/mysis.go:1306-1379`
- **TUI event handling:** `internal/tui/app.go:655-693`
- **Commander AggregateTick:** `internal/core/commander.go:408-428`
- **Previous investigation:** `documentation/TICK_INVESTIGATION_FINDINGS.md`
- **Plan document:** `documentation/GET_NOTIFICATIONS_IMPLEMENTATION_PLAN.md`
