package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func TestErrorDisplayInDashboard(t *testing.T) {
	testErr := errors.New("test error message")

	// Render dashboard with error
	output := RenderDashboard(
		[]MysisInfo{},
		[]SwarmMessageInfo{},
		0,
		100,
		30,
		make(map[string]bool),
		"⠋",
		0,
		testErr,
	)

	// Error should appear in the hint line
	if !strings.Contains(output, "Error: test error message") {
		t.Error("expected error message to appear in dashboard output")
	}

	// Error should be on the same line as keyboard controls
	lines := strings.Split(output, "\n")
	foundErrorLine := false
	for _, line := range lines {
		if strings.Contains(line, "HELP") && strings.Contains(line, "Error:") {
			foundErrorLine = true
			break
		}
	}
	if !foundErrorLine {
		t.Error("expected error to appear on same line as keyboard controls")
	}
}

func TestErrorDisplayInFocusView(t *testing.T) {
	testErr := errors.New("focus view error")

	mysis := MysisInfo{
		ID:       "test-id",
		Name:     "TestMysis",
		State:    "running",
		Provider: "opencode",
	}

	vp := viewport.New(120, 20)
	vp.SetContent("Test content")

	// Render focus view with error (use wider terminal to fit error)
	output := RenderFocusViewWithViewport(
		mysis,
		vp,
		120,
		false,
		"⠋",
		false,
		1,
		1,
		1,
		0,
		testErr,
	)

	// Error should appear in the hint line
	if !strings.Contains(output, "Error: focus view error") {
		t.Logf("Output:\n%s", output)
		t.Error("expected error message to appear in focus view output")
	}

	// Error should be on the same line as keyboard controls
	lines := strings.Split(output, "\n")
	foundErrorLine := false
	for _, line := range lines {
		if strings.Contains(line, "ESC") && strings.Contains(line, "Error:") {
			foundErrorLine = true
			break
		}
	}
	if !foundErrorLine {
		t.Error("expected error to appear on same line as keyboard controls in focus view")
	}
}

func TestNoErrorDisplay(t *testing.T) {
	// Render dashboard without error
	output := RenderDashboard(
		[]MysisInfo{},
		[]SwarmMessageInfo{},
		0,
		100,
		30,
		make(map[string]bool),
		"⠋",
		0,
		nil,
	)

	// Should not contain "Error:"
	if strings.Contains(output, "Error:") {
		t.Error("expected no error message when err is nil")
	}
}

func TestLongErrorTruncation(t *testing.T) {
	longErr := errors.New("this is a very long error message that should be truncated to fit within the available space on the terminal line without breaking the layout")

	output := RenderDashboard(
		[]MysisInfo{},
		[]SwarmMessageInfo{},
		0,
		100,
		30,
		make(map[string]bool),
		"⠋",
		0,
		longErr,
	)

	// Error should be present but truncated
	if !strings.Contains(output, "Error:") {
		t.Error("expected error message to appear")
	}

	// Check that no line exceeds terminal width (using display width, not byte length)
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		// Use lipgloss.Width for accurate display width (ignores ANSI codes)
		displayWidth := lipgloss.Width(line)
		if displayWidth > 100 {
			t.Errorf("line %d exceeds terminal width: %d display chars", i, displayWidth)
		}
	}
}
