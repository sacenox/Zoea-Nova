package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/xonecas/zoea-nova/internal/store"
)

func TestLogEntryFromMemoryWithSender(t *testing.T) {
	currentMysisID := "current-mysis"
	otherMysisID := "other-mysis"

	tests := []struct {
		name       string
		memory     *store.Memory
		currentID  string
		wantPrefix string
		senderName string
	}{
		{
			name: "direct message",
			memory: &store.Memory{
				Role:    store.MemoryRoleUser,
				Source:  store.MemorySourceDirect,
				Content: "direct msg",
			},
			currentID:  currentMysisID,
			wantPrefix: "YOU:",
		},
		{
			name: "broadcast from self",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: currentMysisID,
				Content:  "my broadcast",
			},
			currentID:  currentMysisID,
			wantPrefix: "YOU (BROADCAST):",
			senderName: "alpha",
		},
		{
			name: "broadcast from other",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: otherMysisID,
				Content:  "other's broadcast",
			},
			currentID:  currentMysisID,
			wantPrefix: "SWARM:",
			senderName: "beta",
		},
		{
			name: "broadcast legacy (no sender)",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: "",
				Content:  "legacy broadcast",
			},
			currentID:  currentMysisID,
			wantPrefix: "SWARM:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntryFromMemory(tt.memory, tt.currentID, tt.senderName)
			if entry.Role != string(tt.memory.Role) {
				t.Errorf("role: got %q, want %q", entry.Role, tt.memory.Role)
			}
			if entry.SenderID != tt.memory.SenderID {
				t.Errorf("sender_id: got %q, want %q", entry.SenderID, tt.memory.SenderID)
			}
			if entry.SenderName != tt.senderName {
				t.Errorf("sender_name: got %q, want %q", entry.SenderName, tt.senderName)
			}
		})
	}
}

func TestRenderLogEntryWithReasoning(t *testing.T) {
	// Force color output for testing
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	entry := LogEntry{
		Role:      "assistant",
		Source:    "llm",
		Content:   "I will mine the asteroid.",
		Reasoning: "The asteroid contains valuable ore. I have enough fuel and cargo space.",
		Timestamp: time.Now(),
	}

	maxWidth := 80
	lines := renderLogEntryImpl(entry, maxWidth, false, 0)

	// Should have content lines + reasoning section
	if len(lines) < 4 {
		t.Errorf("Expected at least 4 lines (padding + content + reasoning header + reasoning), got %d", len(lines))
	}

	// Join lines and check for reasoning content
	output := strings.Join(lines, "\n")
	if !strings.Contains(output, "REASONING:") {
		t.Error("Expected reasoning header 'REASONING:' in output")
	}
	if !strings.Contains(output, "The asteroid contains valuable ore") {
		t.Error("Expected reasoning content in output")
	}

	// Verify ANSI codes are present (styling is applied)
	if !strings.Contains(output, "\x1b[") {
		t.Error("Expected ANSI escape codes (styling) in output")
	}
}

