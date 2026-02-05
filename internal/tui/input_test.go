package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestInputModelViewAlwaysNoOverflow(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		mode   InputMode
		target string
	}{
		{"inactive small", 40, InputModeNone, ""},
		{"inactive medium", 80, InputModeNone, ""},
		{"inactive large", 120, InputModeNone, ""},
		{"broadcast small", 40, InputModeBroadcast, ""},
		{"broadcast medium", 80, InputModeBroadcast, ""},
		{"message small", 40, InputModeMessage, "mysis-1"},
		{"message medium", 80, InputModeMessage, "mysis-1"},
		{"new mysis small", 40, InputModeNewMysis, ""},
		{"new mysis medium", 80, InputModeNewMysis, ""},
		{"config provider small", 40, InputModeConfigProvider, ""},
		{"config provider medium", 80, InputModeConfigProvider, ""},
		{"config model small", 40, InputModeConfigModel, ""},
		{"config model medium", 80, InputModeConfigModel, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewInputModel()
			input.SetWidth(tt.width - 4) // Simulate WindowSizeMsg handling

			if tt.mode != InputModeNone {
				input.SetMode(tt.mode, tt.target)
			}

			view := input.ViewAlways(tt.width)
			if view == "" {
				t.Error("ViewAlways should return non-empty view")
			}

			renderedWidth := lipgloss.Width(view)
			if renderedWidth != tt.width {
				t.Errorf("rendered width = %d, want %d (overflow: %d)", renderedWidth, tt.width, renderedWidth-tt.width)
			}

			// Check that no line exceeds terminal width
			lines := strings.Split(view, "\n")
			for i, line := range lines {
				lineWidth := lipgloss.Width(line)
				if lineWidth > tt.width {
					t.Errorf("line %d width = %d, exceeds terminal width %d", i, lineWidth, tt.width)
					t.Logf("Line content: %q", line)
				}
			}
		})
	}
}

func TestInputModelSetWidthUpdatesTextInput(t *testing.T) {
	input := NewInputModel()

	widths := []int{40, 60, 80, 100, 120}
	for _, width := range widths {
		input.SetWidth(width - 4)
		expected := (width - 4) - 4 // SetWidth subtracts 4
		if input.textInput.Width != expected {
			t.Errorf("SetWidth(%d): textInput.Width = %d, want %d", width-4, input.textInput.Width, expected)
		}
	}
}

func TestInputModelAllModesRender(t *testing.T) {
	modes := []struct {
		mode InputMode
		name string
	}{
		{InputModeNone, "none"},
		{InputModeBroadcast, "broadcast"},
		{InputModeMessage, "message"},
		{InputModeNewMysis, "new_mysis"},
		{InputModeConfigProvider, "config_provider"},
		{InputModeConfigModel, "config_model"},
	}

	width := 80
	input := NewInputModel()
	input.SetWidth(width - 4)

	for _, tt := range modes {
		t.Run(tt.name, func(t *testing.T) {
			input.SetMode(tt.mode, "test-target")
			view := input.ViewAlways(width)

			if view == "" {
				t.Error("ViewAlways should return non-empty view")
			}

			renderedWidth := lipgloss.Width(view)
			if renderedWidth != width {
				t.Errorf("rendered width = %d, want %d", renderedWidth, width)
			}
		})
	}
}

func TestInputModelViewAlwaysZeroWidth(t *testing.T) {
	input := NewInputModel()

	// Test with zero width (before WindowSizeMsg)
	view := input.ViewAlways(0)
	if view == "" {
		t.Error("ViewAlways should return non-empty view even with zero width")
	}

	// Should use minimum width of 10
	renderedWidth := lipgloss.Width(view)
	if renderedWidth != 10 {
		t.Errorf("rendered width = %d, want 10 (minimum width)", renderedWidth)
	}
}

func TestInputModelViewAlwaysSmallWidth(t *testing.T) {
	input := NewInputModel()
	input.SetWidth(5)

	// Test with very small width
	view := input.ViewAlways(5)
	if view == "" {
		t.Error("ViewAlways should return non-empty view")
	}

	// Should use minimum width of 10
	renderedWidth := lipgloss.Width(view)
	if renderedWidth != 10 {
		t.Errorf("rendered width = %d, want 10 (minimum width)", renderedWidth)
	}
}
