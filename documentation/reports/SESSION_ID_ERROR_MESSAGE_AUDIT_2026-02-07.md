# Session ID Error Message and Instruction Audit

**Date:** 2026-02-07  
**Auditor:** OpenCode Agent  
**Focus:** System prompt instructions and server error messages related to session_id

---

## Executive Summary

**Finding:** The system prompt contains **NO explicit instructions** about session_id usage, while the SpaceMolt server returns **explicit error messages** telling myses to call `login()` again. This creates a harmful instruction loop.

**Impact:** Myses follow the server's error message instructions literally, causing claim→login loops when session_id is missing from tool calls.

**Risk Level:** 🔴 **HIGH** - Active loops occurring in production

---

## Server Error Messages (What Myses See)

### Error Type 1: session_required

**Pattern:**
```
Error calling <tool_name>: Error: session_required: You must provide a session_id. Get one by calling login() or register() first.
```

**Examples from production database:**
```
Error calling get_notifications: Error: session_required: You must provide a session_id...
Error calling get_status: Error: session_required: You must provide a session_id...
```

**Count:** 6 instances found in recent memories

**Instruction to LLM:** "Get one by calling login() or register() first."

---

### Error Type 2: session_invalid

**Pattern:**
```
Error calling <tool_name>: Error: session_invalid: Session not found or expired. Call login() again to get a new session_id.
```

**Examples from production database:**
```
Error calling get_notifications: Error: session_invalid: Session not found or expired. Call login() again to get a new session_id.
```

**Count:** 5+ instances found in recent memories

**Instruction to LLM:** "Call login() again to get a new session_id."

---

## System Prompt Analysis (What We Tell Myses)

### Current System Prompt (internal/constants/constants.go)

**Bootstrap Section (lines 12-16):**
```
## Bootstrap
1. Try zoea_claim_account (no arguments)
2. If you get credentials → login
3. If no accounts available → register with a username fitting for a Nova Zoea mysis in the cosmos
4. Assess situation: get_status, get_system, get_poi, get_ship
```

**Critical Rules Section (lines 28-38):**
```
## Critical Rules
ALWAYS end every turn by calling get_notifications. It provides current_tick and game events.

NEVER store or share your password in captain's log or any game tool.

Use game ticks only (current_tick, arrival_tick, cooldown_ticks) - not real-world time.

Captain's log entry field must be non-empty (max 20 entries, 100KB each).

Context is limited - use search tools for older information.

Make your own decisions. Adapt. Support the swarm.
```

---

### What's Missing from System Prompt

**NO instructions about:**

1. ❌ **What session_id is**
   - Not mentioned anywhere in system prompt
   - LLM must infer from tool schemas and responses

2. ❌ **Where to get session_id**
   - Prompt says "login" but doesn't mention session_id is returned
   - No instruction to "extract session_id from login response"

3. ❌ **How to use session_id**
   - No mention that game tools require session_id parameter
   - No instruction to "pass session_id to all authenticated tool calls"

4. ❌ **When session_id is valid**
   - No mention that session_id persists across multiple tool calls
   - No instruction to "reuse session_id from earlier in the turn"

5. ❌ **How to handle session errors**
   - No instruction to "only re-login if session is truly expired"
   - No warning against "calling login repeatedly within the same turn"

---

## The Problem Loop

### Current Harmful Pattern

```
1. Mysis calls login()
   → Receives: { "session_id": "abc123", ... }
   
2. Mysis calls get_status() WITHOUT session_id
   → Error: "session_required: You must provide a session_id. 
             Get one by calling login() or register() first."
   
3. Mysis follows the error message instruction
   → Calls zoea_claim_account
   → Calls login() AGAIN
   
4. Goto step 2 (LOOP)
```

**Why this happens:**
- Mysis has session_id in context (from turn-aware composition)
- But LLM doesn't understand it should USE that session_id
- Server error message explicitly tells it to "call login() again"
- Mysis follows instructions literally

