package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
)

// TestInputPromptIndicators verifies each input mode has the correct indicator.
func TestInputPromptIndicators(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name      string
		mode      InputMode
		targetID  string
		indicator string
	}{
		{"broadcast_mode", InputModeBroadcast, "", "⬧"},
		{"message_mode", InputModeMessage, "mysis-1", "⬥"},
		{"new_mysis_mode", InputModeNewMysis, "", "⬡"},
		{"config_provider_mode", InputModeConfigProvider, "mysis-1", "⚙"},
		{"config_model_mode", InputModeConfigModel, "mysis-1", "cfg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewInputModel()
			input.SetMode(tt.mode, tt.targetID)

			prompt := input.textInput.Prompt
			if !strings.Contains(prompt, tt.indicator) {
				t.Errorf("prompt %q does not contain expected indicator %q", prompt, tt.indicator)
			}
			if input.Mode() != tt.mode {
				t.Errorf("mode = %v, want %v", input.Mode(), tt.mode)
			}
			if input.TargetID() != tt.targetID {
				t.Errorf("targetID = %q, want %q", input.TargetID(), tt.targetID)
			}
			if !input.IsActive() {
				t.Error("expected input to be active")
			}
		})
	}
}

// TestInputPromptNone verifies that InputModeNone clears the prompt.
func TestInputPromptNone(t *testing.T) {
	input := NewInputModel()
	input.SetMode(InputModeBroadcast, "")
	if !input.IsActive() {
		t.Error("expected input to be active after setting broadcast mode")
	}

	input.SetMode(InputModeNone, "")
	if input.IsActive() {
		t.Error("expected input to be inactive after setting none mode")
	}
	if input.textInput.Prompt != "" {
		t.Errorf("expected empty prompt, got %q", input.textInput.Prompt)
	}
	if input.textInput.Placeholder != "" {
		t.Errorf("expected empty placeholder, got %q", input.textInput.Placeholder)
	}
}

// TestInputPromptWidthHandling verifies prompt rendering at various widths.
func TestInputPromptWidthHandling(t *testing.T) {
	tests := []struct {
		name     string
		mode     InputMode
		width    int
		inputLen int
	}{
		{"narrow_width_empty", InputModeBroadcast, 20, 0},
		{"narrow_width_short_text", InputModeMessage, 30, 5},
		{"normal_width_medium_text", InputModeBroadcast, 60, 20},
		{"wide_width_long_text", InputModeMessage, 120, 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewInputModel()
			input.SetMode(tt.mode, "test-target")
			input.SetWidth(tt.width)

			if tt.inputLen > 0 {
				text := strings.Repeat("a", tt.inputLen)
				input.textInput.SetValue(text)
			}

			output := inputStyle.Width(tt.width - 2).Render(input.textInput.View())
			if output == "" {
				t.Error("expected non-empty output")
			}
			displayWidth := lipgloss.Width(output)
			if displayWidth == 0 {
				t.Error("expected non-zero display width")
			}
		})
	}
}

// TestInputPromptLongText verifies handling of long input text.
func TestInputPromptLongText(t *testing.T) {
	defer setupGoldenTest(t)()

	input := NewInputModel()
	input.SetMode(InputModeBroadcast, "")
	input.SetWidth(60)

	longText := "This is a very long message that exceeds the typical display width and should be handled gracefully by the input component without causing any rendering issues."
	input.textInput.SetValue(longText)

	output := inputStyle.Width(58).Render(input.textInput.View())
	if !strings.Contains(output, "⬧") {
		t.Error("expected output to contain broadcast indicator ⬧")
	}
	if input.Value() != longText {
		t.Error("expected full text to be preserved in value")
	}
}

