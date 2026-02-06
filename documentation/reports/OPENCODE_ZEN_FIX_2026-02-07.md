# OpenCode Zen 500 Error Fix - 2026-02-07

## Problem

OpenCode Zen provider was failing with 500 errors:
```
"Cannot read properties of undefined (reading 'prompt_tokens')"
```

## Root Causes Identified

1. **Missing Stream Parameter**: Requests didn't set `Stream:false`, causing server to default to streaming mode
2. **Aggressive Message Merging**: System message merging collapsed 21 messages to 2, losing conversation history
3. **No Message Validation**: Invalid message structures (assistant without user) were sent to API

## Fixes Applied

### 1. Add Stream Parameter
- **File:** `internal/provider/opencode.go`
- **Commit:** `22b77f5`
- Added `Stream: false` to Chat and ChatWithTools methods
- Ensures server returns complete JSON response

### 2. Improve System Message Merging
- **File:** `internal/provider/openai_common.go`
- **Commit:** `9348f50`
- New strategy: Merge all system messages to start, preserve all conversation messages
- **Before:** 21 messages → 2 messages (broken)
- **After:** 21 messages → reasonable count with conversation intact

### 3. Add Message Validation
- **File:** `internal/provider/openai_common.go`
- **Commit:** `e5860b4`
- New function: `validateOpenAIMessages()`
- Checks for empty arrays, assistant without user, consecutive assistant messages
- Logs warnings for potential issues

## Test Coverage Added

### Unit Tests (8 tests)
- **Commit:** `f7ffc26`
- 4 tests for improved message merging:
  - TestMergeSystemMessagesOpenAI_PreservesConversation
  - TestMergeSystemMessagesOpenAI_OnlySystemMessages
  - TestMergeSystemMessagesOpenAI_NoSystemMessages
  - TestMergeSystemMessagesOpenAI_EmptyInput
- 4 tests for message validation:
  - TestValidateOpenAIMessages_Valid
  - TestValidateOpenAIMessages_EmptyArray
  - TestValidateOpenAIMessages_AssistantWithoutUser
  - TestValidateOpenAIMessages_OnlySystemMessages

### Integration Tests (2 tests)
- **Commit:** `604a7c5`
- TestOpenCode_PreservesConversationHistory
- TestOpenCode_StreamParameterSetCorrectly

**Total:** 10 new tests, all passing ✓

## Verification

- ✓ All tests pass (`make test` - 87.0% provider coverage)
- ✓ Builds successfully (`make build`)
- ✓ Unit tests verify correct behavior
- ✓ Integration tests verify API contract

## Commits

1. `22b77f5` - fix(provider): add explicit Stream:false parameter to OpenCode requests
2. `e5860b4` - feat(provider): add OpenAI message validation
3. `9348f50` - fix(provider): improve system message merging to preserve conversation
4. `f7ffc26` - test(provider): add tests for improved system message merging and validation
5. `604a7c5` - test(provider): add integration tests for OpenCode fixes

## Impact

- OpenCode Zen provider now sends proper API requests
- Conversation history preserved correctly
- Better error detection and logging
- Improved test coverage for provider layer (87.0%)
- Message validation prevents invalid API calls

## Technical Details

### Before Fix
```json
{
  "model": "glm-4.7-free",
  "messages": [
    {"role": "system", "content": "... 20 merged system messages ..."},
    {"role": "assistant", "content": "..."}
  ]
  // No Stream parameter (defaults to true)
  // Only 2 messages (conversation destroyed)
  // No user message (invalid)
}
```

### After Fix
```json
{
  "model": "glm-4.7-free",
  "messages": [
    {"role": "system", "content": "... merged system messages ..."},
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."},
    {"role": "user", "content": "..."}
    // ... conversation preserved ...
  ],
  "stream": false,
  "temperature": 0.7,
  "tools": [...]
}
```

## Next Steps

To verify the fix with real API:
1. Rebuild: `make build`
2. Start app: `./bin/zoea -debug`
3. Try starting `deadlock-test` mysis (OpenCode Zen)
4. Check logs for success: `tail -f ~/.zoea-nova/zoea.log`

Expected: No 500 errors, mysis runs successfully
