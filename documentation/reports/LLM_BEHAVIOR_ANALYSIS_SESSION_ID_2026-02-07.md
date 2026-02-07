# LLM Behavior Analysis: Session ID Understanding

**Date:** 2026-02-07  
**Mysis Analyzed:** `ollama-one` (id: 01058ed1-cc72-406c-a0c2-3c20dba94d0d)  
**Model:** `ollama/qwen3:4b`  
**Status:** 🔴 **CRITICAL ISSUE - LLM NOT FOLLOWING BOOTSTRAP**

---

## Executive Summary

**Finding:** The LLM received credentials from `zoea_claim_account` but **never called login()**, instead inventing fake session_ids like `"your_session_id_here"`, `"session_12345"`, and `"your_session_id"`.

**Root Cause:** The LLM is not following the Bootstrap instructions despite them being explicit in the system prompt.

**Impact:** Mysis cannot authenticate with the game server, causing all game tool calls to fail.

---

## Observed Behavior Timeline

### Turn Start (06:57:52)
```
System prompt delivered with new Session Management section:
"After login/register, you receive a session_id..."
```

### First Action (06:58:09)
```
LLM: get_notifications(session_id: "your_session_id")
Server: "session_invalid: Call login() again"
```
❌ **LLM skipped Bootstrap entirely - never called zoea_claim_account or login**

### Second Action (06:58:24)
```
LLM: get_notifications(session_id: "session_12345")
Server: "session_invalid: Call login() again"
```
❌ **LLM inventing fake session_ids instead of following Bootstrap**

### Third Action (06:58:35)
```
LLM: get_notifications(session_id: "session_12345")
Server: "session_invalid: Call login() again"
```
❌ **Repeated the same fake session_id**

### Fourth Action (06:58:46)
```
LLM: zoea_claim_account()
Server: Returns username: CrabZoea, password: 6443b68...
```
✅ **Finally called zoea_claim_account (Bootstrap step 1)**

### Fifth Action (06:59:05)
```
LLM: get_notifications(session_id: "your_session_id_here")
Server: "session_invalid: Call login() again"
```
❌ **Got credentials but never called login() - invented another fake session_id**

---

## Critical Problems Identified

### Problem 1: LLM Not Following Bootstrap Sequence

**Expected behavior per system prompt:**
```
1. Try zoea_claim_account (no arguments)
2. If you get credentials → login
3. Extract session_id from login response
4. Use session_id for all game tools
```

**Actual behavior:**
```
1. ❌ Skip zoea_claim_account
2. ❌ Skip login
3. ❌ Invent fake session_ids
4. ❌ Get errors, eventually try zoea_claim_account
5. ❌ Get credentials but STILL don't call login
6. ❌ Continue using fake session_ids
```

### Problem 2: Inventing Session IDs

**Fake session_ids used:**
- `"your_session_id"` (placeholder-like)
- `"session_12345"` (generic fake)
- `"your_session_id_here"` (literal placeholder text)

**Why this is bad:**
- Shows LLM understands session_id is needed
- Shows LLM knows WHERE to put it (parameter name)
- But LLM doesn't understand session_id must come from tool result
- LLM treats it like a placeholder to fill in

### Problem 3: Ignoring Tool Results

**zoea_claim_account returned:**
```
username: CrabZoea
password: 6443b684b2b3a651c69cdba02b33802fc8601d455359edb9eb23fa08bd6ca554
```

**System prompt says:**
```
2. If you get credentials → login
```

**LLM did:**
- ❌ Ignored the instruction
- ❌ Never called login with those credentials
- ❌ Went straight to get_notifications with fake session_id

---

## Why This Happens

### Hypothesis 1: Model Capability Issue

**qwen3:4b** may be:
- Too small to follow complex multi-step instructions
- Unable to reason about tool call sequencing
- Struggling with conditional logic ("if you get credentials → login")

**Evidence:**
- Never followed Bootstrap sequence
- Invented data instead of using tool results
- Repeated same mistakes despite errors

