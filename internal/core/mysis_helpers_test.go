package core

import (
	"encoding/json"
	"testing"

	"github.com/xonecas/zoea-nova/internal/constants"
	"github.com/xonecas/zoea-nova/internal/store"
)

func TestNormalizeInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
		shouldOK bool
	}{
		{"int", 42, 42, true},
		{"int64", int64(123), 123, true},
		{"float64", float64(456.7), 456, true},
		{"json_number", json.Number("789"), 789, true},
		{"string_valid", "999", 999, true},
		{"string_invalid", "not-a-number", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
		{"slice", []int{1, 2}, 0, false},
		{"negative_int", -42, -42, true},
		{"negative_string", "-100", -100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeInt(tt.input)
			if ok != tt.shouldOK {
				t.Errorf("normalizeInt(%v) ok = %v, want %v", tt.input, ok, tt.shouldOK)
			}
			if ok && got != tt.expected {
				t.Errorf("normalizeInt(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
		shouldOK bool
	}{
		{"int", 42, 42.0, true},
		{"int64", int64(123), 123.0, true},
		{"float64", float64(456.789), 456.789, true},
		{"json_number", json.Number("789.123"), 789.123, true},
		{"string_valid", "999.456", 999.456, true},
		{"string_invalid", "not-a-number", 0, false},
		{"nil", nil, 0, false},
		{"bool", false, 0, false},
		{"map", map[string]int{"a": 1}, 0, false},
		{"negative_float", -42.5, -42.5, true},
		{"negative_string", "-100.25", -100.25, true},
		{"scientific_notation", "1.23e2", 123.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeFloat(tt.input)
			if ok != tt.shouldOK {
				t.Errorf("normalizeFloat(%v) ok = %v, want %v", tt.input, ok, tt.shouldOK)
			}
			if ok && got != tt.expected {
				t.Errorf("normalizeFloat(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsSnapshotTool(t *testing.T) {
	m := &Mysis{}

	tests := []struct {
		name     string
		toolName string
		expected bool
	}{
		{"get_prefix", "get_status", true},
		{"get_system", "get_system", true},
		{"get_anything", "get_foo", true},
		{"zoea_swarm_status", "zoea_swarm_status", true},
		{"zoea_list_myses", "zoea_list_myses", true},
		{"action_tool", "move_to", false},
		{"mine_action", "mine_resources", false},
		{"empty", "", false},
		{"zoea_send_broadcast", "zoea_send_broadcast", false},
		{"zoea_message_mysis", "zoea_message_mysis", false},
		{"attack", "attack_target", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.isSnapshotTool(tt.toolName)
			if got != tt.expected {
				t.Errorf("isSnapshotTool(%q) = %v, want %v", tt.toolName, got, tt.expected)
			}
		})
	}
}

func TestExtractToolNameFromResult(t *testing.T) {
	m := &Mysis{}

	toolCallNames := map[string]string{
		"call_123": "get_status",
		"call_456": "mine_resources",
		"call_789": "zoea_swarm_status",
	}

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "valid_call_id",
			content:  "call_123" + constants.ToolCallStorageFieldDelimiter + `{"status": "ok"}`,
			expected: "get_status",
		},
		{
			name:     "another_valid_call",
			content:  "call_456" + constants.ToolCallStorageFieldDelimiter + `{"result": "success"}`,
			expected: "mine_resources",
		},
		{
			name:     "unknown_call_id",
			content:  "call_999" + constants.ToolCallStorageFieldDelimiter + `{"error": "not found"}`,
			expected: "",
		},
		{
			name:     "no_delimiter",
			content:  "call_123",
			expected: "",
		},
		{
			name:     "empty_content",
			content:  "",
			expected: "",
		},
		{
			name:     "delimiter_at_start",
			content:  constants.ToolCallStorageFieldDelimiter + "call_123",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.extractToolNameFromResult(tt.content, toolCallNames)
			if got != tt.expected {
				t.Errorf("extractToolNameFromResult(%q) = %q, want %q", tt.content, got, tt.expected)
			}
		})
	}
}

func TestToolCallNameIndex(t *testing.T) {
	m := &Mysis{}

	tests := []struct {
		name     string
		memories []*store.Memory
		expected map[string]string
	}{
		{
			name: "single_tool_call",
			memories: []*store.Memory{
				{
					Role:    store.MemoryRoleAssistant,
					Content: constants.ToolCallStoragePrefix + `call_1:get_status:{}`,
				},
			},
			expected: map[string]string{
				"call_1": "get_status",
			},
		},
		{
			name: "multiple_tool_calls",
			memories: []*store.Memory{
				{
					Role:    store.MemoryRoleAssistant,
					Content: constants.ToolCallStoragePrefix + `call_1:get_status:{}|call_2:mine_resources:{"amount":5}`,
				},
			},
			expected: map[string]string{
				"call_1": "get_status",
				"call_2": "mine_resources",
			},
		},
		{
			name: "multiple_memories",
			memories: []*store.Memory{
				{
					Role:    store.MemoryRoleAssistant,
					Content: constants.ToolCallStoragePrefix + `call_1:get_status:{}`,
				},
				{
					Role:    store.MemoryRoleAssistant,
					Content: constants.ToolCallStoragePrefix + `call_2:move_to:{"x":10,"y":20}`,
				},
			},
			expected: map[string]string{
				"call_1": "get_status",
				"call_2": "move_to",
			},
		},
		{
			name: "skip_non_assistant",
			memories: []*store.Memory{
				{
					Role:    store.MemoryRoleUser,
					Content: constants.ToolCallStoragePrefix + `call_1:get_status:{}`,
				},
			},
			expected: map[string]string{},
		},
		{
			name: "skip_without_prefix",
			memories: []*store.Memory{
				{
					Role:    store.MemoryRoleAssistant,
					Content: `Regular message without tool call prefix`,
				},
			},
			expected: map[string]string{},
		},
		{
			name: "skip_empty_id",
			memories: []*store.Memory{
				{
					Role:    store.MemoryRoleAssistant,
					Content: constants.ToolCallStoragePrefix + `:get_status:{}`,
				},
			},
			expected: map[string]string{},
		},
		{
			name: "skip_empty_name",
			memories: []*store.Memory{
				{
					Role:    store.MemoryRoleAssistant,
					Content: constants.ToolCallStoragePrefix + `call_1::{}`,
				},
			},
			expected: map[string]string{},
		},
		{
			name:     "empty_memories",
			memories: []*store.Memory{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.toolCallNameIndex(tt.memories)
			if len(got) != len(tt.expected) {
				t.Errorf("toolCallNameIndex() length = %d, want %d", len(got), len(tt.expected))
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("toolCallNameIndex()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
