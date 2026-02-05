package core

import (
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
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

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

	// Stop again should error
	if err := mysis.Stop(); err == nil {
		t.Error("expected error stopping already stopped mysis")
	}
}

func TestMysisSendMessage(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("msg-mysis", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "I received your message!")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Can't send to non-running mysis
	if err := mysis.SendMessage("Hello", store.MemorySourceDirect); err == nil {
		t.Error("expected error sending to non-running mysis")
	}

	// Start mysis
	mysis.Start()

	// Subscribe to events
	events := bus.Subscribe()

	// Wait for the initial autonomous turn to finish
	// It should emit a response event for the ContinuePrompt
	timeout := time.After(2 * time.Second)
initialTurnLoop:
	for {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				break initialTurnLoop
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial autonomous turn")
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
				data, ok := e.Data.(MessageData)
				if ok && data.Content == "Hello, mysis!" {
					messageEvent = true
				}
			}
			if e.Type == EventMysisResponse {
				data := e.Data.(MessageData)
				if data.Content == "I received your message!" {
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
	// Expected: system prompt, continue prompt (initial trigger), assistant response (initial),
	//           user message, assistant response
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
	receiver := NewMysis(receiverStored.ID, receiverStored.Name, receiverStored.CreatedAt, mock, s, bus)

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
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

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
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

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
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	events := bus.Subscribe()

	mysis.Start()

	// Should get state change event
	select {
	case e := <-events:
		if e.Type != EventMysisStateChanged {
			t.Errorf("expected state change event, got %s", e.Type)
		}
		data := e.Data.(StateChangeData)
		if data.OldState != MysisStateIdle {
			t.Errorf("expected old state=idle, got %s", data.OldState)
		}
		if data.NewState != MysisStateRunning {
			t.Errorf("expected new state=running, got %s", data.NewState)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for state change event")
	}

	mysis.Stop()
}

func TestMysisContextMemoryLimit(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("context-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Add system prompt
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")

	// Add more memories than MaxContextMessages
	for i := 0; i < constants.MaxContextMessages+10; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "user message", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "assistant response", "", "")
	}

	// Get context memories
	memories, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// Should have system prompt + MaxContextMessages recent messages
	expectedCount := constants.MaxContextMessages + 1 // +1 for system prompt
	if len(memories) != expectedCount {
		t.Errorf("expected %d memories, got %d", expectedCount, len(memories))
	}

	// First memory should be system prompt
	if memories[0].Role != store.MemoryRoleSystem {
		t.Errorf("expected first memory to be system prompt, got %s", memories[0].Role)
	}
	if memories[0].Content != "System prompt" {
		t.Errorf("expected system prompt content, got %s", memories[0].Content)
	}
}

func TestMysisContextMemoryWithRecentSystemPrompt(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("context-test-2", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Add system prompt
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")

	// Add fewer memories than MaxContextMessages
	for i := 0; i < 5; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "user message", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "assistant response", "", "")
	}

	// Get context memories
	memories, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// Should have all memories (system + 10 messages)
	expectedCount := 11 // 1 system + 5*2 messages
	if len(memories) != expectedCount {
		t.Errorf("expected %d memories, got %d", expectedCount, len(memories))
	}

	// First memory should still be system prompt
	if memories[0].Role != store.MemoryRoleSystem {
		t.Errorf("expected first memory to be system prompt, got %s", memories[0].Role)
	}
}

func TestSystemPromptContainsCaptainsLogExamples(t *testing.T) {
	if !strings.Contains(constants.SystemPrompt, "captains_log_add({\"entry\":") {
		t.Fatal("SystemPrompt missing captains_log_add example")
	}
	if !strings.Contains(constants.SystemPrompt, "non-empty entry field") {
		t.Fatal("SystemPrompt missing non-empty entry reminder")
	}
}

func TestContinuePromptContainsCriticalReminders(t *testing.T) {
	if !strings.Contains(constants.ContinuePrompt, "captains_log_add") {
		t.Fatal("ContinuePrompt missing captains_log_add reminder")
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

func TestMysisContextCompaction(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("compaction-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Add system prompt
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")

	// Add multiple get_ship tool results (should be compacted to keep only the latest)
	for i := 0; i < 5; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "check ship", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, constants.ToolCallStoragePrefix+"call_1:get_ship:{}", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			fmt.Sprintf(`call_1:{"ship_id":"ship_%d","hull":100}`, i), "", "")
	}

	// Add multiple get_system tool results (should also be compacted)
	for i := 0; i < 3; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "check system", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, constants.ToolCallStoragePrefix+"call_2:get_system:{}", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			fmt.Sprintf(`call_2:{"system_id":"sys_%d","police_level":1}`, i), "", "")
	}

	// Add a non-snapshot tool result (should be kept)
	s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "mine ore", "", "")
	s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, constants.ToolCallStoragePrefix+"call_3:mine:{}", "", "")
	s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool, `call_3:{"result":"mining"}`, "", "")

	// Get context memories
	memories, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// First memory should be system prompt
	if memories[0].Role != store.MemoryRoleSystem {
		t.Errorf("expected first memory to be system prompt, got %s", memories[0].Role)
	}

	// Count get_ship tool results - should only have 1 (the latest)
	shipResults := 0
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, `"ship_id"`) {
			shipResults++
		}
	}
	if shipResults != 1 {
		t.Errorf("expected 1 get_ship result after compaction, got %d", shipResults)
	}

	// Count get_system tool results - should only have 1 (the latest)
	systemResults := 0
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, `"system_id"`) {
			systemResults++
		}
	}
	if systemResults != 1 {
		t.Errorf("expected 1 get_system result after compaction, got %d", systemResults)
	}

	// Non-snapshot tool result should be kept
	mineResults := 0
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, `"result":"mining"`) {
			mineResults++
		}
	}
	if mineResults != 1 {
		t.Errorf("expected 1 mine result (non-snapshot), got %d", mineResults)
	}

	// Verify the latest get_ship result is kept (ship_4, not ship_0)
	foundLatestShip := false
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, `"ship_id":"ship_4"`) {
			foundLatestShip = true
		}
	}
	if !foundLatestShip {
		t.Error("expected latest get_ship result (ship_4) to be kept")
	}
}

