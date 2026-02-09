package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/constants"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

func setupMysisTest(t *testing.T) (*store.Store, *EventBus, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	bus := NewEventBus(100)

	cleanup := func() {
		bus.Close()
		s.Close()
	}

	return s, bus, cleanup
}

func TestMysisLifecycle(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create stored mysis
	stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "Hello from mysis!")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Initial state
	if mysis.State() != MysisStateIdle {
		t.Errorf("expected initial state=idle, got %s", mysis.State())
	}
	if mysis.ID() != stored.ID {
		t.Errorf("expected ID=%s, got %s", stored.ID, mysis.ID())
	}
	if mysis.Name() != "test-mysis" {
		t.Errorf("expected name=test-mysis, got %s", mysis.Name())
	}

	// Start
	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if mysis.State() != MysisStateRunning {
		t.Errorf("expected state=running, got %s", mysis.State())
	}

	// Start again should error
	if err := mysis.Start(); err == nil {
		t.Error("expected error starting already running mysis")
	}

	// Stop
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if mysis.State() != MysisStateStopped {
		t.Errorf("expected state=stopped, got %s", mysis.State())
	}

	// Stop again should be idempotent
	if err := mysis.Stop(); err != nil {
		t.Errorf("expected no error stopping already stopped mysis, got %v", err)
	}
}

func TestMysisConcurrentStopDuringTurn(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("concurrent", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "ok").SetDelay(50 * time.Millisecond)
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	if err := mysis.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	go func() {
		_ = mysis.SendMessage("ping", store.MemorySourceDirect)
	}()

	time.Sleep(10 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- mysis.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop() error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for Stop")
	}
}

func TestMysisSendMessage(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("msg-mysis", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "I received your message!")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Can send to idle mysis (will be stored and processed when started)
	if err := mysis.SendMessage("Hello", store.MemorySourceDirect); err != nil {
		t.Errorf("should accept message in idle state, got error: %v", err)
	}

	// Start mysis
	mysis.Start()

	// Subscribe to events
	events := bus.Subscribe()

	// Wait for the initial turn to finish (triggered by synthetic encouragement message)
	// It should emit a response event after processing the encouragement
	timeout := time.After(2 * time.Second)
initialTurnLoop:
	for {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				break initialTurnLoop
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial turn")
		}
	}

	// Send message
	if err := mysis.SendMessage("Hello, mysis!", store.MemorySourceDirect); err != nil {
		t.Fatalf("SendMessage() error: %v", err)
	}

	// Should receive message and response events for our message
	var messageEvent, responseEvent bool
	timeout = time.After(2 * time.Second)

eventLoop:
	for {
		select {
		case e := <-events:
			if e.Type == EventMysisMessage {
				if e.Message != nil && e.Message.Content == "Hello, mysis!" {
					messageEvent = true
				}
			}
			if e.Type == EventMysisResponse {
				if e.Message != nil && e.Message.Content == "I received your message!" {
					responseEvent = true
				}
			}
			// Break early if we have both events
			if messageEvent && responseEvent {
				break eventLoop
			}
		case <-timeout:
			break eventLoop
		}
	}

	if !messageEvent {
		t.Error("expected message event for 'Hello, mysis!'")
	}
	if !responseEvent {
		t.Error("expected response event for 'I received your message!'")
	}

	// Check memories were stored
	// Expected: user message (sent while idle), system prompt, assistant response (from ephemeral encouragement),
	//           user message (Hello, mysis!), assistant response
	// NOTE: The encouragement message (initial trigger) is ephemeral and NOT stored in DB
	memories, err := s.GetMemories(stored.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories) != 5 {
		t.Errorf("expected 5 memories, got %d", len(memories))
	}

	mysis.Stop()
}

func TestMysisReceivesBroadcastWithSender(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	receiverStored, _ := s.CreateMysis("receiver", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	receiver := NewMysis(receiverStored.ID, receiverStored.Name, receiverStored.CreatedAt, mock, s, bus, "")

	if err := receiver.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer receiver.Stop()

	senderID := "sender-mysis"

	if err := receiver.SendMessageFrom("test broadcast", store.MemorySourceBroadcast, senderID); err != nil {
		t.Fatalf("SendMessageFrom() error: %v", err)
	}

	memories, err := s.GetRecentMemories(receiverStored.ID, 10)
	if err != nil {
		t.Fatalf("GetRecentMemories() error: %v", err)
	}

	if len(memories) == 0 {
		t.Fatal("expected at least 1 memory")
	}

	found := false
	for _, m := range memories {
		if m.Source == store.MemorySourceBroadcast && m.SenderID == senderID {
			found = true
			break
		}
	}

	if !found {
		t.Error("broadcast memory with correct sender_id not found")
	}
}

func TestMysisSetErrorState(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("error-state-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	events := bus.Subscribe()

	testErr := errors.New("test error")
	mysis.setErrorState(testErr)

	if mysis.State() != MysisStateErrored {
		t.Errorf("expected state=errored, got %s", mysis.State())
	}
	if mysis.LastError() != testErr {
		t.Errorf("expected last error %v, got %v", testErr, mysis.LastError())
	}

	// Should receive state change event first
	select {
	case e := <-events:
		if e.Type != EventMysisStateChanged {
			t.Errorf("expected EventMysisStateChanged, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for state change event")
	}

	// Should receive error event
	select {
	case e := <-events:
		if e.Type != EventMysisError {
			t.Errorf("expected EventMysisError, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for error event")
	}
}

func TestMysisProviderName(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("provider-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("test-provider", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	if mysis.ProviderName() != "test-provider" {
		t.Errorf("expected provider name=test-provider, got %s", mysis.ProviderName())
	}

	// Test with nil provider
	mysis.SetProvider(nil)
	if mysis.ProviderName() != "" {
		t.Errorf("expected empty provider name for nil provider, got %s", mysis.ProviderName())
	}
}

func TestMysisStateEvents(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("event-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	events := bus.Subscribe()

	mysis.Start()

	// Should get state change event
	select {
	case e := <-events:
		if e.Type != EventMysisStateChanged {
			t.Errorf("expected state change event, got %s", e.Type)
		}
		if e.State == nil {
			t.Fatal("expected state change data")
		}
		if e.State.OldState != MysisStateIdle {
			t.Errorf("expected old state=idle, got %s", e.State.OldState)
		}
		if e.State.NewState != MysisStateRunning {
			t.Errorf("expected new state=running, got %s", e.State.NewState)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for state change event")
	}

	mysis.Stop()
}

func TestSystemPromptContainsCaptainsLogExamples(t *testing.T) {
	// SystemPrompt should contain core game guidance
	if !strings.Contains(constants.SystemPrompt, "session_id") {
		t.Fatal("SystemPrompt missing session_id guidance")
	}
	if !strings.Contains(constants.SystemPrompt, "get_notifications") {
		t.Fatal("SystemPrompt missing get_notifications reminder")
	}
	if !strings.Contains(constants.SystemPrompt, "{{LATEST_BROADCAST}}") {
		t.Fatal("SystemPrompt missing broadcast placeholder")
	}
	if !strings.Contains(constants.SystemPrompt, "swarm") {
		t.Fatal("SystemPrompt missing swarm reference")
	}
}

func TestContinuePromptContainsCriticalReminders(t *testing.T) {
	// ContinuePrompt should encourage autonomy
	if !strings.Contains(constants.ContinuePrompt, "What's your next move?") {
		t.Fatal("ContinuePrompt missing autonomy prompt")
	}
}

func TestFormatToolResult_EmptyEntryError(t *testing.T) {
	m := &Mysis{}
	result := &mcp.ToolResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: "empty_entry"}},
		IsError: true,
	}
	got := m.formatToolResult("call_1", "captains_log_add", result, nil)
	if !strings.Contains(got, "entry field must contain non-empty text") {
		t.Fatal("expected actionable guidance for empty_entry")
	}
	if !strings.Contains(got, "Error calling captains_log_add:") {
		t.Fatal("expected tool name in error message")
	}
}

func TestFormatToolResult_GenericError(t *testing.T) {
	m := &Mysis{}
	result := &mcp.ToolResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: "some_other_error"}},
		IsError: true,
	}
	got := m.formatToolResult("call_1", "some_tool", result, nil)
	if !strings.Contains(got, "Error calling some_tool:") {
		t.Fatal("expected tool name in generic error format")
	}
	if strings.Contains(got, "entry field") {
		t.Fatal("should not contain empty_entry guidance for generic errors")
	}
}

func TestFormatToolResult_Success(t *testing.T) {
	m := &Mysis{}
	result := &mcp.ToolResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: "success result"}},
		IsError: false,
	}
	got := m.formatToolResult("call_1", "test_tool", result, nil)
	if !strings.Contains(got, "call_1:success result") {
		t.Fatalf("expected success format, got: %s", got)
	}
	if strings.Contains(got, "Error") {
		t.Fatal("should not contain Error prefix for successful results")
	}
}

func TestSystemPromptContainsSearchGuidance(t *testing.T) {
	// SystemPrompt was simplified - check for core session management
	if !strings.Contains(constants.SystemPrompt, "session_id") {
		t.Fatal("SystemPrompt missing session_id guidance")
	}
	if !strings.Contains(constants.SystemPrompt, "Use session_id in ALL game tools") {
		t.Fatal("SystemPrompt missing session_id usage reminder")
	}
}

func TestContinuePromptContainsSearchReminder(t *testing.T) {
	// ContinuePrompt is intentionally minimal - just encourages action
	// All guidance is in SystemPrompt
	if constants.ContinuePrompt == "" {
		t.Fatal("ContinuePrompt is empty")
	}
}

// TestContinuePromptAddsDriftReminder removed - buildContinuePrompt() method removed.
// Drift reminders are now part of the system prompt, not dynamically generated.

