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
			entry := LogEntryFromMemory(tt.memory, tt.currentID)
			if entry.Role != string(tt.memory.Role) {
				t.Errorf("role: got %q, want %q", entry.Role, tt.memory.Role)
			}
			if entry.SenderID != tt.memory.SenderID {
				t.Errorf("sender_id: got %q, want %q", entry.SenderID, tt.memory.SenderID)
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
	lines := renderLogEntryImpl(entry, maxWidth, false)

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
	lines := renderLogEntryImpl(entry, maxWidth, false)

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
		Provider: "ollama",
	}

	width := 100
	totalLines := 50
	output := RenderFocusViewWithViewport(mysis, vp, width, false, "⬡", true, false, totalLines)

	// Should contain scrollbar characters
	if !strings.Contains(output, "█") && !strings.Contains(output, "│") {
		t.Error("Expected scrollbar characters in focus view output")
	}
}
