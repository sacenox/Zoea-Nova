# Goroutine Cleanup Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all goroutine cleanup issues to ensure clean application shutdown without hangs

**Architecture:** Implement proper cleanup order with quit callbacks, close event bus before TUI exit, add WaitGroup tracking for myses, ensure all resources cleaned up gracefully

**Tech Stack:** Go 1.22+, Bubble Tea TUI, context-based cancellation, sync.WaitGroup

**Reference:** `GOROUTINE_CLEANUP_ANALYSIS.md` - Complete analysis of issues

---

## Phase 1: Fix Event Bus Cleanup Order (Critical)

**Root Cause:** Event bus is closed AFTER program.Run() returns, but TUI event listener blocks on channel waiting for events. This creates a deadlock.

**Solution:** Add onQuit callback to TUI that closes bus BEFORE tea.Quit is processed.

---

### Task 1.1: Verify onQuit callback already exists

**Files:**
- Check: `internal/tui/app.go`

**Step 1: Check if SetOnQuit already exists**

Run: `grep -n "SetOnQuit\|onQuit" internal/tui/app.go`
Expected: Should find existing onQuit field and SetOnQuit method

**Step 2: Check current usage in main.go**

Run: `grep -n "SetOnQuit\|onQuit" cmd/zoea/main.go`
Expected: See if it's already being used

**Step 3: Document findings**

Note: Based on CLEANUP_ORDER_ANALYSIS.md, onQuit callback already exists at app.go:70 and SetOnQuit at app.go:116-118.

---

### Task 1.2: Update onQuit callback to close event bus

**Files:**
- Modify: `cmd/zoea/main.go:145-155`

**Step 1: Update model setup to set onQuit callback**

**Current code (main.go:145-155):**
```go
eventCh := bus.Subscribe()
model := tui.New(commander, s, eventCh)
program := tea.NewProgram(model, tea.WithAltScreen())

go func() {
	<-sigCh
	log.Info().Msg("Received shutdown signal")
	commander.StopAll()
	program.Quit()
}()
```

**New code:**
```go
eventCh := bus.Subscribe()
model := tui.New(commander, s, eventCh)

// Set quit callback to close bus before TUI exits
model.SetOnQuit(func() {
	bus.Close() // Unblocks event listener
})

program := tea.NewProgram(model, tea.WithAltScreen())

go func() {
	<-sigCh
	log.Info().Msg("Received shutdown signal")
	commander.StopAll()
	bus.Close() // Close bus to unblock TUI
	program.Quit()
}()
```

**Step 2: Update cleanup after program.Run()**

**Current code (main.go:166-172):**
```go
commander.StopAll()

if err := s.ReleaseAllAccounts(); err != nil {
	log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}

log.Info().Msg("Zoea Nova shutdown complete")
```

**New code:**
```go
// Stop all myses (redundant but safe if signal handler didn't run)
commander.StopAll()

// Close bus (idempotent if already closed by onQuit or signal handler)
bus.Close()

// Release accounts
if err := s.ReleaseAllAccounts(); err != nil {
	log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}

log.Info().Msg("Zoea Nova shutdown complete")
```

**Step 3: Remove deferred bus.Close()**

**Current code (main.go:81):**
```go
defer bus.Close()
defer s.Close()
```

**New code:**
```go
// Event bus closed explicitly in shutdown sequence (onQuit callback or signal handler)
// Store closed last after all operations complete
defer s.Close()
```

**Step 4: Build and verify**

Run: `make build`
Expected: Clean build with no errors

**Step 5: Test normal exit**

Run: `./bin/zoea --offline` then press 'q'
Expected: Exits immediately (< 1 second)

**Step 6: Test signal exit**

Run: `./bin/zoea --offline` in background, then `pkill -TERM zoea`
Expected: Exits cleanly within 1 second

**Step 7: Commit**

```bash
git add cmd/zoea/main.go
git commit -m "fix: close event bus before TUI exit to prevent hang

- Add onQuit callback that closes bus before tea.Quit processes
- Close bus in signal handler before calling program.Quit()
- Remove deferred bus.Close() (now explicit in shutdown sequence)
- Ensures listenForEvents() unblocks cleanly on exit

Fixes deadlock where TUI event listener blocks on channel read
while waiting for bus.Close() that only runs after program.Run()
returns."
```

---

## Phase 2: Add WaitGroup for Mysis Goroutines

