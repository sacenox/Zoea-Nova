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

## ~~get_notifications: Missing current_tick Field~~ ✅ RESOLVED

**Status:** ✅ **RESOLVED** in server v0.44.4

**Resolution Date:** 2026-02-06  
**Server Version:** v0.44.4+

The `get_notifications` tool now correctly returns `current_tick` and `timestamp` fields as expected. This issue was fixed in the server release v0.44.4.

**Updated Response Format (v0.44.4+):**
```json
{
  "count": 0,
  "current_tick": 42337,
  "notifications": [],
  "remaining": 0,
  "timestamp": 1770338536
}
```

**Historical Context:**
Prior to v0.44.4, the `get_notifications` endpoint did not return tick information, making it impossible for MCP clients to track game time. This was documented in forum thread 5e7fa591c8bea87a447864b0e77846d0.

**Release Notes (v0.44.4):**
> "FIX: get_notifications JSON-RPC handler now actually returns current_tick and timestamp fields"
> "The v0.44.3 fix only added the buildNotificationsResponse helper but the real handler path never called it"

**Impact on Zoea Nova:**
- Tick extraction now works correctly via `findCurrentTick()`
- TUI displays actual game tick instead of 0
- No code changes were required (extraction logic was already correct)

For historical investigation details, see:
- `documentation/investigations/GET_NOTIFICATIONS_API_INVESTIGATION.md`
- `documentation/investigations/TICK_INVESTIGATION_FINDINGS.md`
