# OpenCode Zen API Bug Verdict - 2026-02-06

## Question
Is the "Cannot read properties of undefined (reading 'input_tokens'/'prompt_tokens')" error an OpenCode Zen API bug or a Zoea Nova bug?

## Verdict: **OPENCODE ZEN API BUG**

## Evidence

### Test 1: Minimal Valid Request (System + User)
**Request:**
```bash
curl -X POST https://opencode.ai/zen/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENCODE_API_KEY}" \
  -d '{
    "model": "glm-4.7-free",
    "messages": [
      {"role": "system", "content": "Test"},
      {"role": "user", "content": "Hello"}
    ],
    "stream": false
  }'
```

**Result:** ✅ SUCCESS
- Returns valid response with content
- Usage tokens reported correctly
- No errors

---

### Test 2: System-Only Request (Reproduces Our Error)
**Request:**
```bash
curl -X POST https://opencode.ai/zen/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENCODE_API_KEY}" \
  -d '{
    "model": "glm-4.7-free",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."}
    ],
    "stream": false
  }'
```

**Result:** ❌ ERROR
```json
{
  "type": "error",
  "error": {
    "type": "error",
    "message": "Cannot read properties of undefined (reading 'prompt_tokens')"
  }
}
```

**This is the EXACT error we see in Zoea Nova when sending system-only messages.**

---

### Test 3: Known OpenCode Zen Issue
Found related issue in OpenCode Zen GitHub:

**Issue #8228: OpenCode Zen Gemini Integration - 500 Internal Server Error**
- URL: https://github.com/anomalyco/opencode/issues/8228
- Opened: Jan 13, 2026
- Status: OPEN
- Error: `"Cannot read properties of undefined (reading 'promptTokenCount')"`
- Same pattern: OpenCode Zen backend fails to parse usage/token metadata
- Affects Gemini models specifically, but demonstrates systemic issue with token counting

---

### Test 4: Message Pattern Validation (From Previous Investigation)

| Message Pattern | OpenCode Zen Result |
|----------------|-------------------|
| System + User | ✅ Works |
| System + User + Tools | ✅ Works |
| **System only** | ❌ **500 Error** |
| System + Assistant | ⚠️ Weird output |
| Only Assistant | ⚠️ Empty response |

**Source:** `documentation/investigations/OPENCODE_ZEN_API_TESTS_2026-02-06.md`

---

## Root Cause Analysis

### OpenCode Zen API Bug
The OpenCode Zen API has a server-side bug where:

1. **System-only messages crash the token counting logic**
   - Error: `"Cannot read properties of undefined (reading 'prompt_tokens')"`
   - The API attempts to read token counts from an undefined object
   - This is an unhandled null/undefined case in their response parsing

2. **Inconsistent token metadata handling**
   - Related issue #8228 shows same pattern with Gemini: `"Cannot read properties of undefined (reading 'promptTokenCount')"`
   - Suggests systemic issue in how OpenCode Zen wraps/transforms upstream API responses
   - Token counting fails when response metadata is missing or in unexpected format

3. **No graceful degradation**
   - Returns 500 Internal Server Error instead of handling edge cases
   - Should either return partial response or meaningful error message
   - Breaking on system-only messages is not OpenAI-spec compliant

### Our Workaround is Correct
**File:** `internal/provider/openai_common.go`

```go
// If only system messages remain, add a dummy user message
if allMessagesAreSystem {
    messages = append(messages, openAIMessage{
        Role:    "user",
        Content: "Continue.",
    })
}
```

**This workaround is necessary and correct** because:
- It prevents hitting the OpenCode Zen API bug
- It's a defensive measure against their server-side issue
- It follows OpenAI best practices (conversations should have user input)
- It's documented in our codebase as a known limitation

---

## Comparison to OpenAI Spec

According to OpenAI's official API:
- System-only messages are technically valid (though discouraged)
- Should not crash with 500 errors
- OpenAI's API handles this gracefully (returns empty response or asks for user input)

OpenCode Zen is **not fully OpenAI-compatible** in this edge case.

---

## Implications for Zoea Nova

### What We Did Right
1. ✅ Implemented fallback logic for system-only messages
2. ✅ Added validation to ensure messages are OpenAI-spec compliant
3. ✅ Documented the limitation in investigation reports
4. ✅ Our fix prevents hitting the OpenCode Zen bug

### What We Should Do
1. **Keep the workaround** - It's protecting us from an upstream bug
2. **Document as known OpenCode Zen limitation** - Update `KNOWN_SERVER_ISSUES.md`
3. **Consider reporting to OpenCode** - Issue #8228 shows they're aware of token counting bugs
4. **Add test coverage** - Ensure our fallback logic is tested (already exists)

---

## Conclusion

**THIS IS AN OPENCODE ZEN API BUG, NOT A ZOEA NOVA BUG.**

Our fallback logic is a correct defensive measure against their server-side issue. We should:
1. Keep the workaround in place
2. Document it as a known OpenCode Zen limitation
3. Monitor OpenCode Zen issue tracker for fixes
4. Consider reporting if not already documented

---

## Related Files
- `internal/provider/opencode.go` - Our OpenCode provider implementation
- `internal/provider/openai_common.go` - Contains fallback logic
- `documentation/investigations/OPENCODE_ZEN_API_TESTS_2026-02-06.md` - Detailed test results
- `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md` - Our fix implementation

---

## External References
- OpenCode Issue #8228: https://github.com/anomalyco/opencode/issues/8228
- OpenCode Zen Docs: https://opencode.ai/docs/zen/
- OpenCode Zen Models API: https://opencode.ai/zen/v1/models