// TestInputPromptReset verifies reset clears all state.
func TestInputPromptReset(t *testing.T) {
	input := NewInputModel()
	input.SetMode(InputModeBroadcast, "target-id")
	input.textInput.SetValue("test message")
	input.AddToHistory("previous message")

	input.Reset()

	if input.Mode() != InputModeNone {
		t.Errorf("mode = %v, want InputModeNone", input.Mode())
	}
	if input.TargetID() != "" {
		t.Errorf("targetID = %q, want empty", input.TargetID())
	}
	if input.Value() != "" {
		t.Errorf("value = %q, want empty", input.Value())
	}
	if input.IsActive() {
		t.Error("expected input to be inactive after reset")
	}
	if input.historyIndex != -1 {
		t.Errorf("historyIndex = %d, want -1", input.historyIndex)
	}
	if input.draft != "" {
		t.Errorf("draft = %q, want empty", input.draft)
	}
}

// TestInputPromptViewAlways verifies ViewAlways behavior.
func TestInputPromptViewAlways(t *testing.T) {
	tests := []struct {
		name         string
		mode         InputMode
		sending      bool
		sendingLabel string
		spinnerView  string
		width        int
	}{
		{"inactive_placeholder", InputModeNone, false, "", "", 60},
		{"active_broadcast", InputModeBroadcast, false, "", "", 60},
		{"sending_broadcast", InputModeNone, true, "Broadcasting...", "⬡", 60},
		{"sending_message", InputModeNone, true, "Sending...", "⬢", 60},
		{"narrow_width", InputModeNone, false, "", "", 20},
		{"minimum_width", InputModeNone, false, "", "", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewInputModel()
			if tt.mode != InputModeNone {
				input.SetMode(tt.mode, "test-target")
			}

			output := input.ViewAlways(tt.width, tt.sending, tt.sendingLabel, tt.spinnerView)
			if output == "" {
				t.Error("expected non-empty output from ViewAlways")
			}
			if tt.sending && tt.sendingLabel != "" {
				if !strings.Contains(output, tt.sendingLabel) {
					t.Errorf("expected output to contain sending label %q", tt.sendingLabel)
				}
			}
			if tt.mode == InputModeNone && !tt.sending && tt.width >= 40 {
				if !strings.Contains(output, "Press") || !strings.Contains(output, "message") {
					t.Error("expected output to contain placeholder prompt")
				}
			}
		})
	}
}

// TestInputPromptUpdate verifies input updates propagate correctly.
func TestInputPromptUpdate(t *testing.T) {
	input := NewInputModel()
	input.SetMode(InputModeBroadcast, "")

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	updated, _ := input.Update(keyMsg)

	if updated.Value() != "a" {
		t.Errorf("value = %q, want 'a'", updated.Value())
	}
	if updated.Mode() != InputModeBroadcast {
		t.Errorf("mode = %v, want InputModeBroadcast", updated.Mode())
	}
}

// TestInputPromptFocus verifies focus command is returned.
func TestInputPromptFocus(t *testing.T) {
	input := NewInputModel()
	cmd := input.Focus()
	if cmd == nil {
		t.Error("expected non-nil focus command")
	}
}

// Golden file tests for input prompts
func TestInputPrompt(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name     string
		mode     InputMode
		text     string
		width    int
		targetID string
	}{
		{"broadcast_empty", InputModeBroadcast, "", 60, ""},
		{"broadcast_with_text", InputModeBroadcast, "Attack sector 7", 60, ""},
		{"message_empty", InputModeMessage, "", 60, "mysis-alpha"},
		{"message_with_text", InputModeMessage, "Check your cargo", 60, "mysis-beta"},
		{"long_text", InputModeBroadcast, "This is a very long message that will exceed the normal display width and test the input component's handling of long text without wrapping issues.", 60, ""},
		{"narrow_width", InputModeMessage, "Short", 30, "test"},
		{"wide_width", InputModeBroadcast, "Wide terminal test message", 120, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewInputModel()
			input.SetMode(tt.mode, tt.targetID)
			input.SetWidth(tt.width)

			if tt.text != "" {
				input.textInput.SetValue(tt.text)
			}

			output := inputStyle.Width(tt.width - 2).Render(input.textInput.View())

			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(output))
			})

			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(output)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}
