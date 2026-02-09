package store

import (
	"path/filepath"
	"testing"
)

func setupStoreTest(t *testing.T) (*Store, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	return s, func() { s.Close() }
}

func TestOpenMemory(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	defer s.Close()

	// Verify tables exist by querying them
	_, err = s.db.Exec("SELECT 1 FROM myses LIMIT 1")
	if err != nil {
		t.Errorf("myses table not created: %v", err)
	}

	_, err = s.db.Exec("SELECT 1 FROM memories LIMIT 1")
	if err != nil {
		t.Errorf("memories table not created: %v", err)
	}
}

func TestMysisCRUD(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create
	mysis, err := s.CreateMysis("test-mysis", "ollama", "llama3", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}
	if mysis.ID == "" {
		t.Error("expected non-empty mysis ID")
	}
	if mysis.Name != "test-mysis" {
		t.Errorf("expected name=test-mysis, got %s", mysis.Name)
	}
	if mysis.State != MysisStateIdle {
		t.Errorf("expected state=idle, got %s", mysis.State)
	}

	// Get
	fetched, err := s.GetMysis(mysis.ID)
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if fetched.ID != mysis.ID {
		t.Errorf("expected ID=%s, got %s", mysis.ID, fetched.ID)
	}

	// List
	myses, err := s.ListMyses()
	if err != nil {
		t.Fatalf("ListMyses() error: %v", err)
	}
	if len(myses) != 1 {
		t.Errorf("expected 1 mysis, got %d", len(myses))
	}

	// Update state
	if err := s.UpdateMysisState(mysis.ID, MysisStateRunning); err != nil {
		t.Fatalf("UpdateMysisState() error: %v", err)
	}
	fetched, _ = s.GetMysis(mysis.ID)
	if fetched.State != MysisStateRunning {
		t.Errorf("expected state=running, got %s", fetched.State)
	}

	// Update config
	if err := s.UpdateMysisConfig(mysis.ID, "opencode_zen", "zen-model", 0.6); err != nil {
		t.Fatalf("UpdateMysisConfig() error: %v", err)
	}
	fetched, _ = s.GetMysis(mysis.ID)
	if fetched.Provider != "opencode_zen" {
		t.Errorf("expected provider=opencode_zen, got %s", fetched.Provider)
	}
	if fetched.Temperature != 0.6 {
		t.Errorf("expected temperature=0.6, got %v", fetched.Temperature)
	}

	// Count
	count, err := s.CountMyses()
	if err != nil {
		t.Fatalf("CountMyses() error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	// Delete
	if err := s.DeleteMysis(mysis.ID); err != nil {
		t.Fatalf("DeleteMysis() error: %v", err)
	}
	count, _ = s.CountMyses()
	if count != 0 {
		t.Errorf("expected count=0 after delete, got %d", count)
	}
}

func TestMemoryCRUD(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create mysis first
	mysis, err := s.CreateMysis("memory-test", "ollama", "llama3", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Add memories
	if err := s.AddMemory(mysis.ID, MemoryRoleSystem, MemorySourceSystem, "You are a helpful assistant.", "", ""); err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}

	if err := s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "Hello!", "", ""); err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}

	if err := s.AddMemory(mysis.ID, MemoryRoleAssistant, MemorySourceLLM, "Hi there!", "thinking...", ""); err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}

	// Get all memories
	memories, err := s.GetMemories(mysis.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories) != 3 {
		t.Errorf("expected 3 memories, got %d", len(memories))
	}
	if memories[0].ID == 0 {
		t.Error("expected non-zero memory ID")
	}
	if memories[2].Reasoning != "thinking..." {
		t.Errorf("expected reasoning=thinking..., got %q", memories[2].Reasoning)
	}

	// Verify order (chronological)
	if memories[0].Role != MemoryRoleSystem {
		t.Errorf("expected first memory role=system, got %s", memories[0].Role)
	}

	// Get recent memories
	recent, err := s.GetRecentMemories(mysis.ID, 2)
	if err != nil {
		t.Fatalf("GetRecentMemories() error: %v", err)
	}
	if len(recent) != 2 {
		t.Errorf("expected 2 recent memories, got %d", len(recent))
	}
	// Should be in chronological order (user, assistant)
	if recent[0].Role != MemoryRoleUser {
		t.Errorf("expected recent[0] role=user, got %s", recent[0].Role)
	}

	// Count
	count, err := s.CountMemories(mysis.ID)
	if err != nil {
		t.Fatalf("CountMemories() error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}

	// Delete memories
	if err := s.DeleteMemories(mysis.ID); err != nil {
		t.Fatalf("DeleteMemories() error: %v", err)
	}
	count, _ = s.CountMemories(mysis.ID)
	if count != 0 {
		t.Errorf("expected count=0 after delete, got %d", count)
	}

	// Verify IDs for coverage
	_ = memories[1].ID
	_ = memories[2].ID
}

func TestMemoryWithReasoning(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	mysis, err := s.CreateMysis("reasoning-test", "ollama", "llama3", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	reasoning := "Step by step thinking"
	err = s.AddMemory(mysis.ID, MemoryRoleAssistant, MemorySourceLLM, "", reasoning, "")
	if err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}

	memories, err := s.GetMemories(mysis.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}
	if memories[0].Reasoning != reasoning {
		t.Errorf("expected reasoning=%q, got %q", reasoning, memories[0].Reasoning)
	}
}

func TestCascadeDelete(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	mysis, _ := s.CreateMysis("cascade-test", "ollama", "llama3", 0.7)
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "test message", "", "")

	// Delete mysis should cascade to memories
	if err := s.DeleteMysis(mysis.ID); err != nil {
		t.Fatalf("DeleteMysis() error: %v", err)
	}

	// Memories should be gone
	memories, err := s.GetMemories(mysis.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories) != 0 {
		t.Errorf("expected 0 memories after cascade delete, got %d", len(memories))
	}
}

