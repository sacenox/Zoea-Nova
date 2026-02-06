package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/provider"
)

// TestStopRaceAggressive creates a tighter race window by calling setError
// immediately when context is cancelled (no delay)
func TestStopRaceAggressive(t *testing.T) {
	var stats struct {
		mu                sync.Mutex
		totalRuns         int
		finalStateErrored int
		finalStateStopped int
		racesDetected     int
	}

	const iterations = 100

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			// Setup test mysis
			s, bus, cleanup := setupMysisTest(t)
			defer cleanup()

			stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
			if err != nil {
				t.Fatalf("CreateMysis() error: %v", err)
			}

			mock := provider.NewMock("mock", "Hello!")
			m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

			// Start mysis
			if err := m.Start(); err != nil {
				t.Fatalf("Failed to start mysis: %v", err)
			}

			// Give it a moment to fully start
			time.Sleep(10 * time.Millisecond)

			// Capture context before Stop() nulls it out
			m.mu.RLock()
			ctx := m.ctx
			m.mu.RUnlock()

			// Aggressive: call setError immediately on context cancel (no delay)
			errorDone := make(chan struct{})
			go func() {
				<-ctx.Done()
				// NO DELAY - try to race with Stop()
				m.setError(fmt.Errorf("context canceled"))
				close(errorDone)
			}()

			// Stop mysis
			stopDone := make(chan struct{})
			go func() {
				if err := m.Stop(); err != nil {
					t.Errorf("Stop failed: %v", err)
				}
				close(stopDone)
			}()

			// Wait for both to complete
			<-stopDone
			<-errorDone

			// Give events time to propagate
			time.Sleep(20 * time.Millisecond)

			// Check final state
			finalState := m.State()

			stats.mu.Lock()
			stats.totalRuns++
			if finalState == MysisStateErrored {
				stats.finalStateErrored++
				stats.racesDetected++
				t.Logf("[RACE] Final state: Errored")
			} else if finalState == MysisStateStopped {
				stats.finalStateStopped++
				t.Logf("[OK] Final state: Stopped")
			}
			stats.mu.Unlock()
		})
	}

	// Report
	t.Logf("\n=== AGGRESSIVE RACE TEST (n=%d) ===", iterations)
	t.Logf("Total runs: %d", stats.totalRuns)
	t.Logf("Races detected (final state Errored): %d (%.1f%%)",
		stats.racesDetected,
		float64(stats.racesDetected)/float64(stats.totalRuns)*100)
	t.Logf("Correct behavior (final state Stopped): %d (%.1f%%)",
		stats.finalStateStopped,
		float64(stats.finalStateStopped)/float64(stats.totalRuns)*100)

	if stats.racesDetected > 0 {
		t.Errorf("RACE CONDITION DETECTED: %d out of %d runs ended in Errored state",
			stats.racesDetected, stats.totalRuns)
	}
}

// TestStopRaceTiming tests different delay timings to find race window
func TestStopRaceTiming(t *testing.T) {
	delays := []time.Duration{
		0, // No delay
		100 * time.Nanosecond,
		500 * time.Nanosecond,
		1 * time.Microsecond,
		10 * time.Microsecond,
		100 * time.Microsecond,
		1 * time.Millisecond,
	}

	for _, delay := range delays {
		t.Run(fmt.Sprintf("delay_%s", delay), func(t *testing.T) {
			var raceCount atomic.Int32

			const runsPerDelay = 50

			for i := 0; i < runsPerDelay; i++ {
				// Setup test mysis
				s, bus, cleanup := setupMysisTest(t)

				stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
				if err != nil {
					cleanup()
					t.Fatalf("CreateMysis() error: %v", err)
				}

				mock := provider.NewMock("mock", "Hello!")
				m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

				// Start mysis
				if err := m.Start(); err != nil {
					cleanup()
					t.Fatalf("Failed to start mysis: %v", err)
				}

				// Give it a moment to fully start
				time.Sleep(10 * time.Millisecond)

				// Capture context
				m.mu.RLock()
				ctx := m.ctx
				m.mu.RUnlock()

				// setError goroutine with configurable delay
				errorDone := make(chan struct{})
				go func() {
					<-ctx.Done()
					if delay > 0 {
						time.Sleep(delay)
					}
					m.setError(fmt.Errorf("context canceled"))
					close(errorDone)
				}()

				// Stop mysis
				stopDone := make(chan struct{})
				go func() {
					if err := m.Stop(); err != nil {
						t.Errorf("Stop failed: %v", err)
					}
					close(stopDone)
				}()

				// Wait for both
				<-stopDone
				<-errorDone

				time.Sleep(10 * time.Millisecond)

				// Check result
				if m.State() == MysisStateErrored {
					raceCount.Add(1)
				}

				cleanup()
			}

			races := raceCount.Load()
			t.Logf("Delay %s: %d/%d races (%.1f%%)",
				delay, races, runsPerDelay,
				float64(races)/float64(runsPerDelay)*100)

			if races > 0 {
				t.Errorf("RACE DETECTED with delay %s: %d races out of %d runs",
					delay, races, runsPerDelay)
			}
		})
	}
}
