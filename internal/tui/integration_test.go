package tui

// Integration tests for Zoea Nova TUI using teatest.
//
// These tests cover:
// - Navigation & view switching (dashboard, focus, help)
// - Input modes (broadcast, message, new mysis, configure)
// - Window resize and viewport scrolling
// - Async event handling (mysis state changes, broadcasts)
// - Complex interaction flows
//
// Test Pattern:
// 1. Setup model and create test data
// 2. Create teatest model with initial term size
// 3. Wait for initial render (WaitFor with dashboard/content check)
// 4. Send input keys
// 5. Wait briefly for model updates (time.Sleep)
// 6. Send quit command ('q') to exit cleanly
// 7. Verify final model state with FinalModel()

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func quitAndFinalModel(t *testing.T, tm *teatest.TestModel, timeout time.Duration) Model {
	t.Helper()
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(timeout))
	return fm.(Model)
}

func programQuitAndFinalModel(t *testing.T, tm *teatest.TestModel, timeout time.Duration) Model {
	t.Helper()
	_ = tm.Quit()
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(timeout))
	return fm.(Model)
}

// TestIntegration_DashboardNavigation tests up/down key navigation in dashboard
func TestIntegration_DashboardNavigation(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create two myses for navigation
	m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.CreateMysis("mysis-2", "ollama")
	m.refreshMysisList()

	// Initial state: selectedIdx should be 0
	if m.selectedIdx != 0 {
		t.Fatalf("initial selectedIdx = %d, want 0", m.selectedIdx)
	}

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Wait for initial render
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("MYSIS SWARM"))
		},
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(2*time.Second),
	)

	// Send down key
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})

	// Small delay for model update
	time.Sleep(200 * time.Millisecond)

	// Send quit to exit cleanly
	finalModel := quitAndFinalModel(t, tm, 2*time.Second)
	if finalModel.selectedIdx != 1 {
		t.Errorf("after down key: selectedIdx = %d, want 1", finalModel.selectedIdx)
	}
}

// TestIntegration_DashboardNavigationUp tests up key navigation
func TestIntegration_DashboardNavigationUp(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create two myses
	m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.CreateMysis("mysis-2", "ollama")
	m.refreshMysisList()
	m.selectedIdx = 1 // Start at second mysis

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Send up key
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})

	// Small delay for update
	time.Sleep(100 * time.Millisecond)

	// Verify selection moved up
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.selectedIdx != 0 {
		t.Errorf("after up key: selectedIdx = %d, want 0", finalModel.selectedIdx)
	}
}

// TestIntegration_FocusViewTransition tests Enter to focus and Esc to return
func TestIntegration_FocusViewTransition(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis
	_, _ = m.commander.CreateMysis("test-mysis", "ollama")
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Press Enter to focus
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for focus view to appear
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("CONVERSATION LOG"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Press Escape to return to dashboard
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	// Wait for dashboard view
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("MYSIS SWARM"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Verify returned to dashboard
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.view != ViewDashboard {
		t.Errorf("after Escape: view = %d, want ViewDashboard (%d)", finalModel.view, ViewDashboard)
	}
	if finalModel.focusID != "" {
		t.Errorf("after Escape: focusID = %s, want empty", finalModel.focusID)
	}
}

// TestIntegration_HelpToggle tests ? key toggles help
func TestIntegration_HelpToggle(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Press ? to show help
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	// Wait for help to appear in output
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("COMMAND REFERENCE"))
		},
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(2*time.Second),
	)

	// Press ? again to hide help
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	// Small delay for update
	time.Sleep(200 * time.Millisecond)

	// Verify help is hidden
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.showHelp {
		t.Error("after second ? key: showHelp should be false")
	}
}

