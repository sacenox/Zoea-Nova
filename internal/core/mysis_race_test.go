package core

import (
	"sync"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestStopWithMultipleInFlightMessages tests if multiple SendMessage goroutines
// cause the race condition where Stop() completes but state ends up as Errored.
//
// Test scenario:
// 1. Create mock provider with 2 second delay to simulate slow LLM
// 2. Start mysis
// 3. Send 3 messages rapidly (they all queue up waiting for turnMu)
// 4. Call Stop() while multiple SendMessages are in-flight
// 5. Verify final state is Stopped (not Errored)
//
// Run with: go test ./internal/core -run TestStopWithMultipleInFlightMessages -count=10 -race
func TestStopWithMultipleInFlightMessages(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create stored mysis
	stored, err := s.CreateMysis("race-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Mock provider with 2s delay and respects context cancellation
	mock := provider.NewMock("mock", "ok").SetDelay(2 * time.Second)
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Start the mysis
	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Wait for initial autonomous turn to complete
	events := bus.Subscribe()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				// Initial turn complete, proceed to test
				goto testStart
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial autonomous turn")
		}
	}

testStart:
	// Track SendMessage completion and errors
	var wg sync.WaitGroup
	msgErrors := make([]error, 3)

	// Send 3 messages rapidly in parallel
	// These will all block on turnMu after the first one acquires it
	for i := 0; i < 3; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			msgErrors[idx] = mysis.SendMessage("test message", store.MemorySourceDirect)
		}()
	}

	// Give messages time to queue up and start competing for turnMu
	// First message will acquire turnMu and start its 2s delay
	// The other 2 will be blocked waiting for turnMu
	time.Sleep(100 * time.Millisecond)

	// Now call Stop() while:
	// - Message 1 is in-flight (has turnMu, doing 2s delay in provider)
	// - Messages 2 and 3 are queued (blocked on turnMu.Lock())
	stopErr := mysis.Stop()
	if stopErr != nil {
		t.Fatalf("Stop() error: %v", stopErr)
	}

	// Wait for all SendMessage goroutines to complete
	// They should all exit due to context cancellation or state check
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for SendMessage goroutines to complete - possible goroutine leak")
	}

	// CRITICAL ASSERTION: Final state must be Stopped, not Errored
	finalState := mysis.State()
	if finalState != MysisStateStopped {
		t.Errorf("RACE DETECTED: expected final state=Stopped, got %s", finalState)

		// Additional diagnostics
		if finalState == MysisStateErrored {
			t.Errorf("State is Errored. LastError: %v", mysis.LastError())
		}

		t.Logf("SendMessage errors:")
		for i, err := range msgErrors {
			t.Logf("  Message %d: %v", i+1, err)
		}
	}

	// Verify no error was set
	if mysis.LastError() != nil {
		t.Errorf("expected no lastError, got: %v", mysis.LastError())
	}

	// Log SendMessage results for analysis
	t.Logf("SendMessage results:")
	for i, err := range msgErrors {
		t.Logf("  Message %d: %v", i+1, err)
	}
}

// TestStopWithMultipleInFlightMessages_Shorter is a faster variant with 200ms delay
// This can be run with higher -count values for stress testing
func TestStopWithMultipleInFlightMessages_Shorter(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("race-test-short", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Shorter delay for faster test iteration
	mock := provider.NewMock("mock", "ok").SetDelay(200 * time.Millisecond)
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Wait for initial turn
	events := bus.Subscribe()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				goto shortTestStart
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial autonomous turn")
		}
	}

shortTestStart:
	var wg sync.WaitGroup
	msgErrors := make([]error, 5) // Test with 5 messages

	// Send 5 messages rapidly
	for i := 0; i < 5; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			msgErrors[idx] = mysis.SendMessage("test", store.MemorySourceDirect)
		}()
	}

	time.Sleep(50 * time.Millisecond)

	stopErr := mysis.Stop()
	if stopErr != nil {
		t.Fatalf("Stop() error: %v", stopErr)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for goroutines")
	}

	finalState := mysis.State()
	if finalState != MysisStateStopped {
		t.Errorf("RACE: expected Stopped, got %s (lastError: %v)", finalState, mysis.LastError())
	}
}
