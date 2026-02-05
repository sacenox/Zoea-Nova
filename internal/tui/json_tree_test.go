package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderJSONTree_SimpleObject(t *testing.T) {
	jsonStr := `{"name": "mysis-1", "state": "running", "id": "abc123"}`

	tree, err := renderJSONTree(jsonStr, false, 80)
	if err != nil {
		t.Fatalf("Failed to render JSON tree: %v", err)
	}

	// Should use tree structure
	if !strings.Contains(tree, "├─") && !strings.Contains(tree, "└─") {
		t.Error("Expected tree box characters in output")
	}

	// Should contain field names
	if !strings.Contains(tree, "name") || !strings.Contains(tree, "state") {
		t.Error("Expected field names in output")
	}
}

func TestRenderJSONTree_ArrayTruncation(t *testing.T) {
	items := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		items[i] = map[string]interface{}{"id": i, "value": i * 100}
	}
	jsonBytes, _ := json.Marshal(items)

	tree, err := renderJSONTree(string(jsonBytes), false, 80)
	if err != nil {
		t.Fatalf("Failed to render JSON tree: %v", err)
	}

	// Should show truncation indicator
	if !strings.Contains(tree, "[4 more]") {
		t.Error("Expected '[4 more]' truncation indicator for 10-item array")
	}

	// Should show first 3 items (0, 1, 2)
	if !strings.Contains(tree, `"id": 0`) || !strings.Contains(tree, `"id": 1`) || !strings.Contains(tree, `"id": 2`) {
		t.Error("Expected first 3 items to be shown")
	}

	// Should show last 3 items (7, 8, 9)
	if !strings.Contains(tree, `"id": 7`) || !strings.Contains(tree, `"id": 8`) || !strings.Contains(tree, `"id": 9`) {
		t.Error("Expected last 3 items to be shown")
	}

	// Should NOT show middle items
	if strings.Contains(tree, `"id": 3`) || strings.Contains(tree, `"id": 4`) {
		t.Error("Middle items should be truncated")
	}
}

func TestRenderJSONTree_VerboseMode(t *testing.T) {
	items := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		items[i] = map[string]interface{}{"id": i}
	}
	jsonBytes, _ := json.Marshal(items)

	tree, err := renderJSONTree(string(jsonBytes), true, 80)
	if err != nil {
		t.Fatalf("Failed to render JSON tree: %v", err)
	}

	// Should NOT truncate in verbose mode
	if strings.Contains(tree, "more") {
		t.Error("Should not show truncation in verbose mode")
	}

	// Should show all items
	for i := 0; i < 10; i++ {
		expected := strings.Replace(`"id": X`, "X", string(rune('0'+i)), 1)
		if !strings.Contains(tree, expected) {
			t.Errorf("Expected item %d to be shown in verbose mode", i)
		}
	}
}

func TestRenderJSONTree_InvalidJSON(t *testing.T) {
	jsonStr := `{invalid json`

	_, err := renderJSONTree(jsonStr, false, 80)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestRenderJSONTree_LongValueTruncation(t *testing.T) {
	// Create JSON with a very long string value
	longValue := strings.Repeat("a", 200)
	jsonStr := `{"id": "` + longValue + `", "name": "test"}`

	maxWidth := 50
	tree, err := renderJSONTree(jsonStr, false, maxWidth)
	if err != nil {
		t.Fatalf("Failed to render JSON tree: %v", err)
	}

	lines := strings.Split(tree, "\n")
	for i, line := range lines {
		// Use lipgloss.Width for proper Unicode display width
		width := lipgloss.Width(line)
		if width > maxWidth {
			t.Errorf("Line %d exceeds maxWidth %d: got %d display width\nLine: %s", i, maxWidth, width, stripANSI(line))
		}
	}

	// Should contain truncation indicator
	if !strings.Contains(tree, "...") {
		t.Error("Expected truncation indicator '...' for long value")
	}
}