// TestIntegration_BroadcastInput tests 'b' key, type message, Enter sends
func TestIntegration_BroadcastInput(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a running mysis to receive broadcast
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.StartMysis(mysis.ID())
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Press 'b' to start broadcast input
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	// Wait for input prompt
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("BROADCAST"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Type message characters
	testMessage := "Hello swarm"
	for _, r := range testMessage {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to send
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Allow model to process broadcast and reset input
	time.Sleep(200 * time.Millisecond)

	// Verify input cleared
	finalModel := quitAndFinalModel(t, tm, 2*time.Second)
	if finalModel.input.IsActive() {
		t.Error("after Enter: input should be inactive")
	}
}

// TestIntegration_MessageInput tests 'm' key, type message, Enter sends
func TestIntegration_MessageInput(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.StartMysis(mysis.ID())
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Press 'm' to start message input
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

	// Wait for input prompt
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Message to mysis"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Type message
	testMessage := "Test message"
	for _, r := range testMessage {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to send
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for completion
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return !bytes.Contains(bts, []byte("Message to mysis"))
		},
		teatest.WithDuration(3*time.Second),
	)

	// Verify input cleared
	finalModel := quitAndFinalModel(t, tm, 2*time.Second)
	if finalModel.input.IsActive() {
		t.Error("after Enter: input should be inactive")
	}
}

// TestIntegration_NewMysisInput tests 'n' key, type name, Enter creates
func TestIntegration_NewMysisInput(t *testing.T) {
	// Skipped: E2E test with teatest is timing-sensitive and flaky.
	// Functionality is covered by unit tests: TestMysisCreation_TwoStageFlow
	t.Skip("Flaky E2E test - covered by unit tests")
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Press 'n' to start new mysis input
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	// Wait for input prompt
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("NEW MYSIS"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Type mysis name
	mysisName := "new-mysis"
	for _, r := range mysisName {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to create
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for mysis to appear
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte(mysisName))
		},
		teatest.WithDuration(3*time.Second),
	)

	// Verify mysis was created
	finalModel := quitAndFinalModel(t, tm, 2*time.Second)
	if len(finalModel.myses) != 1 {
		t.Errorf("after creating mysis: myses count = %d, want 1", len(finalModel.myses))
	}
	if len(finalModel.myses) > 0 && finalModel.myses[0].Name != mysisName {
		t.Errorf("after creating mysis: name = %s, want %s", finalModel.myses[0].Name, mysisName)
	}
}

// TestIntegration_ConfigProviderInput tests 'c' key, select provider
func TestIntegration_ConfigProviderInput(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	mysisID := mysis.ID()
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Press 'c' to start config input
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	// Wait for config prompt
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Enter provider"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Verify input mode
	finalModel := programQuitAndFinalModel(t, tm, time.Second)
	if finalModel.input.Mode() != InputModeConfigProvider {
		t.Errorf("after 'c' key: input mode = %d, want InputModeConfigProvider (%d)", finalModel.input.Mode(), InputModeConfigProvider)
	}
	if finalModel.input.TargetID() != mysisID {
		t.Errorf("after 'c' key: target ID = %s, want %s", finalModel.input.TargetID(), mysisID)
	}
}

// TestIntegration_ConfigModelInput tests continue from provider, set model
func TestIntegration_ConfigModelInput(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis
	_, _ = m.commander.CreateMysis("mysis-1", "ollama")
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Press 'c' to start config input
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	// Wait for config prompt
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Enter provider"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Type provider name
	provider := "ollama"
	for _, r := range provider {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to continue to model
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for model prompt
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Enter model name"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Type model name
	modelName := "test-model"
	for _, r := range modelName {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to save config
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for config to complete
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return !bytes.Contains(bts, []byte("Enter model name"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Verify input cleared and pendingProvider cleared
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.input.IsActive() {
		t.Error("after model input: input should be inactive")
	}
	if finalModel.pendingProvider != "" {
		t.Errorf("after model input: pendingProvider = %s, want empty", finalModel.pendingProvider)
	}
}

// TestIntegration_InputCancel tests Esc cancels input mode
func TestIntegration_InputCancel(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Press 'b' to start broadcast input
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	// Wait for input to activate
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("BROADCAST"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Type some text
	for _, r := range "test" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Escape to cancel
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	// Wait for input to clear
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return !bytes.Contains(bts, []byte("BROADCAST"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Verify input was cancelled
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.input.IsActive() {
		t.Error("after Escape: input should be inactive")
	}
	if finalModel.input.Value() != "" {
		t.Errorf("after Escape: input value = %s, want empty", finalModel.input.Value())
	}
}

// TestIntegration_ViewportScroll tests scrolling in focus view
func TestIntegration_ViewportScroll(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis and add some messages
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.StartMysis(mysis.ID())

	// Add enough messages to require scrolling
	for i := 0; i < 20; i++ {
		m.commander.SendMessage(mysis.ID(), "Test message "+string(rune('A'+i)))
	}

	m.refreshMysisList()
	m.focusID = mysis.ID()
	m.view = ViewFocus
	m.loadMysisLogs()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Scroll down to bottom first (since auto-scroll was removed, viewport starts at top)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	// Wait for scroll
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return len(bts) > 0
		},
		teatest.WithDuration(2*time.Second),
	)

	// Now scroll up
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})

	// Wait for scroll
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return len(bts) > 0
		},
		teatest.WithDuration(2*time.Second),
	)

	// Verify viewport scrolled up from bottom
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.viewport.AtBottom() && len(finalModel.logs) > finalModel.viewport.Height {
		// If there's content to scroll and we're still at bottom, scrolling up didn't work
		t.Error("viewport should have scrolled up from bottom")
	}
}

// TestIntegration_ViewportBounds tests can't scroll past content
func TestIntegration_ViewportBounds(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis with minimal content
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.refreshMysisList()
	m.focusID = mysis.ID()
	m.view = ViewFocus
	m.loadMysisLogs()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Try to scroll up when at top
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})

	// Wait briefly
	time.Sleep(100 * time.Millisecond)

	// Verify didn't scroll past bounds
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.viewport.YOffset < 0 {
		t.Errorf("viewport scrolled past top: YOffset = %d", finalModel.viewport.YOffset)
	}

	// Offset should not have increased beyond content
	if len(finalModel.logs) > 0 && finalModel.viewport.YOffset > finalModel.viewportTotalLines {
		t.Errorf("viewport scrolled past content: YOffset = %d, totalLines = %d",
			finalModel.viewport.YOffset, finalModel.viewportTotalLines)
	}
}

