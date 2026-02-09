package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// Global atomic counter to track Stop() phases
var stopPhase atomic.Int32 // 0=not started, 1=cancelled, 2=waiting, 3=stopped

// instrumentedStop is a wrapper around Stop() that tracks phases
func (m *Mysis) instrumentedStop(t *testing.T) error {
	t.Helper()

	// Phase 0 → 1: After cancel()
	m.mu.Lock()
	if m.state != MysisStateRunning {
		m.mu.Unlock()
		return nil
	}

	if m.cancel != nil {
		m.cancel()
		stopPhase.Store(1) // PHASE 1: Context cancelled
		t.Logf("[STOP] Phase 1: Context cancelled")
	}
	m.mu.Unlock()

	// Phase 1 → 2: During turnMu wait
	stopPhase.Store(2) // PHASE 2: Waiting for turn to complete
	t.Logf("[STOP] Phase 2: Waiting for turnMu")

	done := make(chan struct{})
	go func() {
		m.turnMu.Lock()
		close(done)
		m.turnMu.Unlock()
	}()

	select {
	case <-done:
		t.Logf("[STOP] Phase 2: turnMu acquired")
	case <-time.After(5 * time.Second):
		t.Logf("[STOP] Phase 2: Timeout waiting for turnMu")
	}

	// Phase 2 → 3: After setting Stopped
	m.mu.Lock()
	if m.state != MysisStateRunning {
		m.mu.Unlock()
		return nil
	}
	m.cancel = nil
	m.ctx = nil

	oldState := m.state
	m.state = MysisStateStopped
	stopPhase.Store(3) // PHASE 3: State set to Stopped
	t.Logf("[STOP] Phase 3: State set to Stopped")
	m.mu.Unlock()

	// Update store
	if err := m.store.UpdateMysisState(m.id, store.MysisStateStopped); err != nil {
		return err
	}

	// Emit state change event
	m.emitStateChange(oldState, MysisStateStopped)
	m.releaseCurrentAccount()

	// Close provider HTTP client
	if m.provider != nil {
		if err := m.provider.Close(); err != nil {
			t.Logf("[STOP] Warning: Failed to close provider: %v", err)
		}
	}

	return nil
}

// instrumentedSetError is a wrapper that logs the Stop phase when called
func (m *Mysis) instrumentedSetError(t *testing.T, err error) {
	t.Helper()

	// Read current stop phase
	phase := stopPhase.Load()
	t.Logf("[setError] Called during Stop phase: %d (err: %v)", phase, err)

	m.mu.Lock()
	oldState := m.state
	t.Logf("[setError] Current state before check: %s", oldState)

	// If mysis was intentionally stopped, don't override with error state
	if oldState == MysisStateStopped {
		m.mu.Unlock()
		t.Logf("[setError] Ignoring error - mysis already Stopped (phase %d)", phase)
		return
	}

	t.Logf("[setError] Setting state to Errored (was %s, Stop phase %d)", oldState, phase)
	m.lastError = err
	m.state = MysisStateErrored
	m.mu.Unlock()

	// Update store
	if updateErr := m.store.UpdateMysisState(m.id, store.MysisStateErrored); updateErr != nil {
		t.Logf("[setError] Failed to update store: %v", updateErr)
	}

	// Emit state change event
	m.emitStateChange(oldState, MysisStateErrored)

	// Release account (if any)
	m.releaseCurrentAccount()

	// Emit error event
	m.publishCriticalEvent(Event{
		Type:      EventMysisError,
		MysisID:   m.id,
		MysisName: m.name,
		Error:     &ErrorData{Error: err.Error()},
		Timestamp: time.Now(),
	})
}