**Root Cause:** No way to wait for all mysis goroutines to complete before proceeding with cleanup.

**Solution:** Add sync.WaitGroup to Commander to track running myses.

---

### Task 2.1: Add WaitGroup to Commander

**Files:**
- Modify: `internal/core/commander.go:18-30`

**Step 1: Add WaitGroup field to Commander struct**

**Current code (commander.go:18-30):**
```go
type Commander struct {
	store           *store.Store
	mcpClient       MCPClient
	toolRegistry    *ToolRegistry
	providerFactory map[string]ProviderFactory
	eventBus        *EventBus
	config          *config.Config

	mu    sync.RWMutex
	myses map[string]*Mysis

	aggregateTick atomic.Int64
}
```

**New code:**
```go
type Commander struct {
	store           *store.Store
	mcpClient       MCPClient
	toolRegistry    *ToolRegistry
	providerFactory map[string]ProviderFactory
	eventBus        *EventBus
	config          *config.Config

	mu    sync.RWMutex
	myses map[string]*Mysis
	wg    sync.WaitGroup // Tracks running mysis goroutines

	aggregateTick atomic.Int64
}
```

**Step 2: Increment WaitGroup in StartMysis**

**Location:** `internal/core/commander.go` (find StartMysis method)

Run: `grep -n "func (c \*Commander) StartMysis" internal/core/commander.go`

**Current code (commander.go:~120-140):**
```go
func (c *Commander) StartMysis(id string) error {
	// ...existing validation...
	
	if err := mysis.Start(); err != nil {
		return fmt.Errorf("start mysis: %w", err)
	}
	
	return nil
}
```

**New code:**
```go
func (c *Commander) StartMysis(id string) error {
	// ...existing validation...
	
	c.wg.Add(1) // Track this goroutine
	if err := mysis.Start(); err != nil {
		c.wg.Done() // Failed to start, don't track
		return fmt.Errorf("start mysis: %w", err)
	}
	
	return nil
}
```

**Step 3: Decrement WaitGroup in Mysis.run() when exiting**

**Files:**
- Modify: `internal/core/mysis.go:913-944`

**Current code:**
```go
func (m *Mysis) run(ctx context.Context) {
	a := m
	ticker := time.NewTicker(constants.IdleNudgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// ...
		case <-a.nudgeCh:
			// ...
		}
	}
}
```

**New code:**
```go
func (m *Mysis) run(ctx context.Context) {
	a := m
	defer a.commander.wg.Done() // Signal goroutine completion
	
	ticker := time.NewTicker(constants.IdleNudgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// ...
		case <-a.nudgeCh:
			// ...
		}
	}
}
```

**Note:** Mysis needs reference to Commander. Check if it already exists.

Run: `grep -n "commander.*Commander" internal/core/mysis.go | head -5`

**Step 4: Add Commander reference to Mysis if missing**

Check current Mysis struct:

Run: `grep -A 30 "^type Mysis struct" internal/core/mysis.go`

If commander field doesn't exist, add it:

```go
type Mysis struct {
	// ...existing fields...
	commander *Commander  // Reference to parent commander for WaitGroup
}
```

Update NewMysis to accept commander parameter.

**Step 5: Wait for all goroutines in StopAll**

**Files:**
- Modify: `internal/core/commander.go:354-368`

**Current code:**
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
			m.Stop()
		}
	}
}
```

**New code:**
```go
func (c *Commander) StopAll() {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		myses = append(myses, m)
	}
	c.mu.RUnlock()

	// Stop all running myses
	for _, m := range myses {
		if m.State() == MysisStateRunning {
			m.Stop()
		}
	}
	
	// Wait for all mysis goroutines to complete
	c.wg.Wait()
}
```

**Step 6: Build and verify**

Run: `make build`
Expected: Clean build

**Step 7: Test with running myses**

Run: `./bin/zoea --offline`
Steps: Create 2 myses, start them, wait 2 seconds, press 'q'
Expected: Exits cleanly within 2 seconds

**Step 8: Commit**

```bash
git add internal/core/commander.go internal/core/mysis.go
git commit -m "feat: add WaitGroup to track mysis goroutines

- Add sync.WaitGroup to Commander struct
- Increment WaitGroup when starting mysis
- Decrement WaitGroup when mysis.run() exits
- Wait for all goroutines in StopAll()

