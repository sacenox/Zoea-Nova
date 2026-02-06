# Agent 4 - Timing Test Report

## Test: TestStopAtVariousTimings

**Date:** 2026-02-06  
**Test Runs:** 10 iterations per timing window  
**Command:** `go test ./internal/core -run TestStopAtVariousTimings -count=10 -v`

## Test Methodology

Parameterized test calling `Stop()` at different delays after `Start()` to systematically explore timing windows for race conditions.

**Timing windows tested:**
- **0ms** - Immediate (before goroutine starts)
- **10ms** - Very fast (goroutine just started)
- **50ms** - During initial message processing
- **100ms** - After initial message likely done
- **500ms** - After idle nudge interval

**Mock provider delay:** 25ms (simulates real LLM processing)

## Results Summary

| Timing Window | Pass Rate | Status |
|--------------|-----------|--------|
| 0ms          | 7/10      | **INTERMITTENT FAILURE** |
| 10ms         | 0/10      | **CONSISTENT FAILURE** |
| 50ms         | 10/10     | ✅ PASS |
| 100ms        | 10/10     | ✅ PASS |
| 500ms        | 10/10     | ✅ PASS |

## Pattern Analysis

### ✅ CONSISTENTLY PASSES (50ms+)
- **50ms delay:** 10/10 passes
- **100ms delay:** 10/10 passes
- **500ms delay:** 10/10 passes

**Why they pass:** By 50ms, the initial `SendMessage` goroutine spawned by `Start()` has:
1. Acquired `turnMu`
2. Entered `provider.Chat()` (which has a 25ms delay)
3. Context cancellation is properly detected inside the turn
4. `Stop()` waits for turn completion via `turnMu`

### ⚠️ INTERMITTENTLY FAILS (0ms)
- **0ms delay:** 7/10 passes, 3/10 fails

**Why it's intermittent:** Race between:
- `Start()` spawning `go a.SendMessage()` goroutine (line 268)
- Test calling `Stop()` immediately (no sleep)
- Sometimes the goroutine hasn't acquired `turnMu` yet when context is canceled

**Failure mode:**
```
expected state=stopped, got errored (lastError: context canceled)
```

### ❌ CONSISTENTLY FAILS (10ms)
- **10ms delay:** 0/10 passes (100% failure rate)

**Why it consistently fails:** This is the CRITICAL RACE WINDOW.

**Timeline at 10ms:**
1. `Start()` completes, spawns `go a.SendMessage(ContinuePrompt, ...)` (line 268)
2. 10ms delay allows goroutine to start but NOT complete
3. Goroutine acquires `turnMu` and enters turn processing
4. Context logs are visible: `"stage":"before_llm_call"` appears in output
5. Test calls `Stop()` → cancels context
6. `SendMessage` goroutine detects canceled context
7. `SendMessage` calls `a.setErrorState(ctx.Err())` (line in SendMessage)
8. `setErrorState()` sets `state = MysisStateErrored` and `lastError = context canceled`
9. **RACE:** `Stop()` tries to set `state = MysisStateStopped`, but `setErrorState()` wins

**Failure mode:**
```
expected state=stopped, got errored (lastError: context canceled)
```

## Root Cause Identified

**The 10ms window consistently reproduces the race between:**
- `Stop()` setting state to `Stopped`
- Concurrent `SendMessage` goroutine detecting canceled context and calling `setErrorState(Errored)`

**Critical code locations:**

1. **mysis.go:268** - `Start()` spawns initial SendMessage goroutine
   ```go
   go a.SendMessage(a.buildContinuePrompt(), store.MemorySourceSystem)
   ```

2. **mysis.go:274-312** - `Stop()` cancels context and waits for turn
   ```go
   if a.cancel != nil {
       a.cancel()  // Line 283
   }
   // Wait for turnMu (lines 288-293)
   a.state = MysisStateStopped  // Line 312
   ```

3. **SendMessage somewhere** - Detects canceled context and calls `setErrorState()`
   ```go
   if ctx.Err() != nil {
       a.setErrorState(ctx.Err())  // Sets state to Errored
   }
   ```

## Why Longer Delays Work

**50ms+ delays work because:**
- By 50ms, `SendMessage` goroutine is INSIDE `provider.Chat()` call
- `provider.Chat()` respects context cancellation properly
- Turn completes (or is interrupted cleanly)
- `Stop()` acquires `turnMu` after turn finishes
- State is set to `Stopped` without race

**0ms is intermittent because:**
- Race between goroutine spawn and context cancellation
- Sometimes goroutine hasn't started yet → no setErrorState() call → passes
- Sometimes goroutine starts just in time → setErrorState() called → fails

**10ms is the "sweet spot" for the race:**
- Goroutine ALWAYS has time to start (deterministic)
- Goroutine is in the vulnerable window (checking context before LLM call)
- `setErrorState()` is ALWAYS called
- Race with `Stop()` state change is ALWAYS present

## Recommended Fix

The fix must ensure `setErrorState()` checks current state before overriding:

```go
func (m *Mysis) setErrorState(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // DO NOT override Stopped state with Errored
    if m.state == MysisStateStopped {
        return
    }
    
    oldState := m.state
    m.state = MysisStateErrored
    m.lastError = err
    m.mu.Unlock()
    
    m.emitStateChange(oldState, MysisStateErrored)
    m.bus.Publish(Event{Type: EventMysisError, MysisID: m.id, Error: err})
}
```

## Test Coverage

This parameterized test provides:
- ✅ Reproducible evidence of the race condition (10ms window)
- ✅ Clear boundary identification (passes at 50ms+)
- ✅ Intermittent behavior detection (0ms)
- ✅ Statistical confidence (10 runs per window)

**Next steps:**
- Implement the `setErrorState()` guard (Agent 5-10)
- Re-run this test with `-count=50` to verify fix
- All timing windows should pass consistently after fix