func TestUpdateNonExistentMysis(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	err := s.UpdateMysisState("nonexistent-id", MysisStateRunning)
	if err == nil {
		t.Error("expected error updating non-existent mysis")
	}

	err = s.UpdateMysisConfig("nonexistent-id", "ollama", "llama3", 0.7)
	if err == nil {
		t.Error("expected error updating non-existent mysis config")
	}

	err = s.DeleteMysis("nonexistent-id")
	if err == nil {
		t.Error("expected error deleting non-existent mysis")
	}
}

func TestAccountCRUD(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create account (automatically marked as in_use)
	acc, err := s.CreateAccount("crab_01", "pass123")
	if err != nil {
		t.Fatalf("CreateAccount() error: %v", err)
	}
	if !acc.InUse {
		t.Error("new account should be in use after registration")
	}

	// Release it to make it available
	if err := s.ReleaseAccount("crab_01"); err != nil {
		t.Fatalf("ReleaseAccount() error: %v", err)
	}

	// List available
	available, _ := s.ListAvailableAccounts()
	if len(available) != 1 {
		t.Errorf("expected 1 available, got %d", len(available))
	}

	// Claim (no arguments - returns first available and atomically marks as in_use)
	claimed, err := s.ClaimAccount()
	if err != nil {
		t.Fatalf("ClaimAccount() error: %v", err)
	}
	if claimed.Username != "crab_01" {
		t.Errorf("expected crab_01, got %s", claimed.Username)
	}
	if !claimed.InUse {
		t.Error("ClaimAccount() should atomically mark account as in_use")
	}

	// Verify marked as in_use (ClaimAccount does this atomically now)
	fetched, _ := s.GetAccount("crab_01")
	if !fetched.InUse {
		t.Error("account should be in use after ClaimAccount()")
	}

	// List available (should now be empty since account is claimed)
	available, _ = s.ListAvailableAccounts()
	if len(available) != 0 {
		t.Errorf("expected 0 available after claim, got %d", len(available))
	}

	// Verify marked as in_use
	fetched, _ = s.GetAccount("crab_01")
	if !fetched.InUse {
		t.Error("account should be in use after MarkAccountInUse()")
	}

	// Release
	if err := s.ReleaseAccount("crab_01"); err != nil {
		t.Fatalf("ReleaseAccount() error: %v", err)
	}

	// Verify released
	fetched, _ = s.GetAccount("crab_01")
	if fetched.InUse {
		t.Error("account should be released")
	}
}

func TestReleaseAllAccounts(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create and release two accounts to make them available
	s.CreateAccount("acc1", "pass1")
	s.CreateAccount("acc2", "pass2")
	s.ReleaseAccount("acc1")
	s.ReleaseAccount("acc2")

	// Mark both in use directly (simulating login handler behavior)
	s.MarkAccountInUse("acc1")
	s.MarkAccountInUse("acc2")

	// Verify both in use
	available, _ := s.ListAvailableAccounts()
	if len(available) != 0 {
		t.Errorf("expected 0 available, got %d", len(available))
	}

	// Release all
	if err := s.ReleaseAllAccounts(); err != nil {
		t.Fatalf("ReleaseAllAccounts() error: %v", err)
	}

	// Verify both released
	available, _ = s.ListAvailableAccounts()
	if len(available) != 2 {
		t.Errorf("expected 2 available, got %d", len(available))
	}
}

