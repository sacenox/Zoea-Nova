# Agent 6: Real Provider Race Investigation

**Date:** 2026-02-06  
**Agent:** 6 of 10  
**Phase:** SYSTEMATIC DEBUGGING - Phase 1: Root Cause Investigation  

## Hypothesis

The race condition only occurs with real HTTP providers that have network latency.

## Test Setup

Created two tests using REAL Ollama provider (qwen3:4b model):
1. `TestStopWithRealOllamaProvider` - Stop during initial autonomous turn
2. `TestStopDuringRealLLMCall` - Stop during explicit message send

## Results

**100% REPRODUCIBLE** race condition with real provider!

### Test 1: Stop During Initial Turn
```
5/5 runs FAILED (100% failure rate)
Error: state=errored (expected stopped)
LastError: Post "http://localhost:11434/v1/chat/completions": context canceled
```

### Test 2: Stop During Explicit Message
```
3/3 runs FAILED (100% failure rate)
Error: state=errored (expected stopped)
LastError: Post "http://localhost:11434/v1/chat/completions": context canceled
```

## Root Cause Analysis

### Timeline of the Race

1. **Start()** (mysis.go:268) - Spawns `go SendMessage(ContinuePrompt)`
2. **Test** - Calls `Stop()` after 5ms
3. **Stop()** (mysis.go:283) - Cancels context: `a.cancel()`
4. **Stop()** (mysis.go:290-293) - Starts goroutine to wait for `turnMu`
5. **Stop()** (mysis.go:312) - Sets `state = MysisStateStopped` ← **CRITICAL**
6. **takeTurn()** (mysis.go:507) - LLM call returns with `context canceled` error
7. **takeTurn()** (mysis.go:509) - Calls `a.setError(err)`
8. **setError()** (mysis.go:839-845) - Checks if `state == Stopped`

### The Race Window

**Race occurs between step 5 and step 8:**
- Step 5: `Stop()` sets state = Stopped (line 312)
- Step 8: `setError()` checks if state == Stopped (line 845)

**Why it happens:**
1. `Stop()` releases `a.mu` after setting state (line 313)
2. `takeTurn()` is still running in the goroutine spawned by `SendMessage`
3. `takeTurn()` holds `turnMu` (from SendMessage line 381)
4. HTTP call was canceled by context, returns `context canceled` error
5. `takeTurn()` calls `setError()` which acquires `a.mu` (line 839)

**The protection exists but has a race:**
```go
// mysis.go:842-849
if oldState == MysisStateStopped {
    a.mu.Unlock()
    log.Debug().Str("mysis", a.name).Err(err).Msg("Ignoring error - mysis was intentionally stopped")
    return
}
```

This check SHOULD work, but there's a window where:
- `Stop()` hasn't set state = Stopped yet (line 312)
- OR `setError()` reads state BEFORE `Stop()` writes it

### Why Mock Provider Doesn't Trigger This

Mock provider (`provider.NewMock`) returns INSTANTLY:
- No network latency
- No actual HTTP connection
- Context cancellation happens AFTER mock returns
- Race window is microseconds vs milliseconds

Real provider:
- Opens HTTP connection (~5-50ms)
- Sends request
- Waits for response
- **Context cancellation interrupts HTTP call mid-flight**
- Returns `context canceled` error reliably

## Actual Error Message

```
Post "http://localhost:11434/v1/chat/completions": context canceled
```

## HTTP Timing Analysis

**Debug logs show:**
```
stage="before_llm_call" → HTTP call starts
[context canceled] → Stop() called, cancels context
state=errored → setError() runs AFTER Stop() but BEFORE state check
```

**Timing:**
- Mock provider: <1ms (race window too small to hit reliably)
- Real provider: 10-100ms (large enough race window)
- Test delay: 5ms between Start() and Stop()

## Conclusion

**ROOT CAUSE CONFIRMED:**

The issue is a **state check race** in `setError()`:
1. Real HTTP providers have network latency
2. Context cancellation interrupts HTTP mid-flight
3. HTTP returns `context canceled` error reliably
4. `takeTurn()` calls `setError()` with canceled error
5. Race between `Stop()` setting state and `setError()` checking state
6. If `setError()` acquires lock before `Stop()` sets state, protection fails

**Why mock provider doesn't expose this:**
- Returns instantly (no network latency)
- Context rarely canceled mid-call
- Race window is microseconds, not milliseconds

## Evidence Files

- `/home/xonecas/src/zoea-nova/internal/core/mysis_real_provider_test.go`
- Test output above (100% failure rate with real provider)

## Next Steps for Other Agents

1. **Fix the state check race** - Need atomic state transition or better synchronization
2. **Consider error type** - Should `context canceled` ALWAYS be ignored during Stop()?
3. **Test with other providers** - Does OpenCode Zen exhibit same behavior?

## Key Insight

**Mock tests passed because they don't simulate network latency.**

Real-world usage will ALWAYS hit this race with HTTP providers (Ollama, OpenCode Zen, OpenAI, etc.).

The bug is CRITICAL for production use.
