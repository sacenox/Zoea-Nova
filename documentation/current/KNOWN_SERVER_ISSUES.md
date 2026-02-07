# Known SpaceMolt Server Issues

This document tracks known issues with upstream APIs (SpaceMolt MCP server, OpenCode Zen). We do not modify or validate external API behavior. Issues are documented here, and we improve prompts, error handling, and workarounds instead.

---

## SpaceMolt MCP Server Issues

### captains_log_add: empty_entry Error

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

---

### ~~get_notifications: Missing current_tick Field~~ ✅ RESOLVED

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

---

## OpenCode Zen API Issues

### OpenCode Zen: System-Only Message Crash

**Issue:** The OpenCode Zen API returns a 500 Internal Server Error when request messages contain ONLY system messages (no user or assistant turns).

**Server Response:**
```json
{
  "type": "error",
  "error": {
    "type": "error",
    "message": "Cannot read properties of undefined (reading 'prompt_tokens')"
  }
}
```

**Root Cause:**
OpenCode Zen's token counting logic fails when response metadata is missing or in unexpected format. The API attempts to read token counts from an undefined object, indicating an unhandled null/undefined case in their response parsing.

**Related Issue:**
OpenCode GitHub Issue #8228: Similar token counting crash with Gemini models
- URL: https://github.com/anomalyco/opencode/issues/8228
- Error: `"Cannot read properties of undefined (reading 'promptTokenCount')"`
- Status: OPEN (as of 2026-02-07)

**Workaround:**
Zoea Nova implements fallback logic in `internal/provider/openai_common.go`:
- Detects system-only message arrays
- Appends fallback user message: `{"role": "user", "content": "Continue."}`
- Prevents hitting the upstream API bug

**Implementation:**
```go
// If only system messages remain, add a dummy user message
if allMessagesAreSystem {
    messages = append(messages, openAIMessage{
        Role:    "user",
        Content: "Continue.",
    })
}
```

**Status:** ✅ **WORKAROUND IMPLEMENTED**

**Resolution Date:** 2026-02-07  
**Implementation:** Commit `9348f50` - fix: improve system message merging to preserve conversation

**Impact on Zoea Nova:**
- OpenCode Zen provider sends valid API requests
- Conversation history preserved correctly
- Message validation prevents invalid API calls
- Provider coverage: 87.0%

**Monitoring:**
- Watch OpenCode issue tracker for upstream fix
- Keep workaround in place until server-side bug is resolved
- No Zoea Nova code changes needed if upstream fixes issue

For implementation details, see:
- `documentation/investigations/OPENCODE_ZEN_BUG_VERDICT_2026-02-06.md`
- `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md`
