# Goroutine Cleanup Analysis - Exit Hang Investigation

**Date:** 2026-02-06  
**Issue:** Application hangs when trying to exit, terminal becomes unresponsive  
**Root Cause:** Multiple goroutines not receiving shutdown signals properly

---

## Executive Summary

The application has **5 distinct categories of goroutines** that are not being properly cleaned up on exit:

1. **Mysis processing loops** - Each running mysis has a goroutine in `run()` that waits on channels
2. **Event bus subscriber** - TUI listens on event channel forever
3. **Signal handler goroutine** - Shutdown handler in main.go
4. **Provider streaming goroutines** - Ollama/OpenCode streaming responses
5. **Async message senders** - Short-lived goroutines spawned by Commander

**Critical Finding:** The TUI's event listening goroutine (`listenForEvents()`) creates an **infinite loop** that never terminates because the event channel is never closed before `tea.Quit()` is called.

---

## Detailed Analysis

### 1. Mysis Processing Loop (`internal/core/mysis.go:913-944`)

**Location:** `Mysis.run(ctx context.Context)`

```go
func (m *Mysis) run(ctx context.Context) {
    ticker := time.NewTicker(constants.IdleNudgeInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return  // ✅ CORRECT: Returns on context cancellation
        case <-ticker.C:
            // Nudge logic
        case <-a.nudgeCh:
            // Process nudge
        }
    }
}
```

**Status:** ✅ **CORRECT** - Properly exits when context is canceled

**How it's cleaned up:**
1. `Mysis.Stop()` (line 225) calls `a.cancel()` which cancels the context
2. `run()` receives signal on `<-ctx.Done()` and returns
3. `commander.StopAll()` (main.go:166) calls `Stop()` on all myses

**Verification:**
- Context is created in `Mysis.Start()` (line 191): `ctx, cancel := context.WithCancel(context.Background())`
- Goroutine is launched (line 216): `go a.run(ctx)`
- Cleanup waits for turn to complete (line 239-240): `a.turnMu.Lock()` ensures current operation finishes

**Potential Issue:** If a mysis is in the middle of a long-running `SendMessage()` call (which can take time for LLM processing), the `Stop()` will block waiting for `turnMu.Lock()`. This could cause **delays but not hangs** since LLM calls have timeouts.

---

### 2. Event Bus Subscriber Loop (`internal/tui/app.go:862-870`)

**Location:** `Model.listenForEvents()`

```go
func (m Model) listenForEvents() tea.Cmd {
    return func() tea.Msg {
        event, ok := <-m.eventCh
        if !ok {
            return nil  // Channel closed
        }
        return EventMsg{Event: event}
    }
}
```

**Status:** ⚠️ **POTENTIAL HANG** - Depends on event channel being closed

**How it's supposed to work:**
1. `bus.Subscribe()` (main.go:146) returns a buffered channel
2. TUI reads from channel in `listenForEvents()`
3. On each event, it calls `listenForEvents()` again (line 191)
4. This creates a **recursive event loop**

**The Problem:**
- `bus.Close()` (main.go:81) is deferred and **runs after `program.Run()` returns**
- `program.Run()` (main.go:161) **blocks until TUI exits**
- TUI is waiting for events from `eventCh`
- **Deadlock potential:** TUI waits for event → program.Run() blocks → bus.Close() never called

**Shutdown sequence (main.go:141-173):**
```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

eventCh := bus.Subscribe()
model := tui.New(commander, s, eventCh)
program := tea.NewProgram(model, tea.WithAltScreen())

go func() {
    <-sigCh
    log.Info().Msg("Received shutdown signal")
    commander.StopAll()  // ← This stops myses
    program.Quit()       // ← This should exit TUI
}()

if _, err := program.Run(); err != nil {  // ← BLOCKS HERE
    log.Fatal().Err(err).Msg("TUI error")
}

commander.StopAll()  // Redundant
```

**The Race Condition:**
1. User presses `q` in TUI → `keys.Quit` matched (app.go:171) → `return m, tea.Quit`
2. Bubble Tea starts shutdown
3. TUI's `Update()` loop exits
4. **BUT** `listenForEvents()` command is still waiting on `<-m.eventCh`
5. Event channel won't close until `bus.Close()` is called
6. `bus.Close()` is deferred until after `program.Run()` returns
7. `program.Run()` won't return until all commands complete
8. **HANG:** Waiting for event that will never come

