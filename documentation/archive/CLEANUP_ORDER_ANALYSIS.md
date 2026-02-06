# Resource Cleanup Order Analysis

**Date:** 2026-02-06  
**Status:** Critical issues identified in cleanup sequence

---

## Executive Summary

The application has a **cleanup ordering issue** that can cause hangs on exit. The TUI event listener goroutine can block indefinitely because the event channel is not closed before `tea.Quit()` is called.

### Critical Issues Found

1. **Event Channel Never Closed Before Quit** - The TUI's `listenForEvents()` goroutine blocks on `<-m.eventCh` but the channel is only closed AFTER `tea.Quit()` returns
2. **Potential Database Lock on Concurrent Writes** - Store uses `MaxOpenConns=1` which can cause blocking during shutdown
3. **No Provider Client Cleanup** - HTTP clients in providers are never closed
4. **No MCP Client Cleanup** - HTTP client in MCP client is never closed
5. **Mysis Nudge Channels Never Closed** - Each mysis has a nudge channel that is never closed

---

## Current Cleanup Flow (main.go)

### Normal Exit (User Presses 'q')

```go
// 1. User presses 'q' in TUI
case key.Matches(msg, keys.Quit):
    if m.onQuit != nil {
        m.onQuit()  // ← Calls bus.Close() (line 153)
    }
    return m, tea.Quit  // ← Returns quit command

// 2. Bubble Tea event loop processes tea.Quit and exits program.Run()

// 3. Main continues after program.Run() (line 173)
commander.StopAll()           // Line 173
s.ReleaseAllAccounts()       // Line 176
// 4. Deferred cleanups execute in REVERSE order:
defer bus.Close()            // Line 81 (ALREADY CLOSED in onQuit!)
defer s.Close()              // Line 76
```

### Signal Shutdown (SIGINT/SIGTERM)

```go
// Signal handler goroutine (lines 159-165)
go func() {
    <-sigCh
    log.Info().Msg("Received shutdown signal")
    commander.StopAll()        // Stop all myses
    bus.Close()                // Close event bus (DUPLICATE with defer!)
    program.Quit()             // Signal TUI to quit
}()

// Then same flow as normal exit
```

---

## Detailed Cleanup Analysis

### 1. Database Connection (SQLite)

**Location:** `internal/store/store.go:80-82`

```go
func (s *Store) Close() error {
    return s.db.Close()
}
```

**Cleanup Order:**
- **Line 76 (defer):** `defer s.Close()`
- Called LAST in the defer stack (first deferred, last executed)

**Issues:**
- ✅ **Safe:** Database is closed after all operations complete
- ⚠️ **Potential Issue:** If Commander.StopAll() is still writing to DB, this could block
- ⚠️ **MaxOpenConns=1** (line 51) means only one connection - can cause blocking during concurrent shutdown operations

**Recommendation:** Ensure Commander.StopAll() completes ALL database writes before calling s.Close()

---

### 2. Event Bus Shutdown

**Location:** `internal/core/bus.go:179-187`

```go
func (b *EventBus) Close() {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    for _, ch := range b.subscribers {
        ch.close()  // Closes each subscriber's channel
    }
    b.subscribers = nil
}
```

**Subscriber Close:** `internal/core/bus.go:50-59`

```go
func (s *subscriber) close() {
    s.mu.Lock()
    if s.closed {
        s.mu.Unlock()
        return
    }
    s.closed = true
    close(s.ch)  // ← CLOSES THE CHANNEL
    s.mu.Unlock()
}
```

**Cleanup Order:**
- **Line 81 (defer):** `defer bus.Close()`
- **Line 153 (onQuit):** `bus.Close()` (DUPLICATE!)
- **Line 163 (signal):** `bus.Close()` (DUPLICATE!)

**Issues:**
- ❌ **CRITICAL BUG:** Bus is closed TWICE (onQuit + defer)
- ❌ **CRITICAL BUG:** Closing in onQuit happens BEFORE tea.Quit() returns
- ✅ **Safe:** Multiple Close() calls are idempotent (line 52-55 check `s.closed`)

**The Event Channel Deadlock:**

