# Restart Errored Mysis Investigation

**Date:** 2026-02-06  
**Issue:** Restarting errored mysis flashes "running" then immediately back to "errored"  
**Status:** ✅ FIXED

---

## Root Cause

**Start() launched async initial SendMessage that could fail AFTER Start() returned success.**

Timeline:
1. User presses 'r' to restart errored mysis
2. Start() returns success → TUI shows "running"
3. Async SendMessage goroutine executes
4. Goroutine encounters error (provider, DB, account, etc.)
5. setError() called → state becomes "errored"
6. **User sees: running → errored flash**

---

## Investigation Method

**10-agent systematic debugging investigation**

Agents deployed:
1. Agent 1-2: Traced Start() and SendMessage flow
2. Agent 3-7: Ruled out contributing factors (accounts, provider, DB, MCP, cleanup)
3. Agent 8: Created reproduction test
4. Agent 9: Designed solution (4 options analyzed)
5. Agent 10: Integration lead - provided fix

---

## Solution

**Move initial nudge from Start() into run() loop**

Before:
```go
func (m *Mysis) Start() error {
    go a.run(ctx)
    go a.SendMessage(...)  // ← ASYNC, can fail after Start() returns
    return nil
}
```

After:
```go
func (m *Mysis) Start() error {
    go a.run(ctx)
    return nil  // Fast return, no async work
}

func (m *Mysis) run(ctx context.Context) {
    // Send initial nudge internally
    select {
    case a.nudgeCh <- struct{}{}:
    default:
    }
    // ... rest of loop
}
```

---

## Benefits

- Start() is fast (returns immediately)
- No async race between Start() success and message failure  
- Clear separation: Start() = lifecycle, run() = work
- Consistent error handling (first message = nth message)
- If first message fails, transitions running → errored inside run() (expected behavior)

---

## Files Modified

- internal/core/mysis.go:267-268 - Removed async SendMessage
- internal/core/mysis.go:1030-1036 - Added initial nudge to run()

---

## Test Results

- All state machine tests: PASS
- TestCommanderRestartErroredMysis: PASS
- Full suite: PASS (77.3% coverage)

---

**Fixed in commit:** b66624a
