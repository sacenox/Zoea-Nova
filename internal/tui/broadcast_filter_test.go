package tui

import (
	"github.com/xonecas/zoea-nova/internal/store"
	"testing"
	"time"
)

func TestBroadcastFiltering(t *testing.T) {
	// Simulate the filtering logic from app.go loadMysisLogs
	memories := []*store.Memory{
		{Role: store.MemoryRoleUser, Source: store.MemorySourceDirect, Content: "Direct message 1", CreatedAt: time.Now()},
		{Role: store.MemoryRoleUser, Source: store.MemorySourceBroadcast, Content: "Broadcast message 1", CreatedAt: time.Now()},
		{Role: store.MemoryRoleAssistant, Source: store.MemorySourceLLM, Content: "LLM response 1", CreatedAt: time.Now()},
		{Role: store.MemoryRoleUser, Source: store.MemorySourceBroadcast, Content: "Broadcast message 2", CreatedAt: time.Now()},
		{Role: store.MemoryRoleTool, Source: store.MemorySourceTool, Content: "Tool result", CreatedAt: time.Now()},
		{Role: store.MemoryRoleUser, Source: store.MemorySourceDirect, Content: "Direct message 2", CreatedAt: time.Now()},
	}

	// Apply the filtering (same logic as loadMysisLogs)
	var filteredLogs []LogEntry
	for _, mem := range memories {
		// Skip broadcast messages
		if mem.Source == store.MemorySourceBroadcast {
			continue
		}
		filteredLogs = append(filteredLogs, LogEntryFromMemory(mem, "test-mysis", ""))
	}

	// Verify filtering
	expectedCount := 4 // 2 direct, 1 LLM, 1 tool (excluding 2 broadcasts)
	if len(filteredLogs) != expectedCount {
		t.Errorf("Expected %d filtered logs, got %d", expectedCount, len(filteredLogs))
	}

	// Verify no broadcasts in filtered logs
	for i, log := range filteredLogs {
		if log.Source == "broadcast" || log.Source == "broadcast_self" {
			t.Errorf("Log %d should not be a broadcast: source=%s, content=%s", i, log.Source, log.Content)
		}
	}

	// Verify expected messages are present
	expectedMessages := []string{
		"Direct message 1",
		"LLM response 1",
		"Tool result",
		"Direct message 2",
	}

	for i, expected := range expectedMessages {
		if i >= len(filteredLogs) {
			t.Errorf("Missing expected message: %s", expected)
			continue
		}
		if filteredLogs[i].Content != expected {
			t.Errorf("Log %d: expected content %q, got %q", i, expected, filteredLogs[i].Content)
		}
	}
}