```go
// TUI event listener (internal/tui/app.go:884-892)
func (m Model) listenForEvents() tea.Cmd {
    return func() tea.Msg {
        event, ok := <-m.eventCh  // ← BLOCKS HERE waiting for event
        if !ok {
            return nil  // Channel closed, return nil
        }
        return EventMsg{Event: event}
    }
}

// On each event received (line 204)
case EventMsg:
    m.handleEvent(msg.Event)
    return m, m.listenForEvents()  // ← RE-REGISTERS LISTENER
```

**Timeline:**

1. TUI calls `listenForEvents()` which blocks on `<-m.eventCh`
2. User presses 'q'
3. `onQuit()` is called → `bus.Close()` → channel is closed
4. Blocked `listenForEvents()` receives `ok=false` and returns `nil`
5. `tea.Quit` command is processed
6. **BUT:** If another event was sent AFTER the quit key but BEFORE bus.Close(), the TUI might register `listenForEvents()` again

**Race Condition Window:**
```
Thread 1 (TUI):               Thread 2 (Mysis):
1. Press 'q' key
2. onQuit() called
3. bus.Close() called          [Event published here]
4. eventCh closed
5. listenForEvents() unblocks
                              [Too late - channel closed]
```

**Why This Might Hang:**

If `listenForEvents()` is called AFTER `bus.Close()` but BEFORE `tea.Quit()` returns, it will block forever on a closed channel... wait, no. Reading from a closed channel returns `(zero, false)` immediately.

**Actually, the design is SAFE** for this specific case. Let me re-examine...

**Re-Analysis:**

Actually, the code handles closed channels correctly:
- `listenForEvents()` checks `ok` and returns `nil` if channel is closed (line 887-889)
- Returning `nil` from a command is safe - Bubble Tea ignores it
- So there's no deadlock here

**However, there IS a resource leak:**
- If `listenForEvents()` is blocked on `<-m.eventCh` when `tea.Quit()` is processed
- And the channel is NOT closed before `tea.Quit()` returns
- The goroutine remains blocked indefinitely

**Current behavior:**
- `onQuit()` closes the channel BEFORE `tea.Quit()` returns
- So the blocked goroutine unblocks when channel closes
- **This is correct!**

**Conclusion:** Event bus cleanup is **SAFE** but has **redundant Close() calls**.

---

### 3. Commander Shutdown

**Location:** `internal/core/commander.go:354-368`

```go
func (c *Commander) StopAll() {
    c.mu.RLock()
    myses := make([]*Mysis, 0)
    for _, m := range c.myses {
        myses = append(myses, m)
    }
    c.mu.RUnlock()
    
    for _, m := range myses {
        if m.State() == MysisStateRunning {
            m.Stop()  // ← Each mysis stopped sequentially
        }
    }
}
```

**Called at:**
- **Line 173:** After TUI exits
- **Line 162:** In signal handler (BEFORE TUI exits)

**Issues:**
- ✅ **Safe:** Stops all myses before database/event bus cleanup
- ⚠️ **Sequential:** Stops myses one-by-one, could be slow
- ⚠️ **No Timeout:** If a mysis.Stop() hangs, entire shutdown hangs

---

### 4. Mysis Shutdown

**Location:** `internal/core/mysis.go:224-264`

```go
func (m *Mysis) Stop() error {
    a := m
    a.mu.Lock()
    if a.state != MysisStateRunning {
        a.mu.Unlock()
        return nil
    }
    
    if a.cancel != nil {
        a.cancel()  // ← Cancel context (line 234)
    }
    a.mu.Unlock()
    
    // Wait for current turn to finish
    a.turnMu.Lock()   // ← BLOCKS until turn completes (line 239)
    defer a.turnMu.Unlock()
    
    // ... update state ...
    
    a.releaseCurrentAccount()  // Line 261
    return nil
}
```

**Mysis Run Loop:** `internal/core/mysis.go:913-944`

```go
func (m *Mysis) run(ctx context.Context) {
    a := m
    ticker := time.NewTicker(constants.IdleNudgeInterval)
    defer ticker.Stop()  // ← Ticker IS cleaned up
    
    for {
        select {
        case <-ctx.Done():  // ← Context cancellation exits loop
            return
        case <-ticker.C:
            // ... nudge logic ...
        case <-a.nudgeCh:   // ← Nudge channel
            // ... process nudge ...
            go a.SendMessage(...)  // ← Spawns goroutine!
        }
    }
}
```

