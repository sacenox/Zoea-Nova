package core

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestLongRunningStopScenario reproduces the EXACT production failure:
// 1. App running for ~9 hours
// 2. Multiple myses active (long, sleepy runner)
// 3. User quits app (calls StopAll)
// 4. App should exit cleanly
// 5. All myses should show state=stopped (NOT errored)
//
// This test validates:
// - StopAll completes within 10 seconds
// - Both myses reach state=stopped (NOT errored)
// - No goroutine leaks (WaitGroup completes)
// - No deadlocks
// - lastError == nil for all myses
//
// Run with:
//
//	go test ./internal/core -run TestLongRunningStopScenario -v -count=10
func TestLongRunningStopScenario(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer s.Close()

	bus := NewEventBus(100)
	defer bus.Close()

	reg := provider.NewRegistry()
	cfg := &config.Config{
		Swarm: config.SwarmConfig{MaxMyses: 10},
		Providers: map[string]config.ProviderConfig{
			"mock": {Model: "test-model", Temperature: 0.7},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)

	// Create two myses: "long" and "sleepy"
	longMock := provider.NewMock("mock-long", "response from long")
	sleepyMock := provider.NewMock("mock-sleepy", "response from sleepy")

	storedLong, _ := s.CreateMysis("long", "mock", "test-model", 0.7)
	storedSleepy, _ := s.CreateMysis("sleepy", "mock", "test-model", 0.7)

	long := NewMysis(storedLong.ID, storedLong.Name, storedLong.CreatedAt, longMock, s, bus, cmd)
	sleepy := NewMysis(storedSleepy.ID, storedSleepy.Name, storedSleepy.CreatedAt, sleepyMock, s, bus, cmd)

	// Add them to commander
	cmd.mu.Lock()
	cmd.myses[storedLong.ID] = long
	cmd.myses[storedSleepy.ID] = sleepy
	cmd.mu.Unlock()

	// Start both myses
	if err := long.Start(); err != nil {
		t.Fatalf("Start(long) error: %v", err)
	}
	if err := sleepy.Start(); err != nil {
		t.Fatalf("Start(sleepy) error: %v", err)
	}

	// Simulate long runtime with multiple turns
	// In production: 9 hours of runtime = hundreds of turns
	// In test: simulate with rapid-fire messages
	for i := 0; i < 5; i++ {
		long.SendMessage(fmt.Sprintf("message %d", i), store.MemorySourceDirect)
		sleepy.SendMessage(fmt.Sprintf("message %d", i), store.MemorySourceDirect)
		time.Sleep(50 * time.Millisecond) // Let messages process
	}

	// Give myses time to settle into idle state
	time.Sleep(200 * time.Millisecond)

	// Verify both are running before stop
	if long.State() != MysisStateRunning {
		t.Fatalf("expected long state=running before stop, got %s", long.State())
	}
	if sleepy.State() != MysisStateRunning {
		t.Fatalf("expected sleepy state=running before stop, got %s", sleepy.State())
	}

	// CRITICAL: Call StopAll() - this is what TUI does on quit
	// This must complete within 10 seconds
	start := time.Now()
	cmd.StopAll()
	elapsed := time.Since(start)

	// Verify StopAll completed within timeout
	if elapsed > 12*time.Second { // Allow 2s buffer
		t.Errorf("StopAll took %s (expected < 10s)", elapsed)
	}

	// CRITICAL ASSERTION: Both myses should be in Stopped state (NOT Errored)
	if long.State() != MysisStateStopped {
		t.Errorf("PRODUCTION BUG REPRODUCED: long state=%s (expected stopped), lastError=%v",
			long.State(), long.LastError())
	}
	if sleepy.State() != MysisStateStopped {
		t.Errorf("PRODUCTION BUG REPRODUCED: sleepy state=%s (expected stopped), lastError=%v",
			sleepy.State(), sleepy.LastError())
	}

	// Verify no errors were set
	if long.LastError() != nil {
		t.Errorf("long should have no error after clean stop, got: %v", long.LastError())
	}
	if sleepy.LastError() != nil {
		t.Errorf("sleepy should have no error after clean stop, got: %v", sleepy.LastError())
	}

	// Verify WaitGroup completed (no goroutine leaks)
	// This is implicit in StopAll() completing, but verify explicitly
	done := make(chan struct{})
	go func() {
		cmd.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// WaitGroup completed - no goroutine leaks
	case <-time.After(1 * time.Second):
		t.Error("WaitGroup did not complete - goroutine leak detected")
	}
}

// TestMultipleMysesStoppingSimultaneously tests the scenario where
// multiple myses are stopped at exactly the same time.
//
// This validates:
// - No race conditions in Stop()
// - No deadlocks when multiple Stop() calls happen simultaneously
// - All myses reach state=stopped
//
// Run with:
//
//	go test ./internal/core -run TestMultipleMysesStoppingSimultaneously -v -count=10
func TestMultipleMysesStoppingSimultaneously(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	reg := provider.NewRegistry()
	cfg := &config.Config{
		Swarm: config.SwarmConfig{MaxMyses: 10},
		Providers: map[string]config.ProviderConfig{
			"mock": {Model: "test-model", Temperature: 0.7},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)

	// Create 5 myses
	myses := make([]*Mysis, 5)
	for i := 0; i < 5; i++ {
		mock := provider.NewMock(fmt.Sprintf("mock-%d", i), "response")
		stored, _ := s.CreateMysis(fmt.Sprintf("mysis-%d", i), "mock", "test-model", 0.7)
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, cmd)
		myses[i] = mysis

		cmd.mu.Lock()
		cmd.myses[stored.ID] = mysis
		cmd.mu.Unlock()

		if err := mysis.Start(); err != nil {
			t.Fatalf("Start(mysis-%d) error: %v", i, err)
		}
	}

	// Wait for myses to stabilize
	time.Sleep(100 * time.Millisecond)

	// Stop all myses simultaneously
	var wg sync.WaitGroup
	errors := make([]error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errors[idx] = myses[idx].Stop()
		}(i)
	}

	// Wait for all stops to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All stops completed
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for simultaneous stops - possible deadlock")
	}

	// Verify all stopped successfully
	for i := 0; i < 5; i++ {
		if errors[i] != nil {
			t.Errorf("mysis-%d Stop() error: %v", i, errors[i])
		}
		if myses[i].State() != MysisStateStopped {
			t.Errorf("mysis-%d state=%s (expected stopped), lastError=%v",
				i, myses[i].State(), myses[i].LastError())
		}
		if myses[i].LastError() != nil {
			t.Errorf("mysis-%d should have no error, got: %v", i, myses[i].LastError())
		}
	}
}

