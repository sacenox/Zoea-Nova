package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
)

// TestReasoningWithTimestamp verifies reasoning displays timestamp like other messages.
func TestReasoningWithTimestamp(t *testing.T) {
	defer setupGoldenTest(t)()

	testCases := []struct {
		name     string
		entry    LogEntry
		verbose  bool
		maxWidth int
		tick     int64
	}{
		{
			name: "verbose_mode_with_timestamp",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "I will mine the asteroid.",
				Reasoning: "The asteroid contains valuable ore. Mining will increase our resources.",
				Timestamp: time.Date(2026, 2, 6, 15, 30, 0, 0, time.UTC),
			},
			verbose:  true,
			maxWidth: 100,
			tick:     42337,
		},
		{
			name: "non_verbose_mode_with_timestamp",
			entry: LogEntry{
				Role:    "assistant",
				Source:  "llm",
				Content: "I will travel to the mining station.",
				Reasoning: strings.Join([]string{
					"First line of reasoning",
					"This is a very long middle section that should be truncated",
					"More middle content",
					"Even more middle content",
					"Second to last line",
					"Last line of reasoning",
				}, "\n"),
				Timestamp: time.Date(2026, 2, 6, 16, 45, 0, 0, time.UTC),
			},
			verbose:  false,
			maxWidth: 100,
			tick:     42500,
		},
		{
			name: "short_reasoning_no_truncation",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "Mining complete.",
				Reasoning: "Short reasoning that won't be truncated.",
				Timestamp: time.Date(2026, 2, 6, 17, 10, 0, 0, time.UTC),
			},
			verbose:  false,
			maxWidth: 100,
			tick:     42600,
		},
		{
			name: "narrow_terminal",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "Action.",
				Reasoning: "This reasoning text is displayed in a narrow terminal and should wrap properly.",
				Timestamp: time.Date(2026, 2, 6, 18, 0, 0, 0, time.UTC),
			},
			verbose:  true,
			maxWidth: 60,
			tick:     42700,
		},
		{
			name: "zero_timestamp",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "No timestamp.",
				Reasoning: "This entry has no timestamp.",
				Timestamp: time.Time{},
			},
			verbose:  true,
			maxWidth: 100,
			tick:     0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := renderLogEntryImpl(tc.entry, tc.maxWidth, tc.verbose, tc.tick)
			rendered := strings.Join(output, "\n")

			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(rendered))
			})

			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(rendered)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// TestReasoningTimestampFormat verifies timestamp format in reasoning matches main content.
func TestReasoningTimestampFormat(t *testing.T) {
	entry := LogEntry{
		Role:      "assistant",
		Source:    "llm",
		Content:   "Test content",
		Reasoning: "Test reasoning",
		Timestamp: time.Date(2026, 2, 6, 15, 30, 0, 0, time.UTC),
	}

	output := renderLogEntryImpl(entry, 100, true, 42337)
	rendered := strings.Join(output, "\n")
	stripped := stripANSIForGolden(rendered)

	// Check that reasoning line contains timestamp components
	lines := strings.Split(stripped, "\n")
	var reasoningLine string
	for _, line := range lines {
		if strings.Contains(line, "REASONING:") {
			reasoningLine = line
			break
		}
	}

	if reasoningLine == "" {
		t.Fatal("No REASONING line found in output")
	}

	// Verify timestamp components are present
	if !strings.Contains(reasoningLine, "T42337") {
		t.Errorf("REASONING line missing tick: %q", reasoningLine)
	}
	if !strings.Contains(reasoningLine, "[15:30]") {
		t.Errorf("REASONING line missing time: %q", reasoningLine)
	}
	if !strings.Contains(reasoningLine, "⬡") {
		t.Errorf("REASONING line missing hexagon separator: %q", reasoningLine)
	}
}