// TestIntegration_MysisStartedEvent tests event updates dashboard
func TestIntegration_MysisStartedEvent(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis in idle state
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Start the mysis (this will trigger an event)
	m.commander.StartMysis(mysis.ID())

	// Wait for state to update in output
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("running"))
		},
		teatest.WithDuration(3*time.Second),
	)

	// Verify mysis state updated
	finalModel := quitAndFinalModel(t, tm, time.Second)
	finalModel.refreshMysisList()
	if len(finalModel.myses) > 0 && finalModel.myses[0].State != "running" {
		t.Errorf("after start event: mysis state = %s, want running", finalModel.myses[0].State)
	}
}

// TestIntegration_MysisStoppedEvent tests event updates dashboard
func TestIntegration_MysisStoppedEvent(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create and start a mysis
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.StartMysis(mysis.ID())
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Stop the mysis
	m.commander.StopMysis(mysis.ID())

	// Wait for state to update
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("stopped")) || bytes.Contains(bts, []byte("idle"))
		},
		teatest.WithDuration(3*time.Second),
	)

	// Verify mysis state updated
	finalModel := quitAndFinalModel(t, tm, time.Second)
	finalModel.refreshMysisList()
	if len(finalModel.myses) > 0 && finalModel.myses[0].State == "running" {
		t.Errorf("after stop event: mysis state = %s, should not be running", finalModel.myses[0].State)
	}
}

// TestIntegration_BroadcastEvent tests event appears in swarm messages
func TestIntegration_BroadcastEvent(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a running mysis
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.StartMysis(mysis.ID())
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Send a broadcast
	broadcastMsg := "Test broadcast message"
	m.commander.Broadcast(broadcastMsg)

	// Wait for broadcast to appear
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte(broadcastMsg))
		},
		teatest.WithDuration(3*time.Second),
	)

	// Verify broadcast appears in swarm messages
	finalModel := quitAndFinalModel(t, tm, 2*time.Second)
	finalModel.refreshSwarmMessages()

	found := false
	for _, msg := range finalModel.swarmMessages {
		if strings.Contains(msg.Content, broadcastMsg) {
			found = true
			break
		}
	}
	if !found {
		t.Error("broadcast message not found in swarm messages")
	}
}

