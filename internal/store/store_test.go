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
	mysis, err := s.CreateMysis("test-mysis", "ollama", "llama3")
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
	if err := s.UpdateMysisConfig(mysis.ID, "opencode_zen", "zen-model"); err != nil {
		t.Fatalf("UpdateMysisConfig() error: %v", err)
	}
	fetched, _ = s.GetMysis(mysis.ID)
	if fetched.Provider != "opencode_zen" {
		t.Errorf("expected provider=opencode_zen, got %s", fetched.Provider)
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
	mysis, err := s.CreateMysis("memory-test", "ollama", "llama3")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Add memories
	m1, err := s.AddMemory(mysis.ID, MemoryRoleSystem, MemorySourceSystem, "You are a helpful assistant.")
	if err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}
	if m1.ID == 0 {
		t.Error("expected non-zero memory ID")
	}

	m2, err := s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "Hello!")
	if err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}

	m3, err := s.AddMemory(mysis.ID, MemoryRoleAssistant, MemorySourceLLM, "Hi there!")
	if err != nil {
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
	_ = m2.ID
	_ = m3.ID
}

func TestCascadeDelete(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	mysis, _ := s.CreateMysis("cascade-test", "ollama", "llama3")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "test message")

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

	err = s.UpdateMysisConfig("nonexistent-id", "ollama", "llama3")
	if err == nil {
		t.Error("expected error updating non-existent mysis config")
	}

	err = s.DeleteMysis("nonexistent-id")
	if err == nil {
		t.Error("expected error deleting non-existent mysis")
	}
}
