package core

import (
	"strings"
	"testing"

	"github.com/xonecas/zoea-nova/internal/constants"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestOrphanedToolResults_ContextCompression tests the bug found in investigation:
// When context window compression removes an assistant message with parallel tool calls,
// but keeps some of the tool results, we get orphaned tool results that violate OpenAI API.
//
// This reproduces Agent 3's finding: Messages 5-6 contained orphaned tool results
// for tool calls that were removed during context compression.
func TestOrphanedToolResults_ContextCompression(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("orphan-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Simulate the exact scenario from the investigation:
	// An assistant message with 4 parallel tool calls gets removed from context window,
	// but 2 of the 4 tool results remain.

	// Step 1: Add system prompt
	err = s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem,
		"You are a test mysis", "", "")
	if err != nil {
		t.Fatalf("AddMemory system: %v", err)
	}

	// Step 2: Add assistant message with 4 parallel tool calls (this will be removed by context window)
	assistantWith4Calls := constants.ToolCallStoragePrefix +
		"call_-7908546686739502339:get_status:{}" + constants.ToolCallStorageFieldDelimiter +
		"call_-7908546686739502338:get_system:{}" + constants.ToolCallStorageFieldDelimiter +
		"call_-7908546686739502337:get_poi:{}" + constants.ToolCallStorageFieldDelimiter +
		"call_-7908546686739502336:get_ship:{}"

	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
		assistantWith4Calls, "", "")
	if err != nil {
		t.Fatalf("AddMemory assistant with 4 calls: %v", err)
	}

	// Step 3: Add tool results for all 4 calls
	toolResults := []struct {
		callID  string
		content string
	}{
		{"call_-7908546686739502339", "get_status result"},
		{"call_-7908546686739502338", "get_system result"},
		{"call_-7908546686739502337", "get_poi result"},
		{"call_-7908546686739502336", "get_ship result"},
	}

	for _, tr := range toolResults {
		result := tr.callID + constants.ToolCallStorageFieldDelimiter + tr.content
		err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			result, "", "")
		if err != nil {
			t.Fatalf("AddMemory tool result: %v", err)
		}
	}

	// Step 4: Add several more messages to push the assistant message out of context window
	// (MaxContextMessages = 20, so we need to add enough to push message #2 out)
	for i := 0; i < 20; i++ {
		// Add assistant message
		assistantMsg := constants.ToolCallStoragePrefix +
			"call_extra_" + string(rune(i)) + ":extra_tool:{}"
		err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
			assistantMsg, "", "")
		if err != nil {
			t.Fatalf("AddMemory extra assistant: %v", err)
		}

		// Add corresponding tool result
		toolResult := "call_extra_" + string(rune(i)) + constants.ToolCallStorageFieldDelimiter + "extra result"
		err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			toolResult, "", "")
		if err != nil {
			t.Fatalf("AddMemory extra tool: %v", err)
		}
	}

	// Step 5: Get context memories with compression (last 20 messages)
	// This should simulate what happens in real usage
	mysis := &Mysis{
		id:    stored.ID,
		name:  stored.Name,
		store: s,
		bus:   bus,
	}

	memories, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// Step 6: Check if orphaned tool results exist
	validToolCalls := mysis.collectValidToolCallIDs(memories)
	orphanedResults := []string{}

	for _, mem := range memories {
		if mem.Role == store.MemoryRoleTool {
			// Extract tool call ID from result
			idx := len(mem.Content)
			for i, ch := range mem.Content {
				if ch == ':' {
					idx = i
					break
				}
			}
			if idx > 0 && idx < len(mem.Content) {
				toolCallID := mem.Content[:idx]
				if !validToolCalls[toolCallID] {
					orphanedResults = append(orphanedResults, toolCallID)
				}
			}
		}
	}

	// ASSERTION: This test should FAIL initially, proving the bug exists
	// After fix, orphanedResults should be empty
	if len(orphanedResults) > 0 {
		t.Errorf("Found %d orphaned tool results (expected 0): %v",
			len(orphanedResults), orphanedResults)
		t.Logf("This proves the bug: orphaned tool results exist after context compression")
		t.Logf("Valid tool calls in context: %d", len(validToolCalls))
		t.Logf("Total memories in context: %d", len(memories))
	}
}

