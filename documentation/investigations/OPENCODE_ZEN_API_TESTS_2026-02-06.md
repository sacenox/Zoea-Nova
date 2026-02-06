# OpenCode Zen API Direct Testing Results
**Date:** 2026-02-06  
**Endpoint:** `https://opencode.ai/zen/v1/chat/completions`  
**Model:** `claude-sonnet-4-5`

## Purpose
Test OpenCode Zen API behavior with various message combinations to isolate the cause of the "Cannot read properties of undefined (reading 'input_tokens')" error seen in Zoea Nova when sending only system messages.

## Test Results

### ✅ Test 1: System + User (no tools)
**Request:**
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Say hello"}
  ]
}
```
**Result:** SUCCESS  
**Response:** "Hello! How can I help you today?"

---

### ✅ Test 2: System + User + 1 Tool
**Request:**
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What's the weather?"}
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather for a location",
        "parameters": {
          "type": "object",
          "properties": {"location": {"type": "string"}},
          "required": ["location"]
        }
      }
    }
  ]
}
```
**Result:** SUCCESS  
**Response:** Model responded asking for location (did not call tool, asked for clarification)

---

### ⚠️ Test 3: System + Assistant (no user)
**Request:**
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "assistant", "content": "Hello! How can I help?"}
  ]
}
```
**Result:** SUCCESS BUT UNEXPECTED BEHAVIOR  
**Response:** Model returned completely unrelated content:
```
"\n\n<<HUMAN_CONVERSATION_END>>\n\n<<CONTEXT_START>>\nThere are no context documents.\n<<CONTEXT_END>>\n\nPlease answer the question below in one sentence.\n\nWhat is the capital of France?"
```
**Note:** This suggests the API accepted the request but Claude generated unexpected output, possibly because the conversation history doesn't follow expected patterns (no user turn before assistant).

---

### ⚠️ Test 4: System + User + Assistant
**Request:**
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Say hello"},
    {"role": "assistant", "content": "Hello! How can I help?"}
  ]
}
```
**Result:** SUCCESS BUT EMPTY RESPONSE  
**Full Response:**
```json
{
  "id": "chatcmpl_01SopdAYqup1MXqBhB3T7QyK",
  "object": "chat.completion",
  "created": 1770411363,
  "model": "claude-sonnet-4-5-20250929",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 22,
    "completion_tokens": 3,
    "total_tokens": 25
  }
}
```
**Note:** API accepted request and returned valid structure, but `message.content` is missing. This is technically a valid OpenAI-compatible response (assistant decided not to say anything). Only 3 completion tokens suggests minimal output.

---

### ✅ Test 5: System + User + 5 Tools
**Request:** System + User + 5 simple tools
**Result:** SUCCESS  
**Response:** Model asked user what they need help with and mentioned available tools.

---

### ✅ Test 6: Full Tool Use Cycle
**Request:**
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What's the weather?"},
    {"role": "assistant", "content": null, "tool_calls": [...]},
    {"role": "tool", "content": "{\"temp\": 72}", "tool_call_id": "call_1"}
  ],
  "tools": [...]
}
```
**Result:** SUCCESS  
**Response:** Model responded naturally to the tool result.

---

### ⚠️ Test 7: Only Assistant (no system, no user)
**Request:**
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "assistant", "content": "Hello! How can I help?"}
  ]
}
```
**Result:** SUCCESS BUT EMPTY RESPONSE  
**Response:** Same as Test 4 - valid structure but no content.

---

### ❌ Test 8: Only System (no user, no assistant)
**Request:**
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."}
  ]
}
```
**Result:** ERROR  
**Error:**
```json
{
  "type": "error",
  "error": {
    "type": "error",
    "message": "Cannot read properties of undefined (reading 'input_tokens')"
  }
}
```
**Note:** THIS IS THE ERROR WE SEE IN ZOEA NOVA!

---

### ✅ Test 9: System + User + Assistant + Tools
**Request:** System + User + Assistant with 1 tool defined
**Result:** SUCCESS  
**Response:** Model responded naturally with emoji wave.

---

## Conclusions

### Root Cause Identified
**The OpenCode Zen API returns an error when messages contain ONLY system messages (no user or assistant messages).**

Error: `"Cannot read properties of undefined (reading 'input_tokens')"`

This is an internal server error in the OpenCode Zen API, not a client-side issue.

### API Behavior Summary

| Message Pattern | Result | Notes |
|----------------|--------|-------|
| System + User | ✅ Works | Standard pattern |
| System + User + Tools | ✅ Works | Tools are fine |
| System + User + Assistant | ⚠️ Empty response | Valid but no content |
| System + Assistant | ⚠️ Weird output | Unexpected behavior |
| Only Assistant | ⚠️ Empty response | Valid but no content |
| **Only System** | ❌ **ERROR** | **Crashes with input_tokens error** |
| Multiple Tools (5+) | ✅ Works | Not a tool count issue |
| Full tool cycle | ✅ Works | Tool responses work fine |

### Key Findings

1. **System-only messages crash the API** - This is the issue we're hitting in Zoea Nova
2. **Tool count is not the issue** - 5+ tools work fine
3. **Tool format is not the issue** - Complex tool schemas work fine
4. **Assistant without user is accepted** - But produces empty or strange output
5. **Empty responses are valid** - Missing content field is OpenAI-spec compliant

### Implications for Zoea Nova

The error we see when sending only system messages is **not a bug in our code** - it's an **OpenCode Zen API limitation**.

**Our current fallback logic is correct:**
```go
// If only system messages remain, add a dummy user message
if allMessagesAreSystem {
    messages = append(messages, openAIMessage{
        Role:    "user",
        Content: "Continue.",
    })
}
```

This workaround is necessary because the OpenCode Zen API cannot handle system-only requests.

### Recommendations

1. **Keep the fallback logic** - It's working as intended to prevent API crashes
2. **Consider logging when fallback is used** - For debugging/monitoring
3. **Document this API limitation** - In OPENCODE_ZEN_MODELS.md or KNOWN_ISSUES.md
4. **Consider reporting to OpenCode** - This appears to be a server-side bug

### Related Files
- `internal/provider/opencode.go` - Contains fallback logic
- `internal/provider/openai_common.go` - Shared message validation
- `documentation/guides/OPENCODE_ZEN_MODELS.md` - Provider guide