Ensures all mysis processing loops complete before cleanup
proceeds. Prevents race conditions during shutdown."
```

---

## Phase 3: Add Graceful Shutdown Timeout

**Root Cause:** If a mysis hangs during Stop(), the entire shutdown hangs forever.

**Solution:** Add timeout to StopAll() to force exit if myses don't stop in reasonable time.

---

### Task 3.1: Add timeout to StopAll

**Files:**
- Modify: `internal/core/commander.go:354-368`

**Step 1: Import time package if not already imported**

Check: `grep "^import" -A 10 internal/core/commander.go | grep time`

**Step 2: Implement timeout in StopAll**

**Current code:**
```go
func (c *Commander) StopAll() {
	// ... stop all myses ...
	c.wg.Wait()
}
```

**New code:**
```go
func (c *Commander) StopAll() {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		myses = append(myses, m)
	}
	c.mu.RUnlock()

	// Stop all running myses
	for _, m := range myses {
		if m.State() == MysisStateRunning {
			m.Stop()
		}
	}
	
	// Wait for all mysis goroutines with timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines completed
		log.Info().Int("count", len(myses)).Msg("All myses stopped")
	case <-time.After(5 * time.Second):
		// Timeout - force exit
		log.Warn().Int("count", len(myses)).Msg("StopAll timeout - forcing shutdown")
	}
}
```

**Step 3: Add log import if needed**

Check: `grep "github.com/rs/zerolog/log" internal/core/commander.go`

If not found, add to imports:
```go
import (
	// ...existing imports...
	"github.com/rs/zerolog/log"
)
```

**Step 4: Build and verify**

Run: `make build`
Expected: Clean build

**Step 5: Test normal shutdown**

Run: `./bin/zoea --offline`, create myses, press 'q'
Expected: Log shows "All myses stopped" with count

**Step 6: Commit**

```bash
git add internal/core/commander.go
git commit -m "feat: add graceful shutdown timeout to StopAll

- Add 5-second timeout for StopAll() to complete
- Log completion with mysis count
- Log warning if timeout occurs (force shutdown)

Prevents infinite hang if a mysis fails to stop cleanly.
Application will exit after 5 seconds regardless of mysis state."
```

---

## Phase 4: Close HTTP Clients Properly

**Root Cause:** Provider and MCP HTTP clients are never closed, leaving idle connections open.

**Solution:** Add Close() methods and call them during shutdown.

---

### Task 4.1: Add Close() to Provider interface

**Files:**
- Modify: `internal/provider/provider.go:42-55`

**Step 1: Add Close() to Provider interface**

**Current code:**
```go
type Provider interface {
	Name() string
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
	Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}