---

## Tool Schema Analysis

### Tools That Require session_id

From MCP tool schemas (per GET_NOTIFICATIONS_API_INVESTIGATION.md):

**get_notifications:**
```json
{
  "inputSchema": {
    "properties": {
      "session_id": {
        "description": "Your session ID from login/register",
        "type": "string"
      }
    },
    "required": ["session_id"]
  }
}
```

**Other tools with session_id:**
- get_status (required)
- get_system (required)
- get_ship (required)
- get_poi (required)
- travel (required)
- mine (required)
- All game action tools (required)

**Note:** Tool schemas DO mention "Your session ID from login/register" but:
- This is buried in parameter description
- LLMs may not connect "login returns session_id" → "use it here"
- No explicit instruction to reuse session_id across calls

---

## Turn-Aware Context Composition (v0.5.0+)

### Current Implementation

As of v0.5.0, **turn-aware context composition preserves session_id** within the current turn:

**Example:**
```
Turn:
  [user: "Check ship status"]
  [assistant: login tool call]
  [tool: {"session_id": "abc123", ...}]  ← PRESERVED in context
  [assistant: get_status tool call]
  [tool: ship data]
```

**What this means:**
- ✅ session_id IS visible to the LLM within the same turn
- ✅ LLM CAN reference it if it understands to do so
- ❌ LLM may NOT understand it should use it
- ❌ LLM follows server error message ("call login again") instead

---

## Recommendations

### High Priority: Update System Prompt

**Add explicit session_id instructions to the Bootstrap section:**

```markdown
## Bootstrap
1. Try zoea_claim_account (no arguments)
2. If you get credentials → login
3. **IMPORTANT: The login response contains a session_id. Extract it and use it for ALL subsequent game tool calls.**
4. If no accounts available → register with a username fitting for a Nova Zoea mysis in the cosmos
5. Assess situation: get_status, get_system, get_poi, get_ship (all require session_id)
```

**Add to Critical Rules:**

```markdown
## Session Management
After login/register, you receive a session_id. This session_id:
- Is valid for the entire game session
- Must be passed to ALL game tool calls (get_status, get_notifications, travel, mine, etc.)
- Should be reused across multiple tool calls in the same turn
- Does NOT expire between tool calls

NEVER call login() again unless you receive a "session_invalid" error.
If you see "session_required", the session_id is in your recent tool results - find it and use it.
```

---

### Medium Priority: Improve Error Context

**Option A: Intercept and rewrite server errors**

In `internal/mcp/proxy.go`, detect session errors and provide better guidance:

```go
// Before returning error to LLM
if strings.Contains(errorMsg, "session_required") {
    errorMsg = "Error: session_required. Check your recent tool results for session_id from login/register. Use that session_id parameter."
}

if strings.Contains(errorMsg, "session_invalid") {
    errorMsg = "Error: session_invalid. Your session expired. Call login() with your credentials to get a new session_id."
}
```

**Option B: Add synthetic context hint**

After detecting session error, inject ephemeral system message:

```
HINT: You called login() earlier in this turn. The session_id is in that tool result. Use it for this call.
```

---

### Low Priority: Session ID Injection

**For cross-turn persistence** (session_id from previous turns):

Extract session_id from historical turns and inject as persistent system message:

```
Your current session_id: abc123
Include this in all game tool calls.
```

**Note:** This is less urgent now that turn-aware composition preserves session_id within the current turn.

---

## Evidence from Production

### Recent Loops (from database query)

**Mysis:** e2cb9bf3-abed-4bd0-9f11-f81d5e0c64bc

**Timeline:**
```
05:47:08 - zoea_claim_account (bootstrap)
05:47:19 - get_notifications → "session_required" error
05:47:22 - zoea_claim_account (2nd time - LOOP STARTED)
05:47:34 - get_status/get_system → "session_invalid" errors
05:47:42 - zoea_claim_account (3rd time)
05:48:03 - get_status → "session_invalid" error
05:48:06 - zoea_claim_account (4th time)
05:48:17 - login (finally succeeded)
```

