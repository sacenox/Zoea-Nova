package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/xonecas/zoea-nova/internal/constants"
)

// TestToolCallsRendering tests the rendering of assistant messages containing [TOOL_CALLS].
func TestToolCallsRendering(t *testing.T) {
	defer setupGoldenTest(t)()

	// Current tick for timestamp formatting
	currentTick := int64(42000)

	testCases := []struct {
		name    string
		content string
		verbose bool
		width   int
	}{
		{
			name: "single_tool_call",
			content: constants.ToolCallStoragePrefix +
				"call_abc123:get_ship:{}",
			verbose: false,
			width:   120,
		},
		{
			name: "multiple_tool_calls",
			content: constants.ToolCallStoragePrefix +
				"call_1:get_ship:{}|call_2:get_system:{\"system_id\":\"sol\"}|call_3:mine:{}",
			verbose: false,
			width:   120,
		},
		{
			name: "tool_call_with_string_params",
			content: constants.ToolCallStoragePrefix +
				"call_xyz:travel:{\"destination\":\"sol_base\"}",
			verbose: false,
			width:   120,
		},
		{
			name: "tool_call_with_number_params",
			content: constants.ToolCallStoragePrefix +
				"call_123:buy:{\"item_id\":42,\"quantity\":10,\"price\":150.5}",
			verbose: false,
			width:   120,
		},
		{
			name: "tool_call_with_array_params",
			content: constants.ToolCallStoragePrefix +
				"call_arr:scan:{\"targets\":[\"ship_001\",\"ship_002\",\"ship_003\"]}",
			verbose: false,
			width:   120,
		},
		{
			name: "tool_call_with_object_params",
			content: constants.ToolCallStoragePrefix +
				"call_obj:configure:{\"settings\":{\"speed\":\"fast\",\"mode\":\"aggressive\",\"target\":{\"x\":100,\"y\":200}}}",
			verbose: false,
			width:   120,
		},
		{
			name: "long_tool_name",
			content: constants.ToolCallStoragePrefix +
				"call_long:zoea_search_messages:{\"mysis_id\":\"abc123\",\"query\":\"ore\",\"limit\":20}",
			verbose: false,
			width:   120,
		},
		{
			name: "tool_call_verbose_mode",
			content: constants.ToolCallStoragePrefix +
				"call_v:get_poi:{\"poi_id\":\"asteroid_field_alpha\"}",
			verbose: true,
			width:   120,
		},
		{
			name: "tool_call_narrow_width",
			content: constants.ToolCallStoragePrefix +
				"call_narrow:travel:{\"destination\":\"sol_base\"}",
			verbose: false,
			width:   80,
		},
		{
			name: "tool_call_very_narrow_width",
			content: constants.ToolCallStoragePrefix +
				"call_vn:get_ship:{}",
			verbose: false,
			width:   60,
		},
		{
			name: "empty_tool_arguments",
			content: constants.ToolCallStoragePrefix +
				"call_empty1:get_notifications:{}|call_empty2:get_status:{}",
			verbose: false,
			width:   120,
		},
		{
			name: "complex_nested_json",
			content: constants.ToolCallStoragePrefix +
				"call_complex:execute:{\"plan\":{\"steps\":[{\"action\":\"travel\",\"target\":\"sol\"},{\"action\":\"mine\",\"resource\":\"iron\"}],\"priority\":\"high\"},\"timeout\":3600}",
			verbose: false,
			width:   120,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   tc.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 14, 30, 45, 0, time.UTC),
			}

			output := renderLogEntryImpl(entry, tc.width-6, tc.verbose, currentTick)
			rendered := joinLines(output)

			// ANSI variant
			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(rendered))
			})

			// Stripped variant
			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(rendered)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// TestToolCallsWithReasoning tests tool calls combined with reasoning output.