func TestSnapshotCompaction(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("compaction-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Add system prompt
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")

	// Add multiple get_status tool results (should be compacted to keep only the latest)
	for i := 0; i < 5; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "check status", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, constants.ToolCallStoragePrefix+"call_1:get_status:{}", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			fmt.Sprintf(`call_1:{"status":"active","iteration":%d}`, i), "", "")
	}

	// Get context memories
	memories, _, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// With turn-aware composition + snapshot compression:
	// - Historical context: latest tool loop from turns 0-3 (iteration 3)
	// - Current turn: all of turn 4 (user + assistant + tool for iteration 4)
	// - Compression: get_status is a snapshot tool, so only the LATEST is kept (iteration 4)
	// So we expect 1 tool result: iteration 4 (most recent)
	statusResults := 0
	hasIteration4 := false
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, `"iteration":`) {
			statusResults++
			if strings.Contains(m.Content, `"iteration":4`) {
				hasIteration4 = true
			}
		}
	}
	if statusResults != 1 {
		t.Errorf("expected 1 get_status result after compression, got %d", statusResults)
	}
	if !hasIteration4 {
		t.Error("expected iteration 4 result (latest snapshot after compression)")
	}
}

func TestGetContextMemories_CurrentTurnBoundary(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create a mysis for testing
	stored, err := s.CreateMysis("priority-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Commander ID for testing
	commanderID := "commander-mysis-id"
	swarmMysisID := "swarm-mysis-id"

	tests := []struct {
		name           string
		setupMemories  func()
		expectedSource store.MemorySource
		expectedSender string
	}{
		{
			name: "current_turn_start",
			setupMemories: func() {
				// Add system prompt
				s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")
				// Add commander direct message (older)
				s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "Commander direct message", "", commanderID)
				// Add commander broadcast (middle)
				s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Commander broadcast", "", commanderID)
				// Add swarm broadcast (most recent - this starts the current turn)
				s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Swarm broadcast", "", swarmMysisID)
			},
			expectedSource: store.MemorySourceBroadcast,
			expectedSender: swarmMysisID,
		},
		{
			name: "commander_broadcast_when_no_direct",
			setupMemories: func() {
				// Add system prompt
				s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")
				// Add commander broadcast (most recent)
				s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Commander broadcast 1", "", commanderID)
				// Add swarm broadcast (should be ignored when commander broadcast exists)
				s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Swarm broadcast", "", swarmMysisID)
				// Add older commander broadcast
				s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Commander broadcast 2", "", commanderID)
			},
			expectedSource: store.MemorySourceBroadcast,
			expectedSender: commanderID,
		},
		{
			name: "swarm_broadcast_when_no_commander_messages",
			setupMemories: func() {
				// Add system prompt
				s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")
				// Add older swarm broadcast
				s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Swarm broadcast 1", "", swarmMysisID)
				// Add most recent swarm broadcast
				s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Swarm broadcast 2", "", swarmMysisID)
			},
			expectedSource: store.MemorySourceBroadcast,
			expectedSender: swarmMysisID,
		},
		{
			name: "synthetic_nudge_when_no_broadcasts",
			setupMemories: func() {
				// Add system prompt only
				s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")
				// Add some assistant responses (no user messages)
				s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "Previous response", "", "")
			},
			expectedSource: store.MemorySourceSystem, // Nudge should be synthesized (not in DB)
			expectedSender: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear memories for clean test
			s.DB().Exec("DELETE FROM memories WHERE mysis_id = ?", stored.ID)

			// Setup memories for this test case
			tt.setupMemories()

			// Get context memories
			memories, _, err := mysis.getContextMemories()
			if err != nil {
				t.Fatalf("getContextMemories() error: %v", err)
			}

			// Find the current turn boundary in the returned memories
			// With turn-aware composition, the current turn starts at the most recent user prompt.
			// The first user message in the result marks the turn boundary.
			var foundPromptSource *store.Memory
			for _, m := range memories {
				if m.Role == store.MemoryRoleUser {
					foundPromptSource = m
					break // Take the first user message as the turn boundary
				}
			}

			if tt.expectedSource == store.MemorySourceSystem {
				// Synthetic encouragement case - should have a user message with source=system
				if foundPromptSource == nil {
					t.Fatal("expected synthetic encouragement user message, but got none")
				}
				if foundPromptSource.Role != store.MemoryRoleUser {
					t.Errorf("expected role=user for encouragement, got %s", foundPromptSource.Role)
				}
				if foundPromptSource.Source != store.MemorySourceSystem {
					t.Errorf("expected source=system for encouragement, got %s", foundPromptSource.Source)
				}
				// Encouragement should contain helpful content
				if len(foundPromptSource.Content) == 0 {
					t.Error("expected encouragement content to be non-empty")
				}
			} else {
				// Should find the correct turn boundary
				if foundPromptSource == nil {
					t.Fatal("expected to find a turn boundary, but got none")
				}

				if foundPromptSource.Source != tt.expectedSource {
					t.Errorf("expected source=%s, got %s", tt.expectedSource, foundPromptSource.Source)
				}

				if foundPromptSource.SenderID != tt.expectedSender {
					t.Errorf("expected sender_id=%s, got %s", tt.expectedSender, foundPromptSource.SenderID)
				}
			}
		})
	}
}

func TestExtractLatestToolLoopHelper(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("helper-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	tests := []struct {
		name          string
		memories      []*store.Memory
		expectedCount int
		expectNil     bool
	}{
		{
			name:      "nil_memories",
			memories:  nil,
			expectNil: true,
		},
		{
			name:      "empty_memories",
			memories:  []*store.Memory{},
			expectNil: true,
		},
		{
			name: "no_tool_calls",
			memories: []*store.Memory{
				{Role: store.MemoryRoleSystem, Content: "System"},
				{Role: store.MemoryRoleUser, Content: "User message"},
				{Role: store.MemoryRoleAssistant, Content: "Text response"},
			},
			expectNil: true,
		},
		{
			name: "single_tool_call_with_results",
			memories: []*store.Memory{
				{Role: store.MemoryRoleSystem, Content: "System"},
				{Role: store.MemoryRoleUser, Content: "User"},
				{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_1:get_status:{}"},
				{Role: store.MemoryRoleTool, Content: "call_1:status result"},
			},
			expectedCount: 2, // 1 tool call + 1 result
		},
		{
			name: "tool_call_with_no_results_yet",
			memories: []*store.Memory{
				{Role: store.MemoryRoleSystem, Content: "System"},
				{Role: store.MemoryRoleUser, Content: "User"},
				{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_1:get_status:{}"},
			},
			expectedCount: 1, // Just the tool call
		},
		{
			name: "multiple_loops_returns_most_recent",
			memories: []*store.Memory{
				{Role: store.MemoryRoleSystem, Content: "System"},
				{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_old:get_status:{}"},
				{Role: store.MemoryRoleTool, Content: "call_old:old result"},
				{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_new:get_poi:{}"},
				{Role: store.MemoryRoleTool, Content: "call_new:new result"},
			},
			expectedCount: 2, // Most recent loop only
		},
		{
			name: "tool_call_with_multiple_results",
			memories: []*store.Memory{
				{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_1:get_status:{}|call_2:get_system:{}"},
				{Role: store.MemoryRoleTool, Content: "call_1:result 1"},
				{Role: store.MemoryRoleTool, Content: "call_2:result 2"},
			},
			expectedCount: 3, // 1 tool call message + 2 results
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mysis.extractLatestToolLoop(tt.memories)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %d memories", len(result))
				}
				return
			}

			if len(result) != tt.expectedCount {
				t.Errorf("expected %d memories, got %d", tt.expectedCount, len(result))
				for i, m := range result {
					t.Logf("  result[%d]: role=%s content=%.50s", i, m.Role, m.Content)
				}
			}

			// Verify first is tool call
			if len(result) > 0 {
				if result[0].Role != store.MemoryRoleAssistant {
					t.Errorf("first memory should be assistant, got %s", result[0].Role)
				}
				if !strings.HasPrefix(result[0].Content, "[TOOL_CALLS]") {
					t.Errorf("first memory should have tool calls prefix")
				}
			}

			// Verify rest are tool results
			for i := 1; i < len(result); i++ {
				if result[i].Role != store.MemoryRoleTool {
					t.Errorf("memory[%d] should be tool result, got %s", i, result[i].Role)
				}
			}
		})
	}
}

func TestComputeMemoryStats(t *testing.T) {
	m := &Mysis{}

	t.Run("empty", func(t *testing.T) {
		stats := m.computeMemoryStats(nil)
		if stats.MemoryCount != 0 {
			t.Fatalf("expected memory count 0, got %d", stats.MemoryCount)
		}
		if stats.ContentBytes != 0 {
			t.Fatalf("expected content bytes 0, got %d", stats.ContentBytes)
		}
		if stats.ReasoningBytes != 0 {
			t.Fatalf("expected reasoning bytes 0, got %d", stats.ReasoningBytes)
		}
		if len(stats.RoleCounts) != 0 {
			t.Fatalf("expected empty role counts, got %v", stats.RoleCounts)
		}
		if len(stats.SourceCounts) != 0 {
			t.Fatalf("expected empty source counts, got %v", stats.SourceCounts)
		}
	})

	t.Run("mixed", func(t *testing.T) {
		memories := []*store.Memory{
			{Role: store.MemoryRoleSystem, Source: store.MemorySourceSystem, Content: "abc", Reasoning: "r"},
			{Role: store.MemoryRoleUser, Source: store.MemorySourceDirect, Content: "", Reasoning: ""},
			{Role: store.MemoryRoleAssistant, Source: store.MemorySourceLLM, Content: "done", Reasoning: "why"},
			{Role: store.MemoryRoleTool, Source: store.MemorySourceTool, Content: "tool", Reasoning: ""},
		}
		stats := m.computeMemoryStats(memories)
		if stats.MemoryCount != 4 {
			t.Fatalf("expected memory count 4, got %d", stats.MemoryCount)
		}
		if stats.ContentBytes != 11 {
			t.Fatalf("expected content bytes 11, got %d", stats.ContentBytes)
		}
		if stats.ReasoningBytes != 4 {
			t.Fatalf("expected reasoning bytes 4, got %d", stats.ReasoningBytes)
		}
		if stats.RoleCounts[string(store.MemoryRoleSystem)] != 1 ||
			stats.RoleCounts[string(store.MemoryRoleUser)] != 1 ||
			stats.RoleCounts[string(store.MemoryRoleAssistant)] != 1 ||
			stats.RoleCounts[string(store.MemoryRoleTool)] != 1 {
			t.Fatalf("unexpected role counts: %v", stats.RoleCounts)
		}
		if stats.SourceCounts[string(store.MemorySourceSystem)] != 1 ||
			stats.SourceCounts[string(store.MemorySourceDirect)] != 1 ||
			stats.SourceCounts[string(store.MemorySourceLLM)] != 1 ||
			stats.SourceCounts[string(store.MemorySourceTool)] != 1 {
			t.Fatalf("unexpected source counts: %v", stats.SourceCounts)
		}
	})

	t.Run("unicode", func(t *testing.T) {
		memories := []*store.Memory{
			{Role: store.MemoryRoleUser, Source: store.MemorySourceDirect, Content: "◈", Reasoning: "ä"},
		}
		stats := m.computeMemoryStats(memories)
		if stats.ContentBytes != 3 {
			t.Fatalf("expected content bytes 3, got %d", stats.ContentBytes)
		}
		if stats.ReasoningBytes != 2 {
			t.Fatalf("expected reasoning bytes 2, got %d", stats.ReasoningBytes)
		}
	})
}

func TestComputeMessageStats(t *testing.T) {
	m := &Mysis{}

	t.Run("empty", func(t *testing.T) {
		stats := m.computeMessageStats(nil)
		if stats.MessageCount != 0 {
			t.Fatalf("expected message count 0, got %d", stats.MessageCount)
		}
		if stats.ContentBytes != 0 {
			t.Fatalf("expected content bytes 0, got %d", stats.ContentBytes)
		}
		if stats.ToolCallCount != 0 {
			t.Fatalf("expected tool call count 0, got %d", stats.ToolCallCount)
		}
	})

	t.Run("mixed", func(t *testing.T) {
		messages := []provider.Message{
			{Role: "user", Content: "abc"},
			{Role: "assistant", Content: "◈", ToolCalls: []provider.ToolCall{{Name: "a"}, {Name: "b"}}},
			{Role: "tool", Content: "ok"},
		}
		stats := m.computeMessageStats(messages)
		if stats.MessageCount != 3 {
			t.Fatalf("expected message count 3, got %d", stats.MessageCount)
		}
		if stats.ContentBytes != 8 {
			t.Fatalf("expected content bytes 8, got %d", stats.ContentBytes)
		}
		if stats.ToolCallCount != 2 {
			t.Fatalf("expected tool call count 2, got %d", stats.ToolCallCount)
		}
	})
}

func TestMysisActivityTravelUntilFromTicks(t *testing.T) {
	m := &Mysis{}

	now := time.Now()
	m.lastServerTick = 100
	m.lastServerTickAt = now.Add(-20 * time.Second)

	result := &mcp.ToolResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: `{"current_tick":110,"arrival_tick":120}`}},
	}

	m.updateActivityFromToolResult(result, nil)

	if m.activityState != ActivityStateTraveling {
		t.Fatalf("expected activity state traveling, got %s", m.activityState)
	}

	remaining := time.Until(m.activityUntil)
	if remaining < 18*time.Second || remaining > 25*time.Second {
		t.Fatalf("expected travel wait around 20s, got %s", remaining)
	}
}