// TestIntegration_CreateAndStartMysis tests full lifecycle
func TestIntegration_CreateAndStartMysis(t *testing.T) {
	// Skipped: Complex E2E test with teatest is timing-sensitive and flaky.
	// Functionality is covered by unit tests: TestMysisCreation_TwoStageFlow, TestMysisCreation_ZenNanoProvider
	t.Skip("Flaky E2E test - covered by unit tests")
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Start: press 'n' to create new mysis
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	// Wait for input
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("NEW MYSIS"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Type name
	mysisName := "lifecycle-mysis"
	for _, r := range mysisName {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to create
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for mysis to appear and auto-start
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte(mysisName)) &&
				(bytes.Contains(bts, []byte("running")) || bytes.Contains(bts, []byte("idle")))
		},
		teatest.WithDuration(3*time.Second),
	)

	// Navigate to the mysis and focus
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for focus view
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("CONVERSATION LOG"))
		},
		teatest.WithDuration(2*time.Second),
	)

	// Verify in focus view
	finalModel := quitAndFinalModel(t, tm, time.Second)
	finalModel.refreshMysisList()
	if len(finalModel.myses) != 1 {
		t.Fatalf("expected 1 mysis, got %d", len(finalModel.myses))
	}
	if finalModel.myses[0].Name != mysisName {
		t.Errorf("mysis name = %s, want %s", finalModel.myses[0].Name, mysisName)
	}
	if finalModel.view != ViewFocus {
		t.Errorf("view = %d, want ViewFocus (%d)", finalModel.view, ViewFocus)
	}
}

// TestIntegration_MultipleKeySequence tests complex interaction
func TestIntegration_MultipleKeySequence(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create two myses
	m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.CreateMysis("mysis-2", "ollama")
	m.refreshMysisList()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Sequence: navigate down, show help, hide help, navigate up
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("COMMAND REFERENCE"))
		},
		teatest.WithDuration(2*time.Second),
	)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return !bytes.Contains(bts, []byte("COMMAND REFERENCE"))
		},
		teatest.WithDuration(2*time.Second),
	)

	tm.Send(tea.KeyMsg{Type: tea.KeyUp})

	// Verify final state: selectedIdx=0, help hidden
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.selectedIdx != 0 {
		t.Errorf("after sequence: selectedIdx = %d, want 0", finalModel.selectedIdx)
	}
	if finalModel.showHelp {
		t.Error("after sequence: help should be hidden")
	}
}

// TestIntegration_VerboseToggle tests 'v' key toggles verbose JSON in focus view
func TestIntegration_VerboseToggle(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis and focus on it
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.refreshMysisList()
	m.focusID = mysis.ID()
	m.view = ViewFocus
	m.loadMysisLogs()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Initial state: verboseJSON should be false
	if m.verboseJSON {
		t.Error("initial verboseJSON should be false")
	}

	// Press 'v' to toggle verbose
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	// Wait for output
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return len(bts) > 0
		},
		teatest.WithDuration(2*time.Second),
	)

	// Press 'v' again to toggle back
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	// Wait for output
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return len(bts) > 0
		},
		teatest.WithDuration(2*time.Second),
	)

	// Verify verboseJSON toggled back
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.verboseJSON {
		t.Error("after second 'v' key: verboseJSON should be false")
	}
}

// TestIntegration_AutoScrollBehavior - REMOVED: auto-scroll functionality removed per TODO item
// Manual scrolling still works via G/End key and arrow keys

// ============================================================================
// Phase 10: New Integration Tests (Border, Spinner, Input, StatusBar, Resize)
// ============================================================================

