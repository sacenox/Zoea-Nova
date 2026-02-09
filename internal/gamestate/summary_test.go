package gamestate

import (
	"strings"
	"testing"
)

func TestSnapshotLinesTUI_CompactFormat(t *testing.T) {
	testJSON := `{
		"ship": {
			"id": "ship-123",
			"position": {
				"x": 100,
				"y": 200,
				"z": 50
			}
		},
		"cargo": [
			{"item": "ore", "qty": 10},
			{"item": "fuel", "qty": 5}
		],
		"status": "docked"
	}`

	llmLines := SnapshotLines(testJSON)
	tuiLines := SnapshotLinesTUI(testJSON)

	// LLM format should have full dotted paths
	llmOutput := strings.Join(llmLines, "\n")
	if !strings.Contains(llmOutput, "ship.position.x: 100") {
		t.Errorf("LLM format should contain full dotted paths, got:\n%s", llmOutput)
	}

	// TUI format should be more compact
	tuiOutput := strings.Join(tuiLines, "\n")

	// Should preserve some context (dotted paths are OK, just more compact than LLM format)
	if !strings.Contains(tuiOutput, "ship.position.x: 100") {
		t.Errorf("TUI format should preserve minimal context paths, got:\n%s", tuiOutput)
	}

	// Should show array counts instead of expanding
	if !strings.Contains(tuiOutput, "[2 items]") {
		t.Errorf("TUI format should show array counts, got:\n%s", tuiOutput)
	}

	// Should contain simple top-level keys
	if !strings.Contains(tuiOutput, "status: docked") {
		t.Errorf("TUI format should contain simple key-value pairs, got:\n%s", tuiOutput)
	}

	// Key difference: arrays should NOT be expanded in TUI format
	if strings.Contains(tuiOutput, "cargo[0].item") {
		t.Errorf("TUI format should not expand arrays, got:\n%s", tuiOutput)
	}

	t.Logf("LLM format (%d lines):\n%s", len(llmLines), llmOutput)
	t.Logf("\nTUI format (%d lines):\n%s", len(tuiLines), tuiOutput)
}

func TestSnapshotLinesTUI_EmptyAndInvalid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   \n\t  ",
			expected: nil,
		},
		{
			name:     "invalid JSON",
			input:    "{not valid json}",
			expected: []string{"(invalid JSON)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SnapshotLinesTUI(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d", len(tt.expected), len(result))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}