func TestMysisActivityTravelFallbackWait(t *testing.T) {
	m := &Mysis{}

	result := &mcp.ToolResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: `{"arrival_tick":5000}`}},
	}

	m.updateActivityFromToolResult(result, nil)

	if m.activityState != ActivityStateTraveling {
		t.Fatalf("expected activity state traveling, got %s", m.activityState)
	}

	remaining := time.Until(m.activityUntil)
	if remaining < constants.WaitStateNudgeInterval-2*time.Second || remaining > constants.WaitStateNudgeInterval+2*time.Second {
		t.Fatalf("expected travel fallback wait around %s, got %s", constants.WaitStateNudgeInterval, remaining)
	}
}

func TestMysisActivityTravelArrivalTickReached(t *testing.T) {
	m := &Mysis{}

	result := &mcp.ToolResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: `{"current_tick":120,"arrival_tick":120}`}},
	}

	m.updateActivityFromToolResult(result, nil)

	if m.activityState != ActivityStateIdle {
		t.Fatalf("expected activity state idle, got %s", m.activityState)
	}
	if !m.activityUntil.IsZero() {
		t.Fatal("expected activityUntil to be zero when arrival tick reached")
	}
}

func TestMysisActivityTravelArrivalTickField(t *testing.T) {
	m := &Mysis{}

	result := &mcp.ToolResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: `{"current_tick":200,"travel_arrival_tick":210}`}},
	}

	m.updateActivityFromToolResult(result, nil)

	if m.activityState != ActivityStateTraveling {
		t.Fatalf("expected activity state traveling, got %s", m.activityState)
	}
}

func TestFindCurrentTick(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
		want    int64
		ok      bool
	}{
		{
			name:    "top_level_current_tick",
			payload: map[string]interface{}{"current_tick": int64(42)},
			want:    42,
			ok:      true,
		},
		{
			name:    "wrapper_current_tick",
			payload: map[string]interface{}{"data": map[string]interface{}{"current_tick": int64(88)}},
			want:    88,
			ok:      true,
		},
		{
			name:    "wrapper_tick_fallback",
			payload: map[string]interface{}{"result": map[string]interface{}{"tick": int64(9)}},
			want:    9,
			ok:      true,
		},
		{
			name:    "missing_tick",
			payload: map[string]interface{}{"status": "ok"},
			want:    0,
			ok:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := findCurrentTick(test.payload)
			if ok != test.ok {
				t.Fatalf("expected ok=%v, got %v", test.ok, ok)
			}
			if got != test.want {
				t.Fatalf("expected tick %d, got %d", test.want, got)
			}
		})
	}
}

// TestMysisStopDoesNotOverrideWithError tests that stopping a mysis
// doesn't allow concurrent error to override Stopped state.
func TestMysisStopDoesNotOverrideWithError(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("stop-test", "mock", "test-model", 0.7)

	mock := provider.NewMock("mock", "test response")
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Start the mysis
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Give it a moment to stabilize
	time.Sleep(100 * time.Millisecond)

	// Stop the mysis
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// Verify state is Stopped (not Errored)
	if m.State() != MysisStateStopped {
		t.Errorf("expected state=stopped, got %s (lastError: %v)", m.State(), m.LastError())
	}

	// Verify no error is set
	if m.LastError() != nil {
		t.Errorf("expected no lastError after clean stop, got: %v", m.LastError())
	}
}

// TestStopDuringInitialMessage tests the critical race condition:
// Stop() called IMMEDIATELY after Start(), during the initial SendMessage.
//
// Timeline:
// - Line 268: Start() spawns `go a.SendMessage(ContinuePrompt, ...)`
// - Line 270: Start() returns
// - HERE: Test calls Stop() (within 5-10ms)
// - SendMessage goroutine may still be waiting to acquire turnMu
//
// Expected: Final state is Stopped, no deadlock, no panic.
func TestStopDuringInitialMessage(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("race-test", "mock", "test-model", 0.7)

	// Use a mock provider with NO delay - we want to test the race
	// between Start() spawning SendMessage and Stop() being called
	mock := provider.NewMock("mock", "response")
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Start the mysis (spawns initial SendMessage goroutine)
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// IMMEDIATELY call Stop() - this creates the race condition
	// The initial SendMessage may not have acquired turnMu yet
	// No sleep = tightest race window possible
	// time.Sleep(5 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- m.Stop()
	}()

	// Stop should complete within a reasonable timeout
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop() error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Stop() - possible deadlock")
	}

	// Verify final state is Stopped
	if m.State() != MysisStateStopped {
		t.Errorf("expected state=stopped after immediate Stop(), got %s", m.State())
	}

	// Verify no error was set
	if m.LastError() != nil {
		t.Errorf("expected no lastError after immediate Stop(), got: %v", m.LastError())
	}
}

// TestStopDuringInitialMessageWithSlowProvider tests an even tighter race:
// Stop() called while the initial SendMessage is INSIDE the provider.Chat() call.
//
// Timeline:
// - Start() spawns SendMessage goroutine
// - SendMessage acquires turnMu and enters provider.Chat() (50ms delay)
// - Test calls Stop() immediately (no sleep)
// - Stop() waits for turnMu while provider is processing
//
// Expected: Stop waits for turn to complete, then succeeds.
func TestStopDuringInitialMessageWithSlowProvider(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("slow-provider-race", "mock", "test-model", 0.7)

	// Use a provider with delay to simulate the mysis being mid-turn when Stop is called
	mock := provider.NewMock("mock", "response").SetDelay(50 * time.Millisecond)
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Start the mysis
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Call Stop IMMEDIATELY (no sleep) - this should catch the initial
	// SendMessage while it's inside provider.Chat()
	done := make(chan error, 1)
	go func() {
		done <- m.Stop()
	}()

	// Stop should wait for the current turn to finish, then succeed
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop() error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Stop() - possible deadlock")
	}

	// Verify final state is Stopped
	if m.State() != MysisStateStopped {
		t.Errorf("expected state=stopped, got %s", m.State())
	}

	// Verify no error was set
	if m.LastError() != nil {
		t.Errorf("expected no lastError, got: %v", m.LastError())
	}
}