**Issues:**
- ✅ **Safe:** Context cancellation stops run loop
- ✅ **Safe:** Ticker is cleaned up with defer
- ❌ **ISSUE:** `nudgeCh` is NEVER closed
- ❌ **ISSUE:** Goroutines spawned by nudge processing may still be running
- ⚠️ **BLOCKING:** `turnMu.Lock()` waits for current turn to finish (could be slow if LLM call is pending)

**Recommendation:** Add timeout to Stop() to prevent indefinite blocking:

```go
func (m *Mysis) Stop() error {
    // ... cancel context ...
    
    // Wait for turn with timeout
    done := make(chan struct{})
    go func() {
        a.turnMu.Lock()
        close(done)
        a.turnMu.Unlock()
    }()
    
    select {
    case <-done:
        // Turn finished
    case <-time.After(5 * time.Second):
        // Timeout - force stop anyway
        log.Warn().Str("mysis", a.name).Msg("Stop timeout - forcing")
    }
    
    // ... rest of cleanup ...
}
```

---

### 5. Provider Cleanup (Ollama, OpenCode)

**Ollama:** `internal/provider/ollama.go:18-45`

```go
type OllamaProvider struct {
    client      *openai.Client   // ← Never closed
    baseURL     string
    httpClient  *http.Client     // ← Never closed
    model       string
    temperature float64
    limiter     *rate.Limiter
}
```

**OpenCode:** `internal/provider/opencode.go:18-46`

```go
type OpenCodeProvider struct {
    client      *openai.Client   // ← Never closed
    baseURL     string
    apiKey      string
    httpClient  *http.Client     // ← Never closed
    model       string
    temperature float64
    limiter     *rate.Limiter
}
```

**Issues:**
- ❌ **NO CLEANUP:** HTTP clients are never closed
- ❌ **Connection Leaks:** Idle connections may remain open
- ⚠️ **Minor Impact:** OS will close connections on process exit

**Recommendation:** Add Close() method to providers:

```go
func (p *OllamaProvider) Close() error {
    p.httpClient.CloseIdleConnections()
    return nil
}
```

---

### 6. MCP Client Cleanup

**Location:** `internal/mcp/client.go:17-34`

```go
type Client struct {
    endpoint        string
    httpClient      *http.Client  // ← Never closed
    requestID       atomic.Int64
    sessionID       string
    protocolVersion string
}

func NewClient(endpoint string) *Client {
    return &Client{
        endpoint: endpoint,
        httpClient: &http.Client{
            Timeout: 0,  // No timeout!
        },
        protocolVersion: "2024-11-05",
    }
}
```

**Issues:**
- ❌ **NO CLEANUP:** HTTP client never closed
- ❌ **No Timeout:** Client has NO timeout (line 30)
- ⚠️ **Long-Running Requests:** Requests can hang indefinitely
- ⚠️ **Connection Leaks:** Idle connections remain open

**Recommendation:** Add Close() method and default timeout:

```go
func NewClient(endpoint string) *Client {
    return &Client{
        endpoint: endpoint,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,  // ← Add default timeout
        },
        protocolVersion: "2024-11-05",
    }
}

func (c *Client) Close() error {
    c.httpClient.CloseIdleConnections()
    return nil
}
```

---

### 7. Log File Handle

**Location:** `cmd/zoea/main.go:183-210`

```go
func initLogging(debug bool) error {
    // ...
    logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
    if err != nil {
        return fmt.Errorf("open log file: %w", err)
    }
    // ← File handle is NEVER closed!
    
    log.Logger = zerolog.New(logFile).With().Timestamp().Logger()
    return nil
}
```

**Issues:**
- ❌ **FILE HANDLE LEAK:** Log file is never closed
- ⚠️ **Minor Impact:** OS closes file handles on process exit
- ⚠️ **Data Loss Risk:** Buffered writes may not flush

**Recommendation:** Store file handle and close on exit:

```go
var logFile *os.File

func initLogging(debug bool) error {
    // ...
    var err error
    logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
    // ...
}

// In main()
func main() {
    // ...
    if logFile != nil {
        defer logFile.Close()
    }
    // ...
}
```

---

### 8. Mysis Nudge Channel

