package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
	"golang.org/x/time/rate"
)

// TestInputReset_DirectMessage verifies that the input box resets after sending a direct message.
func TestInputReset_DirectMessage(t *testing.T) {
	t.Run("non_empty_message_resets_input", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis to message
		mysis, _ := m.commander.CreateMysis("test-mysis", "ollama-qwen")
		m.commander.StartMysis(mysis.ID())
		m.refreshMysisList()
		m.selectedIdx = 0

		// Press 'm' to open message input
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		m = newModel.(Model)

		// Verify input is active
		if !m.input.IsActive() {
			t.Fatal("input should be active after 'm' key")
		}
		if m.input.Mode() != InputModeMessage {
			t.Fatalf("input mode = %d, want InputModeMessage (%d)", m.input.Mode(), InputModeMessage)
		}

		// Type a message
		m.input.textInput.SetValue("hello world")

		// Verify input has value
		if m.input.Value() != "hello world" {
			t.Fatalf("input value = %q, want %q", m.input.Value(), "hello world")
		}

		// Press Enter to send
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newModel.(Model)

		// Verify input is reset immediately (before async completion)
		if m.input.Value() != "" {
			t.Errorf("input value = %q, want empty string (input should reset immediately after Enter)", m.input.Value())
		}
		if m.input.Mode() != InputModeNone {
			t.Errorf("input mode = %d, want InputModeNone (%d)", m.input.Mode(), InputModeNone)
		}
		if m.input.IsActive() {
			t.Error("input should not be active after Enter (showing 'Sending...' indicator)")
		}

		// With SendMessageAsync, m.sending is NOT set (message sent immediately)
		// Network indicator shows activity, but input is ready for next message
		if m.sending {
			t.Error("sending flag should be false (SendMessageAsync returns immediately)")
		}

		// Verify async command was returned
		if cmd == nil {
			t.Fatal("cmd should not be nil (async send should be scheduled)")
		}

		// Execute the async command to get the result
		result := cmd()
		if result == nil {
			t.Fatal("async command should return a result")
		}

		// Process the result
		newModel, _ = m.Update(result)
		m = newModel.(Model)

		// Verify input is still reset after async completion
		if m.input.Value() != "" {
			t.Errorf("input value = %q, want empty string (input should remain reset after async completion)", m.input.Value())
		}

		// Verify sending state is cleared
		if m.sending {
			t.Error("sending flag should be false after async completion")
		}
	})

	t.Run("empty_message_sends_and_shows_indicator", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis to message
		mysis, _ := m.commander.CreateMysis("test-mysis", "ollama-qwen")
		m.commander.StartMysis(mysis.ID())
		m.refreshMysisList()
		m.selectedIdx = 0

		// Press 'm' to open message input
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		m = newModel.(Model)

		// Verify input is active
		if !m.input.IsActive() {
			t.Fatal("input should be active after 'm' key")
		}

		// Press Enter without typing (empty message)
		// NOTE: Direct messages DO NOT validate for empty - this differs from broadcasts
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newModel.(Model)

		// Verify input is reset and showing "Sending..." indicator
		if m.input.Value() != "" {
			t.Errorf("input value = %q, want empty string", m.input.Value())
		}
		if m.input.Mode() != InputModeNone {
			t.Errorf("input mode = %d, want InputModeNone (%d)", m.input.Mode(), InputModeNone)
		}
		if m.input.IsActive() {
			t.Error("input should not be active (showing 'Sending...' indicator)")
		}

		// Direct messages are sent even when empty (app.go:650-658, no empty check)
		if cmd == nil {
			t.Error("cmd should not be nil (direct messages are sent even when empty)")
		}
		// With SendMessageAsync, m.sending is NOT set
		if m.sending {
			t.Error("sending flag should be false (SendMessageAsync returns immediately)")
		}
	})
}