---

### 3. Signal Handler Goroutine (`main.go:152-158`)

**Location:** `go func()` in main

```go
go func() {
    <-sigCh
    log.Info().Msg("Received shutdown signal")
    commander.StopAll()
    program.Quit()
}()
```

**Status:** ✅ **CORRECT** - Will exit when sigCh receives signal

**How it works:**
1. Goroutine blocks on `<-sigCh`
2. When SIGINT/SIGTERM received, calls `program.Quit()`
3. Goroutine exits after calling `Quit()`

**Verification:** This is a standard pattern and should work correctly.

---

### 4. Provider Streaming Goroutines

**Ollama Location:** `internal/provider/ollama.go:240`
**OpenCode Location:** `internal/provider/opencode.go:203`

```go
// Ollama
go func() {
    defer close(streamCh)
    for {
        response, err := stream.Recv()
        if err == io.EOF {
            return
        }
        if err != nil {
            streamCh <- streamResult{err: err}
            return
        }
        streamCh <- streamResult{chunk: response}
    }
}()
```

**Status:** ✅ **CORRECT** - Returns on EOF or error

**How cleanup works:**
1. These goroutines are spawned during LLM streaming calls
2. They exit naturally when stream ends (EOF) or errors
3. Context timeout in `SendMessage()` (mysis.go:312) ensures they don't block forever
4. Short-lived - only exist during active LLM calls

**Verification:** These follow standard streaming patterns and have proper cleanup.

---

### 5. Async Message Sender Goroutines

**Location:** `internal/core/commander.go:266-271`

```go
func (c *Commander) SendMessageAsync(id, content string) error {
    // ...validation...
    go func() {
        if err := mysis.SendMessage(content, store.MemorySourceDirect); err != nil {
            // Error is published to bus by mysis.SendMessage
        }
    }()
    return nil
}
```

**Status:** ✅ **CORRECT** - Fire-and-forget pattern

**How cleanup works:**
1. These are spawned for async operations
2. They complete and exit naturally
3. No cleanup needed - they're not long-lived

**Verification:** Standard async pattern, no issues.

---

## Root Cause Analysis

### Primary Issue: Event Bus Cleanup Order

**The Bug:**
```go
// main.go
defer bus.Close()  // Line 81 - Deferred
// ...
program := tea.NewProgram(model, tea.WithAltScreen())
// ...
if _, err := program.Run(); err != nil {  // Blocks until TUI exits
    log.Fatal().Err(err).Msg("TUI error")
}
// defer bus.Close() executes here (AFTER program.Run returns)
```

**The TUI Event Loop:**
```go
// app.go:862
func (m Model) listenForEvents() tea.Cmd {
    return func() tea.Msg {
        event, ok := <-m.eventCh  // ← BLOCKS FOREVER if channel not closed
        if !ok {
            return nil
        }
        return EventMsg{Event: event}
    }
}

// app.go:191
case EventMsg:
    m.handleEvent(msg.Event)
    return m, m.listenForEvents()  // ← Re-registers listener
```

**Why it hangs:**

1. TUI calls `listenForEvents()` which returns a command
2. Bubble Tea runtime executes command in background
3. Command blocks on `<-m.eventCh`
4. User presses `q` → TUI returns `tea.Quit`
5. Bubble Tea tries to shut down
6. **BUT** the `listenForEvents` command is still blocked waiting for an event
7. `program.Run()` won't return until all pending commands complete
8. Event channel won't be closed until `bus.Close()` runs
9. `bus.Close()` won't run until `program.Run()` returns
10. **DEADLOCK**

---

## Secondary Issues

### Issue 2A: Redundant StopAll Call

```go
// main.go:152-158
go func() {
    <-sigCh
    log.Info().Msg("Received shutdown signal")
    commander.StopAll()  // Called here
    program.Quit()
}()

// main.go:166 (after program.Run returns)
commander.StopAll()  // Called again (redundant)
```

**Impact:** Minor - `StopAll()` is idempotent (checks state before stopping), but wasteful.

### Issue 2B: No Wait for Goroutines

The shutdown sequence doesn't wait for goroutines to complete:

```go
commander.StopAll()  // Initiates stop
// No wait here - proceeds immediately
if err := s.ReleaseAllAccounts(); err != nil {  // Might race with myses still stopping
    log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}
```

**Impact:** Myses might still be finishing their turns when accounts are released.

---

## Solutions

### Solution 1: Close Event Bus Before Waiting for TUI (Recommended)

**Change shutdown sequence to close bus first:**

```go
// main.go (current)
defer bus.Close()  // BAD: Runs after program.Run() returns

// main.go (fixed)
defer func() {
    bus.Close()      // Close before waiting for program
    s.Close()        // Then close store
}()
```

**Better approach - explicit cleanup:**

```go
// main.go:152-158 (in signal handler)
go func() {
    <-sigCh
    log.Info().Msg("Received shutdown signal")
    commander.StopAll()
    bus.Close()       // ← Close bus to unblock TUI event listener
    program.Quit()    // ← Then quit TUI
}()

// main.go:171-173 (after program.Run)
commander.StopAll()   // Ensure all myses stopped
bus.Close()           // Ensure bus closed (idempotent if already closed)
if err := s.ReleaseAllAccounts(); err != nil {
    log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}
s.Close()
```

**Why this works:**
1. Signal handler closes bus **before** calling `program.Quit()`
2. Closing bus makes `<-m.eventCh` return immediately with `ok = false`
3. `listenForEvents()` returns `nil` → command completes
4. Bubble Tea can now exit cleanly
5. `program.Run()` returns
6. Cleanup proceeds normally

---

### Solution 2: Add Context to TUI Event Listener (More Robust)

**Modify TUI to accept a context:**

```go
// tui/app.go
type Model struct {
    // ...existing fields...
    ctx context.Context  // Add shutdown context
}

func New(commander *core.Commander, s *store.Store, eventCh <-chan core.Event, ctx context.Context) Model {
    return Model{
        // ...existing initialization...
        ctx: ctx,
    }
}

func (m Model) listenForEvents() tea.Cmd {
    return func() tea.Msg {
        select {
        case event, ok := <-m.eventCh:
            if !ok {
                return nil
            }
            return EventMsg{Event: event}
        case <-m.ctx.Done():  // ← Exit on context cancellation
            return nil
        }
    }
}
```

**Main.go changes:**

```go
// main.go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

eventCh := bus.Subscribe()
model := tui.New(commander, s, eventCh, ctx)  // Pass context
program := tea.NewProgram(model, tea.WithAltScreen())

go func() {
    <-sigCh
    log.Info().Msg("Received shutdown signal")
    commander.StopAll()
    cancel()          // Cancel context to unblock event listener
    program.Quit()
}()
```

**Why this is more robust:**
- Event listener can exit even if bus isn't closed
- Provides explicit shutdown signal
- Follows Go idioms for goroutine cleanup

---

### Solution 3: Make Bus.Close() Idempotent and Call Earlier

**Modify bus.Close() to allow multiple calls:**

```go
// internal/core/bus.go:179
func (b *EventBus) Close() {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.subscribers == nil {  // Already closed
        return
    }

    for _, ch := range b.subscribers {
        ch.close()
    }
    b.subscribers = nil
}
```

**Status:** Already appears to be idempotent (line 185 sets subscribers to nil).

**Update main.go:**

```go
// Call Close() explicitly before waiting for TUI
go func() {
    <-sigCh
    log.Info().Msg("Received shutdown signal")
    commander.StopAll()
    bus.Close()       // Close here
    program.Quit()
}()

// Also close on normal quit
case key.Matches(msg, keys.Quit):
    bus.Close()       // Close bus before quit
    return m, tea.Quit
```

**Issue:** This requires accessing `bus` from inside TUI, which breaks encapsulation.

---

## Recommended Fix

**Implement Solution 1 + Solution 2 hybrid:**

### Step 1: Close bus in signal handler

```go
// main.go:152-158
go func() {
    <-sigCh
    log.Info().Msg("Received shutdown signal")
    commander.StopAll()
    bus.Close()       // Close bus to unblock TUI
    program.Quit()
}()
```

### Step 2: Close bus on normal quit

**Add a cleanup callback to TUI:**