```

**New code:**
```go
type Provider interface {
	Name() string
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
	Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
	Close() error
}
```

**Step 2: Verify implementations already have Close()**

Check: Based on HTTP_CLIENT_CLEANUP_IMPLEMENTATION.md:
- `internal/provider/ollama.go:360-366` - Already has Close()
- `internal/provider/opencode.go:228-234` - Already has Close()
- `internal/provider/mock.go` - Check if needs Close()

Run: `grep -n "func.*Close()" internal/provider/*.go`

**Step 3: Add Close() to MockProvider if missing**

**Files:**
- Modify: `internal/provider/mock.go`

Check if Close exists:
Run: `grep -n "Close" internal/provider/mock.go`

If missing, add:
```go
// Close is a no-op for mock provider (no resources to clean up)
func (p *MockProvider) Close() error {
	return nil
}
```

**Step 4: Build and verify**

Run: `make build`
Expected: Clean build

**Step 5: Commit**

```bash
git add internal/provider/
git commit -m "feat: add Close() to Provider interface

- Add Close() method to Provider interface
- Ensures all providers implement cleanup
- MockProvider Close() is no-op (no resources)

Enables polymorphic cleanup of provider HTTP clients."
```

---

### Task 4.2: Call provider.Close() in Mysis.Stop()

**Files:**
- Modify: `internal/core/mysis.go:224-264`

**Step 1: Add provider cleanup to Stop()**

**Current code (mysis.go:~258-264):**
```go
func (m *Mysis) Stop() error {
	// ... existing stop logic ...
	
	a.releaseCurrentAccount()
	return nil
}
```

**New code:**
```go
func (m *Mysis) Stop() error {
	// ... existing stop logic ...
	
	a.releaseCurrentAccount()
	
	// Close provider HTTP client
	if a.provider != nil {
		if err := a.provider.Close(); err != nil {
			log.Warn().Err(err).Str("mysis", a.name).Msg("Failed to close provider")
		}
	}
	
	return nil
}
```

**Step 2: Add log import if needed**

Check: `grep "github.com/rs/zerolog/log" internal/core/mysis.go`

**Step 3: Build and verify**

Run: `make build`
Expected: Clean build

**Step 4: Test provider cleanup**

Run: `./bin/zoea --offline`, create mysis, start it, stop it with 's'
Check logs for any errors
Expected: No errors, clean stop

**Step 5: Commit**

```bash
git add internal/core/mysis.go
git commit -m "feat: close provider HTTP client in Mysis.Stop()

- Call provider.Close() after releasing account
- Log warning if close fails
- Prevents idle HTTP connection leaks

Each mysis now properly cleans up its provider resources
when stopped."
```

---

### Task 4.3: Add Close() to MCP Client and call it

**Files:**
- Verify: `internal/mcp/client.go:320-326` (already has Close() per HTTP_CLIENT_CLEANUP_IMPLEMENTATION.md)
- Modify: `cmd/zoea/main.go` cleanup sequence

**Step 1: Verify MCP Client has Close()**

Run: `grep -n "func.*Close" internal/mcp/client.go`
Expected: Should find Close() method at line ~320

**Step 2: Add MCP client cleanup to main.go**

**Location:** After `commander.StopAll()` in main.go

**Current code (main.go:~166-172):**
```go
commander.StopAll()
bus.Close()

if err := s.ReleaseAllAccounts(); err != nil {
	log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}
```

**New code:**
```go
commander.StopAll()
bus.Close()

// Close MCP client if initialized
if mcpProxy != nil {
	if client, ok := mcpProxy.Upstream().(*mcp.Client); ok {
		if err := client.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close MCP client")
		}
	}
}

if err := s.ReleaseAllAccounts(); err != nil {
	log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}
```

**Note:** Check if mcpProxy.Upstream() method exists. May need to adjust based on actual API.

Run: `grep -n "Upstream\|upstream" internal/mcp/proxy.go | head -10`

**Step 3: Build and verify**

Run: `make build`
Expected: Clean build (or error if Upstream() doesn't exist - adjust code)

**Step 4: Commit**

```bash
git add cmd/zoea/main.go
git commit -m "feat: close MCP client on shutdown

- Call client.Close() after stopping myses
- Closes idle HTTP connections to SpaceMolt server
- Prevents connection leaks

MCP client shared by all myses via proxy, so closed once
after all myses have stopped."
```

---

## Phase 5: Close Log File Handle

**Root Cause:** Log file handle opened in initLogging() is never closed, risking buffered write loss.

**Solution:** Track file handle and close it on exit.

---

### Task 5.1: Track and close log file handle

**Files:**
- Modify: `cmd/zoea/main.go:183-210` (initLogging function)
- Modify: `cmd/zoea/main.go:60-90` (main function)

**Step 1: Make logFile a package-level variable**

**Current code (main.go:183):**
```go
func initLogging(debug bool) error {
	// ...
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	// ...
}
```

**Add at top of main.go (after package declaration):**
```go
package main

import (
	// ...
)

var logFile *os.File // Package-level to allow cleanup
```

**Step 2: Update initLogging to use package variable**

**New code:**
```go
func initLogging(debug bool) error {
	// ...
	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	
	log.Logger = zerolog.New(logFile).With().Timestamp().Logger()
	return nil
}
```

**Step 3: Add deferred close in main()**

**Current code (main.go:~76-81):**
```go
defer s.Close()
```

**New code:**
```go
// Close log file last (after all logging complete)
defer func() {
	if logFile != nil {
		logFile.Close()
	}
}()

defer s.Close()
```

**Step 4: Build and verify**

Run: `make build`
Expected: Clean build

**Step 5: Test log file closes**

Run: `./bin/zoea --offline`, press 'q' immediately
Check: `lsof | grep zoea.log` (should be empty after exit)
Expected: No open file handles to zoea.log

**Step 6: Commit**

```bash
git add cmd/zoea/main.go
git commit -m "fix: close log file handle on exit

- Make logFile package-level variable
- Add deferred close in main()
- Ensures buffered writes flush on exit

Prevents potential data loss from unclosed log file."
```

---

## Phase 6: Remove Redundant Cleanup Calls

**Root Cause:** Multiple redundant StopAll() and bus.Close() calls create confusion.

**Solution:** Clean up redundant calls now that proper sequence is established.

---

### Task 6.1: Remove redundant StopAll in signal handler

**Files:**
- Modify: `cmd/zoea/main.go:152-158`

**Step 1: Check current signal handler**

Run: `grep -A 10 "go func()" cmd/zoea/main.go | grep -A 8 sigCh`

**Current code:**
```go
go func() {
	<-sigCh
	log.Info().Msg("Received shutdown signal")
	commander.StopAll()  // ← Will be called again after program.Run()
	bus.Close()
	program.Quit()
}()
```

**New code:**
```go
go func() {
	<-sigCh
	log.Info().Msg("Received shutdown signal")
	// Don't call StopAll here - let main cleanup handle it
	// This avoids double-stop which wastes time
	bus.Close() // Close bus to unblock TUI
	program.Quit()
}()
```

**Rationale:** StopAll() is called after program.Run() returns, so calling it in signal handler is redundant and wastes time waiting for myses to stop twice.

**Step 2: Build and verify**

Run: `make build`
Expected: Clean build

**Step 3: Test signal exit**

Run: `./bin/zoea --offline` with myses running, send SIGTERM
Expected: Exits cleanly, StopAll only called once (check logs)

**Step 4: Commit**

```bash
git add cmd/zoea/main.go
git commit -m "refactor: remove redundant StopAll in signal handler

- StopAll already called after program.Run() returns
- Calling it in signal handler is wasteful (stops twice)
- Simplified signal handler to only close bus and quit TUI

Improves shutdown speed by avoiding redundant mysis stops."
```

---

## Phase 7: Add Goroutine Leak Detection (Optional)

**Root Cause:** No way to detect if goroutines are leaking during development/testing.

**Solution:** Add runtime goroutine count logging at startup and shutdown.

---

### Task 7.1: Add goroutine count logging

**Files:**
- Modify: `cmd/zoea/main.go` (main function)

**Step 1: Log goroutine count at startup**

**Location:** After all initialization (main.go:~145)

**Add:**
```go
log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Application started")
```

**Step 2: Log goroutine count at shutdown**

**Location:** After StopAll completes (main.go:~169)

**Add:**
```go
commander.StopAll()
bus.Close()

// Log goroutine count for leak detection
log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Shutdown in progress")

// ... rest of cleanup ...

log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Zoea Nova shutdown complete")
```

**Step 3: Add runtime import**

Check: `grep "runtime" cmd/zoea/main.go`

If not found, add to imports:
```go
import (
	// ...
	"runtime"
)
```

**Step 4: Build and test**

Run: `make build && ./bin/zoea --offline`
Create myses, start them, wait, then exit
Check logs: Compare goroutine counts at startup vs shutdown
Expected: Counts should be similar (within ~5 goroutines)

**Step 5: Commit**

```bash
git add cmd/zoea/main.go
git commit -m "feat: add goroutine leak detection logging

- Log goroutine count at startup
- Log count during shutdown
- Log final count at exit

Enables detection of goroutine leaks during development
and testing. Baseline count should be similar at start/end."
```

---

## Phase 8: Testing and Verification

### Task 8.1: Create shutdown integration test

**Files:**
- Create: `internal/integration/shutdown_test.go`

**Step 1: Write shutdown test**

```go
package integration

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestGracefulShutdown(t *testing.T) {
	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "/tmp/zoea-test", "./cmd/zoea")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	defer os.Remove("/tmp/zoea-test")

	// Start application
	cmd := exec.Command("/tmp/zoea-test", "--offline")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Give it time to initialize
	time.Sleep(2 * time.Second)

	// Send SIGTERM
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// Wait for exit with timeout
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("exit with error: %v (acceptable for SIGTERM)", err)
		}
		// Success - exited within timeout
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("shutdown timeout - process hung")
	}
}

func TestNormalExit(t *testing.T) {
	// Test that would send 'q' key, but requires TUI automation
	// For now, manual testing is sufficient
	t.Skip("Manual test: run ./bin/zoea --offline and press 'q'")
}
```

**Step 2: Run test**

Run: `go test ./internal/integration -run TestGracefulShutdown -v`
Expected: PASS (exits within 10 seconds)

**Step 3: Commit**

```bash
git add internal/integration/shutdown_test.go
git commit -m "test: add graceful shutdown integration test

- Test SIGTERM handling
- Verify exit within 10-second timeout
- Prevents regression of shutdown hangs

Manual test placeholder for normal 'q' key exit."
```

---

### Task 8.2: Update documentation

**Files:**
- Update: `GOROUTINE_CLEANUP_ANALYSIS.md` - Mark issues as resolved
- Update: `documentation/KNOWN_ISSUES.md` - Remove if listed

**Step 1: Add resolution section to analysis doc**

Add to top of GOROUTINE_CLEANUP_ANALYSIS.md:
```markdown
## Resolution Status: ✅ COMPLETE

**Date Resolved:** 2026-02-06

**All issues fixed:**
1. ✅ Event bus cleanup order - Fixed with onQuit callback
2. ✅ WaitGroup for mysis tracking - Added to Commander
3. ✅ Graceful shutdown timeout - 5-second timeout in StopAll()
4. ✅ Provider HTTP client cleanup - Close() called in Stop()
5. ✅ MCP client cleanup - Close() called in main shutdown
6. ✅ Log file handle cleanup - Deferred close in main()

See commits: [list commit hashes]
```

**Step 2: Check KNOWN_ISSUES.md**

Run: `grep -i "goroutine\|cleanup\|shutdown\|hang" documentation/KNOWN_ISSUES.md`

If shutdown/cleanup issues are listed, remove or mark resolved.

**Step 3: Commit**

```bash
git add GOROUTINE_CLEANUP_ANALYSIS.md documentation/KNOWN_ISSUES.md
git commit -m "docs: mark goroutine cleanup issues as resolved

- Add resolution status to analysis document
- Update KNOWN_ISSUES.md if needed
- Reference implementing commits"
```

---

## Testing Checklist

After all phases complete, verify:

### Manual Testing

- [ ] **Normal exit:** `./bin/zoea --offline`, press 'q' → exits < 1s
- [ ] **Signal exit:** Start app, `pkill -TERM zoea` → exits < 1s
- [ ] **Exit with myses:** Create 3 myses, start them, press 'q' → exits < 3s
- [ ] **Exit during LLM:** Send message, press 'q' while thinking → exits < 5s
- [ ] **Multiple exit attempts:** Press 'q' multiple times rapidly → only exits once
- [ ] **Check goroutine logs:** Compare startup vs shutdown counts → similar

### Automated Testing

- [ ] `make test` → all tests pass
- [ ] `go test ./internal/integration -run Shutdown -v` → pass
- [ ] Build successful: `make build`

### Resource Verification

- [ ] No orphaned processes: `ps aux | grep zoea` → empty after exit
- [ ] No open connections: `lsof -i | grep zoea` → empty after exit
- [ ] No open files: `lsof | grep zoea` → empty after exit

---

## Success Criteria

1. ✅ Application exits within 1 second on 'q' key
2. ✅ Application exits within 1 second on SIGTERM
3. ✅ Application exits within 5 seconds with running myses
4. ✅ No goroutine leaks (startup count ≈ shutdown count ± 5)
5. ✅ All HTTP clients closed (no idle connections)
6. ✅ Log file handle closed (no open file descriptors)
7. ✅ All tests passing
8. ✅ No errors in shutdown logs

---

## Rollback Plan

If issues arise during implementation:

1. **Revert last commit:** `git reset --hard HEAD~1`
2. **Check which phase failed:** Review error messages
3. **Return to Phase 1 of systematic-debugging skill**
4. **Investigate why fix didn't work**
5. **DO NOT attempt another fix without understanding**

---

## Notes

- Event bus Close() is already idempotent (checked in analysis)
- Provider Close() methods already exist (implemented earlier)
- MCP Client Close() already exists (implemented earlier)
- onQuit callback already exists in TUI (app.go:70, 116-118)

**This means most of the infrastructure is already in place** - we just need to wire it up correctly in the shutdown sequence.

---

## Estimated Time

- Phase 1 (Event bus): 15 minutes
- Phase 2 (WaitGroup): 20 minutes
- Phase 3 (Timeout): 10 minutes
- Phase 4 (HTTP clients): 15 minutes
- Phase 5 (Log file): 10 minutes
- Phase 6 (Cleanup): 5 minutes
- Phase 7 (Logging): 10 minutes
- Phase 8 (Testing): 20 minutes

**Total:** ~105 minutes (1.75 hours)

---

**Plan Created:** 2026-02-06  
**Reference Analysis:** `GOROUTINE_CLEANUP_ANALYSIS.md`  
**Target Version:** Next RC (post-v0.1.0)
