package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
)

// TestFocusViewHeaderPresence verifies that the focus view header is present in rendered output.
func TestFocusViewHeaderPresence(t *testing.T) {
	tests := []struct {
		name       string
		termWidth  int
		termHeight int
		mysisName  string
	}{
		{
			name:       "small_terminal_20_lines",
			termWidth:  80,
			termHeight: 20,
			mysisName:  "test-mysis",
		},
		{
			name:       "normal_terminal_40_lines",
			termWidth:  120,
			termHeight: 40,
			mysisName:  "alpha-mysis",
		},
		{
			name:       "large_terminal_60_lines",
			termWidth:  160,
			termHeight: 60,
			mysisName:  "production-bot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test mysis
			mysis := MysisInfo{
				ID:              "test-id",
				Name:            tt.mysisName,
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "test_account",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			}

			// Calculate viewport height (same as app.go)
			headerHeight := 6
			footerHeight := 2
			vpHeight := tt.termHeight - headerHeight - footerHeight - 3
			if vpHeight < 5 {
				vpHeight = 5
			}
			vpWidth := tt.termWidth - 6 - 2

			// Create viewport
			vp := viewport.New(vpWidth, vpHeight)

			// Add some content
			logs := []LogEntry{
				{Role: "user", Source: "direct", Content: "Test message", Timestamp: time.Now()},
			}
			var contentLines []string
			for _, log := range logs {
				lines := renderLogEntryImpl(log, tt.termWidth-4, false, 0)
				contentLines = append(contentLines, lines...)
			}
			vp.SetContent(strings.Join(contentLines, "\n"))
			vp.GotoTop()

			// Render focus view
			output := RenderFocusViewWithViewport(mysis, vp, tt.termWidth, false, "⬡", false, len(contentLines), 1, 1, 0, nil, nil)

			// Split into lines
			lines := strings.Split(output, "\n")

			// Verify output is not empty
			if output == "" {
				t.Fatal("Focus view output is empty")
			}

			// Verify we have lines
			if len(lines) == 0 {
				t.Fatal("Focus view has no lines")
			}

			// Verify first line contains header elements
			firstLine := lines[0]

			// Check for header decorations
			if !strings.Contains(firstLine, "⬥") {
				t.Errorf("First line does not contain header decoration '⬥'\nFirst line: %s", firstLine)
			}

			// Check for mysis name marker
			if !strings.Contains(firstLine, "⬡") {
				t.Errorf("First line does not contain mysis marker '⬡'\nFirst line: %s", firstLine)
			}

			// Check for "MYSIS:" text
			if !strings.Contains(firstLine, "MYSIS:") {
				t.Errorf("First line does not contain 'MYSIS:' text\nFirst line: %s", firstLine)
			}

			// Check for mysis name
			if !strings.Contains(firstLine, tt.mysisName) {
				t.Errorf("First line does not contain mysis name '%s'\nFirst line: %s", tt.mysisName, firstLine)
			}

			// Check for position indicator (1/1)
			if !strings.Contains(firstLine, "(1/1)") {
				t.Errorf("First line does not contain position indicator '(1/1)'\nFirst line: %s", firstLine)
			}

			t.Logf("Header successfully found at terminal height %d:\n%s", tt.termHeight, firstLine)
		})
	}
}