```go
// tui/app.go
type Model struct {
    // ...existing fields...
    onQuit func()  // Cleanup callback
}

func New(commander *core.Commander, s *store.Store, eventCh <-chan core.Event) Model {
    return Model{
        // ...existing...
        onQuit: nil,  // Set by main
    }
}

func (m Model) SetQuitCallback(cb func()) {
    m.onQuit = cb
}

// In Update() when handling quit
case key.Matches(msg, keys.Quit):
    if m.onQuit != nil {
        m.onQuit()  // Call cleanup before quit
    }
    return m, tea.Quit
```

**Main.go:**

```go
model := tui.New(commander, s, eventCh)
model.SetQuitCallback(func() {
    commander.StopAll()
    bus.Close()
})
program := tea.NewProgram(model, tea.WithAltScreen())
```

---

## Testing Strategy

### Test 1: Normal Exit (Press 'q')

**Expected behavior:**
1. TUI receives quit key
2. Calls `commander.StopAll()`
3. Calls `bus.Close()` to unblock event listener
4. Returns `tea.Quit`
5. Bubble Tea exits cleanly
6. `program.Run()` returns
7. Cleanup code runs
8. Application exits

**How to test:**
```bash
./bin/zoea --offline
# Press 'q'
# Should exit immediately (< 1 second)
```

### Test 2: Signal Exit (Ctrl+C)

**Expected behavior:**
1. Signal handler receives SIGINT
2. Calls `commander.StopAll()`
3. Calls `bus.Close()`
4. Calls `program.Quit()`
5. Same as Test 1 from step 5 onward

**How to test:**
```bash
./bin/zoea --offline
# Press Ctrl+C
# Should exit immediately
```

### Test 3: Exit with Running Myses

**Expected behavior:**
1. Create 3 myses, start them
2. Wait for them to be actively processing
3. Press 'q'
4. Each mysis receives `Stop()` call
5. Each mysis finishes current turn (respects `turnMu`)
6. All myses exit cleanly
7. Application exits

**How to test:**
```bash
./bin/zoea --offline
# Press 'n', create 3 myses
# Wait a few seconds for them to start processing
# Press 'q'
# Should exit within 2-3 seconds (waiting for LLM timeouts)
```

### Test 4: Exit During LLM Call

**Expected behavior:**
1. Start mysis, send it a message
2. While LLM is processing (loading indicator visible)
3. Press 'q'
4. Mysis respects context timeout (constants.LLMRequestTimeout)
5. Application exits after timeout expires

**How to test:**
```bash
./bin/zoea
# Create mysis, start it
# Send message with 'm'
# Immediately press 'q' while "thinking"
# Should exit within LLMRequestTimeout seconds
```

---

## Additional Observations

### Good Patterns Found

1. **Context-based cancellation in Mysis.run()** - Clean shutdown pattern
2. **Turnlock mutex in Stop()** - Ensures operations complete before stopping
3. **Deferred ticker.Stop() in run()** - Prevents timer leaks
4. **Buffered event bus** - Reduces blocking risk

### Areas of Concern

1. **No sync.WaitGroup for goroutine tracking** - Can't wait for all goroutines to finish
2. **No graceful shutdown timeout** - If myses hang, app hangs forever
3. **Event bus subscriber never unsubscribed** - TUI doesn't call `Unsubscribe()`
4. **No goroutine leak testing** - Runtime stats not monitored

---

## Implementation Checklist

- [ ] Implement Solution 1: Close bus before quit in signal handler
- [ ] Add quit callback to TUI model
- [ ] Test normal exit (press 'q')
- [ ] Test signal exit (Ctrl+C)
- [ ] Test exit with running myses
- [ ] Test exit during LLM call
- [ ] Add graceful shutdown timeout (optional)
- [ ] Add sync.WaitGroup for goroutine tracking (optional)
- [ ] Add runtime/pprof goroutine profiling (debugging)
- [ ] Document cleanup sequence in AGENTS.md

---

## References

- `internal/core/mysis.go:913-944` - Mysis.run() loop
- `internal/core/commander.go:354-368` - Commander.StopAll()
- `internal/tui/app.go:862-870` - TUI event listener
- `internal/core/bus.go:179-187` - EventBus.Close()
- `cmd/zoea/main.go:141-173` - Main shutdown sequence
- `internal/constants/constants.go` - Timeout constants

---

**Analysis Date:** 2026-02-06  
**Analyst:** OpenCode Agent (Claude Sonnet 4.5)