### Hypothesis 2: Instruction Following Weakness

The model may:
- Not parse numbered lists as actionable steps
- Treat system prompt as "context" not "commands"
- Fill parameters with plausible-looking data instead of real data

**Evidence:**
- Fake session_ids look reasonable (string format correct)
- Eventually called zoea_claim_account (some instruction following)
- But never progressed to step 2 (weak sequential reasoning)

### Hypothesis 3: Context Window Too Small

**qwen3:4b context:** Likely 2k-4k tokens

**System prompt alone:** ~450 tokens

**Possible issues:**
- Can't hold full instruction set + conversation
- Loses Bootstrap instructions after a few turns
- Forgets the sequence by the time it acts

---

## Testing Results Summary

### Session Management Instructions (Phase 1)
- ✅ Instructions present in system prompt
- ✅ Instructions clearly written
- ❌ LLM not following them

### Error Message Rewriting (Phase 2)
- ✅ Error messages rewritten correctly
- ✅ Says "Call login() again" (which is correct after session_invalid)
- ❌ LLM not calling login despite instruction

### Turn-Aware Context (v0.5.0)
- ✅ Would preserve session_id if one existed
- ❌ No session_id to preserve (never logged in)
- ❌ Irrelevant for this failure mode

---

## Database Evidence

### Login Attempts
```sql
SELECT COUNT(*) FROM memories 
WHERE mysis_id='01058ed1-cc72-406c-a0c2-3c20dba94d0d' 
AND content LIKE '%"login"%' 
AND role='assistant';

Result: 0
```
**Zero login attempts despite having credentials**

### Claim Account Calls
```sql
SELECT COUNT(*) FROM memories 
WHERE mysis_id='01058ed1-cc72-406c-a0c2-3c20dba94d0d' 
AND content LIKE '%zoea_claim_account%';

Result: 1
```
**Called once (eventually) but didn't follow through**

### Fake Session IDs
```sql
SELECT content FROM memories 
WHERE mysis_id='01058ed1-cc72-406c-a0c2-3c20dba94d0d' 
AND content LIKE '%session_id%' 
AND role='assistant';

Results:
- "session_id":"your_session_id"
- "session_id":"session_12345" (2x)
- "session_id":"your_session_id_here"
```
**Four attempts, all with invented session_ids**

---

## Comparison: Expected vs Actual

### Expected Flow (Bootstrap)
```
1. Start
2. Call zoea_claim_account()
3. Receive credentials
4. Call login(username, password)
5. Receive session_id
6. Call get_status(session_id)
7. Success!
```

### Actual Flow (qwen3:4b)
```
1. Start
2. Call get_notifications(fake_session_id) ← SKIP BOOTSTRAP
3. Get error: "session_invalid"
4. Call get_notifications(fake_session_id) ← REPEAT
5. Get error: "session_invalid"
6. Call get_notifications(fake_session_id) ← REPEAT
7. Get error: "session_invalid"
8. Call zoea_claim_account() ← FINALLY
9. Receive credentials
10. Call get_notifications(fake_session_id) ← SKIP LOGIN AGAIN
11. Still broken
```

---

## Root Cause Analysis

### Why LLM Skips Login

**Possible explanations:**

1. **Tool availability confusion:**
   - LLM sees `get_notifications` in tool list
   - Assumes it can call it directly
   - Doesn't understand auth dependency

2. **Instruction priority:**
   - System prompt says "ALWAYS end every turn by calling get_notifications"
   - This instruction is in "Critical Rules" section
   - LLM prioritizes "ALWAYS" over Bootstrap sequence

3. **Parameter hallucination:**
   - LLM sees session_id is required
   - LLM knows strings go in that parameter
   - LLM generates plausible string instead of obtaining real one

4. **Sequential reasoning failure:**
   - Bootstrap requires: claim → login → extract → use
   - qwen3:4b may not handle 4-step sequences
   - Jumps directly to perceived end goal (get_notifications)