// TestInputReset_Broadcast verifies that the input box resets after broadcasting.
func TestInputReset_Broadcast(t *testing.T) {
	t.Run("non_empty_broadcast_resets_input", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create myses to broadcast to
		m.commander.CreateMysis("mysis-1", "ollama-qwen")
		m.commander.CreateMysis("mysis-2", "ollama-qwen")
		m.refreshMysisList()

		// Press 'b' to open broadcast input
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
		m = newModel.(Model)

		// Verify input is active
		if !m.input.IsActive() {
			t.Fatal("input should be active after 'b' key")
		}
		if m.input.Mode() != InputModeBroadcast {
			t.Fatalf("input mode = %d, want InputModeBroadcast (%d)", m.input.Mode(), InputModeBroadcast)
		}

		// Type a message
		m.input.textInput.SetValue("broadcast message")

		// Press Enter to send
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newModel.(Model)

		// Verify input is reset immediately
		if m.input.Value() != "" {
			t.Errorf("input value = %q, want empty string (input should reset immediately after Enter)", m.input.Value())
		}
		if m.input.Mode() != InputModeNone {
			t.Errorf("input mode = %d, want InputModeNone (%d)", m.input.Mode(), InputModeNone)
		}
		if m.input.IsActive() {
			t.Error("input should not be active after Enter (showing 'Sending...' indicator)")
		}

		// Verify sending state is set
		if !m.sending {
			t.Error("sending flag should be true after Enter")
		}
		if m.sendingMode != InputModeBroadcast {
			t.Errorf("sendingMode = %d, want InputModeBroadcast (%d)", m.sendingMode, InputModeBroadcast)
		}

		// Verify async command was returned
		if cmd == nil {
			t.Fatal("cmd should not be nil (async broadcast should be scheduled)")
		}
	})

	t.Run("empty_broadcast_resets_without_sending", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Press 'b' to open broadcast input
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
		m = newModel.(Model)

		// Press Enter without typing (empty broadcast)
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newModel.(Model)

		// Verify input is reset
		if m.input.Value() != "" {
			t.Errorf("input value = %q, want empty string", m.input.Value())
		}
		if m.input.Mode() != InputModeNone {
			t.Errorf("input mode = %d, want InputModeNone (%d)", m.input.Mode(), InputModeNone)
		}
		if m.input.IsActive() {
			t.Error("input should not be active after empty broadcast")
		}

		// Empty broadcasts should NOT trigger sending (app.go:629-632)
		if cmd != nil {
			t.Error("cmd should be nil (empty broadcasts should not be sent)")
		}
		if m.sending {
			t.Error("sending flag should be false (empty broadcasts should not trigger sending)")
		}
	})
}

// TestInputReset_Integration verifies input reset behavior in integration scenarios.
func TestInputReset_Integration(t *testing.T) {
	t.Run("send_multiple_messages_sequentially", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis
		mysis, _ := m.commander.CreateMysis("test-mysis", "ollama-qwen")
		m.commander.StartMysis(mysis.ID())
		m.refreshMysisList()
		m.selectedIdx = 0

		// Send first message
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		m = newModel.(Model)
		m.input.textInput.SetValue("message 1")
		newModel, cmd1 := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newModel.(Model)

		// Verify input is reset after first message
		if m.input.Value() != "" {
			t.Errorf("input value = %q after first message, want empty", m.input.Value())
		}
		if m.input.Mode() != InputModeNone {
			t.Errorf("input mode = %d after first message, want InputModeNone (%d)", m.input.Mode(), InputModeNone)
		}
		if m.input.IsActive() {
			t.Error("input should not be active after first message (showing 'Sending...' indicator)")
		}

		// Complete first async send
		if cmd1 != nil {
			result1 := cmd1()
			newModel, _ = m.Update(result1)
			m = newModel.(Model)
		}

		// Wait a bit for async completion
		time.Sleep(10 * time.Millisecond)

		// Send second message
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		m = newModel.(Model)
		m.input.textInput.SetValue("message 2")
		newModel, cmd2 := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newModel.(Model)

		// Verify input is reset after second message
		if m.input.Value() != "" {
			t.Errorf("input value = %q after second message, want empty", m.input.Value())
		}
		if m.input.Mode() != InputModeNone {
			t.Errorf("input mode = %d after second message, want InputModeNone (%d)", m.input.Mode(), InputModeNone)
		}
		if m.input.IsActive() {
			t.Error("input should not be active after second message (showing 'Sending...' indicator)")
		}

		// Complete second async send
		if cmd2 != nil {
			result2 := cmd2()
			newModel, _ = m.Update(result2)
			m = newModel.(Model)
		}

		// Verify final state
		if m.input.Value() != "" {
			t.Errorf("final input value = %q, want empty", m.input.Value())
		}
		if m.sending {
			t.Error("sending flag should be false after all messages sent")
		}
	})
}

