package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// InputMode represents the current input mode.
type InputMode int

const (
	InputModeNone InputMode = iota
	InputModeBroadcast
	InputModeMessage
	InputModeNewMysis
	InputModeConfigProvider
)

const maxHistorySize = 100

// InputModel handles text input for messages.
type InputModel struct {
	textInput    textinput.Model
	mode         InputMode
	targetID     string   // For direct messages
	history      []string // Previous messages
	historyIndex int      // Current position in history (-1 = not browsing)
	draft        string   // Saved draft when browsing history
}

// NewInputModel creates a new input model.
func NewInputModel() InputModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 1000
	ti.Width = 60

	return InputModel{
		textInput:    ti,
		mode:         InputModeNone,
		history:      make([]string, 0, maxHistorySize),
		historyIndex: -1,
	}
}

// SetMode sets the input mode and updates the prompt.
func (m *InputModel) SetMode(mode InputMode, targetID string) {
	m.mode = mode
	m.targetID = targetID
	m.textInput.Reset()

	switch mode {
	case InputModeBroadcast:
		m.textInput.Placeholder = "Broadcast message to all myses..."
		m.textInput.Prompt = inputPromptStyle.Render("📢 ") + " "
	case InputModeMessage:
		m.textInput.Placeholder = "Message to mysis..."
		m.textInput.Prompt = inputPromptStyle.Render("💬 ") + " "
	case InputModeNewMysis:
		m.textInput.Placeholder = "Enter mysis name..."
		m.textInput.Prompt = inputPromptStyle.Render("🤖 ") + " "
	case InputModeConfigProvider:
		m.textInput.Placeholder = "Enter provider (ollama/opencode_zen)..."
		m.textInput.Prompt = inputPromptStyle.Render("⚙️ ") + " "
	default:
		m.textInput.Placeholder = ""
		m.textInput.Prompt = ""
	}

	if mode != InputModeNone {
		m.textInput.Focus()
	} else {
		m.textInput.Blur()
	}
}

// Mode returns the current input mode.
func (m InputModel) Mode() InputMode {
	return m.mode
}

// TargetID returns the target mysis ID for direct messages.
func (m InputModel) TargetID() string {
	return m.targetID
}

// Value returns the current input value.
func (m InputModel) Value() string {
	return m.textInput.Value()
}

// IsActive returns true if input is active.
func (m InputModel) IsActive() bool {
	return m.mode != InputModeNone
}

// Focus returns the command to start the text input cursor.
func (m InputModel) Focus() tea.Cmd {
	return textinput.Blink
}

// History key bindings
var historyKeys = struct {
	Up   key.Binding
	Down key.Binding
}{
	Up:   key.NewBinding(key.WithKeys("up")),
	Down: key.NewBinding(key.WithKeys("down")),
}

// Update handles input updates.
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	// Handle history navigation for message modes
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.mode == InputModeBroadcast || m.mode == InputModeMessage {
			switch {
			case key.Matches(keyMsg, historyKeys.Up):
				m.navigateHistory(1) // Go back in history
				return m, nil
			case key.Matches(keyMsg, historyKeys.Down):
				m.navigateHistory(-1) // Go forward in history
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// navigateHistory moves through the history.
// direction: 1 = older (up), -1 = newer (down)
func (m *InputModel) navigateHistory(direction int) {
	if len(m.history) == 0 {
		return
	}

	// Save current input as draft when starting to browse
	if m.historyIndex == -1 && direction == 1 {
		m.draft = m.textInput.Value()
	}

	newIndex := m.historyIndex + direction

	// Clamp to valid range
	if newIndex < -1 {
		newIndex = -1
	}
	if newIndex >= len(m.history) {
		newIndex = len(m.history) - 1
	}

	m.historyIndex = newIndex

	// Update input value
	if m.historyIndex == -1 {
		// Back to draft
		m.textInput.SetValue(m.draft)
		m.textInput.CursorEnd()
	} else {
		// Show history item (most recent is at end of slice)
		historyIdx := len(m.history) - 1 - m.historyIndex
		m.textInput.SetValue(m.history[historyIdx])
		m.textInput.CursorEnd()
	}
}

// View renders the input.
func (m InputModel) View() string {
	if m.mode == InputModeNone {
		return ""
	}
	return inputStyle.Render(m.textInput.View())
}

// ViewAlways renders the input bar, showing placeholder when not active.
func (m InputModel) ViewAlways(width int) string {
	if m.mode != InputModeNone {
		// Active - show the actual input
		return inputStyle.Width(width - 2).Render(m.textInput.View())
	}
	// Inactive - show placeholder prompt
	placeholder := dimmedStyle.Render("Press 'm' to message, 'b' to broadcast...")
	return inputStyle.Width(width - 2).Render(placeholder)
}

// Reset clears the input.
func (m *InputModel) Reset() {
	m.textInput.Reset()
	m.mode = InputModeNone
	m.targetID = ""
	m.historyIndex = -1
	m.draft = ""
	m.textInput.Blur()
}

// AddToHistory adds a message to the history.
func (m *InputModel) AddToHistory(message string) {
	if message == "" {
		return
	}

	// Avoid duplicate consecutive entries
	if len(m.history) > 0 && m.history[len(m.history)-1] == message {
		return
	}

	m.history = append(m.history, message)

	// Trim if too large
	if len(m.history) > maxHistorySize {
		m.history = m.history[len(m.history)-maxHistorySize:]
	}
}

// SetWidth sets the input width.
func (m *InputModel) SetWidth(width int) {
	m.textInput.Width = width - 4 // Account for padding/border
}