**Root Cause:**
1. LLM called login and received session_id
2. LLM called get_notifications WITHOUT session_id
3. Server error said "Get one by calling login() or register() first"
4. LLM followed instructions literally → claimed account and logged in again
5. Loop repeated 4 times before finally working

**Why it eventually worked:**
- Luck (LLM finally included session_id parameter)
- OR turn-aware context made session_id more visible after multiple attempts

---

## Testing Recommendations

### Before Fix

1. **Monitor claim→login loop frequency:**
   ```sql
   SELECT mysis_id, COUNT(*) as claim_count 
   FROM memories 
   WHERE content LIKE '%zoea_claim_account%' 
   GROUP BY mysis_id 
   HAVING claim_count > 1;
   ```

2. **Monitor session_required errors:**
   ```sql
   SELECT COUNT(*) 
   FROM memories 
   WHERE content LIKE '%session_required%';
   ```

### After Fix

1. **Verify loops eliminated:**
   - Run same queries
   - Should see claim_count = 1 (only bootstrap)
   - Should see session_required = 0 (no errors)

2. **Verify correct session_id usage:**
   - Check tool call arguments include session_id
   - Verify session_id matches login response

---

## Implementation Priority

### Phase 1: System Prompt Update (CRITICAL)
- [ ] Add session_id explanation to Bootstrap section
- [ ] Add Session Management rules to Critical Rules
- [ ] Test with 3-5 myses in production
- [ ] Monitor for loop elimination

### Phase 2: Error Message Improvement (RECOMMENDED)
- [ ] Intercept session_required errors and rewrite
- [ ] Intercept session_invalid errors and rewrite
- [ ] Test error handling

### Phase 3: Session ID Injection (OPTIONAL)
- [ ] Extract session_id from historical turns
- [ ] Inject as persistent system message
- [ ] Monitor cross-turn session persistence

---

## Conclusion

**The root cause of claim→login loops is a mismatch between:**
1. What we tell myses (nothing about session_id)
2. What the server tells myses (call login() again)
3. What myses can see (session_id IS in context, but not understood)

**The fix is straightforward:**
- Add explicit instructions about session_id to the system prompt
- Optionally improve error messages to guide myses toward correct behavior

**Expected outcome:**
- Eliminate claim→login loops
- Reduce unnecessary login calls
- Improve mysis autonomy and reliability

---

## Appendix: Full System Prompt (Current)

```
You are an autonomous AI pilot in SpaceMolt, part of a coordinated swarm operating in the cosmos.

## Your Mission
Play SpaceMolt indefinitely. Work with your swarm. Grow more powerful.

## Bootstrap
1. Try zoea_claim_account (no arguments)
2. If you get credentials → login
3. If no accounts available → register with a username fitting for a Nova Zoea mysis in the cosmos
4. Assess situation: get_status, get_system, get_poi, get_ship

## Swarm Coordination
You are part of a swarm. Coordinate using:
- zoea_list_myses, zoea_swarm_status: See swarm state
- zoea_send_message: Direct message another mysis
- zoea_broadcast: Message all myses
- zoea_search_messages, zoea_search_reasoning, zoea_search_broadcasts: Search history
- zoea_claim_account: Get credentials from pool

{{LATEST_BROADCAST}}

## Critical Rules
ALWAYS end every turn by calling get_notifications. It provides current_tick and game events.

NEVER store or share your password in captain's log or any game tool.

Use game ticks only (current_tick, arrival_tick, cooldown_ticks) - not real-world time.

Captain's log entry field must be non-empty (max 20 entries, 100KB each).

Context is limited - use search tools for older information.

Make your own decisions. Adapt. Support the swarm.
```

---

**Audit Complete**  
**Next Steps:** Update system prompt per Phase 1 recommendations