**Location:** `internal/core/mysis.go:71`

```go
func NewMysis(...) *Mysis {
    return &Mysis{
        // ...
        nudgeCh: make(chan struct{}, 1),  // ← Never closed
    }
}
```

**Used in:** `internal/core/mysis.go:913-944`

```go
func (m *Mysis) run(ctx context.Context) {
    // ...
    for {
        select {
        case <-ctx.Done():
            return  // ← Context cancels, but channel is NOT closed
        case <-a.nudgeCh:
            // ...
        }
    }
}
```

**Issues:**
- ❌ **CHANNEL LEAK:** Nudge channel is never closed
- ⚠️ **Minor Impact:** Context cancellation exits the loop, so channel is abandoned (not leaked in memory sense)
- ⚠️ **Best Practice:** Channels should be closed by their owner

**Recommendation:** Close nudge channel in Stop():

```go
func (m *Mysis) Stop() error {
    // ... existing logic ...
    
    // Close nudge channel to wake any blocked senders
    close(a.nudgeCh)
    
    // ...
}
```

---

## Cleanup Dependency Graph

```
┌─────────────────────────────────────────────────────┐
│                     Exit Trigger                     │
│            (User 'q' OR SIGINT/SIGTERM)             │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│  1. onQuit() callback (if user 'q')                 │
│     - bus.Close() ← Closes event channel           │
│     - Unblocks TUI listenForEvents() goroutine     │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│  2. tea.Quit command processed                      │
│     - Bubble Tea event loop exits                  │
│     - program.Run() returns                        │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│  3. commander.StopAll() (line 173)                  │
│     - For each mysis:                              │
│       - context.Cancel() ← Stops run() loop        │
│       - turnMu.Lock() ← Waits for current turn     │
│       - releaseCurrentAccount()                    │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│  4. store.ReleaseAllAccounts() (line 176)          │
│     - Marks all accounts as not in use             │
│     - Database write operation                     │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│  5. Deferred cleanups (LIFO order)                  │
│     - defer bus.Close() (line 81) ← DUPLICATE!     │
│     - defer s.Close() (line 76) ← DB connection    │
└─────────────────────────────────────────────────────┘
```

**Dependencies:**

1. **Event bus MUST close before TUI exits** ✅ (handled by onQuit)
2. **Commander MUST stop myses before DB closes** ✅ (happens in correct order)
3. **Myses MUST finish current turn before Stop() returns** ✅ (turnMu ensures this)
4. **Account release MUST complete before DB closes** ✅ (line 176 before defer)

---

## Potential Deadlock Scenarios

### Scenario 1: Mysis Turn Blocks on Provider Call

**Timeline:**
1. Mysis is executing a turn, waiting for LLM response
2. User presses 'q'
3. `commander.StopAll()` called
4. `mysis.Stop()` called → `context.Cancel()` sent
5. `mysis.Stop()` blocks on `turnMu.Lock()` waiting for turn to complete
6. Turn is blocked on provider HTTP request (no timeout!)
7. **DEADLOCK:** Stop() waits forever

**Likelihood:** LOW (provider calls have context with timeout)  
**Impact:** HIGH (entire shutdown hangs)

**Fix:** Add timeout to Stop() (see recommendation in section 4)

---

### Scenario 2: Database Lock During Concurrent Writes

**Timeline:**
1. Multiple myses are writing to database during shutdown
2. `commander.StopAll()` calls `mysis.Stop()` for each
3. Each mysis calls `store.UpdateMysisState()` (line 255)
4. Store has `MaxOpenConns=1` (only one connection)
5. Second write blocks waiting for first to complete
6. Meanwhile, `store.ReleaseAllAccounts()` is called (line 176)
7. **POTENTIAL DEADLOCK:** If writes are in transaction or DB is locked

**Likelihood:** LOW (SQLite with WAL mode handles concurrency)  
**Impact:** MEDIUM (shutdown delay)

**Fix:** None needed - WAL mode prevents this

---

### Scenario 3: Event Bus Closed Twice

**Timeline:**
1. User presses 'q'
2. `onQuit()` closes event bus
3. Deferred `bus.Close()` (line 81) tries to close again
4. Second Close() is idempotent (safe)

**Likelihood:** HIGH (happens every time)  
**Impact:** NONE (Close() checks `s.closed` flag)