// TestReasoningVerboseToggle verifies verbose mode affects reasoning display.
func TestReasoningVerboseToggle(t *testing.T) {
	entry := LogEntry{
		Role:    "assistant",
		Source:  "llm",
		Content: "Test",
		Reasoning: strings.Join([]string{
			"Line 1",
			"Line 2",
			"Line 3",
			"Line 4",
			"Line 5",
			"Line 6",
		}, "\n"),
		Timestamp: time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
	}

	// Verbose mode should show all lines
	verboseOutput := renderLogEntryImpl(entry, 100, true, 42000)
	verboseRendered := strings.Join(verboseOutput, "\n")
	verboseStripped := stripANSIForGolden(verboseRendered)

	// Non-verbose mode should truncate
	nonVerboseOutput := renderLogEntryImpl(entry, 100, false, 42000)
	nonVerboseRendered := strings.Join(nonVerboseOutput, "\n")
	nonVerboseStripped := stripANSIForGolden(nonVerboseRendered)

	// Verbose should be longer
	if len(verboseStripped) <= len(nonVerboseStripped) {
		t.Error("Verbose mode should produce longer output than non-verbose")
	}

	// Non-verbose should contain truncation indicator
	if !strings.Contains(nonVerboseStripped, "[") || !strings.Contains(nonVerboseStripped, "more]") {
		t.Error("Non-verbose mode should contain truncation indicator like '[3 more]'")
	}

	// Verbose should contain all lines
	for i := 1; i <= 6; i++ {
		lineText := "Line " + string(rune('0'+i))
		if !strings.Contains(verboseStripped, lineText) {
			t.Errorf("Verbose mode missing line: %s", lineText)
		}
	}
}

// TestReasoningContentLengths verifies reasoning handles various content lengths.
func TestReasoningContentLengths(t *testing.T) {
	defer setupGoldenTest(t)()

	testCases := []struct {
		name      string
		reasoning string
		verbose   bool
	}{
		{
			name:      "empty_reasoning",
			reasoning: "",
			verbose:   true,
		},
		{
			name:      "single_word",
			reasoning: "Think.",
			verbose:   true,
		},
		{
			name:      "single_line",
			reasoning: "This is a single line of reasoning.",
			verbose:   true,
		},
		{
			name: "three_lines_exact_threshold",
			reasoning: strings.Join([]string{
				"Line 1",
				"Line 2",
				"Line 3",
			}, "\n"),
			verbose: false,
		},
		{
			name: "four_lines_triggers_truncation",
			reasoning: strings.Join([]string{
				"Line 1",
				"Line 2",
				"Line 3",
				"Line 4",
			}, "\n"),
			verbose: false,
		},
		{
			name: "very_long_reasoning",
			reasoning: strings.Join([]string{
				"First line of very long reasoning",
				"Second line",
				"Third line",
				"Fourth line",
				"Fifth line",
				"Sixth line",
				"Seventh line",
				"Eighth line",
				"Ninth line",
				"Tenth and final line",
			}, "\n"),
			verbose: false,
		},
		{
			name: "very_long_reasoning_verbose",
			reasoning: strings.Join([]string{
				"First line of very long reasoning in verbose mode",
				"Second line",
				"Third line",
				"Fourth line",
				"Fifth line",
				"Sixth line",
				"Seventh line",
				"Eighth line",
				"Ninth line",
				"Tenth and final line",
			}, "\n"),
			verbose: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "Action taken",
				Reasoning: tc.reasoning,
				Timestamp: time.Date(2026, 2, 6, 14, 0, 0, 0, time.UTC),
			}

			output := renderLogEntryImpl(entry, 100, tc.verbose, 42200)
			rendered := strings.Join(output, "\n")

			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(rendered))
			})

			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(rendered)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// TestReasoningNoTimestamp verifies reasoning with zero timestamp shows placeholder.
func TestReasoningNoTimestamp(t *testing.T) {
	entry := LogEntry{
		Role:      "assistant",
		Source:    "llm",
		Content:   "Content",
		Reasoning: "Reasoning text",
		Timestamp: time.Time{}, // Zero timestamp
	}

	output := renderLogEntryImpl(entry, 100, true, 0)
	rendered := strings.Join(output, "\n")
	stripped := stripANSIForGolden(rendered)

	// Find reasoning line
	lines := strings.Split(stripped, "\n")
	var reasoningLine string
	for _, line := range lines {
		if strings.Contains(line, "REASONING:") {
			reasoningLine = line
			break
		}
	}

	if reasoningLine == "" {
		t.Fatal("No REASONING line found in output")
	}

	// Should contain placeholder timestamp
	if !strings.Contains(reasoningLine, "T0") {
		t.Errorf("Expected T0 placeholder, got: %q", reasoningLine)
	}
	if !strings.Contains(reasoningLine, "[--:--]") {
		t.Errorf("Expected [--:--] placeholder, got: %q", reasoningLine)
	}
}
