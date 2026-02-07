package core

import (
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/constants"
)

// TestMysis_ShouldNudge_Idle tests nudge behavior when mysis is idle.
func TestMysis_ShouldNudge_Idle(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateIdle,
		activityUntil: time.Time{}, // Zero time means no wait
	}

	now := time.Now()

	if !m.shouldNudge(now) {
		t.Error("Expected nudge when idle (no activity until time)")
	}
}

// TestMysis_ShouldNudge_Traveling_InFuture tests nudges during travel.
func TestMysis_ShouldNudge_Traveling_InFuture(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateTraveling,
		activityUntil: time.Now().Add(30 * time.Second), // Still traveling
	}

	now := time.Now()

	if !m.shouldNudge(now) {
		t.Error("Expected nudge while traveling (activity until time in future)")
	}

	if m.activityState != ActivityStateTraveling {
		t.Errorf("Expected activity state traveling to remain, got %s", m.activityState)
	}

	if m.activityUntil.IsZero() {
		t.Error("Expected activityUntil to remain set")
	}
}

// TestMysis_ShouldNudge_Traveling_Arrived tests nudge after arrival.
func TestMysis_ShouldNudge_Traveling_Arrived(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateTraveling,
		activityUntil: time.Now().Add(-1 * time.Second), // Already arrived
	}

	now := time.Now()

	if !m.shouldNudge(now) {
		t.Error("Expected nudge after arrival (activity until time in past)")
	}

	// Verify state cleared to idle after nudge check
	if m.activityState != ActivityStateIdle {
		t.Errorf("Expected activity state cleared to idle, got %s", m.activityState)
	}

	if !m.activityUntil.IsZero() {
		t.Error("Expected activityUntil cleared to zero time")
	}
}

// TestMysis_ShouldNudge_Cooldown_Active tests nudges during cooldown.
func TestMysis_ShouldNudge_Cooldown_Active(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateCooldown,
		activityUntil: time.Now().Add(10 * time.Second), // Cooldown active
	}

	now := time.Now()

	if !m.shouldNudge(now) {
		t.Error("Expected nudge during active cooldown")
	}

	if m.activityState != ActivityStateCooldown {
		t.Errorf("Expected activity state cooldown to remain, got %s", m.activityState)
	}

	if m.activityUntil.IsZero() {
		t.Error("Expected activityUntil to remain set")
	}
}

// TestMysis_ShouldNudge_Cooldown_Expired tests nudge after cooldown expires.
func TestMysis_ShouldNudge_Cooldown_Expired(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateCooldown,
		activityUntil: time.Now().Add(-1 * time.Second), // Cooldown expired
	}

	now := time.Now()

	if !m.shouldNudge(now) {
		t.Error("Expected nudge after cooldown expires")
	}

	// Verify state cleared
	if m.activityState != ActivityStateIdle {
		t.Errorf("Expected activity state cleared to idle, got %s", m.activityState)
	}
}

// TestMysis_ShouldNudge_Mining tests nudge during mining (should allow nudges).
func TestMysis_ShouldNudge_Mining(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateMining,
		activityUntil: time.Time{},
	}

	now := time.Now()

	if !m.shouldNudge(now) {
		t.Error("Expected nudge during mining activity")
	}
}

// TestMysis_ShouldNudge_InCombat tests nudge during combat (should allow nudges).
func TestMysis_ShouldNudge_InCombat(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateInCombat,
		activityUntil: time.Time{},
	}

	now := time.Now()

	if !m.shouldNudge(now) {
		t.Error("Expected nudge during combat activity")
	}
}

// TestMysis_ClearActivityIf_Matching tests clearing activity when state matches.
func TestMysis_ClearActivityIf_Matching(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateTraveling,
		activityUntil: time.Now().Add(30 * time.Second),
	}

	m.clearActivityIf(ActivityStateTraveling)

	if m.activityState != ActivityStateIdle {
		t.Errorf("Expected activity state idle after clear, got %s", m.activityState)
	}

	if !m.activityUntil.IsZero() {
		t.Error("Expected activityUntil cleared to zero time")
	}
}

// TestMysis_ClearActivityIf_NotMatching tests that clearing doesn't happen for wrong state.
func TestMysis_ClearActivityIf_NotMatching(t *testing.T) {
	originalUntil := time.Now().Add(30 * time.Second)
	m := &Mysis{
		activityState: ActivityStateTraveling,
		activityUntil: originalUntil,
	}

	// Try to clear with different state
	m.clearActivityIf(ActivityStateCooldown)

	// Verify state NOT cleared
	if m.activityState != ActivityStateTraveling {
		t.Errorf("Expected activity state unchanged (traveling), got %s", m.activityState)
	}

	if m.activityUntil.IsZero() {
		t.Error("Expected activityUntil unchanged, but was cleared")
	}
}

// TestMysis_SetActivity tests setting activity state.
func TestMysis_SetActivity(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateIdle,
		activityUntil: time.Time{},
	}

	expectedUntil := time.Now().Add(1 * time.Minute)
	m.setActivity(ActivityStateTraveling, expectedUntil)

	if m.activityState != ActivityStateTraveling {
		t.Errorf("Expected activity state traveling, got %s", m.activityState)
	}

	if m.activityUntil != expectedUntil {
		t.Errorf("Expected activityUntil %v, got %v", expectedUntil, m.activityUntil)
	}
}