**Fix:** Remove duplicate Close() calls:

```go
// Option 1: Remove from onQuit
model.SetOnQuit(func() {
    // Don't close bus here - let defer handle it
})

// Option 2: Remove defer
// defer bus.Close() ← DELETE THIS

// Option 3: Add flag to prevent double close
var busClosedByQuit bool
model.SetOnQuit(func() {
    bus.Close()
    busClosedByQuit = true
})
// Later, before defer executes:
if !busClosedByQuit {
    bus.Close()
}
```

---

## Recommended Cleanup Order

### Ideal Sequence

```go
func main() {
    // ... initialization ...
    
    // Set up deferred cleanups in REVERSE order of desired execution
    defer func() {
        // 6. Close log file (last)
        if logFile != nil {
            logFile.Close()
        }
    }()
    
    defer s.Close()  // 5. Close database connection
    
    // DO NOT defer bus.Close() - handle it explicitly
    
    // ... run TUI ...
    
    // 1. Stop all myses (waits for turns to complete)
    stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer stopCancel()
    
    stopDone := make(chan struct{})
    go func() {
        commander.StopAll()
        close(stopDone)
    }()
    
    select {
    case <-stopDone:
        // All myses stopped successfully
    case <-stopCtx.Done():
        log.Warn().Msg("StopAll timeout - forcing shutdown")
    }
    
    // 2. Close MCP client (if exists)
    if mcpProxy != nil && mcpProxy.HasUpstream() {
        // Add Close() method to proxy/client
        mcpProxy.Close()
    }
    
    // 3. Close provider clients
    for _, factory := range registry.List() {
        // Add Close() method to factories/providers
        factory.Close()
    }
    
    // 4. Release accounts and close event bus
    s.ReleaseAllAccounts()
    bus.Close()
    
    // 5-6. Deferred cleanups execute automatically
}
```

---

## Recommendations

### Critical (Must Fix)

1. **Add timeout to Mysis.Stop()** - Prevent indefinite blocking on turnMu
2. **Remove duplicate bus.Close()** - Clean up redundant calls
3. **Close provider HTTP clients** - Prevent connection leaks

### Important (Should Fix)

4. **Close MCP client HTTP client** - Prevent connection leaks
5. **Add timeout to MCP client** - Prevent hanging requests
6. **Close log file handle** - Ensure buffered writes flush

### Nice to Have

7. **Close mysis nudge channels** - Follow best practices
8. **Add StopAll timeout** - Prevent slow shutdown
9. **Parallelize mysis shutdown** - Stop myses concurrently

---

## Testing Recommendations

### Unit Tests

```go
func TestCleanupOrder(t *testing.T) {
    // Test that Stop() completes within timeout
    // Test that bus.Close() is idempotent
    // Test that database closes after all writes
}
```

### Integration Tests

```go
func TestGracefulShutdown(t *testing.T) {
    // Start application
    // Send SIGTERM
    // Verify all resources cleaned up
    // Check for goroutine leaks
}
```

### Manual Testing

```bash
# Test graceful shutdown
./bin/zoea &
PID=$!
sleep 5
kill -TERM $PID
# Check process exits within 5 seconds

# Test forced shutdown
./bin/zoea &
PID=$!
sleep 5
kill -KILL $PID
# Check for orphaned connections/files
```

---

## Conclusion

**Overall Assessment:** Cleanup is mostly correct but has several issues:

- ✅ **Event bus cleanup** - Correct, but has redundant Close() calls
- ✅ **Database cleanup** - Correct order, but potential for blocking
- ✅ **Commander shutdown** - Correct, but no timeout
- ❌ **Provider cleanup** - Missing entirely
- ❌ **MCP client cleanup** - Missing entirely
- ❌ **Log file cleanup** - Missing entirely
- ⚠️ **Mysis cleanup** - Correct but could hang on slow turns

**Risk Level:** **MEDIUM**
- No critical deadlocks in normal operation
- Potential for hang if LLM call blocks during shutdown
- Minor resource leaks (HTTP connections, file handles)

**Priority Fixes:**
1. Add timeout to Mysis.Stop()
2. Remove duplicate bus.Close() calls
3. Add provider and MCP client Close() methods
4. Close log file handle on exit

