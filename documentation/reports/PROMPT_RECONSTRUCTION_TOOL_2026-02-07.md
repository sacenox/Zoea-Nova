# Prompt Reconstruction Tool

Quick reference for reconstructing what was sent to the LLM provider.

## Usage

```bash
# Latest mysis
./scripts/reconstruct_prompt.sh

# Specific mysis
./scripts/reconstruct_prompt.sh <mysis_id>

# With provider payload simulation
./scripts/reconstruct_prompt.sh <mysis_id> --full
```

## What Gets Sent to Provider

### 1. Memory Retrieval
```go
// internal/core/mysis.go:459
memories, err := a.getContextMemories()
```

**getContextMemories() logic:**
- System prompt (with broadcast injection)
- Historical context (compressed via extractLatestToolLoop)
- Current turn (complete, from last user prompt onward)

### 2. Memory → Message Conversion
```go
// internal/core/mysis.go:480
messages := a.memoriesToMessages(memories)
```

**Conversion rules:**
- `system` role → `{"role": "system", "content": "..."}`
- `user` role → `{"role": "user", "content": "..."}`
- `assistant` with `[TOOL_CALLS]` → parse tool calls, empty content
- `tool` role → extract call_id from `call_id:content` format

### 3. Provider Formatting

**OpenAI-compatible (opencode_zen):**
```json
{
  "model": "gpt-5-nano",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."},
    {"role": "assistant", "tool_calls": [...]},
    {"role": "tool", "tool_call_id": "...", "content": "..."}
  ],
  "tools": [...],
  "temperature": 0.7
}
```

## Example Reconstruction

**Mysis:** prompt-test (baa89cbd-71ff-4e43-b7ea-cc4813b530cd)

### Database Memories (chronological)
```
[0] system: "You are a Nova Zoea mysis..."
[1] assistant: [TOOL_CALLS]call_X:zoea_claim_account:{}
[2] tool: call_X:Use the game's login tool with credentials...
[3] assistant: [TOOL_CALLS]call_Y:zoea_claim_account:{} (REPEAT!)
[4] tool: call_Y:Use the game's login tool with credentials... (REPEAT!)
[5] assistant: [TOOL_CALLS]call_Z:get_status:{"session_id":"fake_123"}
[6] tool: call_Z:Error: session_invalid
```

### Converted to Provider Messages
```json
[
  {
    "role": "system",
    "content": "You are a Nova Zoea mysis in SpaceMolt..."
  },
  {
    "role": "assistant",
    "content": "",
    "tool_calls": [
      {
        "id": "call_X",
        "type": "function",
        "function": {
          "name": "zoea_claim_account",
          "arguments": "{}"
        }
      }
    ]
  },
  {
    "role": "tool",
    "tool_call_id": "call_X",
    "content": "Use the game's login tool with these credentials:\nusername: CrabZoea\npassword: 6443..."
  },
  {
    "role": "assistant",
    "content": "",
    "tool_calls": [
      {
        "id": "call_Y",
        "type": "function",
        "function": {
          "name": "zoea_claim_account",
          "arguments": "{}"
        }
      }
    ]
  },
  {
    "role": "tool",
    "tool_call_id": "call_Y",
    "content": "Use the game's login tool with these credentials:\nusername: CrabZoea\npassword: 6443..."
  }
]
```

### Problem Pattern Visible

**Repetition detected:**
- Same tool called twice in a row (zoea_claim_account)
- Same response twice in a row (credentials)
- LLM sees: claim → credentials → claim → credentials
- LLM learns: "This is the pattern, keep doing it"

**Missing step:**
- Got credentials but never called `login`
- Jumped to `get_status` with invented session_id

## Script: reconstruct_prompt.sh

```bash
#!/bin/bash
MYSIS_ID=${1:-$(sqlite3 ~/.zoea-nova/zoea.db "SELECT id FROM myses ORDER BY created_at DESC LIMIT 1")}
MYSIS_NAME=$(sqlite3 ~/.zoea-nova/zoea.db "SELECT name FROM myses WHERE id='$MYSIS_ID'")

echo "=== PROMPT RECONSTRUCTION FOR: $MYSIS_NAME ==="
echo ""

# 1. Show raw memories
sqlite3 ~/.zoea-nova/zoea.db << SQL
SELECT 
  '[' || row_number() OVER (ORDER BY created_at ASC) || ']',
  role,
  CASE 
    WHEN length(content) > 150 THEN substr(content, 1, 150) || '...'
    ELSE content
  END
FROM memories 
WHERE mysis_id='$MYSIS_ID'
ORDER BY created_at ASC;
SQL

# 2. Detect repetition
echo ""
echo "## REPETITION ANALYSIS"
sqlite3 ~/.zoea-nova/zoea.db << SQL
WITH tool_calls AS (
  SELECT 
    content,
    LAG(content) OVER (ORDER BY created_at) as prev_content
  FROM memories
  WHERE mysis_id='$MYSIS_ID' AND role='assistant'
)
SELECT 'REPEATED TOOL CALL: ' || content 
FROM tool_calls 
WHERE content = prev_content;
SQL

sqlite3 ~/.zoea-nova/zoea.db << SQL
WITH tool_results AS (
  SELECT 
    substr(content, instr(content, ':') + 1, 80) as result,
    LAG(substr(content, instr(content, ':') + 1, 80)) OVER (ORDER BY created_at) as prev_result
  FROM memories
  WHERE mysis_id='$MYSIS_ID' AND role='tool'
)
SELECT 'REPEATED TOOL RESULT: ' || result 
FROM tool_results 
WHERE result = prev_result;
SQL
```

## Findings

### Issue 1: Repetition Training
When LLMs see:
```
A → B → A → B → A → B
```

They learn this is the correct pattern and continue it.

**Solution:** Compress repeated tool results in context (already implemented via `compactSnapshots()` but not for all tools).

### Issue 2: Never Calling Login
Prompt says:
```
1. Call zoea_claim_account
2. You get username and password
3. Call login with that username and password
```

But LLM:
```
1. ✓ Called zoea_claim_account
2. ✓ Got credentials
3. ✗ SKIPPED login
4. ✗ Invented session_id
```

**Solution:** Add explicit blocker or simplify further.

### Issue 3: Tool Result Format
Tool results like:
```
"Use the game's login tool with these credentials:
username: CrabZoea
password: 6443b684..."
```

This is **instructional text**, not actual credentials.

**LLM confusion:**
- Sees "Use the game's login tool"
- Might interpret as "I should use zoea_claim_account again"
- Doesn't extract the actual credentials

**Better format:**
```json
{
  "action": "login_required",
  "username": "CrabZoea",
  "password": "6443b684..."
}
```

## Next Steps

1. ✅ Simplify prompt (done)
2. ✅ Remove confusing tools (done - removed zoea_swarm_status)
3. ⏳ Test with simplified prompt
4. 🔄 If still failing: Change zoea_claim_account response format
5. 🔄 If still failing: Add code-level blocker (can't call game tools without session_id)
