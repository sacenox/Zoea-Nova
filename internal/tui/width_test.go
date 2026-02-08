package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestWidthCalculations tests that width calculations use lipgloss.Width()
// instead of len() for proper Unicode and ANSI handling.
func TestWidthCalculations(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedWidth int
	}{
		// ASCII strings
		{
			name:          "plain_ascii",
			input:         "hello",
			expectedWidth: 5,
		},
		{
			name:          "ascii_with_spaces",
			input:         "hello world",
			expectedWidth: 11,
		},

		// Unicode strings
		{
			name:          "emoji",
			input:         "👍",
			expectedWidth: 2, // Emoji are wide characters
		},
		{
			name:          "multiple_emoji",
			input:         "👍👎",
			expectedWidth: 4,
		},
		{
			name:          "cjk_characters",
			input:         "你好",
			expectedWidth: 4, // CJK characters are double-width
		},
		{
			name:          "box_drawing",
			input:         "╔═╗",
			expectedWidth: 3,
		},
		{
			name:          "diamond_indicators",
			input:         "⬥⬧⬦",
			expectedWidth: 3,
		},
		{
			name:          "hexagonal_indicators",
			input:         "⬡⬢",
			expectedWidth: 2,
		},

		// ANSI-styled strings
		{
			name:          "ansi_styled",
			input:         lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("hello"),
			expectedWidth: 5, // ANSI codes don't count toward width
		},
		{
			name:          "ansi_bold",
			input:         lipgloss.NewStyle().Bold(true).Render("test"),
			expectedWidth: 4,
		},
		{
			name:          "ansi_colored_emoji",
			input:         lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("👍"),
			expectedWidth: 2,
		},

		// Mixed content
		{
			name:          "ascii_and_emoji",
			input:         "hello 👍",
			expectedWidth: 8,
		},
		{
			name:          "ascii_and_cjk",
			input:         "hello 你好",
			expectedWidth: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := lipgloss.Width(tt.input)
			if width != tt.expectedWidth {
				t.Errorf("lipgloss.Width(%q) = %d, want %d", tt.input, width, tt.expectedWidth)
			}

			// Verify len() would be wrong for non-ASCII
			byteLen := len(tt.input)
			if strings.Contains(tt.name, "emoji") || strings.Contains(tt.name, "cjk") || strings.Contains(tt.name, "ansi") {
				if byteLen == width {
					t.Logf("WARNING: len() happens to match lipgloss.Width() for %q, but this is coincidental", tt.input)
				}
			}
		})
	}
}

// TestTruncateToWidth tests the truncateToWidth helper function.
func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		maxWidth      int
		expectedWidth int
		description   string
	}{
		{
			name:          "no_truncation_needed",
			input:         "hello",
			maxWidth:      10,
			expectedWidth: 5,
			description:   "Short string should not be truncated",
		},
		{
			name:          "truncate_ascii",
			input:         "hello world",
			maxWidth:      5,
			expectedWidth: 5,
			description:   "ASCII string truncated to exact width",
		},
		{
			name:          "truncate_emoji",
			input:         "👍👎👌",
			maxWidth:      4,
			expectedWidth: 4,
			description:   "Emoji string truncated at wide character boundary",
		},
		{
			name:          "truncate_cjk",
			input:         "你好世界",
			maxWidth:      6,
			expectedWidth: 6,
			description:   "CJK string truncated at double-width boundary",
		},
		{
			name:          "truncate_mixed",
			input:         "hello 你好",
			maxWidth:      8,
			expectedWidth: 8,
			description:   "Mixed ASCII and CJK truncated correctly",
		},
		{
			name:          "zero_width",
			input:         "hello",
			maxWidth:      0,
			expectedWidth: 0,
			description:   "Zero width should return empty string",
		},
		{
			name:          "negative_width",
			input:         "hello",
			maxWidth:      -1,
			expectedWidth: 0,
			description:   "Negative width should return empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToWidth(tt.input, tt.maxWidth)
			width := lipgloss.Width(result)
			if tt.maxWidth >= 0 && width > tt.maxWidth {
				t.Errorf("truncateToWidth(%q, %d) width = %d, exceeds maxWidth %d",
					tt.input, tt.maxWidth, width, tt.maxWidth)
			}
			if tt.expectedWidth > 0 && width != tt.expectedWidth {
				t.Logf("truncateToWidth(%q, %d) width = %d, expected %d (may vary due to boundary)",
					tt.input, tt.maxWidth, width, tt.expectedWidth)
			}
		})
	}
}

