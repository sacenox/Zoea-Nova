# Known Provider Issues

This document tracks known issues with LLM provider APIs that affect Zoea Nova. These are upstream bugs in the provider APIs, not Zoea Nova bugs.

## OpenCode Zen

### System-Only Messages Crash Token Counting

**Status:** 🔴 OPEN (Upstream Bug)  
**Severity:** High  
**Reported:** 2026-02-06  
**Affects:** All OpenCode Zen models (`glm-4.7-free`, `kimi-k2.5-free`, etc.)

**Issue:**
OpenCode Zen API returns a 500 Internal Server Error when chat completion requests contain only system messages (no user or assistant messages).

**Error Response:**
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
Server-side bug in OpenCode Zen's response transformation logic. The API fails to handle cases where token count metadata is undefined or missing from upstream provider responses.

**Workaround (Implemented):**
```go
// internal/provider/openai_common.go
// If only system messages remain, add a dummy user message
if allMessagesAreSystem {
    messages = append(messages, openAIMessage{
        Role:    "user",
        Content: "Continue.",
    })
}
```

**Impact:**
- ✅ Workaround prevents API crashes
- ✅ No functional impact on Zoea Nova
- ✅ Fallback is OpenAI-spec compliant

**Related OpenCode Zen Issues:**
- Issue #8228: Gemini models have similar token counting bugs (`"Cannot read properties of undefined (reading 'promptTokenCount')"`)
- Pattern suggests systemic issue in how OpenCode Zen wraps upstream API responses

**References:**
- Investigation: `documentation/investigations/OPENCODE_ZEN_BUG_VERDICT_2026-02-06.md`
- API Tests: `documentation/investigations/OPENCODE_ZEN_API_TESTS_2026-02-06.md`
- Fix Report: `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md`
- OpenCode Issue: https://github.com/anomalyco/opencode/issues/8228

---

## Ollama

### Occasional Context Deadline Exceeded Errors

**Status:** 🟡 INVESTIGATING  
**Severity:** Medium  
**Reported:** 2026-02-05  
**Affects:** Local Ollama installations

**Issue:**
Occasional timeout errors when calling Ollama chat completions:
```
Post "http://localhost:11434/v1/chat/completions": context deadline exceeded
```

**Evidence:**
- 24h log analysis (2026-02-05): 65 HTTP 500s, 19 HTTP 400s, 3 prompt truncations, 23 client disconnects
- Ollama logs show prompt truncation (`limit=32768`, `prompt=41611`) before bursts of errors
- No corresponding errors in `~/.zoea-nova/zoea.log` during same timeframe

**Possible Causes:**
1. Model size vs. context window limits
2. Request timeout configuration
3. Rate limiting interaction with burst requests
4. Ollama server resource constraints

**Current Status:**
- Documented but not resolved
- Needs root cause analysis
- May be related to model configuration or system resources

**References:**
- `documentation/current/KNOWN_ISSUES.md` (Provider Reliability section)

---

## Adding New Provider Issues

When documenting new provider issues:

1. **Verify it's an upstream bug** - Test with minimal curl requests
2. **Check provider issue tracker** - See if already reported
3. **Document workaround** - If implemented in Zoea Nova
4. **Link to investigation** - Reference detailed analysis in `documentation/investigations/`
5. **Update status** - Mark as resolved when fixed upstream

---

Last updated: 2026-02-06
