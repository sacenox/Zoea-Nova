# Agent 3 - Race Condition Reproduction Report

**Mission:** Test if multiple SendMessage goroutines cause the Stop() race condition.

**Status:** ✅ **RACE CONDITION SUCCESSFULLY REPRODUCED**

---

## Test Design

Created two comprehensive tests in `internal/core/mysis_race_test.go`:

### Test 1: `TestStopWithMultipleInFlightMessages`
- Mock provider with **2 second delay** (simulates slow LLM)
- **3 concurrent messages** sent rapidly
- Stop() called while messages are queued/in-flight
- Expected: State = Stopped
- Actual: State = Errored

### Test 2: `TestStopWithMultipleInFlightMessages_Shorter`
- Mock provider with **200ms delay** (faster iteration)
- **5 concurrent messages** sent rapidly
- Same Stop() scenario
- Same race condition

---

## Test Results

### Reproduction Rate
```
Run count: 10 iterations
Success rate: 10/10 (100%)
Race detector: No memory safety issues
```

**BOTH tests consistently reproduce the race condition on every run.**

---

## Evidence of Race Condition

### Failing Test Output
```
=== RUN   TestStopWithMultipleInFlightMessages
    mysis_race_test.go:104: RACE DETECTED: expected final state=Stopped, got errored
    mysis_race_test.go:108: State is Errored. LastError: context canceled
    mysis_race_test.go:111: SendMessage errors:
    mysis_race_test.go:113:   Message 1: mysis not running
    mysis_race_test.go:113:   Message 2: mysis not running
    mysis_race_test.go:113:   Message 3: provider chat: context canceled
```

### Key Observations

1. **Message 3** (in-flight): Gets `context canceled` error
2. **Messages 1 & 2** (queued): Get `mysis not running` error
3. **Final state**: `errored` instead of `stopped`
4. **LastError**: `context canceled`

---

## Root Cause Analysis

### The Race Window

```
Timeline of events:

1. Message 3 acquires turnMu, starts 2s LLM call
2. Messages 1 & 2 queue up, blocked on turnMu
3. Stop() is called:
   - Sets cancel() -> Message 3's context gets canceled
   - Waits for turnMu with 5s timeout
4. Message 3's provider returns error: "context canceled"
5. SendMessageFrom() calls setError() at line 509
   - Changes state to Errored
   - Sets lastError = "context canceled"
6. Message 3 releases turnMu
7. Stop() acquires turnMu
8. Stop() checks state at line 304:
   - State is now Errored (not Running)
   - Returns early without setting state to Stopped
9. Messages 1 & 2 try to acquire turnMu:
   - State is Errored -> return "mysis not running"
```

### The Bug

**In `SendMessageFrom()` at line 507-510:**
```go
if err != nil {
    a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})
    a.setError(err)  // ← BUG: Changes state to Errored during Stop()
    return fmt.Errorf("provider chat: %w", err)
}
```

**In `Stop()` at line 304-306:**
```go
a.mu.Lock()
if a.state != MysisStateRunning {  // ← State is Errored, so we return early
    a.mu.Unlock()
    return nil
}
```

**Problem:** SendMessage's error handling doesn't check if Stop() is in progress before calling `setError()`.

---

## Race Mechanism

### Why Multiple Messages Trigger This

With **single message**:
- Stop() waits for turnMu
- Message returns error, calls setError()
- Stop() completes before checking state
- Usually works (timing dependent)

With **multiple messages**:
- Message 3 has turnMu, gets canceled
- Stop() is waiting for turnMu
- Message 3 calls setError() → state = Errored
- Stop() acquires turnMu, sees state != Running
- Stop() returns without fixing state
- **Race window is wider and more consistent**

---

## Race Detector Results

```bash
go test ./internal/core -run TestStopWithMultipleInFlightMessages -count=10 -race
```

**Result:** No data race warnings from Go race detector

**Interpretation:** This is a **logical race condition** (state machine ordering issue), not a memory safety issue. The mutex protections are correct, but the state transition logic has a bug.

---

## Test Commands

### Run once with race detector:
```bash
go test ./internal/core -run TestStopWithMultipleInFlightMessages -count=1 -race -v
```

### Stress test (10 runs):
```bash
go test ./internal/core -run TestStopWithMultipleInFlightMessages -count=10 -race
```

### Fast iteration (shorter delay):
```bash
go test ./internal/core -run TestStopWithMultipleInFlightMessages_Shorter -count=20
```

---

## Conclusion

✅ **Successfully reproduced the race condition with 100% consistency**

The test proves that multiple in-flight SendMessage calls create a race where:
1. Context cancellation causes SendMessage to error
2. SendMessage calls setError() and changes state to Errored
3. Stop() sees Errored state and returns early
4. Final state remains Errored instead of Stopped

**Next Steps:** Pass this evidence to Agent 4 for implementing the fix.

---

## Test File Location

`internal/core/mysis_race_test.go` - 202 lines, 2 test functions