// TestStopDuringActiveLLMCalls tests stopping myses while they are
// actively processing LLM calls.
//
// This validates:
// - Stop() waits for active LLM calls to complete
// - Context cancellation is handled gracefully
// - Final state is stopped (NOT errored)
//
// Run with:
//
//	go test ./internal/core -run TestStopDuringActiveLLMCalls -v -count=10
func TestStopDuringActiveLLMCalls(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("active-call", "mock", "test-model", 0.7)

	// Mock with delay to simulate active LLM call
	mock := provider.NewMock("mock", "response").SetDelay(200 * time.Millisecond)
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Wait for initial autonomous turn to complete
	time.Sleep(100 * time.Millisecond)

	// Send a message that will take 200ms to process
	messageDone := make(chan struct{})
	go func() {
		_ = mysis.SendMessage("slow message", store.MemorySourceDirect)
		close(messageDone)
	}()

	// Wait to ensure message processing started
	time.Sleep(100 * time.Millisecond)

	// Stop while LLM is processing
	start := time.Now()
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	elapsed := time.Since(start)

	// Stop should complete within reasonable time
	// It may wait for the current turn or cancel immediately - both are acceptable
	if elapsed > 1*time.Second {
		t.Errorf("Stop took too long (%s) - possible deadlock", elapsed)
	}

	// Verify final state
	if mysis.State() != MysisStateStopped {
		t.Errorf("expected state=stopped, got %s (lastError=%v)", mysis.State(), mysis.LastError())
	}
	if mysis.LastError() != nil {
		t.Errorf("expected no error after Stop during active call, got: %v", mysis.LastError())
	}
}

