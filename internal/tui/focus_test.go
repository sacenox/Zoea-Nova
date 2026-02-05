package tui

import (
	"strings"
	"testing"
	"time"

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
	lines := renderLogEntry(entry, maxWidth)

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
