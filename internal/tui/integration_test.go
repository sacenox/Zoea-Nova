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

// TestIntegration_WindowResize tests WindowSizeMsg, verify layout adapts
func TestIntegration_WindowResize(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Send window resize message
	newWidth := 140
	newHeight := 50
	tm.Send(tea.WindowSizeMsg{Width: newWidth, Height: newHeight})

	// Wait for output to update
	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			// Just wait for any output after resize
			return len(bts) > 0
		},
		teatest.WithDuration(2*time.Second),
	)

	// Verify dimensions updated
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.width != newWidth {
		t.Errorf("after resize: width = %d, want %d", finalModel.width, newWidth)
	}
	if finalModel.height != newHeight {
		t.Errorf("after resize: height = %d, want %d", finalModel.height, newHeight)
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

	// Scroll up
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

	// Verify viewport scrolled
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.viewport.YOffset == 0 && len(finalModel.logs) > finalModel.viewport.Height {
		// If there's content to scroll and we're still at offset 0, something is wrong
		t.Error("viewport should have scrolled up but YOffset is still 0")
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

// TestIntegration_AutoScrollBehavior tests auto-scroll enables/disables correctly
func TestIntegration_AutoScrollBehavior(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis with some messages
	mysis, _ := m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.StartMysis(mysis.ID())

	// Add messages
	for i := 0; i < 10; i++ {
		m.commander.SendMessage(mysis.ID(), "Message "+string(rune('0'+i)))
	}

	m.refreshMysisList()
	m.focusID = mysis.ID()
	m.view = ViewFocus
	m.loadMysisLogs()
	m.autoScroll = true // Start at bottom with auto-scroll

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(TestTerminalWidth, TestTerminalHeight),
	)
	defer tm.Quit()

	// Scroll up - should disable auto-scroll
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})

	// Wait briefly
	time.Sleep(100 * time.Millisecond)

	// Verify auto-scroll disabled
	finalModel := quitAndFinalModel(t, tm, time.Second)
	if finalModel.autoScroll {
		t.Error("after scrolling up: autoScroll should be false")
	}
}
