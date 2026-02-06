package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
)

// TestMessageFrameLayout tests the conversation log message frame structure.
// This test verifies that:
// 1. Timestamp and message type label are on their own line
// 2. Content starts on the next line (not on same line as timestamp)
// 3. All message types render correctly with proper frame structure
func TestMessageFrameLayout(t *testing.T) {
	defer setupGoldenTest(t)()

	// Fixed timestamp for deterministic output
	fixedTime := time.Date(2026, 2, 6, 14, 30, 0, 0, time.UTC)
	const currentTick = int64(42337)
	const maxWidth = 100

	tests := []struct {
		name    string
		entry   LogEntry
		verbose bool
	}{
		// System message
		{
			name: "system_message_short",
			entry: LogEntry{
				Role:      "system",
				Source:    "system",
				Content:   "You are a helpful assistant.",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		{
			name: "system_message_long",
			entry: LogEntry{
				Role:      "system",
				Source:    "system",
				Content:   "You are a helpful assistant. Your goal is to help users navigate the SpaceMolt universe. Always check your status before taking actions. Use the captain's log for persistent memory.",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		// User direct message
		{
			name: "user_direct_short",
			entry: LogEntry{
				Role:      "user",
				Source:    "direct",
				Content:   "Check your cargo",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		{
			name: "user_direct_long",
			entry: LogEntry{
				Role:      "user",
				Source:    "direct",
				Content:   "Travel to the nearest mining station and sell all your ore. After that, buy fuel and repair kits for the next mission. Make sure to check prices before purchasing.",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		// User broadcast (self)
		{
			name: "user_broadcast_self_short",
			entry: LogEntry{
				Role:      "user",
				Source:    "broadcast_self",
				Content:   "All units: mine asteroids",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		{
			name: "user_broadcast_self_long",
			entry: LogEntry{
				Role:      "user",
				Source:    "broadcast_self",
				Content:   "All units: proceed to sector 7 and begin mining operations. Prioritize iron and copper ore. Report back when cargo is full or if you encounter any hostiles. Stay in formation and watch for pirates.",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		// Swarm broadcast (from another mysis)
		{
			name: "swarm_broadcast_short",
			entry: LogEntry{
				Role:       "user",
				Source:     "broadcast",
				SenderID:   "mysis-123",
				SenderName: "alpha",
				Content:    "Enemy spotted in sector 5",
				Timestamp:  fixedTime,
			},
			verbose: false,
		},
		{
			name: "swarm_broadcast_long",
			entry: LogEntry{
				Role:       "user",
				Source:     "broadcast",
				SenderID:   "mysis-456",
				SenderName: "beta-prime",
				Content:    "I've discovered a rich asteroid field at coordinates [100, 200, 300]. High concentrations of rare ore detected. Recommend all mining units converge on this location. I'm claiming the northern quadrant for efficiency.",
				Timestamp:  fixedTime,
			},
			verbose: false,
		},
		// Assistant message
		{
			name: "assistant_message_short",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "Checking cargo now.",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		{
			name: "assistant_message_long",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "I'll travel to the nearest mining station to sell the ore. First, let me check my current location and find the closest station with good ore prices. Then I'll plan the route and execute the travel command.",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		// Assistant with reasoning (verbose off)
		{
			name: "assistant_with_reasoning_verbose_off",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "I'll check my cargo status first.",
				Reasoning: "The user asked me to check cargo. I need to verify what items I'm carrying and how much space is available. This will help me plan my next actions. I should use the get_cargo tool to retrieve this information. The tool requires no parameters and will return a list of items in my cargo hold along with capacity information.",
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		// Assistant with reasoning (verbose on)
		{
			name: "assistant_with_reasoning_verbose_on",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "I'll check my cargo status first.",
				Reasoning: "The user asked me to check cargo. I need to verify what items I'm carrying and how much space is available. This will help me plan my next actions. I should use the get_cargo tool to retrieve this information. The tool requires no parameters and will return a list of items in my cargo hold along with capacity information.",
				Timestamp: fixedTime,
			},
			verbose: true,
		},
		// Tool call
		{
			name: "tool_call_short",
			entry: LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   `call_abc123: {"status": "success"}`,
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		{
			name: "tool_call_json_tree_verbose_off",
			entry: LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   `call_xyz789: {"ship": {"name": "Crab Cruiser", "hull": 100, "fuel": 500, "cargo": {"capacity": 1000, "used": 250, "items": [{"name": "Iron Ore", "quantity": 100}, {"name": "Copper Ore", "quantity": 150}]}}}`,
				Timestamp: fixedTime,
			},
			verbose: false,
		},
		{
			name: "tool_call_json_tree_verbose_on",
			entry: LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   `call_xyz789: {"ship": {"name": "Crab Cruiser", "hull": 100, "fuel": 500, "cargo": {"capacity": 1000, "used": 250, "items": [{"name": "Iron Ore", "quantity": 100}, {"name": "Copper Ore", "quantity": 150}]}}}`,
				Timestamp: fixedTime,
			},
			verbose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Render the log entry
			lines := renderLogEntryImpl(tt.entry, maxWidth, tt.verbose, currentTick)

			// Join lines to create output
			output := ""
			for _, line := range lines {
				output += line + "\n"
			}

			// Test ANSI variant (with color codes)
			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(output))
			})

			// Test Stripped variant (content only, no ANSI)
			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(output)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// TestMessageFrameStructure tests the structural requirements of message frames.
// This is a unit test (not golden) to explicitly verify layout rules.
func TestMessageFrameStructure(t *testing.T) {
	fixedTime := time.Date(2026, 2, 6, 14, 30, 0, 0, time.UTC)
	const currentTick = int64(42337)
	const maxWidth = 100

	tests := []struct {
		name  string
		entry LogEntry
	}{
		{
			name: "system_message",
			entry: LogEntry{
				Role:      "system",
				Source:    "system",
				Content:   "Test content",
				Timestamp: fixedTime,
			},
		},
		{
			name: "user_message",
			entry: LogEntry{
				Role:      "user",
				Source:    "direct",
				Content:   "Test content",
				Timestamp: fixedTime,
			},
		},
		{
			name: "assistant_message",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "Test content",
				Timestamp: fixedTime,
			},
		},
		{
			name: "tool_message",
			entry: LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   "Test content",
				Timestamp: fixedTime,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := renderLogEntryImpl(tt.entry, maxWidth, false, currentTick)

			// Verify minimum structure
			if len(lines) < 2 {
				t.Fatalf("Expected at least 2 lines (timestamp line + content line), got %d", len(lines))
			}

			// Strip ANSI codes for content checking
			stripped := make([]string, len(lines))
			for i, line := range lines {
				stripped[i] = stripANSIForGolden(line)
			}

			// Line 0 should be empty padding (may contain spaces for width)
			trimmedLine0 := strings.TrimSpace(stripped[0])
			if trimmedLine0 != "" {
				t.Errorf("Line 0 should be empty padding (possibly with spaces), got: %q", stripped[0])
			}

			// Line 1 should contain timestamp, role prefix, and horizontal separator (no content)
			// Expected format: " T42337 ⬡ [14:30] ROLE: ────────────────────"
			// Content should start on line 2
			if len(stripped) > 1 {
				headerLine := stripped[1]

				// Check that timestamp format is present
				if !containsAny(headerLine, []string{"T42337", "[14:30]"}) {
					t.Errorf("Line 1 should contain timestamp (T42337 or [14:30]), got: %q", headerLine)
				}

				// Check that role prefix is present
				var expectedRole string
				switch tt.entry.Role {
				case "system":
					expectedRole = "SYS:"
				case "user":
					expectedRole = "YOU:"
				case "assistant":
					expectedRole = "AI:"
				case "tool":
					expectedRole = "TOOL:"
				}
				if !containsAny(headerLine, []string{expectedRole}) {
					t.Errorf("Line 1 should contain role prefix %q, got: %q", expectedRole, headerLine)
				}

				// Check that horizontal separator is present (indicates no content on this line)
				if !containsAny(headerLine, []string{"─", "──", "───"}) {
					t.Errorf("Line 1 should contain horizontal separator, got: %q", headerLine)
				}

				// Content should NOT be on this line (header line)
				if containsAny(headerLine, []string{"Test content"}) {
					t.Errorf("Line 1 should NOT contain message content (should be on line 2), got: %q", headerLine)
				}
			}

			// Line 2 should contain the content
			if len(stripped) > 2 {
				contentLine := stripped[2]
				if !containsAny(contentLine, []string{"Test content"}) {
					t.Errorf("Line 2 should contain message content, got: %q", contentLine)
				}
			}
		})
	}
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