// TestRemoveOrphanedToolMessages_WithOrphanedResults tests that removeOrphanedToolMessages
// correctly identifies and removes tool results without corresponding tool calls.
//
// This tests the function that SHOULD have caught the bug in Agent 3's findings.
func TestRemoveOrphanedToolMessages_WithOrphanedResults(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mysis := &Mysis{
		id:    stored.ID,
		name:  stored.Name,
		store: s,
		bus:   bus,
	}

	// Create memories with orphaned tool results (matching Agent 3's finding)
	memories := []*store.Memory{
		{
			ID:      1,
			MysisID: stored.ID,
			Role:    store.MemoryRoleSystem,
			Content: "System prompt",
		},
		{
			ID:      2,
			MysisID: stored.ID,
			Role:    store.MemoryRoleAssistant,
			Content: constants.ToolCallStoragePrefix + "call_123:tool_a:{}",
		},
		{
			ID:      3,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_123" + constants.ToolCallStorageFieldDelimiter + "result for tool_a",
		},
		// ORPHANED TOOL RESULTS (no matching assistant tool calls)
		{
			ID:      4,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_-7908546686739502338" + constants.ToolCallStorageFieldDelimiter + "get_system result",
		},
		{
			ID:      5,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_-7908546686739502336" + constants.ToolCallStorageFieldDelimiter + "get_ship result",
		},
		{
			ID:      6,
			MysisID: stored.ID,
			Role:    store.MemoryRoleAssistant,
			Content: constants.ToolCallStoragePrefix + "call_456:tool_b:{}",
		},
		{
			ID:      7,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_456" + constants.ToolCallStorageFieldDelimiter + "result for tool_b",
		},
	}

	// Run the removal function
	cleaned := mysis.removeOrphanedToolMessages(memories)

	// ASSERTION 1: Orphaned tool results should be removed
	expectedCount := 5 // system + 2 assistant + 2 tool results (not orphaned)
	if len(cleaned) != expectedCount {
		t.Errorf("Expected %d memories after cleanup, got %d", expectedCount, len(cleaned))
		t.Logf("This proves removeOrphanedToolMessages is not catching orphaned results")
	}

	// ASSERTION 2: Verify the orphaned results are gone
	for _, mem := range cleaned {
		if mem.Role == store.MemoryRoleTool {
			if strings.Contains(mem.Content, "call_-7908546686739502338") ||
				strings.Contains(mem.Content, "call_-7908546686739502336") {
				t.Errorf("Orphaned tool result %d was not removed", mem.ID)
			}
		}
	}

	// ASSERTION 3: Valid tool results should remain
	validResults := 0
	for _, mem := range cleaned {
		if mem.Role == store.MemoryRoleTool {
			validResults++
		}
	}
	if validResults != 2 {
		t.Errorf("Expected 2 valid tool results, got %d", validResults)
	}
}

// TestRemoveOrphanedToolCalls_WithOrphanedCalls tests that removeOrphanedToolCalls
// correctly identifies and removes assistant messages with tool calls that have no results.
//
// This tests the NEW function we added based on the investigation.
func TestRemoveOrphanedToolCalls_WithOrphanedCalls(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mysis := &Mysis{
		id:    stored.ID,
		name:  stored.Name,
		store: s,
		bus:   bus,
	}

	// Create memories with orphaned tool calls
	memories := []*store.Memory{
		{
			ID:      1,
			MysisID: stored.ID,
			Role:    store.MemoryRoleSystem,
			Content: "System prompt",
		},
		{
			ID:      2,
			MysisID: stored.ID,
			Role:    store.MemoryRoleAssistant,
			Content: constants.ToolCallStoragePrefix + "call_123:tool_a:{}",
		},
		{
			ID:      3,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_123" + constants.ToolCallStorageFieldDelimiter + "result for tool_a",
		},
		// ORPHANED TOOL CALL (no matching tool result)
		{
			ID:      4,
			MysisID: stored.ID,
			Role:    store.MemoryRoleAssistant,
			Content: constants.ToolCallStoragePrefix + "call_orphan:orphan_tool:{}",
		},
		{
			ID:      5,
			MysisID: stored.ID,
			Role:    store.MemoryRoleAssistant,
			Content: constants.ToolCallStoragePrefix + "call_456:tool_b:{}",
		},
		{
			ID:      6,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_456" + constants.ToolCallStorageFieldDelimiter + "result for tool_b",
		},
	}

	// Run the removal function
	cleaned := mysis.removeOrphanedToolCalls(memories)

	// ASSERTION: Orphaned tool call should be removed
	expectedCount := 5 // system + 2 assistant (with results) + 2 tool results
	if len(cleaned) != expectedCount {
		t.Errorf("Expected %d memories after cleanup, got %d", expectedCount, len(cleaned))
	}

	// Verify the orphaned call is gone
	for _, mem := range cleaned {
		if mem.ID == 4 {
			t.Errorf("Orphaned tool call (ID=4) was not removed")
		}
	}
}

// TestContextCompressionPreservesToolCallPairs tests that when context window
// compression occurs, tool call/result pairs are preserved together or removed together.
//
// This is the IDEAL behavior that should prevent the bug.
func TestContextCompressionPreservesToolCallPairs(t *testing.T) {
	t.Skip("This test documents the DESIRED behavior - implement after fixing the bug")

	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Add a long conversation that will exceed MaxContextMessages
	for i := 0; i < 30; i++ {
		// Assistant with tool call
		assistantMsg := constants.ToolCallStoragePrefix +
			"call_" + string(rune(i)) + ":tool_" + string(rune(i)) + ":{}"
		err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
			assistantMsg, "", "")
		if err != nil {
			t.Fatalf("AddMemory assistant: %v", err)
		}

		// Corresponding tool result
		toolResult := "call_" + string(rune(i)) + constants.ToolCallStorageFieldDelimiter +
			"result_" + string(rune(i))
		err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			toolResult, "", "")
		if err != nil {
			t.Fatalf("AddMemory tool: %v", err)
		}
	}

	mysis := &Mysis{
		id:    stored.ID,
		name:  stored.Name,
		store: s,
		bus:   bus,
	}

	memories, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// ASSERTION: Every assistant message with tool calls should have matching results
	toolCalls := mysis.collectValidToolCallIDs(memories)
	toolResults := mysis.collectValidToolResultIDs(memories)

	// Every tool call should have a result
	for callID := range toolCalls {
		if !toolResults[callID] {
			t.Errorf("Tool call %s has no matching result", callID)
		}
	}

	// Every tool result should have a call
	for resultID := range toolResults {
		if !toolCalls[resultID] {
			t.Errorf("Tool result %s has no matching call", resultID)
		}
	}
}
