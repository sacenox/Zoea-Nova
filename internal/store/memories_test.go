package store

import (
	"path/filepath"
	"testing"
)

func setupMemoriesTest(t *testing.T) (*Store, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	return s, func() { s.Close() }
}

func TestGetSystemMemory(t *testing.T) {
	s, cleanup := setupMemoriesTest(t)
	defer cleanup()

	mysis, _ := s.CreateMysis("test", "mock", "model")

	// No system memory yet
	_, err := s.GetSystemMemory(mysis.ID)
	if err == nil {
		t.Error("expected error getting non-existent system memory")
	}

	// Add system memory
	expected := "You are a test."
	s.AddMemory(mysis.ID, MemoryRoleSystem, MemorySourceSystem, expected)

	// Add other memories
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "hello")

	system, err := s.GetSystemMemory(mysis.ID)
	if err != nil {
		t.Fatalf("GetSystemMemory() error: %v", err)
	}
	if system.Content != expected {
		t.Errorf("expected content %q, got %q", expected, system.Content)
	}
}

func TestSearchMemories(t *testing.T) {
	s, cleanup := setupMemoriesTest(t)
	defer cleanup()

	mysis, _ := s.CreateMysis("test", "mock", "model")

	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "I like apples")
	s.AddMemory(mysis.ID, MemoryRoleAssistant, MemorySourceLLM, "Apples are great")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "What about oranges?")

	// Search for "apples"
	results, err := s.SearchMemories(mysis.ID, "apples", 10)
	if err != nil {
		t.Fatalf("SearchMemories() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Search for "oranges"
	results, err = s.SearchMemories(mysis.ID, "oranges", 10)
	if err != nil {
		t.Fatalf("SearchMemories() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchBroadcasts(t *testing.T) {
	s, cleanup := setupMemoriesTest(t)
	defer cleanup()

	mysis1, _ := s.CreateMysis("mysis1", "mock", "model")
	mysis2, _ := s.CreateMysis("mysis2", "mock", "model")

	s.AddMemory(mysis1.ID, MemoryRoleUser, MemorySourceBroadcast, "Broadcast message 1")
	s.AddMemory(mysis2.ID, MemoryRoleUser, MemorySourceBroadcast, "Broadcast message 2")
	s.AddMemory(mysis1.ID, MemoryRoleUser, MemorySourceDirect, "Direct message")

	// Search for "Broadcast"
	results, err := s.SearchBroadcasts("Broadcast", 10)
	if err != nil {
		t.Fatalf("SearchBroadcasts() error: %v", err)
	}
	// Should find both broadcast messages
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Search for "Direct"
	results, err = s.SearchBroadcasts("Direct", 10)
	if err != nil {
		t.Fatalf("SearchBroadcasts() error: %v", err)
	}
	// Should NOT find direct message
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestGetRecentBroadcasts(t *testing.T) {
	s, cleanup := setupMemoriesTest(t)
	defer cleanup()

	mysis, _ := s.CreateMysis("test", "mock", "model")

	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceBroadcast, "B1")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceBroadcast, "B2")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceBroadcast, "B3")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "D1")

	results, err := s.GetRecentBroadcasts(2)
	if err != nil {
		t.Fatalf("GetRecentBroadcasts() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	// Should be B2, B3 (most recent)
	if results[0].Content != "B2" {
		t.Errorf("expected B2, got %s", results[0].Content)
	}
	if results[1].Content != "B3" {
		t.Errorf("expected B3, got %s", results[1].Content)
	}
}
