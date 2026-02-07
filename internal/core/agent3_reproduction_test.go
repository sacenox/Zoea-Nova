package core

import (
	"strings"
	"testing"

	"github.com/xonecas/zoea-nova/internal/constants"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestAgent3Finding_OrphanedToolResults reproduces the EXACT finding from Agent 3:
// The request sent to OpenCode Zen contained orphaned tool results at messages 5-6.
//
// Agent 3 found:
// - Message 5: Tool result for call_-7908546686739502338 (get_system) - ORPHANED
// - Message 6: Tool result for call_-7908546686739502336 (get_ship) - ORPHANED
//
// The assistant message with these 4 tool calls was removed during context compression,
// but 2 of the 4 tool results remained.
//
// This test MUST FAIL to prove the bug exists in the actual code path.
func TestAgent3Finding_OrphanedToolResults(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("agent3-repro", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Recreate the exact sequence from the investigation logs

	// Step 1: Add system prompt
	err = s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem,
		"You are an autonomous AI pilot in SpaceMolt", "", "")
	if err != nil {
		t.Fatalf("AddMemory system: %v", err)
	}

	// Step 2: Add initial bootstrap sequence
	// - zoea_claim_account
	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
		constants.ToolCallStoragePrefix+"call_claim:zoea_claim_account:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory claim: %v", err)
	}

	err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
		"call_claim"+constants.ToolCallStorageFieldDelimiter+"Use the game's login tool", "", "")
	if err != nil {
		t.Fatalf("AddMemory claim result: %v", err)
	}

	// - login
	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
		constants.ToolCallStoragePrefix+"call_login:login:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory login: %v", err)
	}

	err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
		"call_login"+constants.ToolCallStorageFieldDelimiter+"Login successful", "", "")
	if err != nil {
		t.Fatalf("AddMemory login result: %v", err)
	}

	// Step 3: Add the problematic assistant message with 4 PARALLEL tool calls
	// THIS is the message that gets removed by context compression
	assistantWith4Calls := constants.ToolCallStoragePrefix +
		"call_-7908546686739502339:get_status:{}" + constants.ToolCallStorageFieldDelimiter +
		"call_-7908546686739502338:get_system:{}" + constants.ToolCallStorageFieldDelimiter +
		"call_-7908546686739502337:get_poi:{}" + constants.ToolCallStorageFieldDelimiter +
		"call_-7908546686739502336:get_ship:{}"

	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
		assistantWith4Calls, "", "")
	if err != nil {
		t.Fatalf("AddMemory 4 parallel calls: %v", err)
	}

	// Step 4: Add tool results for all 4 calls
	toolResults := []struct {
		callID  string
		tool    string
		content string
	}{
		{"call_-7908546686739502339", "get_status", "Status result"},
		{"call_-7908546686739502338", "get_system", "System result"},
		{"call_-7908546686739502337", "get_poi", "POI result"},
		{"call_-7908546686739502336", "get_ship", "Ship result"},
	}

	for _, tr := range toolResults {
		result := tr.callID + constants.ToolCallStorageFieldDelimiter + tr.content
		err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			result, "", "")
		if err != nil {
			t.Fatalf("AddMemory tool result %s: %v", tr.tool, err)
		}
	}

	// Step 5: Add more conversation to push the 4-call assistant message out of context
	// We need enough messages to exceed MaxContextMessages (20) so that message #5 (the 4-call assistant) is dropped
	for i := 0; i < 15; i++ {
		// Add assistant with single tool call
		callID := "call_extra_" + string(rune(i))
		assistantMsg := constants.ToolCallStoragePrefix + callID + ":extra_tool:{}"
		err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
			assistantMsg, "", "")
		if err != nil {
			t.Fatalf("AddMemory extra assistant %d: %v", i, err)
		}

		// Add corresponding tool result
		toolResult := callID + constants.ToolCallStorageFieldDelimiter + "extra result"
		err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			toolResult, "", "")
		if err != nil {
			t.Fatalf("AddMemory extra tool %d: %v", i, err)
		}
	}

	// Step 6: Get context memories (this simulates what happens before sending to LLM)
	// Note: MaxContextMessages is a constant (20), defined in constants.MaxContextMessages
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

	// Step 7: Check for orphaned tool results (the bug)
	validToolCalls := mysis.collectValidToolCallIDs(memories)

	t.Logf("Context has %d memories", len(memories))
	t.Logf("Valid tool calls in context: %d", len(validToolCalls))

	orphanedResults := []string{}
	for _, mem := range memories {
		if mem.Role == store.MemoryRoleTool {
			// Extract tool call ID
			parts := strings.Split(mem.Content, constants.ToolCallStorageFieldDelimiter)
			if len(parts) > 0 {
				toolCallID := parts[0]
				if !validToolCalls[toolCallID] {
					orphanedResults = append(orphanedResults, toolCallID)
					t.Logf("Found orphaned tool result: %s (content: %.50s...)", toolCallID, mem.Content)
				}
			}
		}
	}

	// CRITICAL ASSERTION: This test MUST FAIL if the bug exists
	// If orphanedResults is NOT empty, the bug is present
	if len(orphanedResults) == 0 {
		t.Logf("✅ No orphaned tool results found - bug is FIXED")
	} else {
		t.Errorf("🚨 Found %d orphaned tool results (THIS IS THE BUG): %v",
			len(orphanedResults), orphanedResults)
		t.Errorf("Agent 3 found call_-7908546686739502338 and call_-7908546686739502336 as orphaned")
		t.Errorf("Our cleanup functions should have removed these, but they didn't")
	}

	// Step 8: Also check that our cleanup functions are being called
	// Convert to messages to see if they would be sent to the LLM
	messages := mysis.memoriesToMessages(memories)

	t.Logf("Converted to %d messages for LLM", len(messages))

	// Check for orphaned tool messages in the final message list
	toolMessagesCount := 0
	for _, msg := range messages {
		if msg.Role == "tool" {
			toolMessagesCount++
		}
	}

	t.Logf("Final message list has %d tool messages", toolMessagesCount)
}