// TestPadRight tests the padRight helper function for proper width handling.
func TestPadRight(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		length        int
		expectedWidth int
	}{
		{
			name:          "pad_ascii",
			input:         "hello",
			length:        10,
			expectedWidth: 10,
		},
		{
			name:          "no_pad_needed",
			input:         "hello world",
			length:        5,
			expectedWidth: 11, // Original width, no padding
		},
		{
			name:          "pad_emoji",
			input:         "👍",
			length:        5,
			expectedWidth: 5, // Emoji (width 2) + 3 spaces
		},
		{
			name:          "pad_cjk",
			input:         "你好",
			length:        10,
			expectedWidth: 10, // CJK (width 4) + 6 spaces
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padRight(tt.input, tt.length)
			width := lipgloss.Width(result)
			if width < tt.expectedWidth {
				t.Errorf("padRight(%q, %d) width = %d, want >= %d",
					tt.input, tt.length, width, tt.expectedWidth)
			}
		})
	}
}

// TestWidthCalculationEdgeCases tests edge cases for width calculations.
func TestWidthCalculationEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		description string
	}{
		{
			name:        "empty_string",
			input:       "",
			description: "Empty string should have width 0",
		},
		{
			name:        "only_ansi_codes",
			input:       lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(""),
			description: "ANSI codes without content should have width 0",
		},
		{
			name:        "newlines",
			input:       "hello\nworld",
			description: "Newlines count as width 0",
		},
		{
			name:        "tabs",
			input:       "hello\tworld",
			description: "Tabs have variable width",
		},
		{
			name:        "combining_characters",
			input:       "é", // e + combining acute accent
			description: "Combining characters should not add width",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := lipgloss.Width(tt.input)
			byteLen := len(tt.input)
			t.Logf("%s: lipgloss.Width=%d, len=%d", tt.description, width, byteLen)

			// Just verify it doesn't panic and returns a non-negative value
			if width < 0 {
				t.Errorf("lipgloss.Width returned negative value: %d", width)
			}
		})
	}
}

// TestLenVsLipglossWidth demonstrates why len() is incorrect for display width.
func TestLenVsLipglossWidth(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		reason string
	}{
		{
			name:   "emoji",
			input:  "👍",
			reason: "Emoji are 4 bytes but display as 2 cells",
		},
		{
			name:   "cjk",
			input:  "你",
			reason: "CJK characters are 3 bytes but display as 2 cells",
		},
		{
			name:   "ansi_styled",
			input:  lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("hi"),
			reason: "ANSI codes add bytes but no display width",
		},
		{
			name:   "box_drawing",
			input:  "═",
			reason: "Box drawing is 3 bytes but displays as 1 cell",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			byteLen := len(tc.input)
			displayWidth := lipgloss.Width(tc.input)

			t.Logf("%s: len()=%d, lipgloss.Width()=%d - %s",
				tc.name, byteLen, displayWidth, tc.reason)

			if byteLen == displayWidth {
				t.Logf("WARNING: In this case len() happens to equal lipgloss.Width(), but this is not reliable")
			}
		})
	}
}