// TestStopRaceInstrumented runs 100 iterations to capture race statistics
func TestStopRaceInstrumented(t *testing.T) {
	// Statistics tracking
	var stats struct {
		mu                   sync.Mutex
		totalRuns            int
		raceDetected         int
		phaseWhenErrorOccurs map[int32]int
		finalStateErrored    int
		finalStateStopped    int
		errorBeforeStopped   int
		errorAfterStopped    int
		errorDuringWait      int
	}

	stats.phaseWhenErrorOccurs = make(map[int32]int)

	const iterations = 100

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			// Reset phase counter
			stopPhase.Store(0)

			// Setup test mysis
			s, bus, cleanup := setupMysisTest(t)
			defer cleanup()

			stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
			if err != nil {
				t.Fatalf("CreateMysis() error: %v", err)
			}

			mock := provider.NewMock("mock", "Hello!")
			m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

			// Start mysis
			if err := m.Start(); err != nil {
				t.Fatalf("Failed to start mysis: %v", err)
			}

			// Give it a moment to fully start
			time.Sleep(10 * time.Millisecond)

			// Create a channel to simulate context cancellation causing setError
			errorCh := make(chan struct{})

			// Capture context before Stop() nulls it out
			m.mu.RLock()
			ctx := m.ctx
			m.mu.RUnlock()

			// Goroutine that will trigger setError when context is cancelled
			go func() {
				<-ctx.Done()
				// Simulate some work that might fail when context is cancelled
				time.Sleep(1 * time.Millisecond) // Small delay to increase race window
				m.instrumentedSetError(t, fmt.Errorf("context canceled"))
				close(errorCh)
			}()

			// Stop mysis (this will cancel context)
			if err := m.instrumentedStop(t); err != nil {
				t.Errorf("Stop failed: %v", err)
			}

			// Wait for error goroutine to complete
			<-errorCh

			// Give events time to propagate
			time.Sleep(50 * time.Millisecond)

			// Check final state
			finalState := m.State()
			t.Logf("[FINAL] State: %s", finalState)

			// Record statistics
			stats.mu.Lock()
			stats.totalRuns++

			phase := stopPhase.Load()
			stats.phaseWhenErrorOccurs[phase]++

			if finalState == MysisStateErrored {
				stats.finalStateErrored++
				stats.raceDetected++
				t.Logf("[RACE DETECTED] setError won - final state is Errored (phase %d)", phase)

				// Categorize when error occurred relative to stop phases
				switch phase {
				case 1:
					stats.errorBeforeStopped++
				case 2:
					stats.errorDuringWait++
				case 3:
					stats.errorAfterStopped++
				}
			} else if finalState == MysisStateStopped {
				stats.finalStateStopped++
				t.Logf("[CORRECT] Stop won - final state is Stopped (phase %d)", phase)
			}
			stats.mu.Unlock()
		})
	}

	// Report statistics
	t.Logf("\n=== RACE STATISTICS (n=%d) ===", iterations)
	t.Logf("Total runs: %d", stats.totalRuns)
	t.Logf("Races detected (final state Errored): %d (%.1f%%)",
		stats.raceDetected,
		float64(stats.raceDetected)/float64(stats.totalRuns)*100)
	t.Logf("Correct behavior (final state Stopped): %d (%.1f%%)",
		stats.finalStateStopped,
		float64(stats.finalStateStopped)/float64(stats.totalRuns)*100)

	t.Logf("\n=== PHASE DISTRIBUTION ===")
	for phase := int32(0); phase <= 3; phase++ {
		count := stats.phaseWhenErrorOccurs[phase]
		if count > 0 {
			t.Logf("Phase %d: %d occurrences (%.1f%%)",
				phase,
				count,
				float64(count)/float64(stats.totalRuns)*100)
		}
	}

	t.Logf("\n=== ERROR TIMING ===")
	t.Logf("Errors during phase 1 (cancelled, before stopped): %d", stats.errorBeforeStopped)
	t.Logf("Errors during phase 2 (waiting for turnMu): %d", stats.errorDuringWait)
	t.Logf("Errors during phase 3 (after stopped set): %d", stats.errorAfterStopped)

	t.Logf("\n=== CORRELATION ANALYSIS ===")
	if stats.raceDetected > 0 {
		t.Logf("When race occurs:")
		if stats.errorBeforeStopped > 0 {
			t.Logf("  - Phase 1 (cancelled): %.1f%% of races",
				float64(stats.errorBeforeStopped)/float64(stats.raceDetected)*100)
		}
		if stats.errorDuringWait > 0 {
			t.Logf("  - Phase 2 (waiting): %.1f%% of races",
				float64(stats.errorDuringWait)/float64(stats.raceDetected)*100)
		}
		if stats.errorAfterStopped > 0 {
			t.Logf("  - Phase 3 (stopped): %.1f%% of races",
				float64(stats.errorAfterStopped)/float64(stats.raceDetected)*100)
		}
	}
}

// TestStopRaceInstrumentedSingle runs a single iteration for detailed debugging
func TestStopRaceInstrumentedSingle(t *testing.T) {
	// Reset phase counter
	stopPhase.Store(0)

	// Setup test mysis
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "Hello!")
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Start mysis
	if err := m.Start(); err != nil {
		t.Fatalf("Failed to start mysis: %v", err)
	}

	// Give it a moment to fully start
	time.Sleep(10 * time.Millisecond)

	// Create a channel to simulate context cancellation causing setError
	errorCh := make(chan struct{})

	// Capture context before Stop() nulls it out
	m.mu.RLock()
	ctx := m.ctx
	m.mu.RUnlock()

	// Goroutine that will trigger setError when context is cancelled
	go func() {
		<-ctx.Done()
		t.Logf("[ERROR GOROUTINE] Context cancelled, triggering setError")
		time.Sleep(1 * time.Millisecond) // Small delay to increase race window
		m.instrumentedSetError(t, fmt.Errorf("context canceled"))
		close(errorCh)
	}()

	// Stop mysis (this will cancel context)
	t.Logf("[TEST] Calling Stop()")
	if err := m.instrumentedStop(t); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Wait for error goroutine to complete
	t.Logf("[TEST] Waiting for error goroutine")
	<-errorCh

	// Give events time to propagate
	time.Sleep(50 * time.Millisecond)

	// Check final state
	finalState := m.State()
	t.Logf("[TEST] Final state: %s", finalState)

	// Verify state is Stopped (not Errored)
	if finalState != MysisStateStopped {
		t.Errorf("Expected final state Stopped, got %s (race detected)", finalState)
	} else {
		t.Logf("[SUCCESS] Final state is correctly Stopped")
	}
}