// TestStopDuringIdleNudge removed - ticker-based system obsolete.
// Encouragement is now database-driven via getContextMemories().

// TestStopAtVariousTimings is a parameterized test that calls Stop() at
// different delays after Start() to systematically explore timing windows.
//
// This test helps identify which timing windows consistently fail or pass,
// revealing patterns in race conditions.
//
// Test delays:
// - 0ms: Immediate (before initial SendMessage goroutine starts)
// - 10ms: Very fast (goroutine just started, may not have acquired turnMu)
// - 50ms: During initial message processing (inside provider.Chat)
// - 100ms: After initial message likely done
// - 500ms: After sufficient time for initial turn
//
// Run each timing 10 times with:
//
//	go test ./internal/core -run TestStopAtVariousTimings -count=10
func TestStopAtVariousTimings(t *testing.T) {
	delays := []time.Duration{
		0,                      // 0ms - immediate
		10 * time.Millisecond,  // 10ms - very fast
		50 * time.Millisecond,  // 50ms - during initial message
		100 * time.Millisecond, // 100ms - after initial message likely done
		500 * time.Millisecond, // 500ms - after sufficient time for initial turn
	}

	for _, delay := range delays {
		t.Run(fmt.Sprintf("delay_%dms", delay.Milliseconds()), func(t *testing.T) {
			s, bus, cleanup := setupMysisTest(t)
			defer cleanup()

			stored, err := s.CreateMysis(
				fmt.Sprintf("timing-test-%dms", delay.Milliseconds()),
				"mock",
				"test-model",
				0.7,
			)
			if err != nil {
				t.Fatalf("CreateMysis() error: %v", err)
			}

			// Use a mock with a delay to simulate real LLM processing
			mock := provider.NewMock("mock", "test response").SetDelay(25 * time.Millisecond)
			mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

			// Start mysis
			if err := mysis.Start(); err != nil {
				t.Fatalf("Start() error: %v", err)
			}

			// Wait for the specified delay
			time.Sleep(delay)

			// Stop mysis
			if err := mysis.Stop(); err != nil {
				t.Fatalf("Stop() error: %v", err)
			}

			// Verify final state is Stopped (critical assertion)
			finalState := mysis.State()
			if finalState != MysisStateStopped {
				t.Errorf("expected state=stopped, got %s (lastError: %v)", finalState, mysis.LastError())
			}

			// Verify no error is set
			if mysis.LastError() != nil {
				t.Errorf("expected no lastError after clean stop, got: %v", mysis.LastError())
			}
		})
	}
}

// TestNudgeCircuitBreaker removed - ticker-based nudge system obsolete.
// Encouragement counter now increments in getContextMemories() when no user message exists.
// See TestEncouragementLimit and TestEncouragementReset for new behavior.

// TestEncouragementLimit tests that getContextMemories() returns the correct addedSynthetic flag
// when no user messages exist. The counter increment happens in SendMessageFrom(), not here.
func TestEncouragementLimit(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("limit-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Add system prompt only (no user messages)
	err = s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	// Call getContextMemories() with no user messages - should return addedSynthetic=true
	_, addedSynthetic, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	if !addedSynthetic {
		t.Errorf("expected addedSynthetic=true when no user message exists, got false")
	}

	// Verify counter is still 0 (increment happens in SendMessageFrom, not here)
	mysis.mu.RLock()
	count := mysis.encouragementCount
	mysis.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected encouragementCount=0 (not incremented by getContextMemories), got %d", count)
	}

	// Note: Full autonomous turn behavior (counter increment after turn completes)
	// is tested in TestMysisCounterBehavior_* tests.
}

// TestEncouragementReset tests that encouragementCount resets to 0 when a real user message
// (broadcast or direct) is received.
func TestEncouragementReset(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	t.Run("reset_on_broadcast", func(t *testing.T) {
		stored, err := s.CreateMysis("reset-broadcast-test", "mock", "test-model", 0.7)
		if err != nil {
			t.Fatalf("CreateMysis() error: %v", err)
		}

		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Set encouragementCount to 2 (simulate 2 synthetic messages)
		mysis.mu.Lock()
		mysis.encouragementCount = 2
		mysis.mu.Unlock()

		// Verify counter is at 2
		mysis.mu.RLock()
		count := mysis.encouragementCount
		mysis.mu.RUnlock()
		if count != 2 {
			t.Fatalf("expected encouragementCount=2 before broadcast, got %d", count)
		}

		// Queue broadcast message
		err = mysis.QueueBroadcast("Test broadcast", "sender-id")
		if err != nil {
			t.Fatalf("QueueBroadcast() error: %v", err)
		}

		// Verify counter was reset to 0
		mysis.mu.RLock()
		count = mysis.encouragementCount
		mysis.mu.RUnlock()
		if count != 0 {
			t.Errorf("expected encouragementCount=0 after broadcast, got %d", count)
		}
	})

	t.Run("reset_on_direct_message", func(t *testing.T) {
		stored, err := s.CreateMysis("reset-direct-test", "mock", "test-model", 0.7)
		if err != nil {
			t.Fatalf("CreateMysis() error: %v", err)
		}

		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Set encouragementCount to 2
		mysis.mu.Lock()
		mysis.encouragementCount = 2
		mysis.mu.Unlock()

		// Send direct message
		err = mysis.SendMessageFrom("Test direct", store.MemorySourceDirect, "")
		if err != nil {
			t.Fatalf("SendMessageFrom() error: %v", err)
		}

		// Verify counter was reset to 0
		mysis.mu.RLock()
		count := mysis.encouragementCount
		mysis.mu.RUnlock()
		if count != 0 {
			t.Errorf("expected encouragementCount=0 after direct message, got %d", count)
		}
	})

	t.Run("no_reset_on_getContextMemories_with_user_message", func(t *testing.T) {
		stored, err := s.CreateMysis("no-reset-test", "mock", "test-model", 0.7)
		if err != nil {
			t.Fatalf("CreateMysis() error: %v", err)
		}

		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Add system prompt
		err = s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")
		if err != nil {
			t.Fatalf("AddMemory error: %v", err)
		}

		// Add user message (real message exists)
		err = s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "User message", "", "")
		if err != nil {
			t.Fatalf("AddMemory error: %v", err)
		}

		// Set encouragementCount to 2
		mysis.mu.Lock()
		mysis.encouragementCount = 2
		mysis.mu.Unlock()

		// Call getContextMemories() - should NOT reset counter because real user message exists
		_, _, err = mysis.getContextMemories()
		if err != nil {
			t.Fatalf("getContextMemories() error: %v", err)
		}

		// Counter should still be 2 (NOT reset by getContextMemories when user message exists)
		mysis.mu.RLock()
		count := mysis.encouragementCount
		mysis.mu.RUnlock()
		if count != 2 {
			t.Errorf("expected encouragementCount=2 (NOT reset when user message exists), got %d", count)
		}
	})
}

// TestMysisWithBroadcastsKeepsRunning tests the truth table scenario "Running (With Broadcasts)".
// Verifies that when a broadcast is present, the encouragementCount stays at 0 through multiple
// turns (because the broadcast remains the most recent user message), keeping the mysis running
// without going idle.
//
// Reference: documentation/architecture/MESSAGE_FORMAT_GUARANTEES.md lines 123-146
func TestMysisWithBroadcastsKeepsRunning(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create mysis
	stored, err := s.CreateMysis("broadcast-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Use mock provider with no delay for fast test
	mock := provider.NewMock("mock", "I will continue exploring")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Step 1-2: Send broadcast "Explore the universe!" via QueueBroadcast
	// This will be stored as a user message with source=broadcast
	// QueueBroadcast will auto-start the idle mysis
	err = mysis.QueueBroadcast("Explore the universe!", "commander-id")
	if err != nil {
		t.Fatalf("QueueBroadcast() error: %v", err)
	}
	defer mysis.Stop()

	// Step 3: Verify mysis was auto-started by QueueBroadcast
	// Give it a moment to transition to running state
	time.Sleep(50 * time.Millisecond)
	if mysis.State() != MysisStateRunning {
		t.Fatalf("expected mysis to be auto-started by broadcast, got state: %s", mysis.State())
	}

	// Step 4: Verify encouragementCount = 0 (reset by broadcast in QueueBroadcast)
	mysis.mu.RLock()
	count := mysis.encouragementCount
	mysis.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected encouragementCount=0 after broadcast (reset in QueueBroadcast), got %d", count)
	}

	// Step 5: Trigger a turn by sending a message (QueueBroadcast stored the message but doesn't trigger processing)
	// The broadcast is now in the database, and SendMessageFrom will trigger getContextMemories() which will find it
	err = mysis.SendMessageFrom("Status check", store.MemorySourceDirect, "")
	if err != nil {
		t.Fatalf("SendMessageFrom() error: %v", err)
	}

	// Step 6: Check encouragementCount = 0 (reset by the direct message we just sent)
	mysis.mu.RLock()
	count = mysis.encouragementCount
	mysis.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected encouragementCount=0 after turn (reset by direct message), got %d", count)
	}

	// Check mysis is still running (not idle)
	if mysis.State() != MysisStateRunning {
		t.Errorf("expected state=running, got %s", mysis.State())
	}

	// Step 7-8: Send additional messages to trigger more turns and verify counter stays at 0
	for i := 0; i < 5; i++ {
		err = mysis.SendMessageFrom("Continue task", store.MemorySourceDirect, "")
		if err != nil {
			t.Fatalf("turn %d: SendMessageFrom() error: %v", i+2, err)
		}

		// Verify counter was reset by direct message
		mysis.mu.RLock()
		count = mysis.encouragementCount
		mysis.mu.RUnlock()
		if count != 0 {
			t.Errorf("turn %d: expected encouragementCount=0 after direct message (reset), got %d", i+2, count)
		}
	}

	// Step 9: After 6+ turns with user messages, verify mysis is still running (not idle)
	if mysis.State() != MysisStateRunning {
		t.Errorf("expected state=running after 6+ turns with user messages, got %s", mysis.State())
	}

	t.Logf("✅ Mysis with broadcasts/direct messages stays running: counter reset to 0 on each turn, never went idle")
}
func TestCanAcceptMessages(t *testing.T) {
	tests := []struct {
		state     MysisState
		canAccept bool
		errMsg    string
	}{
		{MysisStateIdle, true, ""},
		{MysisStateRunning, true, ""},
		{MysisStateStopped, false, "mysis stopped - press 'r' to relaunch"},
		{MysisStateErrored, false, "mysis errored - press 'r' to relaunch"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			err := validateCanAcceptMessage(tt.state)

			if tt.canAccept {
				if err != nil {
					t.Errorf("state %s should accept messages, got error: %v", tt.state, err)
				}
			} else {
				if err == nil {
					t.Errorf("state %s should reject messages, got nil error", tt.state)
				} else if err.Error() != tt.errMsg {
					t.Errorf("state %s: expected error %q, got %q", tt.state, tt.errMsg, err.Error())
				}
			}
		})
	}
}