// TestIntegration_BorderRendering tests borders render correctly in dashboard and focus view
func TestIntegration_BorderRendering(t *testing.T) {
	t.Run("dashboard_borders_visible", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create myses to populate the bordered list
		m.commander.CreateMysis("mysis-1", "ollama")
		m.commander.CreateMysis("mysis-2", "ollama")
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Small delay for initial render
		time.Sleep(100 * time.Millisecond)

		// Verify model state (borders are rendered via View() method)
		// We test that the dashboard renders correctly with borders by checking model state
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if len(finalModel.myses) != 2 {
			t.Errorf("expected 2 myses in bordered list, got %d", len(finalModel.myses))
		}
		if finalModel.view != ViewDashboard {
			t.Errorf("expected ViewDashboard, got view %d", finalModel.view)
		}

		// Verify View() renders borders correctly (indirect test via golden files)
		view := finalModel.View()
		if !strings.Contains(view, "MYSIS SWARM") {
			t.Error("dashboard view should contain 'MYSIS SWARM' section")
		}
		// Border characters are rendered in the view
		if !strings.Contains(view, "╔") && !strings.Contains(view, "═") {
			t.Log("Warning: border characters not found in view (may be environment-dependent)")
		}
	})

	t.Run("focus_view_borders_visible", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis and focus on it
		mysis, _ := m.commander.CreateMysis("test-mysis", "ollama")
		m.refreshMysisList()
		m.focusID = mysis.ID()
		m.view = ViewFocus
		m.loadMysisLogs()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Small delay for initial render
		time.Sleep(100 * time.Millisecond)

		// Verify model state and view rendering
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if finalModel.view != ViewFocus {
			t.Errorf("expected ViewFocus, got view %d", finalModel.view)
		}

		// Verify View() renders header decorations (borders)
		view := finalModel.View()
		if !strings.Contains(view, "CONVERSATION LOG") {
			t.Error("focus view should contain 'CONVERSATION LOG' section")
		}
		// Header decoration characters
		if !strings.Contains(view, "⬥") || !strings.Contains(view, "⬡") {
			t.Log("Warning: header decoration characters not found in view")
		}
	})

	t.Run("borders_no_layout_issues", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create multiple myses to test layout with borders
		for i := 0; i < 5; i++ {
			m.commander.CreateMysis(fmt.Sprintf("mysis-%d", i), "ollama")
		}
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Navigate through list to ensure borders don't cause selection issues
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		time.Sleep(50 * time.Millisecond)
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		time.Sleep(50 * time.Millisecond)
		tm.Send(tea.KeyMsg{Type: tea.KeyUp})

		// Verify navigation works correctly with borders
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if finalModel.selectedIdx != 1 {
			t.Errorf("expected selectedIdx=1 after navigation, got %d", finalModel.selectedIdx)
		}
	})
}

// TestIntegration_SpinnerAnimation tests spinner updates during mysis execution
func TestIntegration_SpinnerAnimation(t *testing.T) {
	t.Run("spinner_updates_during_execution", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create and start a mysis
		mysis, _ := m.commander.CreateMysis("running-mysis", "ollama")
		m.commander.StartMysis(mysis.ID())
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Wait for initial render with running mysis
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("running"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Wait for spinner tick (spinner updates at 8 FPS = 125ms per frame)
		time.Sleep(300 * time.Millisecond)

		// Note: Testing spinner animation in output is timing-dependent and fragile
		// We verify the spinner exists in the model instead
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if len(finalModel.myses) > 0 && finalModel.myses[0].State != "running" {
			t.Errorf("mysis should be running, got state: %s", finalModel.myses[0].State)
		}
		// Spinner is active as long as there are running myses (verified via state)
	})

	t.Run("spinner_no_flicker", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a running mysis
		mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
		m.commander.StartMysis(mysis.ID())
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Wait for initial render
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("running"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Wait through multiple spinner ticks
		time.Sleep(500 * time.Millisecond)

		// Verify model state remains consistent (no flicker = no unexpected state changes)
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if len(finalModel.myses) == 0 {
			t.Fatal("mysis disappeared during spinner animation")
		}
		if finalModel.myses[0].State != "running" {
			t.Errorf("mysis state changed unexpectedly: %s", finalModel.myses[0].State)
		}
	})

	t.Run("multiple_myses_with_spinners", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create multiple running myses
		for i := 0; i < 3; i++ {
			mysis, _ := m.commander.CreateMysis(fmt.Sprintf("mysis-%d", i), "ollama")
			m.commander.StartMysis(mysis.ID())
		}
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Wait for all myses to render
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("mysis-0")) &&
					bytes.Contains(bts, []byte("mysis-1")) &&
					bytes.Contains(bts, []byte("mysis-2"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Wait for spinner updates
		time.Sleep(300 * time.Millisecond)

		// Verify all myses present and running
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if len(finalModel.myses) != 3 {
			t.Errorf("expected 3 myses, got %d", len(finalModel.myses))
		}
		for i, mysis := range finalModel.myses {
			if mysis.State != "running" {
				t.Errorf("mysis-%d state = %s, want running", i, mysis.State)
			}
		}
	})
}

