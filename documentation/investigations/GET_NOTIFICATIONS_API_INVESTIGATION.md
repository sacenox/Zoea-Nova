# get_notifications API Investigation Report

**Date:** 2026-02-05  
**Investigator:** OpenCode Agent  
**Issue:** Tick display always shows T0  
**Investigation Type:** Real API calls to SpaceMolt MCP server

---

# ⚠️ HISTORICAL DOCUMENT - ISSUE RESOLVED

**Date Resolved:** 2026-02-06  
**Server Version Fixed:** v0.44.4  
**Status:** This investigation documented an API limitation that has since been fixed.

The SpaceMolt server **NOW RETURNS tick information** in `get_notifications` responses as of server v0.44.4. The findings below were accurate for server versions v0.44.2 and earlier, but are now outdated.

**Resolution:** Server fix added `current_tick` and `timestamp` fields to `get_notifications` responses. Zoea Nova's implementation was already correct and began working immediately after the server update.

This document is preserved for historical context only.

---

## Executive Summary

**CRITICAL FINDING (RESOLVED):** The SpaceMolt `get_notifications` API **DID NOT return tick information** (prior to v0.44.4).

This contradicts:
1. Our implementation plan (based on skill.md interpretation)
2. The GET_NOTIFICATIONS_IMPLEMENTATION_PLAN.md document
3. Initial assumptions about the API

**Root Cause:** The SpaceMolt skill.md documentation does NOT actually claim that `get_notifications` returns tick. We misread the documentation. The tick information comes from a DIFFERENT mechanism (WebSocket `state_update` messages for WebSocket clients).

---

## API Test Results

### Test Setup

- **API Endpoint:** https://game.spacemolt.com/mcp
- **MCP Protocol:** 2024-11-05
- **Test Account:** zoea_test_1770334640 (registered during test)
- **Test Session:** 517c9acf96bb37a85cc1b32d874b66f8

### Test 1: Empty Notification Queue

**Request:**
```json
{
  "name": "get_notifications",
  "arguments": {
    "session_id": "517c9acf96bb37a85cc1b32d874b66f8"
  }
}
```

**Response:**
```json
{
  "count": 0,
  "notifications": null,
  "remaining": 0
}
```

**Fields Present:** count, notifications, remaining  
**Fields Missing:** tick, current_tick

---

### Test 2: After Game Actions (with notifications)

**Actions Performed:**
1. undock() - left station
2. travel(target_poi="sol_belt") - traveled to asteroid belt
3. Waited 3 seconds
4. Called get_notifications()

**Response:**
```json
{
  "count": 2,
  "notifications": [
    {
      "id": "b7b3ce3db8f6e097b57ae9bb839eef5a",
      "type": "system",
      "timestamp": "2026-02-05T23:37:31.73500264Z",
      "msg_type": "poi_arrival",
      "data": {
        "clan_tag": "DUKE",
        "poi_id": "sol_station",
        "poi_name": "Sol Central",
        "username": "Iron Duke"
      }
    },
    {
      "id": "0ba7773a90aa4968c170b1402e5a4952",
      "type": "system",
      "timestamp": "2026-02-05T23:37:51.735477221Z",
      "msg_type": "ok",
      "data": {
        "action": "undock"
      }
    }
  ],
  "remaining": 0
}
```

**Fields Present:** count, notifications[], remaining  
**Fields Missing:** tick, current_tick

---

### Test 3: With Optional Parameters

**Request:**
```json
{
  "name": "get_notifications",
  "arguments": {
    "session_id": "517c9acf96bb37a85cc1b32d874b66f8",
    "limit": 100,
    "clear": false
  }
}
```

**Response:**
```json
{
  "count": 0,
  "notifications": null,
  "remaining": 0
}
```

**Result:** Same format, no tick information regardless of parameters.

---

### Test 4: Other Tools Checked

**get_status():**  
✅ Returns player and ship data  
❌ No tick field

**get_system():**  
✅ Returns POIs, security status, system data  
❌ No tick field

**travel():**  
✅ Returns arrival_tick (future tick when travel completes)  
❌ No current_tick field

---

## Tool Schema Analysis

From MCP `tools/list` response:

```json
{
  "description": "Poll for pending notifications (chat, combat, trades, etc.).",
  "inputSchema": {
    "properties": {
      "session_id": {
        "description": "Your session ID from login/register",
        "type": "string"
      }
    },
    "required": ["session_id"],
    "type": "object"
  },
  "name": "get_notifications"
}
```

**Note:** No mention of tick in schema or description.

---

## Documentation Review

### What skill.md ACTUALLY Says

The skill.md document describes `get_notifications` as:

> Unlike WebSocket connections which receive real-time push messages, **MCP is polling-based**. Game events (chat messages, combat alerts, trade offers, etc.) queue up while you're working on other actions.

**Notification Types:** chat, combat, trade, faction, friend, forum, system  
**When to Poll:** After each action, when idle (30-60s), before important decisions  
**Queue Size:** Up to 100 events per session

**NOWHERE in skill.md does it claim get_notifications returns tick!**

### What We Assumed (Incorrectly)

Our GET_NOTIFICATIONS_IMPLEMENTATION_PLAN.md stated:

```json
{
  "tick": 41708,
  "notifications": [...]
}
```

**This was based on misreading or misunderstanding the documentation.**

---

## Database Evidence Confirmation

### Query Results

From zoea.db:
- **Tool calls (excluding get_notifications):** 16
- **Automatic get_notifications polls:** 0
- **LLM-initiated get_notifications calls:** 2

### Sample get_notifications Response from Database

```json
{
  "count": 2,
  "notifications": [...],
  "remaining": 0
}
```

**MATCHES** the actual API response format exactly.  
**CONFIRMS** our myses were receiving correct responses (just no tick).

---

## Comparison: Expected vs Actual

| Aspect | Expected (from plan) | Actual (from API) |
|--------|---------------------|-------------------|
| **Top-level tick** | ✅ Present | ❌ NOT present |
| **Top-level current_tick** | ✅ Present | ❌ NOT present |
| **count field** | ✅ Present | ✅ Present |
| **notifications array** | ✅ Present | ✅ Present |
| **remaining field** | ✅ Present | ✅ Present |
| **Notification structure** | Correct | ✅ Correct |
| **Notification types** | Correct | ✅ Correct |

---

## Where Does Tick Information Come From?

### For WebSocket Clients

WebSocket clients receive **push messages** with tick updates:

```
state_update event:
{
  "tick": 41972,
  "updates": [...]
}
```

These arrive **automatically** without polling.

### For MCP/HTTP Clients

**MCP clients DO NOT receive tick updates via get_notifications.**

Tick information can only be inferred from:
1. **arrival_tick** in travel() responses (future tick)
2. **Timestamps** in notifications (but not tick number)
3. **Real-time estimation** (not provided by API)

---

## Implications for Zoea Nova

### Current Implementation

The automatic get_notifications polling we added:
- ✅ Works correctly (makes API calls)
- ✅ Receives proper responses
- ❌ Does NOT provide tick information (API limitation)

### Why Tick Is Always 0

1. No tool returns `current_tick` at top level
2. Only `travel()` returns `arrival_tick` (future tick, not current)
3. Our tick extraction logic (`findCurrentTick()`) correctly looks for tick
4. But the API never provides it

### Fix Options

#### Option 1: Calculate from arrival_tick (Hacky)

When travel() returns arrival_tick, we could:
```
current_tick ≈ arrival_tick - ticks
```

**Pros:** Would provide a tick value  
**Cons:**
- Only works when traveling
- Inaccurate (assumes travel started immediately)
- Doesn't work for idle myses
- Hacky and unreliable

#### Option 2: Remove tick display (Recommended)

Remove the tick display from the status bar entirely.

**Pros:** 
- Clean, no misleading information
- Acknowledges API limitation
- Simple change

**Cons:**
- Loses potential feature

#### Option 3: Show estimated tick (Advanced)

Track real-world time and estimate tick progression based on:
- Last known arrival_tick
- Elapsed time since travel
- Estimated tick rate (10 seconds per tick)

**Pros:** Better than nothing  
**Cons:**
- Complex
- Inaccurate
- Drifts over time

---

## Recommendations

### Immediate Actions

1. ✅ **Keep get_notifications polling** - It's still useful for notifications
2. ❌ **Remove tick extraction expectations** - API doesn't provide it
3. ✅ **Update documentation** - Correct the misunderstanding
4. ⚠️ **Decide on tick display** - Remove or calculate from arrival_tick

### Documentation Updates Needed

1. **GET_NOTIFICATIONS_IMPLEMENTATION_PLAN.md**
   - Remove claims about tick in response
   - Update "Expected Behavior" section
   - Correct response format examples

2. **TICK_INVESTIGATION_FINDINGS.md**
   - Add this investigation
   - Confirm API limitation
   - Document decision on tick display

3. **AUTO_POLLING_DEBUG_REPORT.md**
   - Update with correct expectations
   - Note that polling works but doesn't provide tick

### Code Changes Needed

**If removing tick display:**
```diff
// internal/tui/app.go - renderTickTimestamp()
- Show tick in status bar
+ Remove tick display OR show "N/A"
```

**If calculating from arrival_tick:**
```diff
// internal/core/mysis.go - updateActivityFromToolResult()
+ Extract arrival_tick from travel responses
+ Calculate: current_tick = arrival_tick - ticks
+ Store as lastServerTick (with accuracy warning)
```

---

## Test Artifacts

All test scripts and outputs saved to:
- `/tmp/test_get_notifications.sh`
- `/tmp/test_get_notifications_v2.sh`
- `/tmp/final_tick_test.sh`

**Test Account Created:**
- Username: zoea_test_1770334640
- Session: 517c9acf96bb37a85cc1b32d874b66f8
- Can be used for further testing

---

## Conclusion

**The SpaceMolt MCP API does NOT provide current tick information via get_notifications or any other MCP tool.**

This is an **API limitation**, not a bug in Zoea Nova. MCP clients are expected to work without real-time tick updates, unlike WebSocket clients which receive push notifications with tick numbers.

**Recommendation:** Either remove tick display or calculate approximate tick from arrival_tick values (with clear "estimated" label).

---

**Report Complete**  
**Date:** 2026-02-05  
**Next Steps:** Decide on tick display strategy and update implementation accordingly.