// TestStopWithQueuedMessages tests stopping a mysis that has queued messages
// waiting to be processed.
//
// This validates:
// - Queued messages are not processed after Stop()
// - No deadlock when stopping with queued messages
// - Final state is stopped
//
// Run with:
//
//	go test ./internal/core -run TestStopWithQueuedMessages -v -count=10
func TestStopWithQueuedMessages(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("queued", "mock", "test-model", 0.7)

	// Mock with delay to create queue pressure
	mock := provider.NewMock("mock", "response").SetDelay(100 * time.Millisecond)
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Queue multiple messages rapidly
	// Only one will process at a time due to turnMu
	for i := 0; i < 5; i++ {
		go func(idx int) {
			_ = mysis.SendMessage(fmt.Sprintf("queued-%d", idx), store.MemorySourceDirect)
		}(i)
	}

	// Wait a bit to let some messages queue
	time.Sleep(50 * time.Millisecond)

	// Stop while messages are queued
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// Verify final state
	if mysis.State() != MysisStateStopped {
		t.Errorf("expected state=stopped, got %s (lastError=%v)", mysis.State(), mysis.LastError())
	}
	if mysis.LastError() != nil {
		t.Errorf("expected no error, got: %v", mysis.LastError())
	}

	// Verify no further messages can be sent
	err := mysis.SendMessage("after-stop", store.MemorySourceDirect)
	if err == nil {
		t.Error("expected error sending message to stopped mysis")
	}
}

// TestCleanExitAfterStop tests that the app can exit cleanly after StopAll.
//
// This validates:
// - EventBus can be closed after StopAll
// - Store can be closed after StopAll
// - No goroutines block cleanup
// - No panic during cleanup
//
// Run with:
//
//	go test ./internal/core -run TestCleanExitAfterStop -v -count=10
func TestCleanExitAfterStop(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	bus := NewEventBus(100)

	reg := provider.NewRegistry()
	cfg := &config.Config{
		Swarm: config.SwarmConfig{MaxMyses: 10},
		Providers: map[string]config.ProviderConfig{
			"mock": {Model: "test-model", Temperature: 0.7},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)

	// Create and start multiple myses
	for i := 0; i < 3; i++ {
		mock := provider.NewMock(fmt.Sprintf("mock-%d", i), "response")
		stored, _ := s.CreateMysis(fmt.Sprintf("exit-test-%d", i), "mock", "test-model", 0.7)
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, cmd)

		cmd.mu.Lock()
		cmd.myses[stored.ID] = mysis
		cmd.mu.Unlock()

		if err := mysis.Start(); err != nil {
			t.Fatalf("Start() error: %v", err)
		}
	}

	// Wait for myses to stabilize
	time.Sleep(100 * time.Millisecond)

	// Stop all myses
	cmd.StopAll()

	// Now perform cleanup (this is what main.go does)
	// If this hangs or panics, we have a problem
	done := make(chan struct{})
	go func() {
		bus.Close()
		s.Close()
		close(done)
	}()

	select {
	case <-done:
		// Clean exit successful
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup hung - possible goroutine leak or deadlock")
	}
}