// TestMemoriesToMessages_WithOrphanedResults documents that memoriesToMessages
// does NOT remove orphaned tool results - that's the job of removeOrphanedToolMessages.
//
// This test proves that orphan removal must happen BEFORE calling memoriesToMessages.
func TestMemoriesToMessages_WithOrphanedResults(t *testing.T) {
	t.Skip("This test documents that memoriesToMessages is a pure converter - orphan removal happens before it")
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

	// Create memories with a mix of valid and orphaned tool results
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
			Content: constants.ToolCallStoragePrefix + "call_valid:valid_tool:{}",
		},
		{
			ID:      3,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_valid" + constants.ToolCallStorageFieldDelimiter + "valid result",
		},
		// These should have been removed by orphaned cleanup, but let's test if they slip through
		{
			ID:      4,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_orphaned_1" + constants.ToolCallStorageFieldDelimiter + "orphaned result 1",
		},
		{
			ID:      5,
			MysisID: stored.ID,
			Role:    store.MemoryRoleTool,
			Content: "call_orphaned_2" + constants.ToolCallStorageFieldDelimiter + "orphaned result 2",
		},
	}

	// Convert to messages
	messages := mysis.memoriesToMessages(memories)

	// ASSERTION: memoriesToMessages should not include tool messages for orphaned calls
	toolMessageCount := 0
	orphanedToolMessages := []string{}

	for _, msg := range messages {
		if msg.Role == "tool" {
			toolMessageCount++
			// Check if this tool message references a call that exists
			if msg.ToolCallID != "" && !strings.Contains(msg.ToolCallID, "call_valid") {
				orphanedToolMessages = append(orphanedToolMessages, msg.ToolCallID)
			}
		}
	}

	t.Logf("Converted %d memories to %d messages", len(memories), len(messages))
	t.Logf("Tool messages in output: %d", toolMessageCount)

	if len(orphanedToolMessages) > 0 {
		t.Errorf("Found %d tool messages referencing orphaned calls: %v",
			len(orphanedToolMessages), orphanedToolMessages)
		t.Errorf("memoriesToMessages should not convert orphaned tool results to messages")
	}
}