func TestRenderLogEntryWithReasoningTruncation(t *testing.T) {
	// Force color output for testing
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Create reasoning with multiple lines that will wrap
	longReasoning := "First line of reasoning that explains the initial thought process. " +
		"Second line continues with more detailed analysis of the situation. " +
		"Third line adds even more context about the decision. " +
		"Fourth line provides additional justification. " +
		"Fifth line concludes the reasoning with final thoughts."

	entry := LogEntry{
		Role:      "assistant",
		Source:    "llm",
		Content:   "I will proceed with the plan.",
		Reasoning: longReasoning,
		Timestamp: time.Now(),
	}

	maxWidth := 80

	// Test with verbose=false (should truncate)
	linesTruncated := renderLogEntryImpl(entry, maxWidth, false, 0)
	outputTruncated := strings.Join(linesTruncated, "\n")

	// Should contain truncation indicator
	if !strings.Contains(outputTruncated, "more]") {
		t.Error("Expected truncation indicator '[x more]' in non-verbose output")
	}

	// Should contain first line
	if !strings.Contains(outputTruncated, "First line") {
		t.Error("Expected first line of reasoning in truncated output")
	}

	// Should contain last lines
	if !strings.Contains(outputTruncated, "final thoughts") {
		t.Error("Expected last line of reasoning in truncated output")
	}

	// Test with verbose=true (should show all)
	linesVerbose := renderLogEntryImpl(entry, maxWidth, true, 0)
	outputVerbose := strings.Join(linesVerbose, "\n")

	// Should NOT contain truncation indicator
	if strings.Contains(outputVerbose, "more]") {
		t.Error("Should not show truncation indicator in verbose mode")
	}

	// Should contain all parts of reasoning
	if !strings.Contains(outputVerbose, "First line") {
		t.Error("Expected first line in verbose output")
	}
	if !strings.Contains(outputVerbose, "Third line") {
		t.Error("Expected middle lines in verbose output")
	}
	if !strings.Contains(outputVerbose, "final thoughts") {
		t.Error("Expected last line in verbose output")
	}
}

func TestRenderLogEntryToolWithJSON(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	jsonPayload := `{"ship_id": "abc123", "cargo": [{"item": "iron", "quantity": 50}, {"item": "gold", "quantity": 10}], "fuel": 100}`

	entry := LogEntry{
		Role:      "tool",
		Source:    "tool",
		Content:   jsonPayload,
		Timestamp: time.Now(),
	}

	maxWidth := 80
	lines := renderLogEntryImpl(entry, maxWidth, false, 0)

	output := strings.Join(lines, "\n")

	// Should have tree structure
	if !strings.Contains(output, "├─") && !strings.Contains(output, "└─") {
		t.Error("Expected tree box characters in tool JSON output")
	}

	// Should contain field names
	if !strings.Contains(output, "ship_id") || !strings.Contains(output, "cargo") {
		t.Error("Expected JSON field names in tree output")
	}
}

func TestRenderLogEntryToolWithPrefixedJSON(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Tool result with call ID prefix (format from database)
	toolResult := `call_59orhh05:{"max_myses": 16, "states": {"errored": 0, "idle": 0, "running": 1, "stopped": 0}, "total_myses": 1}`

	entry := LogEntry{
		Role:      "tool",
		Source:    "tool",
		Content:   toolResult,
		Timestamp: time.Now(),
	}

	maxWidth := 80
	lines := renderLogEntryImpl(entry, maxWidth, false, 0)

	output := strings.Join(lines, "\n")

	// Should have tree structure (JSON should be detected despite prefix)
	if !strings.Contains(output, "├─") && !strings.Contains(output, "└─") {
		t.Error("Expected tree box characters in prefixed tool JSON output")
	}

	// Should contain field names from JSON
	if !strings.Contains(output, "max_myses") || !strings.Contains(output, "states") {
		t.Error("Expected JSON field names in tree output")
	}

	// Should NOT contain the call ID prefix in the tree
	if strings.Contains(output, "call_59orhh05:") {
		t.Error("Tool call ID prefix should be stripped from JSON tree")
	}
}
func TestRenderFocusViewWithScrollbar(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Create viewport with content
	vp := viewport.New(80, 10)
	longContent := strings.Repeat("line\n", 50) // 50 lines
	vp.SetContent(longContent)
	vp.GotoTop()

	mysis := MysisInfo{
		ID:       "test-id",
		Name:     "test-mysis",
		State:    "running",
		Provider: "ollama-qwen",
	}

	width := 100
	totalLines := 50
	output := RenderFocusViewWithViewport(mysis, vp, width, false, "⬡", false, totalLines, 1, 1, 0, nil, 0, nil)

	// Should contain scrollbar characters
	if !strings.Contains(output, "█") && !strings.Contains(output, "│") {
		t.Error("Expected scrollbar characters in focus view output")
	}
}