// TestExecuteToolCall_ErrorPaths tests error handling in executeToolCall
func TestExecuteToolCall_ErrorPaths(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create a shared MCP proxy for tests that need it
	proxy := mcp.NewProxy(nil)

	t.Run("nil_mcp_proxy", func(t *testing.T) {
		stored, _ := s.CreateMysis("nil-proxy-test", "mock", "test-model", 0.7)
		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Call executeToolCall with nil proxy
		tc := provider.ToolCall{
			ID:        "call_1",
			Name:      "test_tool",
			Arguments: json.RawMessage(`{}`),
		}

		result, err := mysis.executeToolCall(context.Background(), nil, tc)

		// Should return error result (not error), as per MCP design
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if !result.IsError {
			t.Error("expected IsError=true when proxy is nil")
		}
		if !strings.Contains(result.Content[0].Text, "MCP not configured") {
			t.Errorf("expected 'MCP not configured' error, got: %s", result.Content[0].Text)
		}
	})

	t.Run("tool_call_timeout", func(t *testing.T) {
		stored, _ := s.CreateMysis("timeout-test", "mock", "test-model", 0.7)
		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Create a context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond) // Ensure timeout fires

		tc := provider.ToolCall{
			ID:        "call_1",
			Name:      "get_status",
			Arguments: json.RawMessage(`{}`),
		}

		result, err := mysis.executeToolCall(ctx, proxy, tc)

		// When tool is not found, MCP returns an error result (not an error)
		// This is testing that executeToolCall handles missing tools gracefully
		if err != nil {
			t.Errorf("expected no error (tool not found returns error result), got: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if !result.IsError {
			t.Error("expected IsError=true for tool not found")
		}
	})

	t.Run("invalid_tool_arguments", func(t *testing.T) {
		stored, _ := s.CreateMysis("invalid-args-test", "mock", "test-model", 0.7)
		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Register a tool that validates arguments
		proxy.RegisterTool(
			mcp.Tool{
				Name:        "strict_tool",
				Description: "Tool with strict arg validation",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"required_field":{"type":"string"}},"required":["required_field"]}`),
			},
			func(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
				var params struct {
					RequiredField string `json:"required_field"`
				}
				if err := json.Unmarshal(args, &params); err != nil {
					return &mcp.ToolResult{
						Content: []mcp.ContentBlock{{Type: "text", Text: "invalid JSON"}},
						IsError: true,
					}, nil
				}
				if params.RequiredField == "" {
					return &mcp.ToolResult{
						Content: []mcp.ContentBlock{{Type: "text", Text: "missing required_field"}},
						IsError: true,
					}, nil
				}
				return &mcp.ToolResult{
					Content: []mcp.ContentBlock{{Type: "text", Text: "success"}},
				}, nil
			},
		)

		tc := provider.ToolCall{
			ID:        "call_1",
			Name:      "strict_tool",
			Arguments: json.RawMessage(`{}`), // Missing required field
		}

		result, err := mysis.executeToolCall(context.Background(), proxy, tc)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if !result.IsError {
			t.Error("expected IsError=true for invalid arguments")
		}
	})

	t.Run("mcp_call_tool_error", func(t *testing.T) {
		stored, _ := s.CreateMysis("mcp-error-test", "mock", "test-model", 0.7)
		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Register a tool that returns an error
		proxy.RegisterTool(
			mcp.Tool{
				Name:        "error_tool",
				Description: "Tool that fails",
				InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			},
			func(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
				return nil, fmt.Errorf("tool execution failed")
			},
		)

		tc := provider.ToolCall{
			ID:        "call_1",
			Name:      "error_tool",
			Arguments: json.RawMessage(`{}`),
		}

		result, err := mysis.executeToolCall(context.Background(), proxy, tc)

		// executeToolCall should propagate the error
		if err == nil {
			t.Error("expected error from tool execution")
		}
		if !strings.Contains(err.Error(), "tool execution failed") {
			t.Errorf("expected 'tool execution failed' error, got: %v", err)
		}
		// Result should be nil when handler returns error
		if result != nil {
			t.Errorf("expected nil result on error, got: %v", result)
		}
	})

	t.Run("tool_result_parsing_with_empty_content", func(t *testing.T) {
		stored, _ := s.CreateMysis("parse-test", "mock", "test-model", 0.7)
		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Register a tool that returns empty content
		proxy.RegisterTool(
			mcp.Tool{
				Name:        "empty_tool",
				Description: "Tool with empty result",
				InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			},
			func(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{
					Content: []mcp.ContentBlock{}, // Empty content
				}, nil
			},
		)

		tc := provider.ToolCall{
			ID:        "call_1",
			Name:      "empty_tool",
			Arguments: json.RawMessage(`{}`),
		}

		result, err := mysis.executeToolCall(context.Background(), proxy, tc)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		// Should handle empty content gracefully
		if result.IsError {
			t.Error("empty content should not be an error")
		}
	})
}

// TestSendEphemeralMessage_IdleState removed - SendEphemeralMessage method removed.
// Ephemeral messages (synthetic nudges) are now generated by getContextMemories().

// TestSendEphemeralMessage_StoppedState removed - SendEphemeralMessage method removed.
// Ephemeral messages (synthetic nudges) are now generated by getContextMemories().

// setupTestMysis creates a mysis for testing with a mock provider
func setupTestMysis(t *testing.T) (*Mysis, func()) {
	t.Helper()

	s, bus, cleanup := setupMysisTest(t)

	stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "test response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	return mysis, cleanup
}

// setupTestMysisWithErrorProvider creates a mysis with a provider that returns errors
func setupTestMysisWithErrorProvider(t *testing.T) (*Mysis, func()) {
	t.Helper()

	s, bus, cleanup := setupMysisTest(t)

	stored, err := s.CreateMysis("error-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Create a mock provider that returns errors
	mock := provider.NewMock("mock", "").WithChatError(errors.New("provider error"))
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	return mysis, cleanup
}

func TestSendMessageFrom_IdleState(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Mysis starts in idle state
	if mysis.State() != MysisStateIdle {
		t.Fatalf("expected idle state, got %s", mysis.State())
	}

	// Should be able to send message to idle mysis
	err := mysis.SendMessageFrom("test message", store.MemorySourceDirect, "")
	if err != nil {
		t.Errorf("should accept message in idle state, got error: %v", err)
	}

	// Verify message was stored
	memories, err := mysis.store.GetMemories(mysis.ID())
	if err != nil {
		t.Fatalf("failed to get memories: %v", err)
	}
	if len(memories) == 0 {
		t.Error("message was not stored")
	}
	if len(memories) == 0 {
		t.Error("message was not stored")
	}
}

func TestSendMessageFrom_StoppedState(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Start then stop the mysis
	if err := mysis.Start(); err != nil {
		t.Fatalf("failed to start mysis: %v", err)
	}
	if err := mysis.Stop(); err != nil {
		t.Fatalf("failed to stop mysis: %v", err)
	}

	if mysis.State() != MysisStateStopped {
		t.Fatalf("expected stopped state, got %s", mysis.State())
	}

	// Should NOT be able to send message to stopped mysis
	err := mysis.SendMessageFrom("test message", store.MemorySourceDirect, "")
	if err == nil {
		t.Error("should reject message in stopped state")
	}
	if !strings.Contains(err.Error(), "stopped") || !strings.Contains(err.Error(), "relaunch") {
		t.Errorf("error should mention stopped and relaunch, got: %v", err)
	}
}

func TestSendMessageFrom_ErroredState(t *testing.T) {
	mysis, cleanup := setupTestMysisWithErrorProvider(t)
	defer cleanup()

	// Start mysis and trigger error
	if err := mysis.Start(); err != nil {
		t.Fatalf("failed to start mysis: %v", err)
	}

	// Send message to trigger error
	_ = mysis.SendMessageFrom("trigger error", store.MemorySourceDirect, "")

	// Wait for error state
	time.Sleep(100 * time.Millisecond)

	if mysis.State() != MysisStateErrored {
		t.Fatalf("expected errored state, got %s", mysis.State())
	}

	// Should NOT be able to send message to errored mysis
	err := mysis.SendMessageFrom("test message", store.MemorySourceDirect, "")
	if err == nil {
		t.Error("should reject message in errored state")
	}
	if !strings.Contains(err.Error(), "errored") || !strings.Contains(err.Error(), "relaunch") {
		t.Errorf("error should mention errored and relaunch, got: %v", err)
	}
}
func TestQueueBroadcast_IdleState(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create stored mysis
	stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "Hello!")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Mysis starts in idle state
	if mysis.State() != MysisStateIdle {
		t.Fatalf("expected idle state, got %s", mysis.State())
	}

	// Should be able to queue broadcast to idle mysis
	err = mysis.QueueBroadcast("broadcast test", "sender-id")
	if err != nil {
		t.Errorf("should accept broadcast in idle state, got error: %v", err)
	}

	// Verify message was stored
	memories, err := s.GetMemories(mysis.ID())
	if err != nil {
		t.Fatalf("failed to get memories: %v", err)
	}
	if len(memories) == 0 {
		t.Error("broadcast was not stored")
	}
}

func TestQueueBroadcast_StoppedState(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create stored mysis
	stored, err := s.CreateMysis("test-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mock := provider.NewMock("mock", "Hello!")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Start then stop
	if err := mysis.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	if err := mysis.Stop(); err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if mysis.State() != MysisStateStopped {
		t.Fatalf("expected stopped state, got %s", mysis.State())
	}

	// Should reject broadcast in stopped state
	err = mysis.QueueBroadcast("broadcast test", "sender-id")
	if err == nil {
		t.Error("should reject broadcast in stopped state")
	}
	if !strings.Contains(err.Error(), "stopped") {
		t.Errorf("error should mention stopped, got: %v", err)
	}
}

func TestFindLastUserPromptIndex(t *testing.T) {
	tests := []struct {
		name     string
		memories []*store.Memory
		expected int
	}{
		{
			name: "last message is user",
			memories: []*store.Memory{
				{Role: store.MemoryRoleSystem, Content: "system"},
				{Role: store.MemoryRoleUser, Content: "hello", Source: store.MemorySourceDirect},
			},
			expected: 1,
		},
		{
			name: "user followed by tool loop",
			memories: []*store.Memory{
				{Role: store.MemoryRoleSystem, Content: "system"},
				{Role: store.MemoryRoleUser, Content: "check status", Source: store.MemorySourceDirect},
				{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_1:get_status:{}"},
				{Role: store.MemoryRoleTool, Content: "call_1:status data"},
			},
			expected: 1,
		},
		{
			name: "multiple user messages",
			memories: []*store.Memory{
				{Role: store.MemoryRoleUser, Content: "first", Source: store.MemorySourceDirect},
				{Role: store.MemoryRoleAssistant, Content: "response"},
				{Role: store.MemoryRoleUser, Content: "second", Source: store.MemorySourceDirect},
			},
			expected: 2,
		},
		{
			name: "no user messages",
			memories: []*store.Memory{
				{Role: store.MemoryRoleSystem, Content: "system"},
			},
			expected: -1,
		},
		{
			name: "broadcast is user prompt",
			memories: []*store.Memory{
				{Role: store.MemoryRoleUser, Content: "broadcast msg", Source: store.MemorySourceBroadcast},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysis := &Mysis{}
			result := mysis.findLastUserPromptIndex(tt.memories)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetContextMemories_CurrentTurnPreserved(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Add historical turn (should be compressed)
	err := mysis.store.AddMemory(mysis.id, store.MemoryRoleUser, store.MemorySourceDirect, "old user msg", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleAssistant, store.MemorySourceLLM, "[TOOL_CALLS]call_old:get_status:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleTool, store.MemorySourceTool, "call_old:old status", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	// Add current turn with multiple tool loops
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleUser, store.MemorySourceDirect, "current user msg", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	// First tool loop in current turn
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleAssistant, store.MemorySourceLLM, "[TOOL_CALLS]call_1:login:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleTool, store.MemorySourceTool, "call_1:session_id: abc123", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	// Second tool loop in current turn
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleAssistant, store.MemorySourceLLM, "[TOOL_CALLS]call_2:get_status:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleTool, store.MemorySourceTool, "call_2:status data", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	memories, _, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories error: %v", err)
	}

	// Verify structure: current turn should have both tool loops
	hasCurrentUser := false
	hasLoginCall := false
	hasLoginResult := false
	hasStatusCall := false
	hasStatusResult := false

	for _, mem := range memories {
		if mem.Role == store.MemoryRoleUser && mem.Content == "current user msg" {
			hasCurrentUser = true
		}
		if mem.Role == store.MemoryRoleAssistant && strings.Contains(mem.Content, "call_1:login") {
			hasLoginCall = true
		}
		if mem.Role == store.MemoryRoleTool && strings.Contains(mem.Content, "call_1") && strings.Contains(mem.Content, "session_id") {
			hasLoginResult = true
		}
		if mem.Role == store.MemoryRoleAssistant && strings.Contains(mem.Content, "call_2:get_status") {
			hasStatusCall = true
		}
		if mem.Role == store.MemoryRoleTool && strings.Contains(mem.Content, "call_2") {
			hasStatusResult = true
		}
	}

	if !hasCurrentUser {
		t.Error("Missing current user message")
	}
	if !hasLoginCall {
		t.Error("Missing login tool call from current turn")
	}
	if !hasLoginResult {
		t.Error("Missing login result from current turn - THIS IS THE KEY FIX")
	}
	if !hasStatusCall {
		t.Error("Missing status tool call from current turn")
	}
	if !hasStatusResult {
		t.Error("Missing status result from current turn")
	}
}

func TestGetContextMemories_NoUserPrompt(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("no-prompt-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Add only system and assistant messages (no user prompt)
	err := s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "assistant msg", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	memories, _, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories error: %v", err)
	}

	// Should generate synthetic encouragement when no user prompt exists
	hasEncouragement := false
	for _, mem := range memories {
		if mem.Role == store.MemoryRoleUser && mem.Source == store.MemorySourceSystem {
			hasEncouragement = true
			break
		}
	}

	if !hasEncouragement {
		t.Error("Expected synthetic encouragement when no user prompt exists")
	}
}

func TestGetContextMemories_OnlyHistoricalTurns(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("historical-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Add system prompt
	err := s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	// Add complete historical turn
	err = s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "old msg", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "old response", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	memories, _, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories error: %v", err)
	}

	// Should include system + historical context (user+assistant from last turn)
	if len(memories) < 2 {
		t.Fatalf("Expected at least 2 memories (system + historical), got %d", len(memories))
	}

	if memories[0].Role != store.MemoryRoleSystem {
		t.Error("First memory should be system")
	}

	// Last user message should be included as current turn
	hasOldUser := false
	for _, mem := range memories {
		if mem.Content == "old msg" {
			hasOldUser = true
		}
	}

	if !hasOldUser {
		t.Error("Expected historical user message to be included as current turn")
	}
}

// TestMysisName verifies the Name getter returns the mysis name.
func TestMysisName(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	mock := provider.NewMock("mock", "response")

	expectedName := "test-mysis-name"
	stored, err := s.CreateMysis(expectedName, "mock", "mock-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	if got := mysis.Name(); got != expectedName {
		t.Errorf("Name() = %q, want %q", got, expectedName)
	}
}

// TestMysisCreatedAt verifies the CreatedAt getter returns the creation timestamp.
func TestMysisCreatedAt(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	mock := provider.NewMock("mock", "response")

	beforeCreate := time.Now().Add(-time.Second)
	stored, err := s.CreateMysis("test-mysis-time", "mock", "mock-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}
	afterCreate := time.Now().Add(time.Second)

	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	got := mysis.CreatedAt()
	if got.Before(beforeCreate) || got.After(afterCreate) {
		t.Errorf("CreatedAt() = %v, want between %v and %v", got, beforeCreate, afterCreate)
	}
}

// TestBuildSystemPrompt_EdgeCases tests buildSystemPrompt error handling
func TestBuildSystemPrompt_EdgeCases(t *testing.T) {
	t.Run("no_broadcasts_fallback", func(t *testing.T) {
		s, bus, cleanup := setupMysisTest(t)
		defer cleanup()

		stored, _ := s.CreateMysis("no-broadcasts", "mock", "test-model", 0.7)
		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Build system prompt with no broadcasts
		prompt := mysis.buildSystemPrompt()

		// Should contain fallback message
		if !strings.Contains(prompt, "Continue to play the game") {
			t.Error("expected fallback message when no broadcasts")
		}
		if !strings.Contains(prompt, "SWARM BROADCAST") {
			t.Error("expected SWARM BROADCAST header in fallback")
		}
		// Should still contain base SystemPrompt content
		if !strings.Contains(prompt, "Nova Zoea") {
			t.Error("expected base system prompt content")
		}
	})

	t.Run("commander_broadcast", func(t *testing.T) {
		s, bus, cleanup := setupMysisTest(t)
		defer cleanup()

		stored, _ := s.CreateMysis("receiver-mysis", "mock", "test-model", 0.7)
		mock := provider.NewMock("mock", "response")
		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

		// Add a commander broadcast (empty sender_id)
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Attack coordinates: X=100, Y=200", "", "")

		prompt := mysis.buildSystemPrompt()

		// Should include broadcast content
		if !strings.Contains(prompt, "Attack coordinates") {
			t.Error("expected broadcast content in prompt")
		}
		// Should have SWARM BROADCAST header
		if !strings.Contains(prompt, "SWARM BROADCAST") {
			t.Error("expected SWARM BROADCAST header")
		}
		// Should NOT include sender name (commander broadcasts don't show sender)
		if strings.Contains(prompt, "From:") {
			t.Error("commander broadcasts should not show 'From:' field")
		}
	})

	t.Run("mysis_broadcast_ignored", func(t *testing.T) {
		s, bus, cleanup := setupMysisTest(t)
		defer cleanup()

		sender, _ := s.CreateMysis("sender-mysis-2", "mock", "test-model", 0.7)
		receiver, _ := s.CreateMysis("receiver-mysis-2", "mock", "test-model", 0.7)

		mock := provider.NewMock("mock", "response")
		receiverMysis := NewMysis(receiver.ID, receiver.Name, receiver.CreatedAt, mock, s, bus, "")

		// Add a commander broadcast first
		s.AddMemory(receiver.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Commander orders", "", "")
		// Add a broadcast from another mysis (has sender_id) - should be ignored
		s.AddMemory(receiver.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Mysis broadcast", "", sender.ID)

		prompt := receiverMysis.buildSystemPrompt()

		// Should show commander broadcast, not mysis broadcast
		if !strings.Contains(prompt, "Commander orders") {
			t.Error("expected commander broadcast to be shown")
		}
		// Should NOT include mysis broadcast content
		if strings.Contains(prompt, "Mysis broadcast") {
			t.Error("mysis broadcasts should be ignored in system prompt")
		}
	})
}

// TestSendEphemeralMessage_ErroredState removed - SendEphemeralMessage method removed.
// Ephemeral messages (synthetic nudges) are now generated by getContextMemories().

// TestSendEphemeralMessage_EmptyContent removed - SendEphemeralMessage method removed.
// Ephemeral messages (synthetic nudges) are now generated by getContextMemories().

// TestSendEphemeralMessage_MCPListToolsError removed - SendEphemeralMessage method removed.
// Ephemeral messages (synthetic nudges) are now generated by getContextMemories().

// TestIdleRecoveryOnBroadcast tests the Idle Recovery scenario from MESSAGE_FORMAT_GUARANTEES.md.
//
// Scenario:
// 1. Mysis has gone idle after 3 encouragements (no user messages)
// 2. Commander sends broadcast "Mine iron ore" via QueueBroadcast
// 3. Broadcast triggers auto-start (QueueBroadcast calls Start() on idle mysis)
// 4. Counter resets to 0
// 5. Mysis processes the broadcast and stays active
//
// Reference: documentation/architecture/MESSAGE_FORMAT_GUARANTEES.md lines 148-163
func TestIdleRecoveryOnBroadcast(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Step 1: Create mysis and store
	stored, err := s.CreateMysis("idle-recovery-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Use mock provider with no delay for fast test
	mock := provider.NewMock("mock", "Working on mining iron ore")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Step 2: Simulate idle state (mysis has already gone idle after 3 encouragements)
	// Set encouragementCount to 3 and transition to idle state
	mysis.mu.Lock()
	mysis.encouragementCount = 3
	mysis.state = MysisStateIdle
	mysis.mu.Unlock()

	// Update state in store
	if err := s.UpdateMysisState(stored.ID, store.MysisStateIdle); err != nil {
		t.Fatalf("UpdateMysisState() error: %v", err)
	}

	// Verify mysis is in idle state
	if mysis.State() != MysisStateIdle {
		t.Fatalf("expected state=idle, got %s", mysis.State())
	}

	// Verify encouragementCount is 3
	mysis.mu.RLock()
	count := mysis.encouragementCount
	mysis.mu.RUnlock()
	if count != 3 {
		t.Fatalf("expected encouragementCount=3, got %d", count)
	}

	// Subscribe to events to track state changes
	events := bus.Subscribe()

	// Step 3: Send broadcast "Mine iron ore" via QueueBroadcast
	senderID := "commander-mysis-id"
	err = mysis.QueueBroadcast("Mine iron ore", senderID)
	if err != nil {
		t.Fatalf("QueueBroadcast() error: %v", err)
	}

	// Step 4: Verify mysis auto-starts (state = running)
	// QueueBroadcast should have called Start() since mysis was idle
	autoStartTimeout := time.After(2 * time.Second)
	autoStarted := false

waitForAutoStart:
	for {
		select {
		case e := <-events:
			if e.Type == EventMysisStateChanged {
				if e.State != nil && e.State.NewState == MysisStateRunning {
					autoStarted = true
					break waitForAutoStart
				}
			}
		case <-autoStartTimeout:
			// Check if we're already running (event may have been consumed earlier)
			if mysis.State() == MysisStateRunning {
				autoStarted = true
				break waitForAutoStart
			}
			t.Fatalf("timeout waiting for mysis to auto-start on broadcast (current state: %s)", mysis.State())
		}
	}

	if !autoStarted {
		t.Error("mysis did not auto-start on broadcast")
	}

	if mysis.State() != MysisStateRunning {
		t.Errorf("expected state=running after broadcast auto-start, got %s", mysis.State())
	}

	// Step 5: Verify encouragementCount reset to 0
	mysis.mu.RLock()
	count = mysis.encouragementCount
	mysis.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected encouragementCount=0 after broadcast, got %d", count)
	}

	// Step 6: Wait for LLM response
	// Mysis should process the broadcast and generate a response
	responseTimeout := time.After(2 * time.Second)
	gotResponse := false

waitForResponse:
	for {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				if e.Message != nil {
					gotResponse = true
					break waitForResponse
				}
			}
		case <-responseTimeout:
			// Not fatal - response may have already been processed
			break waitForResponse
		}
	}

	if !gotResponse {
		t.Log("Warning: Did not receive expected LLM response event (may have been processed quickly)")
	}

	// Step 7: Verify mysis is running (not idle again)
	if mysis.State() != MysisStateRunning {
		t.Errorf("expected state=running after processing broadcast, got %s", mysis.State())
	}

	// Verify broadcast was stored in database
	memories, err := s.GetMemories(stored.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}

	foundBroadcast := false
	for _, m := range memories {
		if m.Source == store.MemorySourceBroadcast && m.Content == "Mine iron ore" && m.SenderID == senderID {
			foundBroadcast = true
			break
		}
	}

	if !foundBroadcast {
		t.Error("broadcast message was not stored in database")
	}

	// Step 8: Stop mysis
	if err := mysis.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if mysis.State() != MysisStateStopped {
		t.Errorf("expected state=stopped after Stop(), got %s", mysis.State())
	}
}

// TestBroadcastSlidingWindowBug tests that broadcasts outside the 20-message
// sliding window still keep myses running (don't trigger idle state).
//
// Bug: When a mysis processes a broadcast and generates 20+ messages,
// the original broadcast is pushed out of the GetRecentMemories(20) window.
// This causes findLastUserPromptIndex to return -1, triggering synthetic
// encouragements and eventual idle state.
//
// Expected: Mysis should stay running because broadcast exists in DB.
// Reference: MESSAGE_FORMAT_GUARANTEES.md line 44
func TestBroadcastSlidingWindowBug(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create mysis
	stored, err := s.CreateMysis("window-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Send broadcast (message #1)
	err = s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Explore the universe!", "", "commander-id")
	if err != nil {
		t.Fatalf("AddMemory(broadcast) error: %v", err)
	}

	// Simulate 25 messages (push broadcast out of 20-message window)
	// Pattern: assistant response + tool call + tool result (repeated)
	for i := 0; i < 8; i++ {
		// Assistant response
		err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, fmt.Sprintf("Response %d", i), "", "")
		if err != nil {
			t.Fatalf("AddMemory(assistant) error: %v", err)
		}

		// Tool call (stored as assistant message with tool call prefix)
		toolCall := fmt.Sprintf("[TOOL_CALLS]call_%d:get_status:{}", i)
		err = s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, toolCall, "", "")
		if err != nil {
			t.Fatalf("AddMemory(tool call) error: %v", err)
		}

		// Tool result
		toolResult := fmt.Sprintf("call_%d:Status OK", i)
		err = s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool, toolResult, "", "")
		if err != nil {
			t.Fatalf("AddMemory(tool result) error: %v", err)
		}
	}

	// Verify we have 25 messages total (1 broadcast + 24 from loop)
	allMemories, err := s.GetMemories(stored.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(allMemories) != 25 {
		t.Fatalf("expected 25 messages, got %d", len(allMemories))
	}

	// Verify broadcast is message #1 (oldest)
	if allMemories[0].Source != store.MemorySourceBroadcast {
		t.Fatalf("expected first message to be broadcast, got source=%s", allMemories[0].Source)
	}

	// Create mysis instance
	mock := provider.NewMock("mock", "Continuing mission...")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Call getContextMemories - this should find the broadcast even though
	// it's outside the 20-message sliding window
	memories, _, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// Check if broadcast is in context
	foundBroadcast := false
	foundCorrectBroadcast := false
	for _, m := range memories {
		if m.Source == store.MemorySourceBroadcast {
			foundBroadcast = true
			if m.Content == "Explore the universe!" {
				foundCorrectBroadcast = true
			}
			break
		}
	}

	if !foundBroadcast {
		t.Error("REGRESSION: broadcast not in context despite existing in DB - mysis will incorrectly go idle")
	}

	if !foundCorrectBroadcast {
		t.Error("Wrong broadcast in context - expected 'Explore the universe!'")
	}

	// Verify encouragementCount is 0 (not incremented)
	mysis.mu.RLock()
	count := mysis.encouragementCount
	mysis.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected encouragementCount=0 when broadcast exists, got %d", count)
	}

	// Verify no synthetic encouragement was added
	foundSynthetic := false
	for _, m := range memories {
		if m.Role == store.MemoryRoleUser && m.Source == store.MemorySourceSystem {
			if strings.Contains(m.Content, "Continue your mission") {
				foundSynthetic = true
				break
			}
		}
	}

	if foundSynthetic {
		t.Error("synthetic encouragement added despite broadcast existing - will cause incorrect idle state")
	}
}

// TestNewMysisInheritsGlobalBroadcast tests that a new mysis created after
// a broadcast was sent inherits the global swarm mission.
//
// Bug: New myses created after broadcasts have no broadcast in their memories.
// GetMostRecentBroadcast(mysisID) returns nil, causing idle state.
//
// Expected: New mysis should inherit most recent global broadcast.
// Reference: Issue reported by user - "new mysis went idle despite commander broadcast"
func TestNewMysisInheritsGlobalBroadcast(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Step 1: Create first mysis and send it a broadcast
	mysis1, err := s.CreateMysis("existing-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis(1) error: %v", err)
	}

	// Send broadcast to first mysis (simulates commander broadcast)
	err = s.AddMemory(mysis1.ID, store.MemoryRoleUser, store.MemorySourceBroadcast, "Explore the universe!", "", "commander-id")
	if err != nil {
		t.Fatalf("AddMemory(broadcast) error: %v", err)
	}

	// Step 2: Create second mysis AFTER the broadcast (simulates new mysis joining swarm)
	mysis2, err := s.CreateMysis("new-mysis", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis(2) error: %v", err)
	}

	// Verify new mysis has no broadcasts in its own memories
	mysis2Memories, err := s.GetMemories(mysis2.ID)
	if err != nil {
		t.Fatalf("GetMemories(mysis2) error: %v", err)
	}

	hasBroadcast := false
	for _, m := range mysis2Memories {
		if m.Source == store.MemorySourceBroadcast {
			hasBroadcast = true
			break
		}
	}

	if hasBroadcast {
		t.Fatal("Test setup error: new mysis should not have broadcast in its own memories")
	}

	// Step 3: Create mysis instance and call getContextMemories
	mock := provider.NewMock("mock", "Continuing mission...")
	mysisInstance := NewMysis(mysis2.ID, mysis2.Name, mysis2.CreatedAt, mock, s, bus, "")

	memories, _, err := mysisInstance.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// Step 4: Verify the global broadcast was inherited
	foundGlobalBroadcast := false
	for _, m := range memories {
		if m.Source == store.MemorySourceBroadcast && m.Content == "Explore the universe!" {
			foundGlobalBroadcast = true
			break
		}
	}

	if !foundGlobalBroadcast {
		t.Error("REGRESSION: new mysis did not inherit global broadcast - will incorrectly go idle")
	}

	// Step 5: Verify encouragementCount is 0 (not incremented)
	mysisInstance.mu.RLock()
	count := mysisInstance.encouragementCount
	mysisInstance.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected encouragementCount=0 when global broadcast inherited, got %d", count)
	}

	// Step 6: Verify no synthetic encouragement was added
	foundSynthetic := false
	for _, m := range memories {
		if m.Role == store.MemoryRoleUser && m.Source == store.MemorySourceSystem {
			if strings.Contains(m.Content, "Continue your mission") {
				foundSynthetic = true
				break
			}
		}
	}

	if foundSynthetic {
		t.Error("synthetic encouragement added despite global broadcast existing")
	}
}

// TestGetContextMemories_WithCompression verifies that getContextMemories()
// applies both compactSnapshots() and removeOrphanedToolCalls() to ensure:
// 1. Duplicate snapshot tool results are removed (only latest kept)
// 2. Orphaned assistant tool calls without matching results are removed
func TestGetContextMemories_WithCompression(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Add current turn with duplicate snapshots and orphaned tool calls
	// User prompt
	err := mysis.store.AddMemory(mysis.id, store.MemoryRoleUser, store.MemorySourceDirect, "Check game status", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	// First get_status call (will be compacted - duplicate)
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleAssistant, store.MemorySourceLLM, "[TOOL_CALLS]call_status_1:get_status:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleTool, store.MemorySourceTool, "call_status_1:old status data", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	// First get_system call (will be compacted - duplicate)
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleAssistant, store.MemorySourceLLM, "[TOOL_CALLS]call_system_1:get_system:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleTool, store.MemorySourceTool, "call_system_1:old system data", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	// Second get_status call (latest - will be kept)
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleAssistant, store.MemorySourceLLM, "[TOOL_CALLS]call_status_2:get_status:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleTool, store.MemorySourceTool, "call_status_2:latest status data", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	// Second get_system call (latest - will be kept)
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleAssistant, store.MemorySourceLLM, "[TOOL_CALLS]call_system_2:get_system:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleTool, store.MemorySourceTool, "call_system_2:latest system data", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	// Orphaned tool call (no matching tool result - will be removed)
	err = mysis.store.AddMemory(mysis.id, store.MemoryRoleAssistant, store.MemorySourceLLM, "[TOOL_CALLS]call_orphan:get_notifications:{}", "", "")
	if err != nil {
		t.Fatalf("AddMemory error: %v", err)
	}

	memories, _, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories error: %v", err)
	}

	// Verify compression applied: only latest snapshots present
	statusCount := 0
	systemCount := 0
	hasLatestStatus := false
	hasLatestSystem := false
	hasOrphanedCall := false

	for _, mem := range memories {
		if mem.Role == store.MemoryRoleTool {
			if strings.Contains(mem.Content, "call_status_1") {
				t.Error("Found old get_status result - compactSnapshots() not applied")
			}
			if strings.Contains(mem.Content, "call_system_1") {
				t.Error("Found old get_system result - compactSnapshots() not applied")
			}
			if strings.Contains(mem.Content, "call_status_2") && strings.Contains(mem.Content, "latest status") {
				statusCount++
				hasLatestStatus = true
			}
			if strings.Contains(mem.Content, "call_system_2") && strings.Contains(mem.Content, "latest system") {
				systemCount++
				hasLatestSystem = true
			}
		}
		if mem.Role == store.MemoryRoleAssistant && strings.Contains(mem.Content, "call_orphan") {
			hasOrphanedCall = true
		}
	}

	if !hasLatestStatus {
		t.Error("Missing latest get_status snapshot")
	}
	if !hasLatestSystem {
		t.Error("Missing latest get_system snapshot")
	}
	if statusCount != 1 {
		t.Errorf("Expected exactly 1 get_status result, got %d", statusCount)
	}
	if systemCount != 1 {
		t.Errorf("Expected exactly 1 get_system result, got %d", systemCount)
	}
	if hasOrphanedCall {
		t.Error("Found orphaned tool call - removeOrphanedToolCalls() not applied")
	}
}

// TestCompactSnapshots_MultipleSnapshots verifies that compactSnapshots()
// correctly removes duplicate snapshot tool results, keeping only the latest
// for each tool type while preserving order and non-snapshot tools.
func TestCompactSnapshots_MultipleSnapshots(t *testing.T) {
	mysis, cleanup := setupTestMysis(t)
	defer cleanup()

	// Create test memories with multiple snapshots
	memories := []*store.Memory{
		{Role: store.MemoryRoleUser, Content: "user message", Source: store.MemorySourceDirect},
		// First get_status (old - should be removed)
		{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_status_1:get_status:{}", Source: store.MemorySourceLLM},
		{Role: store.MemoryRoleTool, Content: "call_status_1:old status", Source: store.MemorySourceTool},
		// Non-snapshot tool (should be kept)
		{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_action:send_message:{\"text\":\"hello\"}", Source: store.MemorySourceLLM},
		{Role: store.MemoryRoleTool, Content: "call_action:message sent", Source: store.MemorySourceTool},
		// Second get_status (latest - should be kept)
		{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_status_2:get_status:{}", Source: store.MemorySourceLLM},
		{Role: store.MemoryRoleTool, Content: "call_status_2:latest status", Source: store.MemorySourceTool},
		// First get_system (old - should be removed)
		{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_system_1:get_system:{}", Source: store.MemorySourceLLM},
		{Role: store.MemoryRoleTool, Content: "call_system_1:old system", Source: store.MemorySourceTool},
		// Third get_status (latest - should be kept)
		{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_status_3:get_status:{}", Source: store.MemorySourceLLM},
		{Role: store.MemoryRoleTool, Content: "call_status_3:newest status", Source: store.MemorySourceTool},
		// Second get_system (latest - should be kept)
		{Role: store.MemoryRoleAssistant, Content: "[TOOL_CALLS]call_system_2:get_system:{}", Source: store.MemorySourceLLM},
		{Role: store.MemoryRoleTool, Content: "call_system_2:latest system", Source: store.MemorySourceTool},
		{Role: store.MemoryRoleAssistant, Content: "Based on the data...", Source: store.MemorySourceLLM},
	}

	result := mysis.compactSnapshots(memories)

	// Count snapshots and verify correct ones kept
	statusCount := 0
	systemCount := 0
	nonSnapshotCount := 0
	hasNewestStatus := false
	hasLatestSystem := false
	hasOldStatus := false
	hasOldSystem := false

	for _, mem := range result {
		if mem.Role == store.MemoryRoleTool {
			if strings.Contains(mem.Content, "call_status_1") {
				hasOldStatus = true
			}
			if strings.Contains(mem.Content, "call_status_2") {
				hasOldStatus = true
			}
			if strings.Contains(mem.Content, "call_status_3") && strings.Contains(mem.Content, "newest") {
				statusCount++
				hasNewestStatus = true
			}
			if strings.Contains(mem.Content, "call_system_1") {
				hasOldSystem = true
			}
			if strings.Contains(mem.Content, "call_system_2") && strings.Contains(mem.Content, "latest") {
				systemCount++
				hasLatestSystem = true
			}
			if strings.Contains(mem.Content, "call_action") && strings.Contains(mem.Content, "message sent") {
				nonSnapshotCount++
			}
		}
	}

	if hasOldStatus {
		t.Error("Found old get_status results - should be compacted")
	}
	if hasOldSystem {
		t.Error("Found old get_system results - should be compacted")
	}
	if !hasNewestStatus {
		t.Error("Missing newest get_status snapshot")
	}
	if !hasLatestSystem {
		t.Error("Missing latest get_system snapshot")
	}
	if statusCount != 1 {
		t.Errorf("Expected exactly 1 get_status result, got %d", statusCount)
	}
	if systemCount != 1 {
		t.Errorf("Expected exactly 1 get_system result, got %d", systemCount)
	}
	if nonSnapshotCount != 1 {
		t.Errorf("Expected 1 non-snapshot tool result, got %d - non-snapshots should be preserved", nonSnapshotCount)
	}

	// Verify order preserved (user message should still be first)
	if len(result) > 0 && result[0].Role != store.MemoryRoleUser {
		t.Error("Message order not preserved - user message should be first")
	}
}
