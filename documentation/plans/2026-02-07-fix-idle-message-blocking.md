# Fix Idle State Message Blocking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow messages to be sent to Myses in `idle` state, rejecting only `stopped` and `errored` states.

**Architecture:** The current implementation blocks all messages when state != `MysisStateRunning`. The correct behavior is: `idle` and `running` states should accept messages, while `stopped` and `errored` states should reject them with clear error messages.

**Tech Stack:** Go 1.22+, SQLite, Bubble Tea TUI

---

## Context

**Current Incorrect Behavior:**
- Only `MysisStateRunning` accepts messages
- `idle`, `stopped`, and `errored` all reject with "mysis not running"

**Correct Behavior Per User:**
- `idle` → Accept messages (auto-starts the Mysis)
- `running` → Accept messages (current behavior)
- `stopped` → Reject with "mysis stopped - press 'r' to relaunch"
- `errored` → Reject with "mysis errored - press 'r' to relaunch"

**Affected Code Locations:**
1. `internal/core/mysis.go:352` - `SendMessageFrom` state check
2. `internal/core/mysis.go:660` - `SendEphemeralMessage` state check
3. `internal/core/mysis.go:940` - `QueueBroadcast` state check
4. `internal/core/commander.go:270` - `SendMessageAsync` state check (duplicate, can be removed)
5. `documentation/architecture/MYSIS_STATE_MACHINE.md:62` - Documentation

---

## Task 1: Update State Validation Helper

**Files:**
- Modify: `internal/core/mysis.go` (add new helper function)
- Test: `internal/core/mysis_test.go`

**Step 1: Write failing test for state validation helper**

Add to `internal/core/mysis_test.go`:

```go
func TestCanAcceptMessages(t *testing.T) {
	tests := []struct {
		state    MysisState
		canAccept bool
		errMsg    string
	}{
		{MysisStateIdle, true, ""},
		{MysisStateRunning, true, ""},
		{MysisStateStopped, false, "mysis stopped - press 'r' to relaunch"},
		{MysisStateErrored, false, "mysis errored - press 'r' to relaunch"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			err := validateCanAcceptMessage(tt.state)
			
			if tt.canAccept {
				if err != nil {
					t.Errorf("state %s should accept messages, got error: %v", tt.state, err)
				}
			} else {
				if err == nil {
					t.Errorf("state %s should reject messages, got nil error", tt.state)
				} else if err.Error() != tt.errMsg {
					t.Errorf("state %s: expected error %q, got %q", tt.state, tt.errMsg, err.Error())
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/core -run TestCanAcceptMessages -v
```

Expected: FAIL with "undefined: validateCanAcceptMessage"

**Step 3: Implement state validation helper**

Add to `internal/core/mysis.go` after the state constants (around line 150):

```go
// validateCanAcceptMessage checks if a mysis in the given state can accept messages.
// Returns nil if messages are allowed, error with user-facing message if not.
//
// Valid states for accepting messages:
//   - idle: Mysis is not running but can accept messages (will process when started)
//   - running: Mysis is actively running and processing messages
//
// Invalid states for accepting messages:
//   - stopped: User explicitly stopped the mysis, requires relaunch
//   - errored: Mysis encountered an error, requires relaunch
func validateCanAcceptMessage(state MysisState) error {
	switch state {
	case MysisStateIdle, MysisStateRunning:
		return nil
	case MysisStateStopped:
		return fmt.Errorf("mysis stopped - press 'r' to relaunch")
	case MysisStateErrored:
		return fmt.Errorf("mysis errored - press 'r' to relaunch")
	default:
		return fmt.Errorf("unknown mysis state: %s", state)
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/core -run TestCanAcceptMessages -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "feat: add state validation helper for message acceptance"
```

---

## Task 2: Update SendMessageFrom State Check

**Files:**
- Modify: `internal/core/mysis.go:352-354`
- Test: `internal/core/mysis_test.go`

**Step 1: Write failing test for SendMessageFrom with idle state**

Add to `internal/core/mysis_test.go`:

```go
func TestSendMessageFrom_IdleState(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Mysis starts in idle state
	if mysis.State() != MysisStateIdle {
		t.Fatalf("expected idle state, got %s", mysis.State())
	}

	// Should be able to send message to idle mysis
	err := mysis.SendMessageFrom("test message", store.MemorySourceDirect, "")
	if err != nil {
		t.Errorf("should accept message in idle state, got error: %v", err)
	}

	// Verify message was stored
	memories, err := mysis.store.GetMemories(mysis.ID(), 10)
	if err != nil {
		t.Fatalf("failed to get memories: %v", err)
	}
	if len(memories) == 0 {
		t.Error("message was not stored")
	}
}

func TestSendMessageFrom_StoppedState(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Start then stop the mysis
	if err := mysis.Start(); err != nil {
		t.Fatalf("failed to start mysis: %v", err)
	}
	if err := mysis.Stop(); err != nil {
		t.Fatalf("failed to stop mysis: %v", err)
	}

	if mysis.State() != MysisStateStopped {
		t.Fatalf("expected stopped state, got %s", mysis.State())
	}

	// Should NOT be able to send message to stopped mysis
	err := mysis.SendMessageFrom("test message", store.MemorySourceDirect, "")
	if err == nil {
		t.Error("should reject message in stopped state")
	}
	if !strings.Contains(err.Error(), "stopped") || !strings.Contains(err.Error(), "relaunch") {
		t.Errorf("error should mention stopped and relaunch, got: %v", err)
	}
}

func TestSendMessageFrom_ErroredState(t *testing.T) {
	mysis, cleanup := setupTestMysisWithErrorProvider(t)
	defer cleanup()

	// Start mysis and trigger error
	if err := mysis.Start(); err != nil {
		t.Fatalf("failed to start mysis: %v", err)
	}

	// Send message to trigger error
	_ = mysis.SendMessageFrom("trigger error", store.MemorySourceDirect, "")
	
	// Wait for error state
	time.Sleep(100 * time.Millisecond)

	if mysis.State() != MysisStateErrored {
		t.Fatalf("expected errored state, got %s", mysis.State())
	}

	// Should NOT be able to send message to errored mysis
	err := mysis.SendMessageFrom("test message", store.MemorySourceDirect, "")
	if err == nil {
		t.Error("should reject message in errored state")
	}
	if !strings.Contains(err.Error(), "errored") || !strings.Contains(err.Error(), "relaunch") {
		t.Errorf("error should mention errored and relaunch, got: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/core -run "TestSendMessageFrom_(Idle|Stopped|Errored)" -v
```

Expected: 
- `TestSendMessageFrom_IdleState`: FAIL with "should accept message in idle state, got error: mysis not running"
- `TestSendMessageFrom_StoppedState`: PASS (already rejects correctly, but wrong error message)
- `TestSendMessageFrom_ErroredState`: PASS (already rejects correctly, but wrong error message)

**Step 3: Update SendMessageFrom state check**

In `internal/core/mysis.go`, replace lines 352-354:

```go
// OLD (remove):
if state != MysisStateRunning {
    return fmt.Errorf("mysis not running")
}

// NEW (replace with):
if err := validateCanAcceptMessage(state); err != nil {
    return err
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/core -run "TestSendMessageFrom_(Idle|Stopped|Errored)" -v
```

Expected: All PASS

**Step 5: Run full mysis test suite to check for regressions**

```bash
go test ./internal/core -run TestMysis -v
```

Expected: All PASS (some tests might need adjustment)

**Step 6: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "fix: allow SendMessageFrom in idle state, improve error messages"
```

---

## Task 3: Update SendEphemeralMessage State Check

**Files:**
- Modify: `internal/core/mysis.go:659-661`

**Step 1: Write failing test**

Add to `internal/core/mysis_test.go`:

```go
func TestSendEphemeralMessage_IdleState(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Mysis starts in idle state
	if mysis.State() != MysisStateIdle {
		t.Fatalf("expected idle state, got %s", mysis.State())
	}

	// Should be able to send ephemeral message to idle mysis
	err := mysis.SendEphemeralMessage("ephemeral test", store.MemorySourceDirect)
	if err != nil {
		t.Errorf("should accept ephemeral message in idle state, got error: %v", err)
	}
}