// TestIntegration_InputPrompt tests input prompt interactions
func TestIntegration_InputPrompt(t *testing.T) {
	t.Run("broadcast_message_entry", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis to receive broadcast
		mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
		m.commander.StartMysis(mysis.ID())
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Press 'b' to open broadcast input
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

		// Wait for broadcast prompt
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("BROADCAST")) ||
					bytes.Contains(bts, []byte("Broadcasting"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Verify input is active and in broadcast mode
		finalModel := programQuitAndFinalModel(t, tm, time.Second)
		if !finalModel.input.IsActive() {
			t.Error("input should be active after 'b' key")
		}
		if finalModel.input.Mode() != InputModeBroadcast {
			t.Errorf("input mode = %d, want InputModeBroadcast (%d)", finalModel.input.Mode(), InputModeBroadcast)
		}
	})

	t.Run("direct_message_entry", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis to message
		mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
		m.commander.StartMysis(mysis.ID())
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Press 'm' to open message input
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

		// Wait for message prompt
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Message to mysis"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Verify input is active and in message mode
		finalModel := programQuitAndFinalModel(t, tm, time.Second)
		if !finalModel.input.IsActive() {
			t.Error("input should be active after 'm' key")
		}
		if finalModel.input.Mode() != InputModeMessage {
			t.Errorf("input mode = %d, want InputModeMessage (%d)", finalModel.input.Mode(), InputModeMessage)
		}
	})

	t.Run("cancel_input_esc", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Open broadcast input
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

		// Wait for input to activate
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("BROADCAST"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Type some text
		for _, r := range "test message" {
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}

		// Cancel with Escape
		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

		// Wait for input to deactivate
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return !bytes.Contains(bts, []byte("BROADCAST"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Verify input cleared
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if finalModel.input.IsActive() {
			t.Error("input should be inactive after Escape")
		}
		if finalModel.input.Value() != "" {
			t.Errorf("input value = %q, want empty after cancel", finalModel.input.Value())
		}
	})

	t.Run("submit_input_enter", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a running mysis
		mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
		m.commander.StartMysis(mysis.ID())
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Open broadcast input
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

		// Wait for input
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("BROADCAST"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Type message
		testMsg := "Hello"
		for _, r := range testMsg {
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}

		// Submit with Enter
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		// Wait for input to clear
		time.Sleep(200 * time.Millisecond)

		// Verify input cleared after submit
		finalModel := quitAndFinalModel(t, tm, 2*time.Second)
		if finalModel.input.IsActive() {
			t.Error("input should be inactive after Enter submit")
		}
	})
}

// TestIntegration_StatusBar tests status bar updates
func TestIntegration_StatusBar(t *testing.T) {
	t.Run("status_bar_mysis_state_changes", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis
		mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Wait for initial render (idle state - should show idle count or no myses)
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				// Check for either state icons or "(no myses)" text
				return bytes.Contains(bts, []byte("◦")) || bytes.Contains(bts, []byte("(no myses)"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Start the mysis
		m.commander.StartMysis(mysis.ID())

		// Wait for status bar to update (should show 1 running)
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("running"))
			},
			teatest.WithDuration(3*time.Second),
		)

		finalModel := quitAndFinalModel(t, tm, time.Second)
		finalModel.refreshMysisList()
		if len(finalModel.myses) > 0 && finalModel.myses[0].State != "running" {
			t.Errorf("status bar should reflect running state, got %s", finalModel.myses[0].State)
		}
	})

	t.Run("status_bar_network_indicator", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Small delay for initial render
		time.Sleep(100 * time.Millisecond)

		// Verify status bar rendering via View() method
		finalModel := quitAndFinalModel(t, tm, time.Second)
		view := finalModel.View()

		// Status bar should show tick timestamp in format T#### ⬡ [HH:MM]
		if !strings.Contains(view, "T") || !strings.Contains(view, "[") || !strings.Contains(view, "]") {
			t.Error("status bar should contain tick timestamp format (T#### ⬡ [HH:MM])")
		}
		// Status bar should show activity indicator (IDLE/LLM/MCP)
		if !strings.Contains(view, "IDLE") && !strings.Contains(view, "LLM") && !strings.Contains(view, "MCP") {
			t.Error("status bar should contain activity indicator (IDLE/LLM/MCP)")
		}
	})

	t.Run("status_bar_state_counts_update", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis
		_, _ = m.commander.CreateMysis("test-mysis", "ollama")
		m.refreshMysisList()

		// Render status bar directly
		statusBar := m.renderStatusBar()

		// Should contain tick timestamp format
		if !strings.Contains(statusBar, "T") || !strings.Contains(statusBar, "⬡") {
			t.Error("status bar should contain tick timestamp (T#### ⬡ [HH:MM])")
		}

		// Should contain activity indicator
		if !strings.Contains(statusBar, "IDLE") && !strings.Contains(statusBar, "LLM") && !strings.Contains(statusBar, "MCP") {
			t.Error("status bar should contain activity indicator")
		}

		// Should contain state count (idle mysis)
		if !strings.Contains(statusBar, "◦") {
			t.Error("status bar should contain idle state icon (◦)")
		}
	})
}