func TestSystemPromptContainsSearchGuidance(t *testing.T) {
	if !strings.Contains(constants.SystemPrompt, "zoea_search_messages") {
		t.Fatal("SystemPrompt missing zoea_search_messages reference")
	}
	if !strings.Contains(constants.SystemPrompt, "zoea_search_reasoning") {
		t.Fatal("SystemPrompt missing zoea_search_reasoning reference")
	}
	if !strings.Contains(constants.SystemPrompt, "Context & Memory Management") {
		t.Fatal("SystemPrompt missing Context & Memory Management section")
	}
}

func TestContinuePromptContainsSearchReminder(t *testing.T) {
	if !strings.Contains(constants.ContinuePrompt, "zoea_search_messages") {
		t.Fatal("ContinuePrompt missing zoea_search_messages reminder")
	}
	if !strings.Contains(constants.ContinuePrompt, "zoea_search_reasoning") {
		t.Fatal("ContinuePrompt missing zoea_search_reasoning reminder")
	}
}

func TestZoeaListMysesCompaction(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("compaction-test", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Add system prompt
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")

	// Add multiple zoea_list_myses tool results (should be compacted to keep only the latest)
	for i := 0; i < 5; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "list myses", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, constants.ToolCallStoragePrefix+"call_1:zoea_list_myses:{}", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool,
			fmt.Sprintf(`call_1:[{"id":"mysis-%d","name":"test-%d"}]`, i, i), "", "")
	}

	// Get context memories
	memories, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// Count zoea_list_myses tool results - should only have 1 (the latest)
	listResults := 0
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, `"id":"mysis-`) {
			listResults++
		}
	}
	if listResults != 1 {
		t.Errorf("expected 1 zoea_list_myses result after compaction, got %d", listResults)
	}

	// Verify the latest result is kept (mysis-4, not mysis-0)
	foundLatest := false
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, `"id":"mysis-4"`) {
			foundLatest = true
		}
	}
	if !foundLatest {
		t.Error("expected latest zoea_list_myses result (mysis-4) to be kept")
	}
}

func TestMysisContextCompactionNonSnapshot(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("compaction-non-snapshot", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "", "")

	for i := 0; i < 2; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "travel", "", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, fmt.Sprintf("[TOOL_CALLS]call_%d:travel:{}", i), "", "")
		s.AddMemory(stored.ID, store.MemoryRoleTool, store.MemorySourceTool, fmt.Sprintf(`call_%d:{"ship_id":"ship_%d"}`, i, i), "", "")
	}

	memories, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	shipResults := 0
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, `"ship_id"`) {
			shipResults++
		}
	}

	if shipResults != 2 {
		t.Fatalf("expected 2 travel results to be kept, got %d", shipResults)
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
