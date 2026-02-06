package core

import (
	"testing"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestSendEphemeralMessage_NotPersisted verifies that ephemeral messages (nudges)
// are NOT stored in the database, but their responses ARE stored.
func TestSendEphemeralMessage_NotPersisted(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create stored mysis
	stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	p := provider.NewMock("mock", "I'm working on it!")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, p, s, bus)

	// Start the mysis
	if err := mysis.Start(); err != nil {
		t.Fatalf("Failed to start mysis: %v", err)
	}
	defer mysis.Stop()

	// Get initial memory count (should just be system prompt)
	initialCount, err := s.CountMemories(stored.ID)
	if err != nil {
		t.Fatalf("Failed to count memories: %v", err)
	}

	// Send an ephemeral message (like a nudge)
	ephemeralContent := "Continue your work."
	if err := mysis.SendEphemeralMessage(ephemeralContent, store.MemorySourceDirect); err != nil {
		t.Fatalf("Failed to send ephemeral message: %v", err)
	}

	// Get final memory count
	finalCount, err := s.CountMemories(stored.ID)
	if err != nil {
		t.Fatalf("Failed to count final memories: %v", err)
	}

	// Verify: Should have exactly 1 more memory (the assistant's response)
	// NOT 2 more (which would include the ephemeral message)
	expectedIncrease := 1
	actualIncrease := finalCount - initialCount

	if actualIncrease != expectedIncrease {
		t.Errorf("Memory count increased by %d, expected %d", actualIncrease, expectedIncrease)
		t.Logf("Initial count: %d, Final count: %d", initialCount, finalCount)

		// Debug: Show what was stored
		memories, _ := s.GetMemories(stored.ID)
		t.Logf("All memories:")
		for i, mem := range memories {
			t.Logf("  [%d] Role=%s Source=%s Content=%q", i, mem.Role, mem.Source, mem.Content)
		}
	}

	// Verify the ephemeral message is NOT in the database
	memories, err := s.GetMemories(stored.ID)
	if err != nil {
		t.Fatalf("Failed to get memories: %v", err)
	}

	for _, mem := range memories {
		if mem.Content == ephemeralContent {
			t.Errorf("Ephemeral message was persisted to database (role=%s, source=%s)", mem.Role, mem.Source)
		}
	}

	// Verify the assistant response IS in the database
	foundResponse := false
	for _, mem := range memories {
		if mem.Role == store.MemoryRoleAssistant && mem.Source == store.MemorySourceLLM {
			foundResponse = true
			if mem.Content != "I'm working on it!" {
				t.Errorf("Unexpected assistant response: got %q, want %q", mem.Content, "I'm working on it!")
			}
			break
		}
	}

	if !foundResponse {
		t.Errorf("Assistant response was NOT stored in database")
	}
}

// TestSendEphemeralMessage_VsSendMessage compares ephemeral vs normal message persistence.
func TestSendEphemeralMessage_VsSendMessage(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	p := provider.NewMock("mock", "Response")

	// Test ephemeral message
	stored1, err := s.CreateMysis("ephemeral-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}
	mysis1 := NewMysis(stored1.ID, stored1.Name, stored1.CreatedAt, p, s, bus)
	if err := mysis1.Start(); err != nil {
		t.Fatalf("Failed to start ephemeral mysis: %v", err)
	}
	defer mysis1.Stop()

	initialCount1, _ := s.CountMemories(stored1.ID)
	mysis1.SendEphemeralMessage("Test message", store.MemorySourceDirect)
	finalCount1, _ := s.CountMemories(stored1.ID)

	// Test normal message
	stored2, err := s.CreateMysis("normal-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}
	mysis2 := NewMysis(stored2.ID, stored2.Name, stored2.CreatedAt, p, s, bus)
	if err := mysis2.Start(); err != nil {
		t.Fatalf("Failed to start normal mysis: %v", err)
	}
	defer mysis2.Stop()

	initialCount2, _ := s.CountMemories(stored2.ID)
	mysis2.SendMessage("Test message", store.MemorySourceDirect)
	finalCount2, _ := s.CountMemories(stored2.ID)

	// Ephemeral: should add 1 memory (assistant response only)
	ephemeralIncrease := finalCount1 - initialCount1
	if ephemeralIncrease != 1 {
		t.Errorf("Ephemeral message added %d memories, expected 1", ephemeralIncrease)
	}

	// Normal: should add 2 memories (user message + assistant response)
	normalIncrease := finalCount2 - initialCount2
	if normalIncrease != 2 {
		t.Errorf("Normal message added %d memories, expected 2", normalIncrease)
	}

	// Verify the difference
	if ephemeralIncrease == normalIncrease {
		t.Errorf("Ephemeral and normal messages had same memory increase (%d), expected different", ephemeralIncrease)
	}
}
