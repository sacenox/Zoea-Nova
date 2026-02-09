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

	mysis, _ := s.CreateMysis("test", "mock", "model", 0.7)

	// No system memory yet
	_, err := s.GetSystemMemory(mysis.ID)
	if err == nil {
		t.Error("expected error getting non-existent system memory")
	}

	// Add system memory
	expected := "You are a test."
	s.AddMemory(mysis.ID, MemoryRoleSystem, MemorySourceSystem, expected, "", "")

	// Add other memories
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "hello", "", "")

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

	mysis, _ := s.CreateMysis("test", "mock", "model", 0.7)

	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "I like apples", "", "")
	s.AddMemory(mysis.ID, MemoryRoleAssistant, MemorySourceLLM, "Apples are great", "", "")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "What about oranges?", "", "")

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

func TestSearchReasoning(t *testing.T) {
	s, cleanup := setupMemoriesTest(t)
	defer cleanup()

	mysis, _ := s.CreateMysis("test", "mock", "model", 0.7)

	s.AddMemory(mysis.ID, MemoryRoleAssistant, MemorySourceLLM, "", "I am thinking about mining", "")
	s.AddMemory(mysis.ID, MemoryRoleAssistant, MemorySourceLLM, "", "Trading strategy", "")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "hello", "", "")

	results, err := s.SearchReasoning(mysis.ID, "mining", 10)
	if err != nil {
		t.Fatalf("SearchReasoning() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGetRecentBroadcasts(t *testing.T) {
	s, cleanup := setupMemoriesTest(t)
	defer cleanup()

	mysis, _ := s.CreateMysis("test", "mock", "model", 0.7)

	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceBroadcast, "B1", "", "")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceBroadcast, "B2", "", "")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceBroadcast, "B3", "", "")
	s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceDirect, "D1", "", "")

	results, err := s.GetRecentBroadcasts(2)
	if err != nil {
		t.Fatalf("GetRecentBroadcasts() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	for _, result := range results {
		if result.CreatedAt.IsZero() {
			t.Fatal("expected broadcast CreatedAt to be parsed")
		}
	}
	// Should be B2, B3 (most recent)
	if results[0].Content != "B2" {
		t.Errorf("expected B2, got %s", results[0].Content)
	}
	if results[1].Content != "B3" {
		t.Errorf("expected B3, got %s", results[1].Content)
	}
}

func TestMemoryWithSenderID(t *testing.T) {
	s, cleanup := setupMemoriesTest(t)
	defer cleanup()

	mysis, _ := s.CreateMysis("test", "mock", "model", 0.7)
	senderID := "sender-mysis"

	err := s.AddMemory(mysis.ID, MemoryRoleUser, MemorySourceBroadcast, "test broadcast", "", senderID)
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}

	memories, err := s.GetRecentMemories(mysis.ID, 10)
	if err != nil {
		t.Fatalf("GetRecentMemories failed: %v", err)
	}

	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}

	if memories[0].SenderID != senderID {
		t.Errorf("expected sender_id %q, got %q", senderID, memories[0].SenderID)
	}
}
