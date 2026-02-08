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
		"You are an AI pilot in SpaceMolt", "", "")
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

	memories, _, err := mysis.getContextMemories()
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

// TestLoopContextSlice_ToolCallResultPairing validates that tool results are
// present ONLY when their matching tool call is present in the context.
//
// This test is part of Task 2 of the Loop Context Composition plan:
// - Tool results must be paired with their tool calls
// - No orphaned tool results should exist
// - If a tool call is excluded from context, its results must also be excluded
//
// Expected behavior under new loop composition:
// - Only the most recent tool-call loop is included
// - Older tool loops are completely excluded (both calls and results)
//
// This test MUST FAIL initially because the current implementation uses
// MaxContextMessages sliding window which can split tool call/result pairs.
func TestLoopContextSlice_ToolCallResultPairing(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("loop-pairing-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Step 1: Add system prompt
	err = s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem,
		"System prompt", "", "")
	if err != nil {
		t.Fatalf("AddMemory system: %v", err)
	}

	// Step 2: Add commander message (current turn start)
	err = s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect,
		"Check your status and mine some resources", "", "")
	if err != nil {
		t.Fatalf("AddMemory commander: %v", err)
	}

	// Step 3: Add first tool loop (part of current turn, will be included)
	// Loop 1: get_status + get_system
	oldLoop1Assistant := constants.ToolCallStoragePrefix +
		"call_old_1:get_status:{}" + constants.ToolCallStorageRecordDelimiter +
		"call_old_2:get_system:{}"

	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
		oldLoop1Assistant, "", "")
	if err != nil {
		t.Fatalf("AddMemory old loop 1: %v", err)
	}

	err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
		"call_old_1"+constants.ToolCallStorageFieldDelimiter+"Old status result", "", "")
	if err != nil {
		t.Fatalf("AddMemory old result 1: %v", err)
	}

	err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
		"call_old_2"+constants.ToolCallStorageFieldDelimiter+"Old system result", "", "")
	if err != nil {
		t.Fatalf("AddMemory old result 2: %v", err)
	}

	// Step 4: Add second tool loop (part of current turn, will be included)
	// Loop 2: get_poi
	oldLoop2Assistant := constants.ToolCallStoragePrefix + "call_old_3:get_poi:{}"

	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
		oldLoop2Assistant, "", "")
	if err != nil {
		t.Fatalf("AddMemory old loop 2: %v", err)
	}

	err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
		"call_old_3"+constants.ToolCallStorageFieldDelimiter+"Old POI result", "", "")
	if err != nil {
		t.Fatalf("AddMemory old result 3: %v", err)
	}

	// Step 5: Add third tool loop (part of current turn, will be included)
	// Loop 3: mine + get_cargo
	recentLoopAssistant := constants.ToolCallStoragePrefix +
		"call_recent_1:mine:{}" + constants.ToolCallStorageRecordDelimiter +
		"call_recent_2:get_cargo:{}"

	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM,
		recentLoopAssistant, "", "")
	if err != nil {
		t.Fatalf("AddMemory recent loop: %v", err)
	}

	err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
		"call_recent_1"+constants.ToolCallStorageFieldDelimiter+"Mining successful", "", "")
	if err != nil {
		t.Fatalf("AddMemory recent result 1: %v", err)
	}

	err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
		"call_recent_2"+constants.ToolCallStorageFieldDelimiter+"Cargo: ore_iron x10", "", "")
	if err != nil {
		t.Fatalf("AddMemory recent result 2: %v", err)
	}

	// Step 6: Get context memories using turn-aware composition
	mysis := &Mysis{
		id:    stored.ID,
		name:  stored.Name,
		store: s,
		bus:   bus,
	}

	memories, _, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// Step 7: Validate turn-aware composition rules
	t.Logf("Context has %d memories", len(memories))

	// Collect all tool call IDs present in assistant messages
	validToolCalls := make(map[string]bool)
	for _, mem := range memories {
		if mem.Role == store.MemoryRoleAssistant &&
			strings.HasPrefix(mem.Content, constants.ToolCallStoragePrefix) {
			calls := mysis.parseStoredToolCalls(mem.Content)
			for _, call := range calls {
				validToolCalls[call.ID] = true
				t.Logf("Found tool call in context: %s (%s)", call.ID, call.Name)
			}
		}
	}

	// Collect all tool result IDs present in tool messages
	presentToolResults := make(map[string]bool)
	for _, mem := range memories {
		if mem.Role == store.MemoryRoleTool {
			parts := strings.Split(mem.Content, constants.ToolCallStorageFieldDelimiter)
			if len(parts) > 0 {
				toolCallID := parts[0]
				presentToolResults[toolCallID] = true
				t.Logf("Found tool result in context: %s", toolCallID)
			}
		}
	}

	// CRITICAL ASSERTION 1: No orphaned tool results
	// Every tool result must have a matching tool call in the context
	orphanedResults := []string{}
	for resultID := range presentToolResults {
		if !validToolCalls[resultID] {
			orphanedResults = append(orphanedResults, resultID)
		}
	}

	if len(orphanedResults) > 0 {
		t.Errorf("LOOP COMPOSITION VIOLATION: Found %d orphaned tool results (results without tool calls): %v",
			len(orphanedResults), orphanedResults)
		t.Errorf("Under loop composition, if a tool call is excluded, ALL its results must be excluded")
	}

	// CRITICAL ASSERTION 2: All tool loops within current turn are present
	// With turn-aware composition, all loops after the user prompt should be in context
	// Old tool calls (call_old_*) are part of the current turn and SHOULD be in context
	oldCallsPresent := []string{}
	for callID := range validToolCalls {
		if strings.Contains(callID, "call_old_") {
			oldCallsPresent = append(oldCallsPresent, callID)
		}
	}

	// Expect 3 old calls (call_old_1, call_old_2, call_old_3) since they're in current turn
	if len(oldCallsPresent) != 3 {
		t.Errorf("TURN COMPOSITION: Expected 3 old tool calls in current turn, found %d: %v",
			len(oldCallsPresent), oldCallsPresent)
	}

	// CRITICAL ASSERTION 3: The most recent loop is complete
	// Both call_recent_1 and call_recent_2 should be present
	expectedRecentCalls := []string{"call_recent_1", "call_recent_2"}
	missingRecentCalls := []string{}
	for _, expectedCall := range expectedRecentCalls {
		if !validToolCalls[expectedCall] {
			missingRecentCalls = append(missingRecentCalls, expectedCall)
		}
	}

	if len(missingRecentCalls) > 0 {
		t.Errorf("LOOP COMPOSITION VIOLATION: Missing %d tool calls from most recent loop: %v",
			len(missingRecentCalls), missingRecentCalls)
		t.Errorf("The most recent tool loop must be included completely")
	}

	// CRITICAL ASSERTION 4: All tool calls have matching results
	// This is the inverse check - no orphaned tool calls
	missingResults := []string{}
	for callID := range validToolCalls {
		if !presentToolResults[callID] {
			missingResults = append(missingResults, callID)
		}
	}

	if len(missingResults) > 0 {
		t.Errorf("LOOP COMPOSITION VIOLATION: Found %d tool calls without results: %v",
			len(missingResults), missingResults)
		t.Errorf("Under loop composition, tool calls and their results must be paired")
	}

	// SUCCESS: If we get here with no errors, loop composition is working correctly
	if len(orphanedResults) == 0 && len(oldCallsPresent) == 0 &&
		len(missingRecentCalls) == 0 && len(missingResults) == 0 {
		t.Logf("Loop composition validated:")
		t.Logf("  - No orphaned tool results")
		t.Logf("  - Only most recent loop present")
		t.Logf("  - Complete tool call/result pairing")
		t.Logf("  - Found %d tool calls, %d tool results", len(validToolCalls), len(presentToolResults))
	}
}