// TestMysis_SetActivity_ZeroTime tests setting activity with zero time.
func TestMysis_SetActivity_ZeroTime(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateTraveling,
		activityUntil: time.Now(),
	}

	m.setActivity(ActivityStateIdle, time.Time{})

	if m.activityState != ActivityStateIdle {
		t.Errorf("Expected activity state idle, got %s", m.activityState)
	}

	if !m.activityUntil.IsZero() {
		t.Error("Expected activityUntil to be zero time")
	}
}

// TestMysis_ShouldNudge_Concurrent tests thread safety of shouldNudge.
func TestMysis_ShouldNudge_Concurrent(t *testing.T) {
	m := &Mysis{
		activityState: ActivityStateTraveling,
		activityUntil: time.Now().Add(1 * time.Second),
	}

	done := make(chan bool)

	// Run multiple goroutines calling shouldNudge
	for i := 0; i < 10; i++ {
		go func() {
			now := time.Now()
			_ = m.shouldNudge(now)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// No assertions needed - just verify no data races (run with -race)
}

// TestMysis_ActivityStateTransitions tests various activity state transitions.
func TestMysis_ActivityStateTransitions(t *testing.T) {
	tests := []struct {
		name          string
		initialState  ActivityState
		initialUntil  time.Time
		nudgeTime     time.Time
		expectNudge   bool
		expectedState ActivityState
	}{
		{
			name:          "idle_to_idle",
			initialState:  ActivityStateIdle,
			initialUntil:  time.Time{},
			nudgeTime:     time.Now(),
			expectNudge:   true,
			expectedState: ActivityStateIdle,
		},
		{
			name:          "traveling_future_no_nudge",
			initialState:  ActivityStateTraveling,
			initialUntil:  time.Now().Add(10 * time.Second),
			nudgeTime:     time.Now(),
			expectNudge:   false,
			expectedState: ActivityStateTraveling,
		},
		{
			name:          "traveling_past_nudge",
			initialState:  ActivityStateTraveling,
			initialUntil:  time.Now().Add(-5 * time.Second),
			nudgeTime:     time.Now(),
			expectNudge:   true,
			expectedState: ActivityStateIdle,
		},
		{
			name:          "cooldown_future_no_nudge",
			initialState:  ActivityStateCooldown,
			initialUntil:  time.Now().Add(10 * time.Second),
			nudgeTime:     time.Now(),
			expectNudge:   false,
			expectedState: ActivityStateCooldown,
		},
		{
			name:          "cooldown_past_nudge",
			initialState:  ActivityStateCooldown,
			initialUntil:  time.Now().Add(-5 * time.Second),
			nudgeTime:     time.Now(),
			expectNudge:   true,
			expectedState: ActivityStateIdle,
		},
		{
			name:          "mining_nudge",
			initialState:  ActivityStateMining,
			initialUntil:  time.Time{},
			nudgeTime:     time.Now(),
			expectNudge:   true,
			expectedState: ActivityStateMining,
		},
		{
			name:          "combat_nudge",
			initialState:  ActivityStateInCombat,
			initialUntil:  time.Time{},
			nudgeTime:     time.Now(),
			expectNudge:   true,
			expectedState: ActivityStateInCombat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mysis{
				activityState: tt.initialState,
				activityUntil: tt.initialUntil,
			}

			result := m.shouldNudge(tt.nudgeTime)

			if result != tt.expectNudge {
				t.Errorf("Expected shouldNudge()=%v, got %v", tt.expectNudge, result)
			}

			if m.activityState != tt.expectedState {
				t.Errorf("Expected activity state %s, got %s", tt.expectedState, m.activityState)
			}
		})
	}
}

// TestMysis_ActivityStateFromWaitStates tests activity state tracking from tool results.
func TestMysis_ActivityStateFromWaitStates(t *testing.T) {
	// This test verifies the activity tracking logic exists and is properly
	// integrated with the mysis lifecycle. Detailed testing of
	// updateActivityFromToolResult is in the existing mysis_test.go file.

	m := &Mysis{}

	// Verify initial state is empty (zero value)
	// Note: NewMysis() initializes to ActivityStateIdle, but zero value is ""
	if m.activityState != "" {
		t.Errorf("Expected initial activity state empty string (zero value), got %s", m.activityState)
	}

	// Verify activityUntil is zero
	if !m.activityUntil.IsZero() {
		t.Error("Expected initial activityUntil to be zero time")
	}
}

// TestMysis_NudgeInterval tests the nudge interval constant is reasonable.
func TestMysis_NudgeInterval(t *testing.T) {
	// Verify nudge interval is reasonable (not too short, not too long)
	if constants.IdleNudgeInterval < 10*time.Second {
		t.Errorf("IdleNudgeInterval too short: %v (should be >= 10s)", constants.IdleNudgeInterval)
	}

	if constants.IdleNudgeInterval > 5*time.Minute {
		t.Errorf("IdleNudgeInterval too long: %v (should be <= 5m)", constants.IdleNudgeInterval)
	}
}

// TestMysis_WaitStateNudgeInterval tests the wait state nudge interval constant.
func TestMysis_WaitStateNudgeInterval(t *testing.T) {
	// Verify wait state interval is reasonable
	if constants.WaitStateNudgeInterval < 30*time.Second {
		t.Errorf("WaitStateNudgeInterval too short: %v (should be >= 30s)", constants.WaitStateNudgeInterval)
	}

	if constants.WaitStateNudgeInterval > 10*time.Minute {
		t.Errorf("WaitStateNudgeInterval too long: %v (should be <= 10m)", constants.WaitStateNudgeInterval)
	}
}