func TestToolCallsWithReasoning(t *testing.T) {
	defer setupGoldenTest(t)()

	currentTick := int64(42000)

	testCases := []struct {
		name      string
		content   string
		reasoning string
		verbose   bool
		width     int
	}{
		{
			name: "single_tool_with_reasoning",
			content: constants.ToolCallStoragePrefix +
				"call_r1:get_ship:{}",
			reasoning: "I need to check the ship's current status before deciding on the next action.",
			verbose:   false,
			width:     120,
		},
		{
			name: "single_tool_with_reasoning_verbose",
			content: constants.ToolCallStoragePrefix +
				"call_r2:travel:{\"destination\":\"sol_base\"}",
			reasoning: "The ship is currently at coordinates (150, 200) and needs to travel to sol_base to sell the ore. This will take approximately 5 ticks and consume fuel.",
			verbose:   true,
			width:     120,
		},
		{
			name: "multiple_tools_with_long_reasoning",
			content: constants.ToolCallStoragePrefix +
				"call_m1:get_system:{}|call_m2:get_poi:{}|call_m3:mine:{}",
			reasoning: "First, I'll check the current system to understand what resources are available. Then I'll get the point of interest data to find the best mining location. Finally, I'll execute the mining operation to gather resources. This is a strategic three-step approach that maximizes efficiency.",
			verbose:   false,
			width:     120,
		},
		{
			name: "tool_with_multiline_reasoning",
			content: constants.ToolCallStoragePrefix +
				"call_ml:scan:{\"range\":1000}",
			reasoning: "Step 1: Scan the area for potential threats\nStep 2: Identify resource-rich locations\nStep 3: Plan optimal mining route\nStep 4: Execute mining sequence",
			verbose:   false,
			width:     120,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   tc.content,
				Reasoning: tc.reasoning,
				Timestamp: time.Date(2026, 2, 6, 14, 30, 45, 0, time.UTC),
			}

			output := renderLogEntryImpl(entry, tc.width-6, tc.verbose, currentTick)
			rendered := joinLines(output)

			// ANSI variant
			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(rendered))
			})

			// Stripped variant
			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(rendered)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// TestToolCallsEdgeCases tests edge cases in tool call rendering.
func TestToolCallsEdgeCases(t *testing.T) {
	defer setupGoldenTest(t)()

	currentTick := int64(42000)

	testCases := []struct {
		name    string
		content string
		verbose bool
		width   int
	}{
		{
			name:    "malformed_no_prefix",
			content: "call_bad:get_ship:{}",
			verbose: false,
			width:   120,
		},
		{
			name:    "empty_after_prefix",
			content: constants.ToolCallStoragePrefix,
			verbose: false,
			width:   120,
		},
		{
			name:    "single_field_incomplete",
			content: constants.ToolCallStoragePrefix + "call_only",
			verbose: false,
			width:   120,
		},
		{
			name:    "two_fields_incomplete",
			content: constants.ToolCallStoragePrefix + "call_id:get_ship",
			verbose: false,
			width:   120,
		},
		{
			name: "invalid_json_args",
			content: constants.ToolCallStoragePrefix +
				"call_invalid:get_ship:{not valid json}",
			verbose: false,
			width:   120,
		},
		{
			name: "unicode_in_tool_args",
			content: constants.ToolCallStoragePrefix +
				"call_unicode:send_message:{\"message\":\"Hello 世界 🚀\"}",
			verbose: false,
			width:   120,
		},
		{
			name: "very_long_tool_arguments",
			content: constants.ToolCallStoragePrefix +
				"call_long:execute:{\"data\":\"" + string(make([]byte, 500)) + "\"}",
			verbose: false,
			width:   120,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   tc.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 14, 30, 45, 0, time.UTC),
			}

			output := renderLogEntryImpl(entry, tc.width-6, tc.verbose, currentTick)
			rendered := joinLines(output)

			// ANSI variant
			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(rendered))
			})

			// Stripped variant
			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(rendered)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// joinLines joins a slice of strings with newlines.
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