func TestClaimAccount_ReturnsAvailableAccount(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create and release an account
	s.CreateAccount("crab_01", "pass123")
	if err := s.ReleaseAccount("crab_01"); err != nil {
		t.Fatalf("ReleaseAccount() error: %v", err)
	}

	// ClaimAccount should return the available account
	claimed, err := s.ClaimAccount()
	if err != nil {
		t.Fatalf("ClaimAccount() error: %v", err)
	}
	if claimed.Username != "crab_01" {
		t.Errorf("expected username=crab_01, got %s", claimed.Username)
	}
	if claimed.Password != "pass123" {
		t.Errorf("expected password=pass123, got %s", claimed.Password)
	}
	if !claimed.InUse {
		t.Error("ClaimAccount() should return account with InUse=true (atomically claimed)")
	}
}

func TestClaimAccount_AtomicallyMarksInUse(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create and release an account
	s.CreateAccount("crab_01", "pass123")
	if err := s.ReleaseAccount("crab_01"); err != nil {
		t.Fatalf("ReleaseAccount() error: %v", err)
	}

	// Claim the account
	_, err := s.ClaimAccount()
	if err != nil {
		t.Fatalf("ClaimAccount() error: %v", err)
	}

	// Verify in_use flag is now 1 in database (atomically set)
	var inUse int
	err = s.db.QueryRow("SELECT in_use FROM accounts WHERE username = ?", "crab_01").Scan(&inUse)
	if err != nil {
		t.Fatalf("Query in_use error: %v", err)
	}
	if inUse != 1 {
		t.Errorf("expected in_use=1 after ClaimAccount(), got %d", inUse)
	}

	// Verify GetAccount also shows InUse=true
	fetched, err := s.GetAccount("crab_01")
	if err != nil {
		t.Fatalf("GetAccount() error: %v", err)
	}
	if !fetched.InUse {
		t.Error("account should be marked in use after ClaimAccount()")
	}
}

func TestClaimAccount_PreventsRaceCondition(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create two accounts
	s.CreateAccount("crab_01", "pass123")
	s.CreateAccount("crab_02", "pass456")
	if err := s.ReleaseAccount("crab_01"); err != nil {
		t.Fatalf("ReleaseAccount(crab_01) error: %v", err)
	}
	if err := s.ReleaseAccount("crab_02"); err != nil {
		t.Fatalf("ReleaseAccount(crab_02) error: %v", err)
	}

	// First claim should get crab_01 and lock it
	claimed1, err := s.ClaimAccount()
	if err != nil {
		t.Fatalf("First ClaimAccount() error: %v", err)
	}
	if claimed1.Username != "crab_01" {
		t.Errorf("First claim: expected crab_01, got %s", claimed1.Username)
	}

	// Second claim should get crab_02 (crab_01 is now locked)
	claimed2, err := s.ClaimAccount()
	if err != nil {
		t.Fatalf("Second ClaimAccount() error: %v", err)
	}
	if claimed2.Username != "crab_02" {
		t.Errorf("Second claim: expected crab_02, got %s", claimed2.Username)
	}

	// Third claim should fail (no more available accounts)
	_, err = s.ClaimAccount()
	if err == nil {
		t.Error("Third ClaimAccount() should fail when no accounts available")
	}
}

func TestClaimAccount_FailsWhenNoAccounts(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Try to claim when no accounts exist
	_, err := s.ClaimAccount()
	if err == nil {
		t.Fatal("expected error when no accounts exist")
	}
	if err.Error() != "no accounts available" {
		t.Errorf("expected 'no accounts available' error, got: %v", err)
	}
}

func TestClaimAccount_FailsWhenOnlyInUseAccounts(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create account (in_use=1 by default)
	s.CreateAccount("crab_01", "pass123")

	// Try to claim when only in-use accounts exist
	_, err := s.ClaimAccount()
	if err == nil {
		t.Fatal("expected error when only in-use accounts exist")
	}
	if err.Error() != "no accounts available" {
		t.Errorf("expected 'no accounts available' error, got: %v", err)
	}
}

// TestStoreDB verifies the DB getter returns the underlying sql.DB.
func TestStoreDB(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	db := s.DB()
	if db == nil {
		t.Fatal("DB() returned nil")
	}

	// Verify the DB is functional by querying it
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM myses").Scan(&count)
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	// Initially should have 0 myses
	if count != 0 {
		t.Errorf("expected 0 myses in new DB, got %d", count)
	}

	// Create a mysis and verify the DB reflects it
	_, err = s.CreateMysis("test-mysis", "mock", "mock-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM myses").Scan(&count)
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 mysis after CreateMysis, got %d", count)
	}
}
