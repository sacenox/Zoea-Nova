package tui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
)

// TestJSONContentInToolResults tests tool results that contain JSON.
func TestJSONContentInToolResults(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name    string
		content string
		verbose bool
	}{
		{
			name:    "simple_json_object",
			content: `{"status": "success", "ore_collected": 42}`,
			verbose: false,
		},
		{
			name:    "json_array",
			content: `[{"id": 1, "type": "asteroid"}, {"id": 2, "type": "station"}]`,
			verbose: false,
		},
		{
			name:    "nested_json",
			content: `{"ship": {"id": "abc123", "cargo": {"ore": 10, "fuel": 50}}}`,
			verbose: false,
		},
		{
			name:    "json_with_tool_call_prefix",
			content: `call_abc123:{"status": "success", "message": "Mining complete"}`,
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   tt.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
			}

			output := strings.Join(renderLogEntryImpl(entry, 80, tt.verbose, 42000), "\n")

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

// TestJSONContentWithNestedJSON tests content that contains JSON strings (double-encoding scenario).
func TestJSONContentWithNestedJSON(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name    string
		content string
		verbose bool
	}{
		{
			name: "json_with_escaped_json_string",
			content: func() string {
				// Create a JSON object with a field containing JSON as a string
				inner := map[string]interface{}{"status": "ok"}
				innerJSON, _ := json.Marshal(inner)
				outer := map[string]interface{}{
					"response": string(innerJSON),
					"code":     200,
				}
				outerJSON, _ := json.Marshal(outer)
				return string(outerJSON)
			}(),
			verbose: false,
		},
		{
			name: "json_array_with_json_strings",
			content: func() string {
				// Array where each element is a JSON string
				arr := []string{
					`{"type": "error", "msg": "failed"}`,
					`{"type": "success", "msg": "ok"}`,
				}
				arrJSON, _ := json.Marshal(arr)
				return string(arrJSON)
			}(),
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   tt.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
			}

			output := strings.Join(renderLogEntryImpl(entry, 80, tt.verbose, 42000), "\n")

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

// TestJSONContentInAssistantMessages tests assistant messages that include JSON examples.
func TestJSONContentInAssistantMessages(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name    string
		role    string
		content string
		verbose bool
	}{
		{
			name:    "assistant_with_json_in_text",
			role:    "assistant",
			content: `I found the following data: {"ore": 42, "fuel": 50}. Should I proceed?`,
			verbose: false,
		},
		{
			name:    "assistant_with_json_code_block",
			role:    "assistant",
			content: "Here's the status:\n```json\n{\"status\": \"running\", \"health\": 100}\n```\nLooks good!",
			verbose: false,
		},
		{
			name:    "user_with_json_example",
			role:    "user",
			content: `Please parse this: {"command": "mine", "target": "asteroid_1"}`,
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      tt.role,
				Source:    "direct",
				Content:   tt.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
			}

			output := strings.Join(renderLogEntryImpl(entry, 80, tt.verbose, 42000), "\n")

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

// TestJSONContentMixedWithText tests mixed content with JSON and text.
func TestJSONContentMixedWithText(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name    string
		content string
		verbose bool
	}{
		{
			name:    "text_before_json",
			content: `Mining successful! Result: {"ore": 10, "location": "asteroid_42"}`,
			verbose: false,
		},
		{
			name:    "json_with_text_after",
			content: `{"status": "complete"} - Operation finished successfully`,
			verbose: false,
		},
		{
			name:    "multiple_json_snippets",
			content: `First: {"a": 1} and second: {"b": 2} both valid`,
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   tt.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
			}

			output := strings.Join(renderLogEntryImpl(entry, 80, tt.verbose, 42000), "\n")

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

// TestJSONContentMalformed tests malformed JSON in content.
func TestJSONContentMalformed(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name    string
		content string
		verbose bool
	}{
		{
			name:    "incomplete_json_object",
			content: `{"status": "running", "ore":`,
			verbose: false,
		},
		{
			name:    "invalid_json_syntax",
			content: `{invalid: json, missing: "quotes"}`,
			verbose: false,
		},
		{
			name:    "json_with_trailing_comma",
			content: `{"a": 1, "b": 2,}`,
			verbose: false,
		},
		{
			name:    "looks_like_json_but_not",
			content: `{this looks like json but isn't valid}`,
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   tt.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
			}

			output := strings.Join(renderLogEntryImpl(entry, 80, tt.verbose, 42000), "\n")

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

// TestJSONContentVeryLarge tests very large JSON objects.
func TestJSONContentVeryLarge(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name    string
		content string
		verbose bool
	}{
		{
			name: "large_array_verbose_off",
			content: func() string {
				items := make([]map[string]interface{}, 100)
				for i := 0; i < 100; i++ {
					items[i] = map[string]interface{}{
						"id":    i,
						"value": i * 100,
						"name":  "item_" + string(rune('A'+i%26)),
					}
				}
				jsonBytes, _ := json.Marshal(items)
				return string(jsonBytes)
			}(),
			verbose: false,
		},
		{
			name: "large_array_verbose_on",
			content: func() string {
				items := make([]map[string]interface{}, 20)
				for i := 0; i < 20; i++ {
					items[i] = map[string]interface{}{
						"id":    i,
						"value": i * 100,
					}
				}
				jsonBytes, _ := json.Marshal(items)
				return string(jsonBytes)
			}(),
			verbose: true,
		},
		{
			name: "deeply_nested_json",
			content: func() string {
				// Create deeply nested structure: 10 levels deep
				var data interface{} = "deepest"
				for i := 0; i < 10; i++ {
					data = map[string]interface{}{
						"level": i,
						"next":  data,
					}
				}
				jsonBytes, _ := json.Marshal(data)
				return string(jsonBytes)
			}(),
			verbose: false,
		},
		{
			name: "wide_object_many_fields",
			content: func() string {
				obj := make(map[string]interface{}, 50)
				for i := 0; i < 50; i++ {
					key := "field_" + string(rune('a'+i%26)) + string(rune('0'+i/26))
					obj[key] = i * 10
				}
				jsonBytes, _ := json.Marshal(obj)
				return string(jsonBytes)
			}(),
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   tt.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
			}

			output := strings.Join(renderLogEntryImpl(entry, 80, tt.verbose, 42000), "\n")

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

// TestJSONContentEdgeCases tests edge cases in JSON rendering.
func TestJSONContentEdgeCases(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name    string
		content string
		verbose bool
	}{
		{
			name:    "empty_json_object",
			content: `{}`,
			verbose: false,
		},
		{
			name:    "empty_json_array",
			content: `[]`,
			verbose: false,
		},
		{
			name:    "json_with_null_values",
			content: `{"a": null, "b": null, "c": 123}`,
			verbose: false,
		},
		{
			name:    "json_with_boolean_values",
			content: `{"enabled": true, "disabled": false, "count": 0}`,
			verbose: false,
		},
		{
			name:    "json_with_very_long_string",
			content: `{"description": "` + strings.Repeat("a", 500) + `"}`,
			verbose: false,
		},
		{
			name:    "json_with_unicode_characters",
			content: `{"emoji": "🚀⬡⬥", "text": "Hello 世界"}`,
			verbose: false,
		},
		{
			name:    "json_single_value",
			content: `"just a string"`,
			verbose: false,
		},
		{
			name:    "json_single_number",
			content: `42`,
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   tt.content,
				Reasoning: "",
				Timestamp: time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
			}

			output := strings.Join(renderLogEntryImpl(entry, 80, tt.verbose, 42000), "\n")

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

// TestDoubleParsingIssue specifically tests the double-parsing scenario.
func TestDoubleParsingIssue(t *testing.T) {
	// This test demonstrates the potential double-parsing issue:
	// When tool result content is already JSON, and we try to parse it again,
	// what happens?

	t.Run("tool_result_already_parsed", func(t *testing.T) {
		// Simulate what happens when content is JSON
		jsonContent := `{"status": "success", "data": {"ore": 42}}`

		// First parse (what renderJSONTree does)
		tree1, err1 := renderJSONTree(jsonContent, false, 80)
		if err1 != nil {
			t.Fatalf("First parse failed: %v", err1)
		}

		// Second parse (if we accidentally parse the tree output)
		_, err2 := renderJSONTree(tree1, false, 80)
		if err2 == nil {
			t.Error("Second parse should fail - tree output is not valid JSON")
		}

		// Verify tree1 is NOT valid JSON
		var dummy interface{}
		if json.Unmarshal([]byte(tree1), &dummy) == nil {
			t.Error("Tree output should not be valid JSON (contains Unicode box chars)")
		}
	})

	t.Run("tool_result_with_call_prefix", func(t *testing.T) {
		// Test the tool call ID prefix stripping
		content := `call_abc123:{"status": "success"}`

		// The isJSON function should detect this as JSON
		if !isJSON(content) {
			t.Error("isJSON should detect tool result with call prefix")
		}

		// renderJSONTree does NOT strip the prefix - that's done in renderLogEntryImpl
		// So calling it directly with the prefix should fail
		_, err := renderJSONTree(content, false, 80)
		if err == nil {
			t.Error("renderJSONTree should fail on content with call prefix (prefix stripping is done in renderLogEntryImpl)")
		}

		// The prefix is stripped in renderLogEntryImpl before calling renderJSONTree
		// Let's verify the logic in focus.go lines 330-334 works
		if idx := strings.Index(content, ":"); idx > 0 && strings.HasPrefix(content, "call_") {
			jsonContent := content[idx+1:]
			_, err2 := renderJSONTree(jsonContent, false, 80)
			if err2 != nil {
				t.Errorf("Stripped content should parse: %v", err2)
			}
		}
	})

	t.Run("nested_json_string_not_double_parsed", func(t *testing.T) {
		// Create JSON with a field containing JSON as a STRING (escaped)
		inner := `{"nested": "value"}`
		innerEscaped, _ := json.Marshal(inner) // This escapes it: "\"nested\": \"value\"}"
		outer := `{"data": ` + string(innerEscaped) + `}`

		tree, err := renderJSONTree(outer, false, 80)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		// The tree should show "data" field with an escaped JSON STRING value
		// NOT a parsed nested object
		if strings.Contains(tree, `├─"nested"`) {
			t.Error("Nested JSON string should NOT be parsed as object - it's a string value")
		}

		// It should show the escaped string
		if !strings.Contains(tree, `"data"`) {
			t.Error("Should show outer 'data' field")
		}
	})
}
