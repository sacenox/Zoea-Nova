package core

import (
	"fmt"
	"testing"

	"github.com/xonecas/zoea-nova/internal/provider"
)

// TestAccountReleaseOnError verifies that accounts are properly released
// when a mysis transitions to errored state, allowing restart to succeed.
func TestAccountReleaseOnError(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	p := provider.NewMock("mock", "test response")

	// Create account for testing
	acct, err := s.CreateAccount("test_account", "password")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Create stored mysis
	stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Create mysis
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, p, s, bus, "")

	// Start mysis
	if err := mysis.Start(); err != nil {
		t.Fatalf("Failed to start mysis: %v", err)
	}

	// Simulate mysis acquiring an account (simulating login tool call)
	// First mark account in use, then set current account
	if err := s.MarkAccountInUse(acct.Username, stored.ID); err != nil {
		t.Fatalf("Failed to mark account in use: %v", err)
	}
	mysis.setCurrentAccount(acct.Username, "", "")

	// Verify account is acquired
	if mysis.CurrentAccountUsername() != "test_account" {
		t.Fatalf("Expected account to be acquired")
	}

	// Verify account is locked in store (ClaimAccount should not return this account)
	claimed, err := s.ClaimAccount(stored.ID)
	if err == nil && claimed.Username == acct.Username {
		t.Fatalf("Expected account to be locked, but ClaimAccount returned it")
	}

	// Trigger error state (simulating SendMessage failure)
	mysis.setError(fmt.Errorf("simulated error"))

	// Verify state transitioned to errored
	if mysis.State() != MysisStateErrored {
		t.Fatalf("Expected state to be MysisStateErrored, got %v", mysis.State())
	}

	// KEY TEST: Verify account was released
	if mysis.CurrentAccountUsername() != "" {
		t.Errorf("BUG CONFIRMED: Account not released on error. CurrentAccountUsername=%q", mysis.CurrentAccountUsername())
	}

	// Verify account is unlocked in store (ClaimAccount should return it)
	claimed2, err := s.ClaimAccount(stored.ID)
	if err != nil || claimed2 == nil || claimed2.Username != acct.Username {
		t.Errorf("BUG CONFIRMED: Account still locked in store, ClaimAccount failed: %v", err)
	} else {
		// Clean up acquired lock
		_ = s.ReleaseAccount(claimed2.Username)
	}

	// KEY TEST: Verify restart succeeds
	if err := mysis.Start(); err != nil {
		t.Errorf("RESTART FAILED: %v (likely due to account lock contention)", err)
	}

	// Clean up
	_ = mysis.Stop()
}
