package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// setupStateMachineTest creates a minimal test environment for state transition testing.
func setupStateMachineTest(t *testing.T) (*Commander, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	bus := NewEventBus(100)

	reg := provider.NewRegistry()
	reg.RegisterFactory("mock", provider.NewMockFactory("mock", "mock response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses: 16,
		},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model", Temperature: 0.7},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)
	proxy := mcp.NewProxy(nil)
	cmd.SetMCP(proxy)

	cleanup := func() {
		cmd.StopAll()
		bus.Close()
		s.Close()
	}

	return cmd, cleanup
}

// TestStateTransition_Idle_To_Running tests the idle → running transition.
// Trigger: Start()
// Expected: State transitions to Running, LastError is nil
func TestStateTransition_Idle_To_Running(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	// Create mysis (starts in idle)
	mysis, err := cmd.CreateMysis("test-idle-running", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Verify initial state
	if state := mysis.State(); state != MysisStateIdle {
		t.Fatalf("expected initial state=idle, got %s", state)
	}

	// Trigger transition: Start
	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Verify final state
	if state := mysis.State(); state != MysisStateRunning {
		t.Errorf("expected state=running, got %s", state)
	}

	// Verify no error
	if lastErr := mysis.LastError(); lastErr != nil {
		t.Errorf("expected LastError=nil, got %v", lastErr)
	}

	// Verify store persistence
	stored, err := cmd.Store().GetMysis(mysis.ID())
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if stored.State != store.MysisStateRunning {
		t.Errorf("expected stored state=running, got %s", stored.State)
	}
}

// TestStateTransition_Running_To_Stopped tests the running → stopped transition.
// Trigger: Stop()
// Expected: State transitions to Stopped, NOT Errored, LastError is nil
//
// THIS IS THE BUG WE'RE TESTING FOR:
// When Stop() is called, context cancellation can race with in-flight turns,
// causing setError() to override the Stopped state with Errored.
func TestStateTransition_Running_To_Stopped(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	// Create and start mysis
	mysis, err := cmd.CreateMysis("test-running-stopped", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Verify running state
	if state := mysis.State(); state != MysisStateRunning {
		t.Fatalf("expected state=running before stop, got %s", state)
	}

	// Trigger transition: Stop
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// CRITICAL CHECKS: State MUST be Stopped, NOT Errored
	if state := mysis.State(); state != MysisStateStopped {
		t.Errorf("FAIL: expected state=stopped, got %s (LastError: %v)", state, mysis.LastError())
	}

	// Verify no error recorded
	if lastErr := mysis.LastError(); lastErr != nil {
		t.Errorf("expected LastError=nil after stop, got %v", lastErr)
	}

	// Verify store persistence
	stored, err := cmd.Store().GetMysis(mysis.ID())
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if stored.State != store.MysisStateStopped {
		t.Errorf("expected stored state=stopped, got %s", stored.State)
	}
}

// TestStateTransition_Running_To_Stopped_StressTest runs the critical
// running→stopped transition 100 times to catch race conditions.
func TestStateTransition_Running_To_Stopped_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const iterations = 100
	failures := 0
	var failureMu sync.Mutex

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			cmd, cleanup := setupStateMachineTest(t)
			defer cleanup()

			mysis, err := cmd.CreateMysis(fmt.Sprintf("stress-%d", i), "mock")
			if err != nil {
				t.Fatalf("CreateMysis() error: %v", err)
			}

			if err := mysis.Start(); err != nil {
				t.Fatalf("Start() error: %v", err)
			}

			// Add a tiny delay to let the mysis run loop start
			time.Sleep(10 * time.Millisecond)

			if err := mysis.Stop(); err != nil {
				t.Fatalf("Stop() error: %v", err)
			}

			// Check final state
			finalState := mysis.State()
			if finalState != MysisStateStopped {
				failureMu.Lock()
				failures++
				failureMu.Unlock()
				t.Errorf("iteration %d: expected state=stopped, got %s (LastError: %v)",
					i, finalState, mysis.LastError())
			}

			// Check no error
			if lastErr := mysis.LastError(); lastErr != nil {
				t.Errorf("iteration %d: expected LastError=nil, got %v", i, lastErr)
			}
		})
	}

	// Report summary
	t.Logf("Stress test complete: %d/%d passed, %d failed", iterations-failures, iterations, failures)
	if failures > 0 {
		t.Errorf("RACE DETECTED: %d/%d iterations failed (%.1f%% failure rate)",
			failures, iterations, float64(failures)/float64(iterations)*100)
	}
}

