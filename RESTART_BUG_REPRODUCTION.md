# Restart Bug Reproduction Guide

## Bug Description
When stopping a mysis during active execution (LLM call or tool call), the mysis transitions to "errored" state instead of "stopped". Attempting to restart from errored state may cause panics or unexpected behavior.

## Root Cause (From Agent Analysis)
Race condition between `Stop()` and `setError()`:
1. User presses 's' → `Stop()` cancels context and sets state to `Stopped`
2. In-flight turn errors due to context cancellation
3. `setError()` overwrites `Stopped` with `Errored`
4. User attempts restart from `Errored` state
5. Potential issues with context/goroutine cleanup

## Instrumentation Patch

Add debug logging to trace execution flow:

```go
// File: internal/core/mysis.go

// In Start() method (after line 197):
func (m *Mysis) Start() error {
	a := m
	log.Debug().
		Str("mysis", a.name).
		Str("mysis_id", a.id).
		Msg("=== START CALLED ===")
	
	a.mu.Lock()
	log.Debug().
		Str("mysis", a.name).
		Str("current_state", string(a.state)).
		Bool("has_cancel", a.cancel != nil).
		Bool("has_ctx", a.ctx != nil).
		Msg("Start: acquired lock, checking state")
	
	if a.state == MysisStateRunning {
		a.mu.Unlock()
		log.Debug().Str("mysis", a.name).Msg("Start: already running, returning error")
		return fmt.Errorf("mysis already running")
	}

	oldState := a.state

	// If restarting from errored state, cleanup any existing context/goroutine
	if oldState == MysisStateErrored && a.cancel != nil {
		log.Debug().
			Str("mysis", a.name).
			Msg("Start: restarting from errored, cancelling old context")
		a.cancel() // Cancel old context
		a.mu.Unlock()
		
		log.Debug().Str("mysis", a.name).Msg("Start: waiting for old goroutine to exit")
		// Wait for old goroutine to exit
		done := make(chan struct{})
		go func() {
			a.turnMu.Lock()
			log.Debug().Str("mysis", a.name).Msg("Start: old goroutine acquired turnMu")
			close(done)
			a.turnMu.Unlock()
		}()
		select {
		case <-done:
			log.Debug().Str("mysis", a.name).Msg("Start: old goroutine exited cleanly")
		case <-time.After(2 * time.Second):
			log.Warn().Str("mysis", a.name).Msg("Start: timeout waiting for errored goroutine")
		}
		a.mu.Lock()
		log.Debug().Str("mysis", a.name).Msg("Start: re-acquired lock after cleanup")
	}

	a.mu.Unlock()
	log.Debug().Str("mysis", a.name).Msg("Start: creating new context")

	// Create context first (before any state changes)
	ctx, cancel := context.WithCancel(context.Background())
	log.Debug().
		Str("mysis", a.name).
		Msg("Start: context created, updating store")

	// Update store BEFORE changing in-memory state
	if err := a.store.UpdateMysisState(a.id, store.MysisStateRunning); err != nil {
		cancel()
		log.Error().
			Err(err).
			Str("mysis", a.name).
			Msg("Start: failed to update store")
		return fmt.Errorf("failed to update state in store: %w", err)
	}

	log.Debug().Str("mysis", a.name).Msg("Start: store updated, updating in-memory state")

	// Now that store update succeeded, update in-memory state
	a.mu.Lock()
	a.state = MysisStateRunning
	a.lastError = nil
	a.activityState = ActivityStateIdle
	a.activityUntil = time.Time{}
	a.ctx = ctx
	a.cancel = cancel
	a.mu.Unlock()

	log.Debug().
		Str("mysis", a.name).
		Str("old_state", string(oldState)).
		Str("new_state", string(MysisStateRunning)).
		Msg("Start: state updated, launching goroutine")

	// ... rest of Start() method
}

// In Stop() method (after line 274):
func (m *Mysis) Stop() error {
	a := m
	log.Debug().
		Str("mysis", a.name).
		Str("mysis_id", a.id).
		Msg("=== STOP CALLED ===")
	
	a.mu.Lock()
	log.Debug().
		Str("mysis", a.name).
		Str("current_state", string(a.state)).
		Bool("has_cancel", a.cancel != nil).
		Msg("Stop: acquired lock, checking state")
	
	if a.state != MysisStateRunning {
		a.mu.Unlock()
		log.Debug().
			Str("mysis", a.name).
			Str("state", string(a.state)).
			Msg("Stop: not running, returning early")
		return nil
	}

	log.Debug().Str("mysis", a.name).Msg("Stop: cancelling context")
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()

	log.Debug().Str("mysis", a.name).Msg("Stop: waiting for turnMu")
	// Wait for current turn to finish with timeout
	done := make(chan struct{})
	go func() {
		a.turnMu.Lock()
		log.Debug().Str("mysis", a.name).Msg("Stop: acquired turnMu")
		close(done)
		a.turnMu.Unlock()
	}()

	select {
	case <-done:
		log.Debug().Str("mysis", a.name).Msg("Stop: turn finished successfully")
	case <-time.After(5 * time.Second):
		log.Warn().Str("mysis", a.name).Msg("Stop: timeout - forcing shutdown")
	}

	a.mu.Lock()
	log.Debug().
		Str("mysis", a.name).
		Str("state", string(a.state)).
		Msg("Stop: re-acquired lock, checking state again")
	
	if a.state != MysisStateRunning {
		a.mu.Unlock()
		log.Warn().
			Str("mysis", a.name).
			Str("state", string(a.state)).
			Msg("Stop: state changed during wait (possible race)")
		return nil
	}
	
	// ... rest of Stop() method
}

// In setError() method (after line 771):
func (m *Mysis) setError(err error) {
	a := m
	log.Debug().
		Str("mysis", a.name).
		Str("mysis_id", a.id).
		Err(err).
		Msg("=== SETERROR CALLED ===")
	
	a.mu.Lock()
	log.Debug().
		Str("mysis", a.name).
		Str("current_state", string(a.state)).
		Err(err).
		Msg("setError: acquired lock")
	
	// CHECK: Is mysis already stopped?
	if a.state == MysisStateStopped {
		a.mu.Unlock()
		log.Debug().
			Str("mysis", a.name).
			Err(err).
			Msg("setError: ignoring error because mysis was stopped")
		return
	}
	
	log.Debug().
		Str("mysis", a.name).
		Str("old_state", string(a.state)).
		Str("new_state", string(MysisStateErrored)).
		Err(err).
		Msg("setError: transitioning to errored state")
	
	// ... rest of setError() method
}
```

