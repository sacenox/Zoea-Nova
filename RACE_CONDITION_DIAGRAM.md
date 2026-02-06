# Restart Bug Race Condition - Visual Timeline

## The Race

```
TIME →
═════════════════════════════════════════════════════════════════

Thread 1: Stop()                          Thread 2: SendMessageFrom()
─────────────────                         ────────────────────────

1. User presses 's'                       | LLM call in progress
                                          | (holding turnMu lock)
2. Acquire mu lock                        |
   state = running ✓                      |
                                          |
3. Cancel context                         |
   a.cancel() ──────────────────────────→ Context cancelled!
                                          |
4. Release mu lock                        |
                                          |
5. Wait for turnMu...                     |
   (Start timer: 5 seconds)               |
                                          | Provider.Chat() returns error:
                                          | "context canceled"
                                          |
6. TIMEOUT! (5 sec elapsed)               |
   Timer expires                          |
                                          |
7. Continue without turnMu                |
                                          |
8. Acquire mu lock                        |
   Check: state == running? YES           |
                                          |
9. Set: state = Stopped                   |
   a.state = MysisStateStopped            |
                                          |
10. Release mu lock                       |
                                          |
11. Update DB: state = Stopped ✓          |
                                          |
                                          | 12. Call setError(err)
                                          |     Acquire mu lock
                                          |     
                                          | 13. NO CHECK! 
                                          |     (missing: if state == Stopped)
                                          |     
                                          | 14. Set: state = Errored
                                          |     a.state = MysisStateErrored
                                          |     
                                          | 15. Release mu lock
                                          |     
                                          | 16. Update DB: state = Errored
                                          |     
                                          | ❌ OVERWRITES Stopped!

═════════════════════════════════════════════════════════════════

FINAL STATE: Errored ❌ (should be Stopped ✓)
```

## Without Timeout (Rare but possible)

```
TIME →
═════════════════════════════════════════════════════════════════

Thread 1: Stop()                          Thread 2: SendMessageFrom()
─────────────────                         ────────────────────────

1. Cancel context ──────────────────────→ Context cancelled!
                                          | Provider.Chat() errors quickly
                                          |
2. Wait for turnMu...                     | Release turnMu
                                          |
3. Acquire turnMu immediately             |
   (turn finished)                        |
                                          |
4. Release turnMu                         |
                                          |
5. Acquire mu lock                        |
   Set: state = Stopped                   |
   Release mu lock                        |
                                          |
6. Update DB: state = Stopped             |
                                          |
                                          | 7. Call setError(err)
                                          |    Acquire mu lock
                                          |    NO CHECK!
                                          |    Set: state = Errored ❌
                                          |    Update DB: state = Errored

═════════════════════════════════════════════════════════════════

FINAL STATE: Errored ❌ (should be Stopped ✓)
```

## With Fix (Correct Behavior)

```
TIME →
═════════════════════════════════════════════════════════════════

Thread 1: Stop()                          Thread 2: SendMessageFrom()
─────────────────                         ────────────────────────

1. Cancel context ──────────────────────→ Context cancelled!
                                          | Provider.Chat() errors
                                          |
... (timeout or immediate) ...            |
                                          |
5. Set: state = Stopped                   |
   Update DB: state = Stopped ✓           |
                                          |
                                          | 6. Call setError(err)
                                          |    Acquire mu lock
                                          |    
                                          | 7. CHECK: state == Stopped?
                                          |    YES → return early
                                          |    
                                          | 8. Log: "Ignoring error - 
                                          |         mysis was stopped"
                                          |    
                                          | 9. NO STATE CHANGE ✓

═════════════════════════════════════════════════════════════════

FINAL STATE: Stopped ✓ (correct!)
```

## Key Insight

The bug exists because **setError() has no awareness** of intentional stop.

**Without fix**:
```go
func setError(err error) {
    lock()
    state = Errored      // ❌ Always sets errored
    unlock()
}
```

**With fix**:
```go
func setError(err error) {
    lock()
    if state == Stopped {
        unlock()
        return           // ✓ Respects intentional stop
    }
    state = Errored
    unlock()
}
```

## State Transitions

### Current (Buggy)

```
running --[user stops]--> stopped
   |                         |
   |                         |
   +--[turn errors]--------> errored  ❌ Wrong!
                             (overwrites stopped)
```

### Fixed

```
running --[user stops]--> stopped ✓
   |
   |
   +--[turn errors, but stopped]--X  (ignored)
   |
   +--[real error]------------> errored ✓
```

## Why This Matters

**User Experience**:
- User presses 's' (stop) → expects "stopped" state
- Instead sees "errored" state → confusing
- Must relaunch then stop again → annoying
- Loses trust in system reliability

**System Correctness**:
- State machine violated (stopped → errored transition doesn't exist)
- Database inconsistency (stop succeeded but DB shows errored)
- Incorrect error reporting (context cancellation is not an error)

**The Fix**:
- 5 lines of code
- Preserves stop semantics
- Simple and surgical
- No side effects