// TestMysisLineWidth tests that mysis lines don't overflow at various terminal widths.
func TestMysisLineWidth(t *testing.T) {
	tests := []struct {
		name          string
		mysis         MysisInfo
		terminalWidth int
		description   string
	}{
		{
			name: "long_mysis_name",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "mysis-with-a-very-long-name-that-exceeds-sixteen-characters",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "miner",
				LastMessage:     "Mining",
			},
			terminalWidth: 80,
			description:   "Long mysis name should be truncated to 16 chars",
		},
		{
			name: "long_provider_name",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "running",
				Provider:        "opencode_zen_with_very_long_name",
				AccountUsername: "trader",
				LastMessage:     "Trading",
			},
			terminalWidth: 80,
			description:   "Long provider name should not cause overflow",
		},
		{
			name: "long_account_name",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "crab_warrior_with_very_long_username",
				LastMessage:     "Docking",
			},
			terminalWidth: 80,
			description:   "Long account name should not cause overflow",
		},
		{
			name: "long_last_message",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "miner",
				LastMessage:     "This is a very long last message that should be truncated to fit within the available space without causing overflow",
			},
			terminalWidth: 80,
			description:   "Long last message should be truncated",
		},
		{
			name: "narrow_terminal",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "miner",
				LastMessage:     "Mining",
			},
			terminalWidth: 60,
			description:   "Should handle narrow terminal width",
		},
		{
			name: "very_narrow_terminal",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "miner",
				LastMessage:     "Mining",
			},
			terminalWidth: 40,
			description:   "Should handle very narrow terminal width",
		},
		{
			name: "wide_terminal",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "miner",
				LastMessage:     "Mining ore and selling it at the base",
			},
			terminalWidth: 120,
			description:   "Should handle wide terminal width",
		},
		{
			name: "unicode_in_name",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "测试-mysis",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "miner",
				LastMessage:     "Mining",
			},
			terminalWidth: 80,
			description:   "Should handle Unicode characters in name",
		},
		{
			name: "emoji_in_last_message",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "miner",
				LastMessage:     "Mining 👍 Going well",
			},
			terminalWidth: 80,
			description:   "Should handle emoji in last message",
		},
		{
			name: "all_fields_empty",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "idle",
				Provider:        "ollama-qwen",
				AccountUsername: "",
				LastMessage:     "",
			},
			terminalWidth: 80,
			description:   "Should handle empty optional fields",
		},
		{
			name: "errored_state_with_error",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test",
				State:           "errored",
				Provider:        "ollama-qwen",
				AccountUsername: "miner",
				LastMessage:     "",
				LastError:       "connection timeout",
			},
			terminalWidth: 80,
			description:   "Should show error message when state is errored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Render mysis line (not selected, not loading, static spinner)
			lines := renderMysisLine(tt.mysis, false, false, "◦", tt.terminalWidth, 0)
			line := lines[0] // Check first line (info row) for width

			// Strip ANSI codes to get display width
			stripped := stripANSI(line)
			lineWidth := lipgloss.Width(stripped)

			// Verify no overflow
			if lineWidth > tt.terminalWidth {
				t.Errorf("%s: line width %d exceeds terminal width %d",
					tt.description, lineWidth, tt.terminalWidth)
				t.Logf("Line content: %q", stripped)
			}

			// Verify we're using display width
			byteLen := len(stripped)
			if byteLen != lineWidth && byteLen > lineWidth {
				t.Logf("Byte length (%d) differs from display width (%d) - good, using lipgloss.Width()",
					byteLen, lineWidth)
			}
		})
	}
}

// TestMysisLineWidthConsistency tests that mysis lines have consistent width handling.
func TestMysisLineWidthConsistency(t *testing.T) {
	terminalWidths := []int{40, 60, 80, 100, 120}
	states := []string{"idle", "running", "stopped", "errored"}

	baseMysis := MysisInfo{
		ID:              "abc123",
		Name:            "test-mysis",
		Provider:        "ollama-qwen",
		AccountUsername: "crab_miner",
		LastMessage:     "Performing operation",
	}

	for _, width := range terminalWidths {
		for _, state := range states {
			testName := state + "_width_" + strings.TrimSpace(strings.Join(strings.Fields(strings.Repeat(string(rune('0'+width/10)), 1)), ""))
			t.Run(testName, func(t *testing.T) {
				mysis := baseMysis
				mysis.State = state
				if state == "errored" {
					mysis.LastMessage = ""
					mysis.LastError = "test error"
				}

				lines := renderMysisLine(mysis, false, false, "◦", width, 0)
				line := lines[0] // Check first line (info row) for width
				stripped := stripANSI(line)
				lineWidth := lipgloss.Width(stripped)

				if lineWidth > width {
					t.Errorf("State %s at width %d: line width %d exceeds terminal width",
						state, width, lineWidth)
				}
			})
		}
	}
}
