package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// InputMode represents the current input mode.
type InputMode int

const (
	InputModeNone InputMode = iota
	InputModeBroadcast
	InputModeMessage
	InputModeNewAgent
	InputModeConfigProvider
)

// InputModel handles text input for messages.
type InputModel struct {
	textInput textinput.Model
	mode      InputMode
	targetID  string // For direct messages
}

// NewInputModel creates a new input model.
func NewInputModel() InputModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 1000
	ti.Width = 60

	return InputModel{
		textInput: ti,
		mode:      InputModeNone,
	}
}

// SetMode sets the input mode and updates the prompt.
func (m *InputModel) SetMode(mode InputMode, targetID string) {
	m.mode = mode
	m.targetID = targetID
	m.textInput.Reset()

	switch mode {
	case InputModeBroadcast:
		m.textInput.Placeholder = "Broadcast message to all agents..."
		m.textInput.Prompt = inputPromptStyle.Render("📢 ") + " "
	case InputModeMessage:
		m.textInput.Placeholder = "Message to agent..."
		m.textInput.Prompt = inputPromptStyle.Render("💬 ") + " "
	case InputModeNewAgent:
		m.textInput.Placeholder = "Enter agent name..."
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

// TargetID returns the target agent ID for direct messages.
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

// Update handles input updates.
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View renders the input.
func (m InputModel) View() string {
	if m.mode == InputModeNone {
		return ""
	}
	return inputStyle.Render(m.textInput.View())
}

// Reset clears the input.
func (m *InputModel) Reset() {
	m.textInput.Reset()
	m.mode = InputModeNone
	m.targetID = ""
	m.textInput.Blur()
}

// SetWidth sets the input width.
func (m *InputModel) SetWidth(width int) {
	m.textInput.Width = width - 4 // Account for padding/border
}
