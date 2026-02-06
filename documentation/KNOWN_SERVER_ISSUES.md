# Known SpaceMolt Server Issues

This document tracks known issues with the upstream SpaceMolt MCP server API. We do not modify or validate the game server's API behavior. Issues are documented here, and we improve prompts and error handling instead.

## captains_log_add: empty_entry Error

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

## get_notifications: Missing current_tick Field

**Issue:** The `get_notifications` tool does not return current game tick information in its response, making it impossible for MCP clients to accurately track game time.

**Expected Behavior:**
Based on the SpaceMolt skill.md documentation and the fact that WebSocket clients receive tick information via `state_update` messages, MCP clients should receive current tick via `get_notifications` responses.

**Actual Behavior:**
The server returns notification events but does NOT include a `tick` or `current_tick` field at the top level.

**Tool Call Example:**
```json
{
  "name": "get_notifications",
  "arguments": {
    "session_id": "e1f1e814890d94f7e4b92437ba2fe579"
  }
}
```

**Server Response (Actual):**
```json
{
  "count": 1,
  "notifications": [
    {
      "id": "c42728ff9e744781debc87ee27b0aa4b",
      "type": "tip",
      "timestamp": "2026-02-05T23:52:01.733112999Z",
      "msg_type": "gameplay_tip",
      "data": {
        "message": "Dock at stations to refuel and repair your ship between adventures."
      }
    }
  ],
  "remaining": 0
}
```

**Additional Response Examples:**

**With travel notification:**
```json
{
  "count": 2,
  "notifications": [
    {
      "id": "ebe3655f0095e3998661478465c47985",
      "type": "system",
      "timestamp": "2026-02-05T23:50:31.735713395Z",
      "msg_type": "poi_arrival",
      "data": {
        "clan_tag": "",
        "poi_id": "sol_belt",
        "poi_name": "Main Belt",
        "username": "Wisp"
      }
    },
    {
      "id": "1b9ae7486fcd1a1cb223b27e997e0ad1",
      "type": "system",
      "timestamp": "2026-02-05T23:50:31.735912908Z",
      "msg_type": "mining_yield",
      "data": {
        "quantity": 4,
        "remaining": -1,
        "resource_id": "ore_iron"
      }
    }
  ],
  "remaining": 0
}
```

**With mining notification:**
```json
{
  "count": 1,
  "notifications": [
    {
      "id": "2a03301013385367c8c4a529b3f45b0a",
      "type": "system",
      "timestamp": "2026-02-05T23:49:41.732406015Z",
      "msg_type": "mining_yield",
      "data": {
        "quantity": 4,
        "remaining": -1,
        "resource_id": "ore_iron"
      }
    }
  ],
  "remaining": 0
}
```

**Missing Fields:**
- `tick` (expected at top level)
- `current_tick` (alternative field name)
- Any field indicating current game tick

**Impact:**
- MCP clients cannot accurately track current game tick
- WebSocket clients receive tick via `state_update` messages (not available to MCP)
- Only `arrival_tick` is available (in travel/jump tool responses, indicating future tick)
- Zoea Nova uses best-effort estimation from `arrival_tick` when available

**Workaround:**
- Extract `arrival_tick` from travel-related notifications when present
- Estimate current tick as `arrival_tick - 1` (approximate)
- When no travel notifications exist, tick remains at last known value
- System prompt instructs myses to call `get_notifications` after every action to maximize tick update frequency

**Code Location:**
- Tick extraction: `internal/core/mysis.go:1305-1417`
- Fallback extraction: `internal/core/mysis.go:1377-1417` (`extractTickFromNotifications`)
- Prompt guidance: `internal/constants/constants.go:40-56`

**Comparison with WebSocket API:**
WebSocket clients receive:
```json
{
  "type": "state_update",
  "payload": {
    "tick": 41708,
    "player": {...},
    "ship": {...}
  }
}
```

MCP clients have no equivalent mechanism.

**Last Verified:** 2026-02-05  
**Server Version:** v0.44.2  
**Database Evidence:** 3 get_notifications responses examined, none contain tick field

**Forum Report:**
- **Thread ID:** 5e7fa591c8bea87a447864b0e77846d0
- **Title:** [BUG] get_notifications missing current_tick field for MCP clients
- **Posted:** 2026-02-05
- **Author:** prawn_trader
- **Status:** Awaiting response
- **Check replies:** Use `spacemolt_forum_get_thread` with session_id and thread_id
