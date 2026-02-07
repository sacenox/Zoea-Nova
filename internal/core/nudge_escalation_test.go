package core

import (
	"strings"
	"testing"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestBuildContinuePrompt_Level1Gentle tests the first nudge (gentle).
func TestBuildContinuePrompt_Level1Gentle(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("nudge-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	prompt := mysis.buildContinuePrompt(0)

	// Should contain gentle message
	if !strings.Contains(prompt, "What's your next move?") {
		t.Errorf("Expected gentle nudge message, got: %s", prompt)
	}

	// Should NOT contain firm or urgent messages
	if strings.Contains(prompt, "You need to respond") {
		t.Error("Level 1 should not contain firm message")
	}
	if strings.Contains(prompt, "URGENT") {
		t.Error("Level 1 should not contain urgent message")
	}

	// Should always contain the reminder
	if !strings.Contains(prompt, "Always call get_notifications") {
		t.Error("Missing get_notifications reminder")
	}
}

// TestBuildContinuePrompt_Level2Firm tests the second nudge (firmer).
func TestBuildContinuePrompt_Level2Firm(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("nudge-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	prompt := mysis.buildContinuePrompt(1)

	// Should contain firm message
	if !strings.Contains(prompt, "You need to respond") {
		t.Errorf("Expected firm nudge message, got: %s", prompt)
	}

	// Should NOT contain gentle or urgent messages
	if strings.Contains(prompt, "What's your next move?") {
		t.Error("Level 2 should not contain gentle message")
	}
	if strings.Contains(prompt, "URGENT") {
		t.Error("Level 2 should not contain urgent message")
	}

	// Should always contain the reminder
	if !strings.Contains(prompt, "Always call get_notifications") {
		t.Error("Missing get_notifications reminder")
	}
}

// TestBuildContinuePrompt_Level3Urgent tests the third+ nudge (urgent).
func TestBuildContinuePrompt_Level3Urgent(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("nudge-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Test attempt count 2
	prompt := mysis.buildContinuePrompt(2)
	if !strings.Contains(prompt, "URGENT") {
		t.Errorf("Expected urgent nudge message at attempt 2, got: %s", prompt)
	}

	// Test attempt count 3
	prompt = mysis.buildContinuePrompt(3)
	if !strings.Contains(prompt, "URGENT") {
		t.Errorf("Expected urgent nudge message at attempt 3, got: %s", prompt)
	}

	// Should NOT contain gentle or firm messages
	if strings.Contains(prompt, "What's your next move?") {
		t.Error("Level 3 should not contain gentle message")
	}
	if strings.Contains(prompt, "You need to respond") && !strings.Contains(prompt, "URGENT") {
		t.Error("Level 3 should not contain only firm message")
	}

	// Should always contain the reminder
	if !strings.Contains(prompt, "Always call get_notifications") {
		t.Error("Missing get_notifications reminder")
	}
}

// TestBuildContinuePrompt_WithDriftReminders tests escalation with drift reminders.
func TestBuildContinuePrompt_WithDriftReminders(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("nudge-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Add memory with real-time reference to trigger drift reminder
	s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "Waiting 5 minutes for travel.", "", "")

	// Test all three levels with drift reminders
	tests := []struct {
		attemptCount int
		name         string
		expectMsg    string
	}{
		{0, "Level 1 with drift", "What's your next move?"},
		{1, "Level 2 with drift", "You need to respond"},
		{2, "Level 3 with drift", "URGENT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := mysis.buildContinuePrompt(tt.attemptCount)

			// Check base message
			if !strings.Contains(prompt, tt.expectMsg) {
				t.Errorf("Expected message '%s', got: %s", tt.expectMsg, prompt)
			}

			// Check drift reminders section
			if !strings.Contains(prompt, "DRIFT REMINDERS") {
				t.Error("Expected DRIFT REMINDERS section")
			}
			if !strings.Contains(prompt, "real-world time") {
				t.Error("Expected real-world time drift reminder")
			}

			// Check get_notifications reminder
			if !strings.Contains(prompt, "Always call get_notifications") {
				t.Error("Missing get_notifications reminder")
			}
		})
	}
}

// TestNudgeRoleIsUser tests that nudges create user role messages (not system).
func TestNudgeRoleIsUser(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("nudge-role-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Simulate what happens in run() when a nudge is received
	// The code calls: a.SendMessage(a.buildContinuePrompt(attemptCount), store.MemorySourceDirect)
	prompt := mysis.buildContinuePrompt(0)

	// Idle myses now accept messages, so this should succeed
	err := mysis.SendMessage(prompt, store.MemorySourceDirect)
	if err != nil {
		t.Fatalf("SendMessage should succeed for idle mysis, got: %v", err)
	}

	// The important part: verify that MemorySourceDirect maps to user role
	// Check the stored memory to verify role
	memories, err := s.GetMemories(mysis.ID())
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}

	// Should have stored the nudge message (and possibly LLM response)
	if len(memories) == 0 {
		t.Fatal("Expected stored memory for nudge message")
	}

	// Find the user message (the nudge we sent)
	var userMemory *store.Memory
	for i := range memories {
		if memories[i].Role == store.MemoryRoleUser {
			userMemory = memories[i]
			break
		}
	}

	if userMemory == nil {
		t.Fatal("Expected to find user role memory (the nudge)")
	}

	// Verify the role is user (not system) and source is direct
	if userMemory.Role != store.MemoryRoleUser {
		t.Errorf("Expected nudge to use user role, got: %s", userMemory.Role)
	}
	if userMemory.Source != store.MemorySourceDirect {
		t.Errorf("Expected source to be direct, got: %s", userMemory.Source)
	}
}