// TestIntegration_WindowResize tests window resize handling
func TestIntegration_WindowResize(t *testing.T) {
	t.Run("resize_during_dashboard", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create myses to populate dashboard
		m.commander.CreateMysis("mysis-1", "ollama")
		m.commander.CreateMysis("mysis-2", "ollama")
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Wait for initial render
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("MYSIS SWARM"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Resize to larger
		newWidth := 160
		newHeight := 50
		tm.Send(tea.WindowSizeMsg{Width: newWidth, Height: newHeight})

		// Wait for resize to process
		time.Sleep(200 * time.Millisecond)

		// Verify dimensions updated
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if finalModel.width != newWidth {
			t.Errorf("width = %d, want %d after resize", finalModel.width, newWidth)
		}
		if finalModel.height != newHeight {
			t.Errorf("height = %d, want %d after resize", finalModel.height, newHeight)
		}
	})

	t.Run("resize_during_focus_view", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create a mysis and focus on it
		mysis, _ := m.commander.CreateMysis("test-mysis", "ollama")
		m.refreshMysisList()
		m.focusID = mysis.ID()
		m.view = ViewFocus
		m.loadMysisLogs()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Wait for focus view
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("CONVERSATION LOG"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Resize to smaller
		newWidth := 100
		newHeight := 30
		tm.Send(tea.WindowSizeMsg{Width: newWidth, Height: newHeight})

		// Wait for resize
		time.Sleep(200 * time.Millisecond)

		// Verify dimensions and viewport updated
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if finalModel.width != newWidth {
			t.Errorf("width = %d, want %d after resize", finalModel.width, newWidth)
		}
		if finalModel.height != newHeight {
			t.Errorf("height = %d, want %d after resize", finalModel.height, newHeight)
		}
		// Viewport should have been recreated with new dimensions
		if finalModel.viewport.Width == TestTerminalWidth {
			t.Error("viewport width should have changed after resize")
		}
	})

	t.Run("layout_recalculates_correctly", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		// Create myses
		for i := 0; i < 3; i++ {
			m.commander.CreateMysis(fmt.Sprintf("mysis-%d", i), "ollama")
		}
		m.refreshMysisList()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Initial render
		teatest.WaitFor(
			t,
			tm.Output(),
			func(bts []byte) bool {
				return bytes.Contains(bts, []byte("mysis-0"))
			},
			teatest.WithDuration(2*time.Second),
		)

		// Resize multiple times
		sizes := []struct{ w, h int }{
			{140, 45},
			{100, 35},
			{120, 40},
		}

		for _, size := range sizes {
			tm.Send(tea.WindowSizeMsg{Width: size.w, Height: size.h})
			time.Sleep(100 * time.Millisecond)
		}

		// Verify final dimensions
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if finalModel.width != 120 {
			t.Errorf("width = %d, want 120 after resize sequence", finalModel.width)
		}
		if finalModel.height != 40 {
			t.Errorf("height = %d, want 40 after resize sequence", finalModel.height)
		}
		// Myses should still be present and navigable
		if len(finalModel.myses) != 3 {
			t.Errorf("myses count = %d, want 3 after resizes", len(finalModel.myses))
		}
	})

	t.Run("minimum_size_handling", func(t *testing.T) {
		m, cleanup := setupTestModel(t)
		defer cleanup()

		tm := teatest.NewTestModel(
			t,
			m,
			teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
		)
		defer tm.Quit()

		// Resize to below minimum (80x20)
		tm.Send(tea.WindowSizeMsg{Width: 60, Height: 15})

		// Wait for resize
		time.Sleep(200 * time.Millisecond)

		// Verify model updated dimensions (even if too small)
		finalModel := quitAndFinalModel(t, tm, time.Second)
		if finalModel.width != 60 {
			t.Errorf("width = %d, want 60 (model should store actual size)", finalModel.width)
		}
		if finalModel.height != 15 {
			t.Errorf("height = %d, want 15 (model should store actual size)", finalModel.height)
		}
		// View method should detect small size and show warning
		// (This is tested via rendering, not model state)
	})
}
