package tui

import (
	"testing"

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