// TestStateTransition_Running_To_Errored tests the running → errored transition.
// Trigger: Error during processing
// Expected: State transitions to Errored, LastError is set
func TestStateTransition_Running_To_Errored(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	// Create mysis with provider that will error
	mysis, err := cmd.CreateMysis("test-running-errored", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Replace provider with one that errors
	errorProvider := provider.NewMock("mock", "").WithChatError(errors.New("simulated error"))
	mysis.SetProvider(errorProvider)

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Send a message to trigger error
	_ = mysis.SendMessage("trigger error", store.MemorySourceDirect)

	// Wait for error propagation
	time.Sleep(100 * time.Millisecond)

	// Verify errored state
	if state := mysis.State(); state != MysisStateErrored {
		t.Errorf("expected state=errored, got %s", state)
	}

	// Verify error is recorded
	if lastErr := mysis.LastError(); lastErr == nil {
		t.Errorf("expected LastError to be set, got nil")
	}

	// Verify store persistence
	stored, err := cmd.Store().GetMysis(mysis.ID())
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if stored.State != store.MysisStateErrored {
		t.Errorf("expected stored state=errored, got %s", stored.State)
	}
}

// TestStateTransition_Running_To_Idle tests the running → idle transition.
// Trigger: 3-encouragement limit (simulated)
// Expected: State transitions to Idle, LastError is nil
func TestStateTransition_Running_To_Idle(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	// Create and start mysis
	mysis, err := cmd.CreateMysis("test-running-idle", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Simulate 3-encouragement limit reached
	mysis.setIdle("No user messages after 3 encouragements")

	// Verify idle state
	if state := mysis.State(); state != MysisStateIdle {
		t.Errorf("expected state=idle, got %s", state)
	}

	if lastErr := mysis.LastError(); lastErr != nil {
		t.Errorf("expected LastError=nil after idle transition, got %v", lastErr)
	}

	// Verify store persistence
	stored, err := cmd.Store().GetMysis(mysis.ID())
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if stored.State != store.MysisStateIdle {
		t.Errorf("expected stored state=idle, got %s", stored.State)
	}
}

// TestStateTransition_Stopped_To_Running tests the stopped → running transition.
// Trigger: Start() (relaunch)
// Expected: State transitions to Running, LastError cleared
func TestStateTransition_Stopped_To_Running(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	mysis, err := cmd.CreateMysis("test-stopped-running", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Start, then stop
	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// Verify stopped state
	if state := mysis.State(); state != MysisStateStopped {
		t.Fatalf("expected state=stopped before relaunch, got %s", state)
	}

	// Trigger transition: Relaunch (Start from stopped)
	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() (relaunch) error: %v", err)
	}

	// Verify running state
	if state := mysis.State(); state != MysisStateRunning {
		t.Errorf("expected state=running after relaunch, got %s", state)
	}

	// Verify no error
	if lastErr := mysis.LastError(); lastErr != nil {
		t.Errorf("expected LastError=nil after relaunch, got %v", lastErr)
	}

	// Verify store persistence
	stored, err := cmd.Store().GetMysis(mysis.ID())
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if stored.State != store.MysisStateRunning {
		t.Errorf("expected stored state=running, got %s", stored.State)
	}
}

// TestStateTransition_Errored_To_Running tests the errored → running transition.
// Trigger: Start() (relaunch)
// Expected: State transitions to Running, LastError cleared
func TestStateTransition_Errored_To_Running(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	mysis, err := cmd.CreateMysis("test-errored-running", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Manually set error state
	testErr := errors.New("test error")
	mysis.SetErrorState(testErr)

	// Verify errored state
	if state := mysis.State(); state != MysisStateErrored {
		t.Fatalf("expected state=errored before relaunch, got %s", state)
	}
	if lastErr := mysis.LastError(); lastErr == nil {
		t.Fatalf("expected LastError to be set, got nil")
	}

	// Trigger transition: Relaunch (Start from errored)
	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() (relaunch from error) error: %v", err)
	}

	// Verify running state
	if state := mysis.State(); state != MysisStateRunning {
		t.Errorf("expected state=running after relaunch, got %s", state)
	}

	// Verify error cleared
	if lastErr := mysis.LastError(); lastErr != nil {
		t.Errorf("expected LastError=nil after relaunch, got %v", lastErr)
	}

	// Verify store persistence
	stored, err := cmd.Store().GetMysis(mysis.ID())
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if stored.State != store.MysisStateRunning {
		t.Errorf("expected stored state=running, got %s", stored.State)
	}
}

// TestStateTransition_InvalidTransitions tests that invalid transitions are rejected.
func TestStateTransition_InvalidTransitions(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	mysis, err := cmd.CreateMysis("test-invalid", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Try to start when already running
	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := mysis.Start(); err == nil {
		t.Error("expected error when starting already running mysis")
	}

	// Verify state unchanged
	if state := mysis.State(); state != MysisStateRunning {
		t.Errorf("expected state=running after failed start, got %s", state)
	}
}

// TestStateTransition_ConcurrentStopDuringMessage tests the race condition
// where Stop() is called while a message is being processed.
func TestStateTransition_ConcurrentStopDuringMessage(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	// Create mysis with slow provider
	mysis, err := cmd.CreateMysis("test-concurrent", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	slowProvider := provider.NewMock("mock", "slow response").SetDelay(200 * time.Millisecond)
	mysis.SetProvider(slowProvider)

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Start processing a message (non-blocking)
	go func() {
		_ = mysis.SendMessage("test message", store.MemorySourceDirect)
	}()

	// Wait a bit to ensure message processing started
	time.Sleep(50 * time.Millisecond)

	// Stop while message is in-flight
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// CRITICAL: Final state MUST be Stopped
	if state := mysis.State(); state != MysisStateStopped {
		t.Errorf("RACE: expected state=stopped after concurrent stop, got %s (LastError: %v)",
			state, mysis.LastError())
	}

	// Verify no error
	if lastErr := mysis.LastError(); lastErr != nil {
		t.Errorf("expected LastError=nil after concurrent stop, got %v", lastErr)
	}
}

// TestStateTransition_RapidStartStop tests rapid start/stop cycles.
func TestStateTransition_RapidStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping rapid cycle test in short mode")
	}

	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	mysis, err := cmd.CreateMysis("test-rapid", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Perform 10 rapid start/stop cycles
	for i := 0; i < 10; i++ {
		// Start
		if err := mysis.Start(); err != nil {
			t.Fatalf("cycle %d: Start() error: %v", i, err)
		}

		// Verify running
		if state := mysis.State(); state != MysisStateRunning {
			t.Errorf("cycle %d: expected state=running, got %s", i, state)
		}

		// Brief pause
		time.Sleep(10 * time.Millisecond)

		// Stop
		if err := mysis.Stop(); err != nil {
			t.Fatalf("cycle %d: Stop() error: %v", i, err)
		}

		// CRITICAL: Verify stopped
		if state := mysis.State(); state != MysisStateStopped {
			t.Errorf("cycle %d: expected state=stopped, got %s (LastError: %v)",
				i, state, mysis.LastError())
		}

		// Verify no error
		if lastErr := mysis.LastError(); lastErr != nil {
			t.Errorf("cycle %d: expected LastError=nil, got %v", i, lastErr)
		}
	}
}

// TestStateTransition_StopWithContext tests that context cancellation
// properly propagates through the stop sequence.
func TestStateTransition_StopWithContext(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	mysis, err := cmd.CreateMysis("test-ctx", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Get the mysis context
	mysis.mu.RLock()
	ctx := mysis.ctx
	mysis.mu.RUnlock()

	if ctx == nil {
		t.Fatal("expected context to be set when running")
	}

	// Verify context is not done before stop
	select {
	case <-ctx.Done():
		t.Fatal("context done before stop called")
	default:
		// Good - context not cancelled yet
	}

	// Stop the mysis
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// Verify state
	if state := mysis.State(); state != MysisStateStopped {
		t.Errorf("expected state=stopped, got %s", state)
	}

	// Verify context was cancelled
	select {
	case <-ctx.Done():
		// Good - context was cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("context not cancelled after stop")
	}

	// Verify no error from cancellation
	if lastErr := mysis.LastError(); lastErr != nil {
		t.Errorf("expected LastError=nil after stop, got %v", lastErr)
	}
}

// TestStateTransition_MultipleMyses tests state transitions with multiple
// myses running concurrently to ensure no cross-contamination.
func TestStateTransition_MultipleMyses(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	// Create 5 myses
	const count = 5
	myses := make([]*Mysis, count)
	for i := 0; i < count; i++ {
		m, err := cmd.CreateMysis(fmt.Sprintf("multi-%d", i), "mock")
		if err != nil {
			t.Fatalf("CreateMysis(%d) error: %v", i, err)
		}
		myses[i] = m
	}

	// Start all
	for i, m := range myses {
		if err := m.Start(); err != nil {
			t.Fatalf("Start(%d) error: %v", i, err)
		}
	}

	// Verify all running
	for i, m := range myses {
		if state := m.State(); state != MysisStateRunning {
			t.Errorf("mysis %d: expected state=running, got %s", i, state)
		}
	}

	// Stop all
	for i, m := range myses {
		if err := m.Stop(); err != nil {
			t.Fatalf("Stop(%d) error: %v", i, err)
		}
	}

	// Verify all stopped
	for i, m := range myses {
		if state := m.State(); state != MysisStateStopped {
			t.Errorf("mysis %d: expected state=stopped, got %s (LastError: %v)",
				i, state, m.LastError())
		}
		if lastErr := m.LastError(); lastErr != nil {
			t.Errorf("mysis %d: expected LastError=nil, got %v", i, lastErr)
		}
	}
}

// TestStateTransition_WaitGroupTracking verifies that the WaitGroup properly
// tracks running myses and unblocks when all are stopped.
func TestStateTransition_WaitGroupTracking(t *testing.T) {
	cmd, cleanup := setupStateMachineTest(t)
	defer cleanup()

	mysis, err := cmd.CreateMysis("test-wg", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Start mysis
	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Create a context to test WaitGroup behavior
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// WaitGroup should block while mysis is running
	done := make(chan struct{})
	go func() {
		cmd.wg.Wait()
		close(done)
	}()

	// Should still be blocked
	select {
	case <-done:
		t.Error("WaitGroup unblocked while mysis still running")
	case <-time.After(50 * time.Millisecond):
		// Good - still blocked
	}

	// Stop mysis
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// WaitGroup should unblock
	select {
	case <-done:
		// Good - WaitGroup unblocked
	case <-ctx.Done():
		t.Error("WaitGroup did not unblock after stop")
	}

	// Verify final state
	if state := mysis.State(); state != MysisStateStopped {
		t.Errorf("expected state=stopped, got %s", state)
	}
}