// setupTestModelWithSlowProvider creates a test model with a slow mock provider.
// The slow provider simulates a long LLM response time (5 seconds).
func setupTestModelWithSlowProvider(t *testing.T) (Model, func()) {
	t.Helper()

	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}

	bus := core.NewEventBus(100)
	eventCh := bus.Subscribe()

	reg := provider.NewRegistry()
	limiter := rate.NewLimiter(rate.Limit(1000), 1000)

	// Create a mock factory that returns a provider with a 5-second delay
	slowFactory := &slowMockFactory{
		name:     "ollama-qwen",
		response: "mock response",
		limiter:  limiter,
		delay:    5 * time.Second,
	}
	reg.RegisterFactory("ollama-qwen", slowFactory)
	reg.RegisterFactory("zen-nano", provider.NewMockFactoryWithLimiter("zen-nano", "mock response", limiter))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses:        16,
			DefaultProvider: "ollama-qwen",
		},
		Providers: map[string]config.ProviderConfig{
			"ollama-qwen": {Endpoint: "http://mock", Model: "qwen3:8b", Temperature: 0.7, RateLimit: 1000, RateBurst: 1000},
			"zen-nano":    {Endpoint: "http://mock", Model: "gpt-5-nano", Temperature: 0.7, RateLimit: 1000, RateBurst: 1000},
		},
	}

	commander := core.NewCommander(s, reg, bus, cfg)

	model := New(commander, s, eventCh, false, cfg)
	model.width = 80
	model.height = 24

	// Set fixed test time for deterministic timestamps
	fixedTime := testTime()
	model.testTime = &fixedTime

	cleanup := func() {
		commander.StopAll()
		bus.Close()
		s.Close()
	}

	return model, cleanup
}

// slowMockFactory creates mock providers with a configurable delay.
type slowMockFactory struct {
	name     string
	response string
	limiter  *rate.Limiter
	delay    time.Duration
}

func (f *slowMockFactory) Name() string { return f.name }

func (f *slowMockFactory) Create(model string, temperature float64) provider.Provider {
	p := provider.NewMock(f.name, f.response)
	p.WithLimiter(f.limiter)
	p.SetDelay(f.delay)
	return p
}

// TestInputReset_SendingClearsImmediately verifies that m.sending is cleared
// immediately when Enter is pressed, not after the LLM finishes responding.
// This test demonstrates the bug where m.sending remains true until the async
// operation completes, preventing users from sending multiple messages quickly.
func TestInputReset_SendingClearsImmediately(t *testing.T) {
	m, cleanup := setupTestModelWithSlowProvider(t)
	defer cleanup()

	// Create a mysis with the slow mock provider (5 second delay)
	mysis, _ := m.commander.CreateMysis("slow-mysis", "ollama-qwen")
	m.commander.StartMysis(mysis.ID())
	m.refreshMysisList()
	m.selectedIdx = 0

	// Press 'm' to open message input
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = newModel.(Model)

	// Type a message
	m.input.textInput.SetValue("test message")

	// Press Enter to send
	startTime := time.Now()
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)
	elapsedAfterEnter := time.Since(startTime)

	// BUG: m.sending should be FALSE immediately after Enter,
	// but currently it's TRUE until the async operation completes
	// (which takes 5 seconds due to the slow provider)
	if m.sending {
		t.Errorf("BUG DETECTED: m.sending = true after Enter (elapsed: %v), want false immediately", elapsedAfterEnter)
		t.Logf("This demonstrates the bug: m.sending should be cleared immediately after Enter, not after LLM completes")
		t.Logf("Expected behavior: User can press 'm' again immediately after sending")
		t.Logf("Actual behavior: User must wait %v for LLM to finish before m.sending clears", 5*time.Second)
	}

	// Verify that the elapsed time is small (< 100ms), proving that
	// we're checking immediately after Enter, not after the 5-second delay
	if elapsedAfterEnter > 100*time.Millisecond {
		t.Fatalf("Test error: elapsed time %v is too large, expected < 100ms", elapsedAfterEnter)
	}

	// Verify async command was returned (this is correct behavior)
	if cmd == nil {
		t.Fatal("cmd should not be nil (async send should be scheduled)")
	}

	// Now execute the async command (with SendMessageAsync, this returns IMMEDIATELY)
	asyncStartTime := time.Now()
	result := cmd()
	asyncElapsed := time.Since(asyncStartTime)

	// With SendMessageAsync, the command returns IMMEDIATELY (not after LLM completes)
	// This is the fix - the async operation should be fast (< 100ms)
	if asyncElapsed > 100*time.Millisecond {
		t.Errorf("async operation took %v, expected < 100ms (SendMessageAsync returns immediately)", asyncElapsed)
	}

	// Process the async result
	newModel, _ = m.Update(result)
	m = newModel.(Model)

	// m.sending should still be false (was never set with async approach)
	if m.sending {
		t.Error("m.sending should be false (never set with SendMessageAsync)")
	}

	// The LLM processing happens in the background (via goroutine in SendMessageAsync)
	// The TUI doesn't wait for it, so the user can immediately send another message
	t.Log("SUCCESS: Input cleared immediately, LLM processing happens in background")
}