## Minimal Reproduction Steps

### Prerequisites
1. Build with debug logging: `make build`
2. Clean database: `make db-reset`

### Test Scenario 1: Stop During LLM Call
```bash
# Terminal 1: Start app with debug logging
./bin/zoea --offline --debug 2>&1 | tee /tmp/zoea-restart-test.log

# In TUI:
# 1. Press 'n' to create new mysis
#    - Name: test-mysis
#    - Provider: ollama (local)
#    - Model: qwen2.5-coder:7b (or any slow model)
#
# 2. Press Enter to start mysis
#    - Wait for initial autonomous turn to start
#
# 3. Press 'm' to send message
#    - Message: "Write a detailed analysis of quantum computing"
#    - Wait for LLM call to start
#
# 4. IMMEDIATELY press 's' to stop (while LLM is thinking)
#
# 5. Observe state in dashboard:
#    - Expected: "stopped"
#    - Actual (BUG): "errored"
#
# 6. Press 'r' to restart
#    - Observe logs for panic or errors
#
# 7. Press 'q' to quit
```

### Test Scenario 2: Stop During MCP Tool Call (Offline Mode)
```bash
# Terminal 1: Start app
./bin/zoea --offline --debug 2>&1 | tee /tmp/zoea-restart-test-mcp.log

# In TUI:
# 1. Create mysis (same as above)
# 2. Start mysis
# 3. Send message: "Check the status of my ship"
#    - This will trigger get_ship tool call
# 4. IMMEDIATELY press 's' to stop (during tool execution)
# 5. Observe state (should show "errored" due to bug)
# 6. Press 'r' to restart
# 7. Check logs for panic
```

### Test Scenario 3: Multiple Restart Cycles
```bash
# Stress test the restart logic
./bin/zoea --offline --debug 2>&1 | tee /tmp/zoea-restart-stress.log

# In TUI:
# 1. Create mysis
# 2. Loop 10 times:
#    - Start mysis
#    - Send message
#    - Immediately stop (during LLM call)
#    - Wait 1 second
#    - Restart
#    - Observe state transitions
```

## Log Analysis

After reproduction, analyze logs:

```bash
# Extract all Start/Stop/setError calls
grep -E "START CALLED|STOP CALLED|SETERROR CALLED" /tmp/zoea-restart-test.log

# Check state transitions
grep -E "old_state|new_state|current_state" /tmp/zoea-restart-test.log

# Find race condition evidence
grep -E "state changed during wait|ignoring error because" /tmp/zoea-restart-test.log

# Check for panics
grep -i "panic\|fatal" /tmp/zoea-restart-test.log
```

## Expected Findings

### Race Condition Timeline (Typical)
```
1. [DEBUG] === STOP CALLED === mysis=test-mysis
2. [DEBUG] Stop: current_state=running has_cancel=true
3. [DEBUG] Stop: cancelling context
4. [DEBUG] Stop: waiting for turnMu
5. [DEBUG] === SETERROR CALLED === mysis=test-mysis error="context canceled"
6. [DEBUG] setError: current_state=running
7. [DEBUG] Stop: timeout - forcing shutdown
8. [DEBUG] Stop: re-acquired lock state=running
9. [DEBUG] setError: transitioning to errored state old_state=running new_state=errored
10. [DEBUG] Stop: state changed to stopped (but overwritten!)
```

### Restart Issues (To Investigate)
- Panic when accessing nil context/cancel
- Double context creation
- Goroutine leak (old goroutine never exits)
- WaitGroup imbalance (Add called twice, Done called once)

## Verification Checklist

After applying fix:
- [ ] Stop during LLM call → state = "stopped" (not "errored")
- [ ] Restart from stopped state → works without panic
- [ ] Stop during MCP tool call → state = "stopped"
- [ ] Multiple restart cycles → no goroutine leaks
- [ ] Error logs show "ignoring error because mysis was stopped"
- [ ] No "state changed during wait" warnings

## Files to Review
- `internal/core/mysis.go` (lines 197-330, 771-795)
- `internal/core/mysis_test.go` (add test cases)
- `documentation/architecture/MYSIS_STATE_MACHINE.md` (verify transitions)