func TestSendEphemeralMessage_StoppedState(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Start then stop
	if err := mysis.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	if err := mysis.Stop(); err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	// Should reject ephemeral message in stopped state
	err := mysis.SendEphemeralMessage("ephemeral test", store.MemorySourceDirect)
	if err == nil {
		t.Error("should reject ephemeral message in stopped state")
	}
	if !strings.Contains(err.Error(), "stopped") {
		t.Errorf("error should mention stopped, got: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/core -run "TestSendEphemeralMessage_(Idle|Stopped)" -v
```

Expected: FAIL on idle test

**Step 3: Update SendEphemeralMessage state check**

In `internal/core/mysis.go`, replace lines 659-661:

```go
// OLD (remove):
if state != MysisStateRunning {
    return fmt.Errorf("mysis not running")
}

// NEW (replace with):
if err := validateCanAcceptMessage(state); err != nil {
    return err
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/core -run "TestSendEphemeralMessage_(Idle|Stopped)" -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "fix: allow SendEphemeralMessage in idle state"
```

---

## Task 4: Update QueueBroadcast State Check

**Files:**
- Modify: `internal/core/mysis.go:939-941`

**Step 1: Write failing test**

Add to `internal/core/mysis_test.go`:

```go
func TestQueueBroadcast_IdleState(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Mysis starts in idle state
	if mysis.State() != MysisStateIdle {
		t.Fatalf("expected idle state, got %s", mysis.State())
	}

	// Should be able to queue broadcast to idle mysis
	err := mysis.QueueBroadcast("broadcast test", "sender-id")
	if err != nil {
		t.Errorf("should accept broadcast in idle state, got error: %v", err)
	}

	// Verify message was stored
	memories, err := mysis.store.GetMemories(mysis.ID(), 10)
	if err != nil {
		t.Fatalf("failed to get memories: %v", err)
	}
	if len(memories) == 0 {
		t.Error("broadcast was not stored")
	}
}

func TestQueueBroadcast_StoppedState(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Start then stop
	if err := mysis.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	if err := mysis.Stop(); err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	// Should reject broadcast in stopped state
	err := mysis.QueueBroadcast("broadcast test", "sender-id")
	if err == nil {
		t.Error("should reject broadcast in stopped state")
	}
	if !strings.Contains(err.Error(), "stopped") {
		t.Errorf("error should mention stopped, got: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/core -run "TestQueueBroadcast_(Idle|Stopped)" -v
```

Expected: FAIL on idle test

**Step 3: Update QueueBroadcast state check**

In `internal/core/mysis.go`, replace lines 939-941:

```go
// OLD (remove):
if state != MysisStateRunning {
    return fmt.Errorf("mysis not running")
}

// NEW (replace with):
if err := validateCanAcceptMessage(state); err != nil {
    return err
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/core -run "TestQueueBroadcast_(Idle|Stopped)" -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "fix: allow QueueBroadcast in idle state"
```

---

## Task 5: Remove Duplicate State Check in Commander

**Files:**
- Modify: `internal/core/commander.go:264-278`

**Step 1: Analyze the duplicate check**

The `SendMessageAsync` method in Commander checks state before calling `mysis.SendMessage`, which will check state again. This is redundant and can cause inconsistent error messages.

**Step 2: Remove duplicate check**

In `internal/core/commander.go`, replace lines 264-278:

```go
// OLD (remove entire function):
func (c *Commander) SendMessageAsync(id, content string) error {
	mysis, err := c.GetMysis(id)
	if err != nil {
		return err
	}
	if mysis.State() != MysisStateRunning {
		return fmt.Errorf("mysis not running")
	}
	go func() {
		if err := mysis.SendMessage(content, store.MemorySourceDirect); err != nil {
			// Error is published to bus by mysis.SendMessage
		}
	}()
	return nil
}

// NEW (replace with):
func (c *Commander) SendMessageAsync(id, content string) error {
	mysis, err := c.GetMysis(id)
	if err != nil {
		return err
	}
	// State validation is done inside mysis.SendMessage
	go func() {
		if err := mysis.SendMessage(content, store.MemorySourceDirect); err != nil {
			// Error is published to bus by mysis.SendMessage
		}
	}()
	return nil
}
```

**Step 3: Check if SendMessageAsync is tested**

```bash
grep -n "SendMessageAsync" /home/xonecas/src/zoea-nova/internal/core/*test.go
```

**Step 4: Run commander tests**

```bash
go test ./internal/core -run TestCommander -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/core/commander.go
git commit -m "refactor: remove duplicate state check in SendMessageAsync"
```

---

## Task 6: Update Test Expectations

**Files:**
- Modify: `internal/core/long_running_stop_test.go:339-342`

**Step 1: Update test that expects rejection after stop**

In `internal/core/long_running_stop_test.go`, update the test around line 339:

```go
// OLD (line 339-342):
err := mysis.SendMessage("after-stop", store.MemorySourceDirect)
if err == nil {
	t.Error("expected error sending message to stopped mysis")
}

// NEW (replace with):
err := mysis.SendMessage("after-stop", store.MemorySourceDirect)
if err == nil {
	t.Error("expected error sending message to stopped mysis")
}
if !strings.Contains(err.Error(), "stopped") || !strings.Contains(err.Error(), "relaunch") {
	t.Errorf("expected error about stopped state requiring relaunch, got: %v", err)
}
```

**Step 2: Run the test**

```bash
go test ./internal/core -run TestQueuedMessagesFailAfterStop -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/core/long_running_stop_test.go
git commit -m "test: update stopped state error message expectation"
```

---

## Task 7: Update Documentation

**Files:**
- Modify: `documentation/architecture/MYSIS_STATE_MACHINE.md`

**Step 1: Update state machine documentation**

In `documentation/architecture/MYSIS_STATE_MACHINE.md`, replace line 62 and add clarification:

```markdown
## Message Acceptance

Messages can be sent to Myses in `idle` and `running` states:

- **`idle`**: Accepts messages. Messages are stored and will be processed when the Mysis is started.
- **`running`**: Accepts messages. Messages are processed immediately.
- **`stopped`**: Rejects messages with error "mysis stopped - press 'r' to relaunch"
- **`errored`**: Rejects messages with error "mysis errored - press 'r' to relaunch"

User-initiated stop (`stopped` state) and error conditions (`errored` state) require explicit relaunch before messages can be sent.
```

**Step 2: Update state descriptions**

Around lines 6-10, update state descriptions:

```markdown
## States

- `idle`: Not running, but accepts messages. Messages are stored and processed when started. Set on creation and when a mysis fails 3 nudges; requires a commander message/broadcast or explicit start to resume processing.
- `running`: Active and eligible for nudges; waiting between loop iterations. Accepts messages.
- `stopped`: Explicitly stopped by user action. Rejects messages until relaunched.
- `errored`: Provider or MCP failures after retries; recorded as `lastError`. Rejects messages until relaunched.
```

**Step 3: Review entire document for consistency**

Read through the entire document to ensure no contradictions with the new behavior.

**Step 4: Commit**

```bash
git add documentation/architecture/MYSIS_STATE_MACHINE.md
git commit -m "docs: update state machine message acceptance rules"
```

---

## Task 8: Update Known Issues

**Files:**
- Modify: `documentation/current/KNOWN_ISSUES.md`

**Step 1: Remove the issue if it's listed**

Check if this bug is documented in KNOWN_ISSUES.md. If so, remove it.

```bash
grep -n "idle.*message\|message.*idle\|not running" documentation/current/KNOWN_ISSUES.md
```

**Step 2: Commit if changes made**

```bash
git add documentation/current/KNOWN_ISSUES.md
git commit -m "docs: remove fixed idle state message blocking from known issues"
```

---

## Task 9: Integration Testing

**Files:**
- Test entire flow manually and with existing tests

**Step 1: Run all core tests**

```bash
go test ./internal/core -v -count=1
```

Expected: All PASS

**Step 2: Run all TUI tests**

```bash
go test ./internal/tui -v -count=1
```

Expected: All PASS

**Step 3: Build and manual test**

```bash
make build
```

**Step 4: Manual TUI test**

1. Start application: `./bin/zoea --offline`
2. Create a new Mysis (it starts in `idle` state)
3. Press `m` to send a message to the idle Mysis
4. Verify: Message should be accepted without error
5. Press `r` to start the Mysis
6. Press `s` to stop the Mysis
7. Press `m` to send a message to the stopped Mysis
8. Verify: Should show error "mysis stopped - press 'r' to relaunch"

**Step 5: Document test results**

If any issues found, fix them before proceeding.

---

## Task 10: Final Commit and Verification

**Step 1: Run make test**

```bash
make test
```

Expected: All tests PASS

**Step 2: Run make fmt**

```bash
make fmt
```

**Step 3: Check git status**

```bash
git status
```

Verify all changes are committed.

**Step 4: Create summary commit if needed**

If there are any uncommitted doc changes or minor fixes:

```bash
git add -A
git commit -m "chore: final cleanup for idle state message fix"
```

**Step 5: Review all commits**

```bash
git log --oneline HEAD~10..HEAD
```

Verify commits follow conventional commit style and are logically organized.

---

## Verification Checklist

- [ ] `validateCanAcceptMessage` helper function added and tested
- [ ] `SendMessageFrom` accepts messages in `idle` state
- [ ] `SendEphemeralMessage` accepts messages in `idle` state
- [ ] `QueueBroadcast` accepts messages in `idle` state
- [ ] `stopped` and `errored` states show clear, actionable error messages
- [ ] Duplicate state check removed from `Commander.SendMessageAsync`
- [ ] All existing tests updated for new error messages
- [ ] New tests added for idle state message acceptance
- [ ] Documentation updated to reflect correct behavior
- [ ] Manual TUI testing confirms expected behavior
- [ ] All tests pass: `make test`
- [ ] Code formatted: `make fmt`

---

## Notes

**State Transition Behavior:**

The key insight is that `idle` state means "not actively running but ready to accept work". Messages sent to idle Myses should be stored and ready for processing. This is different from `stopped` (user intentionally paused) and `errored` (system failure) which require explicit relaunch.

**Error Message Design:**

The new error messages guide users to the correct action (`press 'r' to relaunch`) rather than leaving them confused about what "not running" means.

**Test Coverage:**

Tests cover all four states (idle, running, stopped, errored) for all three message methods (SendMessageFrom, SendEphemeralMessage, QueueBroadcast) to ensure consistent behavior.