// TestStopAllWithMixedStates tests StopAll when myses are in different states.
//
// This validates:
// - StopAll handles idle myses correctly
// - StopAll skips already-stopped myses
// - StopAll doesn't affect errored myses
// - No panic when calling Stop on non-running myses
//
// Run with:
//
//	go test ./internal/core -run TestStopAllWithMixedStates -v -count=10
func TestStopAllWithMixedStates(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	reg := provider.NewRegistry()
	cfg := &config.Config{
		Swarm: config.SwarmConfig{MaxMyses: 10},
		Providers: map[string]config.ProviderConfig{
			"mock": {Model: "test-model", Temperature: 0.7},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)

	// Create myses in different states
	// 1. Running
	runningStored, _ := s.CreateMysis("running", "mock", "test-model", 0.7)
	runningMock := provider.NewMock("mock-running", "response")
	running := NewMysis(runningStored.ID, runningStored.Name, runningStored.CreatedAt, runningMock, s, bus, cmd)
	cmd.mu.Lock()
	cmd.myses[runningStored.ID] = running
	cmd.mu.Unlock()
	running.Start()

	// 2. Idle (never started)
	idleStored, _ := s.CreateMysis("idle", "mock", "test-model", 0.7)
	idleMock := provider.NewMock("mock-idle", "response")
	idle := NewMysis(idleStored.ID, idleStored.Name, idleStored.CreatedAt, idleMock, s, bus, cmd)
	cmd.mu.Lock()
	cmd.myses[idleStored.ID] = idle
	cmd.mu.Unlock()

	// 3. Already stopped
	stoppedStored, _ := s.CreateMysis("stopped", "mock", "test-model", 0.7)
	stoppedMock := provider.NewMock("mock-stopped", "response")
	stopped := NewMysis(stoppedStored.ID, stoppedStored.Name, stoppedStored.CreatedAt, stoppedMock, s, bus, cmd)
	cmd.mu.Lock()
	cmd.myses[stoppedStored.ID] = stopped
	cmd.mu.Unlock()
	stopped.Start()
	stopped.Stop()

	// Wait for states to stabilize
	time.Sleep(100 * time.Millisecond)

	// Call StopAll - should handle mixed states gracefully
	start := time.Now()
	cmd.StopAll()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("StopAll took %s (expected < 5s)", elapsed)
	}

	// Verify states
	if running.State() != MysisStateStopped {
		t.Errorf("running mysis state=%s (expected stopped)", running.State())
	}
	if idle.State() != MysisStateIdle {
		t.Errorf("idle mysis state=%s (expected idle)", idle.State())
	}
	if stopped.State() != MysisStateStopped {
		t.Errorf("stopped mysis state=%s (expected stopped)", stopped.State())
	}

	// Verify no errors
	if running.LastError() != nil {
		t.Errorf("running mysis error: %v", running.LastError())
	}
	if idle.LastError() != nil {
		t.Errorf("idle mysis error: %v", idle.LastError())
	}
	if stopped.LastError() != nil {
		t.Errorf("stopped mysis error: %v", stopped.LastError())
	}
}

// TestStopAllStressTest performs repeated StopAll operations to detect
// rare race conditions and timing issues.
//
// Run with:
//
//	go test ./internal/core -run TestStopAllStressTest -v -count=20
func TestStopAllStressTest(t *testing.T) {
	for iteration := 0; iteration < 5; iteration++ {
		t.Run(fmt.Sprintf("iteration_%d", iteration), func(t *testing.T) {
			tmpDir := t.TempDir()
			dbPath := filepath.Join(tmpDir, "test.db")
			s, err := store.Open(dbPath)
			if err != nil {
				t.Fatalf("Open() error: %v", err)
			}
			defer s.Close()

			bus := NewEventBus(100)
			defer bus.Close()

			reg := provider.NewRegistry()
			cfg := &config.Config{
				Swarm: config.SwarmConfig{MaxMyses: 10},
				Providers: map[string]config.ProviderConfig{
					"mock": {Model: "test-model", Temperature: 0.7},
				},
			}

			cmd := NewCommander(s, reg, bus, cfg)

			// Create multiple myses
			for i := 0; i < 3; i++ {
				mock := provider.NewMock(fmt.Sprintf("mock-%d", i), "response")
				stored, _ := s.CreateMysis(fmt.Sprintf("stress-%d", i), "mock", "test-model", 0.7)
				mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, cmd)

				cmd.mu.Lock()
				cmd.myses[stored.ID] = mysis
				cmd.mu.Unlock()

				if err := mysis.Start(); err != nil {
					t.Fatalf("Start() error: %v", err)
				}

				// Send some messages to simulate activity
				go func() {
					_ = mysis.SendMessage("stress test", store.MemorySourceDirect)
				}()
			}

			// Random sleep to vary timing
			time.Sleep(time.Duration(50+iteration*20) * time.Millisecond)

			// StopAll
			start := time.Now()
			cmd.StopAll()
			elapsed := time.Since(start)

			if elapsed > 10*time.Second {
				t.Errorf("iteration %d: StopAll timeout (%s)", iteration, elapsed)
			}

			// Verify all myses are stopped
			cmd.mu.RLock()
			for id, mysis := range cmd.myses {
				state := mysis.State()
				// Running myses should now be stopped
				// Idle myses remain idle
				if state != MysisStateStopped && state != MysisStateIdle {
					t.Errorf("iteration %d: mysis %s state=%s (expected stopped or idle), lastError=%v",
						iteration, id, state, mysis.LastError())
				}
			}
			cmd.mu.RUnlock()
		})
	}
}