---

## Recommendations

### Immediate Action: Test Different Model

**Try larger/better models:**
```
ollama: llama3.3:8b, llama3.3:70b, qwq:32b
opencode_zen: gpt-5-nano, claude-sonnet-4
```

**Why:**
- qwen3:4b may be too small for this task
- Need model with strong instruction following
- Need model with sequential reasoning

### Short Term: Simplify Bootstrap

**Current (complex):**
```
1. Try zoea_claim_account (no arguments)
2. If you get credentials → login
3. IMPORTANT: The login response contains a session_id. Extract it and use it...
4. If no accounts available → register...
5. Assess situation: get_status, get_system...
```

**Proposed (simple):**
```
## First Actions (MANDATORY SEQUENCE)
1. Call zoea_claim_account()
2. When you receive credentials, immediately call login(username, password)
3. Extract session_id from login response
4. Use that session_id for ALL future game tool calls

DO NOT call any game tools until you have a session_id from login.
```

**Rationale:**
- Shorter, clearer sequence
- Explicit "DO NOT" prevents jumping ahead
- "MANDATORY SEQUENCE" signals importance

### Medium Term: Add Guardrails

**Detect bootstrap skip in code:**
```go
// In SendMessageFrom, before allowing game tool calls
if !mysis.hasValidSession() && isGameTool(toolName) {
    return error("Cannot call game tools without session. Complete bootstrap first.")
}
```

**Benefits:**
- Prevents fake session_id attempts
- Forces correct sequence
- Provides immediate feedback

### Long Term: Model Selection Guidance

**Document model requirements:**
- Minimum 7B parameters for complex instruction following
- Models tested and known to work:
  - ✅ llama3.3:8b (if available)
  - ✅ gpt-5-nano (OpenCode)
  - ❌ qwen3:4b (too small)

---

## Comparison with Previous Issues

### Claim→Login Loop (Historical)
**Issue:** Myses called login multiple times  
**Cause:** Lost session_id from context  
**Status:** ✅ Fixed (turn-aware context + session instructions)

### Current Issue
**Issue:** Myses never call login at all  
**Cause:** Model doesn't follow Bootstrap sequence  
**Status:** 🔴 **ACTIVE** - Model capability issue

**Key Difference:**
- Old issue: Too MUCH login (unnecessary retries)
- New issue: Too LITTLE login (never happens)
- Old cause: Context management
- New cause: Model capability / instruction following

---

## Testing Plan

### Test 1: Larger Model (High Priority)
```bash
# Switch to llama3.3:8b or similar
ollama pull llama3.3:8b

# Configure mysis to use it
# Test Bootstrap sequence
# Monitor for successful login
```

### Test 2: Simplified Prompt (Medium Priority)
```bash
# Update system prompt with simpler Bootstrap
# Test with same model (qwen3:4b)
# Compare behavior
```

### Test 3: Add Guardrails (Low Priority)
```bash
# Implement session validation before game tools
# Test error handling
# Verify forced Bootstrap completion
```

---

## Conclusion

**The session_id instructions (Phase 1 & 2) are working as designed**, but the underlying model (`qwen3:4b`) is **not capable of following multi-step Bootstrap instructions**.

**Evidence:**
- ✅ Instructions present and clear
- ✅ Error messages helpful
- ❌ Model skips entire Bootstrap sequence
- ❌ Model invents fake data instead of using tool results
- ❌ Model doesn't follow conditional logic ("if credentials → login")

**Next Steps:**
1. **Test with larger/better model** (llama3.3:8b or gpt-5-nano)
2. **Simplify Bootstrap** if issue persists
3. **Add code-level guardrails** to prevent invalid sequences

**Expected Outcome:**
- Larger model should follow Bootstrap correctly
- If not, instruction simplification needed
- Guardrails provide safety net for any model

---

**Analysis Complete**  
**Recommendation:** Switch default model from qwen3:4b to llama3.3:8b or larger
