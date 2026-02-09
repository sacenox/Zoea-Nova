package core

import (
	"path/filepath"
	"testing"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// setupAccountTest creates a test store with accounts for testing.
func setupAccountTest(t *testing.T) (*store.Store, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	// Add test accounts
	accounts := []struct {
		username string
		password string
	}{
		{"user1", "pass1"},
		{"user2", "pass2"},
		{"user3", "pass3"},
	}

	for _, acc := range accounts {
		if _, err := s.CreateAccount(acc.username, acc.password); err != nil {
			t.Fatalf("CreateAccount(%s) error: %v", acc.username, err)
		}
	}

	cleanup := func() {
		s.Close()
	}

	return s, cleanup
}

func createTestMysis(t *testing.T, s *store.Store, name string) *store.Mysis {
	t.Helper()
	mysis, err := s.CreateMysis(name, "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}
	return mysis
}

// TestMysis_AccountLifecycle tests the full account claim/release lifecycle.
func TestMysis_AccountLifecycle(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	mysis := createTestMysis(t, s, "test-mysis")

	m := &Mysis{
		id:    mysis.ID,
		name:  "test",
		store: s,
	}

	// Initially no account
	if m.CurrentAccountUsername() != "" {
		t.Errorf("Expected no account initially, got %s", m.CurrentAccountUsername())
	}

	// Claim account
	username := "user1"
	m.setCurrentAccount(username, "", "")

	if m.CurrentAccountUsername() != username {
		t.Errorf("Expected username %s, got %s", username, m.CurrentAccountUsername())
	}

	// Verify account marked as in use in store
	acc, err := s.GetAccount(username)
	if err != nil {
		t.Fatalf("GetAccount() error: %v", err)
	}
	if !acc.InUse {
		t.Error("Expected account marked as in use")
	}

	// Release account
	m.releaseCurrentAccount()

	if m.CurrentAccountUsername() != "" {
		t.Errorf("Expected no account after release, got %s", m.CurrentAccountUsername())
	}

	// Verify account marked as available in store
	acc, err = s.GetAccount(username)
	if err != nil {
		t.Fatalf("GetAccount() error after release: %v", err)
	}
	if acc.InUse {
		t.Error("Expected account marked as available after release")
	}
}

// TestMysis_AccountSwitching tests switching between accounts.
func TestMysis_AccountSwitching(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	mysis := createTestMysis(t, s, "test-mysis")

	m := &Mysis{
		id:    mysis.ID,
		name:  "test",
		store: s,
	}

	// Set first account
	m.setCurrentAccount("user1", "", "")

	if m.CurrentAccountUsername() != "user1" {
		t.Errorf("Expected user1, got %s", m.CurrentAccountUsername())
	}

	// Verify first account in use
	acc1, err := s.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount(user1) error: %v", err)
	}
	if !acc1.InUse {
		t.Error("Expected user1 marked as in use")
	}

	// Switch to second account
	m.setCurrentAccount("user2", "", "")

	if m.CurrentAccountUsername() != "user2" {
		t.Errorf("Expected user2 after switch, got %s", m.CurrentAccountUsername())
	}

	// Verify first account released
	acc1, err = s.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount(user1) error after switch: %v", err)
	}
	if acc1.InUse {
		t.Error("Expected user1 released after switch")
	}

	// Verify second account in use
	acc2, err := s.GetAccount("user2")
	if err != nil {
		t.Fatalf("GetAccount(user2) error: %v", err)
	}
	if !acc2.InUse {
		t.Error("Expected user2 marked as in use")
	}
}

// TestMysis_AccountSwitchToSame tests switching to the same account (no-op).
func TestMysis_AccountSwitchToSame(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	mysis := createTestMysis(t, s, "test-mysis")

	m := &Mysis{
		id:    mysis.ID,
		name:  "test",
		store: s,
	}

	// Set account
	m.setCurrentAccount("user1", "", "")

	// Switch to same account
	m.setCurrentAccount("user1", "", "")

	if m.CurrentAccountUsername() != "user1" {
		t.Errorf("Expected user1, got %s", m.CurrentAccountUsername())
	}

	// Verify account still in use (only once)
	acc, err := s.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount() error: %v", err)
	}
	if !acc.InUse {
		t.Error("Expected user1 still marked as in use")
	}
}

// TestMysis_SetCurrentAccount_EmptyString tests setting empty username.
func TestMysis_SetCurrentAccount_EmptyString(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	mysis := createTestMysis(t, s, "test-mysis")

	m := &Mysis{
		id:                     mysis.ID,
		name:                   "test",
		store:                  s,
		currentAccountUsername: "user1",
	}

	// Set to empty string (should be no-op per code)
	m.setCurrentAccount("", "", "")

	// Verify username unchanged (empty string is ignored)
	if m.CurrentAccountUsername() != "user1" {
		t.Errorf("Expected user1 unchanged, got %s", m.CurrentAccountUsername())
	}
}

// TestMysis_ReleaseCurrentAccount_NoAccount tests releasing when no account is set.
func TestMysis_ReleaseCurrentAccount_NoAccount(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	mysis := createTestMysis(t, s, "test-mysis")

	m := &Mysis{
		id:    mysis.ID,
		name:  "test",
		store: s,
	}

	// Release when no account (should be no-op)
	m.releaseCurrentAccount()

	if m.CurrentAccountUsername() != "" {
		t.Errorf("Expected no account, got %s", m.CurrentAccountUsername())
	}
}

// TestMysis_ReleaseCurrentAccount_Twice tests releasing same account twice.
func TestMysis_ReleaseCurrentAccount_Twice(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	mysis := createTestMysis(t, s, "test-mysis")

	m := &Mysis{
		id:    mysis.ID,
		name:  "test",
		store: s,
	}

	m.setCurrentAccount("user1", "", "")
	m.releaseCurrentAccount()

	// Release again (should be no-op)
	m.releaseCurrentAccount()

	if m.CurrentAccountUsername() != "" {
		t.Errorf("Expected no account after double release, got %s", m.CurrentAccountUsername())
	}

	// Verify account still available
	acc, err := s.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount() error: %v", err)
	}
	if acc.InUse {
		t.Error("Expected user1 available after release")
	}
}

// TestMysis_AccountConcurrent tests thread safety of account operations.
func TestMysis_AccountConcurrent(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	mysis := createTestMysis(t, s, "test-mysis")

	m := &Mysis{
		id:    mysis.ID,
		name:  "test",
		store: s,
	}

	done := make(chan bool)

	// Run multiple goroutines switching accounts
	accounts := []string{"user1", "user2", "user3"}
	for i := 0; i < 10; i++ {
		go func(idx int) {
			username := accounts[idx%len(accounts)]
			m.setCurrentAccount(username, "", "")
			_ = m.CurrentAccountUsername()
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// No assertions needed - just verify no data races (run with -race)
}

// TestMysis_Stop_ReleasesAccount tests that Stop() releases the current account.
func TestMysis_Stop_ReleasesAccount(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	// Create stored mysis
	stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	bus := NewEventBus(100)
	defer bus.Close()

	// Use mock provider
	mock := provider.NewMock("mock", "test response")
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Start the mysis so it enters Running state
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Set account after starting
	m.setCurrentAccount("user1", "", "")

	// Verify account in use
	acc, err := s.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount() error: %v", err)
	}
	if !acc.InUse {
		t.Error("Expected user1 marked as in use before stop")
	}

	// Stop mysis (should release account)
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// Verify account released
	acc, err = s.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount() error after stop: %v", err)
	}
	if acc.InUse {
		t.Error("Expected user1 released after stop")
	}

	// Verify mysis has no account
	if m.CurrentAccountUsername() != "" {
		t.Errorf("Expected no account after stop, got %s", m.CurrentAccountUsername())
	}
}

// TestMysis_CurrentAccountUsername_ThreadSafe tests thread safety of account getter.
func TestMysis_CurrentAccountUsername_ThreadSafe(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	mysis := createTestMysis(t, s, "test-mysis")

	m := &Mysis{
		id:                     mysis.ID,
		name:                   "test",
		store:                  s,
		currentAccountUsername: "user1",
	}

	done := make(chan bool)

	// Run multiple goroutines reading account
	for i := 0; i < 10; i++ {
		go func() {
			_ = m.CurrentAccountUsername()
			done <- true
		}()
	}

	// Run multiple goroutines writing account
	for i := 0; i < 10; i++ {
		go func(idx int) {
			accounts := []string{"user1", "user2", "user3"}
			m.setCurrentAccount(accounts[idx%len(accounts)], "", "")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// No assertions needed - just verify no data races (run with -race)
}

// TestMysis_MultipleMyses_SeparateAccounts tests multiple myses with different accounts.
func TestMysis_MultipleMyses_SeparateAccounts(t *testing.T) {
	s, cleanup := setupAccountTest(t)
	defer cleanup()

	m1 := &Mysis{
		id:    "mysis-1",
		name:  "mysis1",
		store: s,
	}

	m2 := &Mysis{
		id:    "mysis-2",
		name:  "mysis2",
		store: s,
	}

	// Each mysis claims different account
	m1.setCurrentAccount("user1", "", "")
	m2.setCurrentAccount("user2", "", "")

	if m1.CurrentAccountUsername() != "user1" {
		t.Errorf("Expected mysis1 to have user1, got %s", m1.CurrentAccountUsername())
	}

	if m2.CurrentAccountUsername() != "user2" {
		t.Errorf("Expected mysis2 to have user2, got %s", m2.CurrentAccountUsername())
	}

	// Verify both accounts in use
	acc1, err := s.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount(user1) error: %v", err)
	}
	if !acc1.InUse {
		t.Error("Expected user1 in use by mysis1")
	}

	acc2, err := s.GetAccount("user2")
	if err != nil {
		t.Fatalf("GetAccount(user2) error: %v", err)
	}
	if !acc2.InUse {
		t.Error("Expected user2 in use by mysis2")
	}

	// Release mysis1 account
	m1.releaseCurrentAccount()

	// Verify only user1 released
	acc1, err = s.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount(user1) error after release: %v", err)
	}
	if acc1.InUse {
		t.Error("Expected user1 released")
	}

	acc2, err = s.GetAccount("user2")
	if err != nil {
		t.Fatalf("GetAccount(user2) error after release: %v", err)
	}
	if !acc2.InUse {
		t.Error("Expected user2 still in use by mysis2")
	}
}
