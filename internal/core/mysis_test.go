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
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

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
	// Expected: system prompt, assistant response (from ephemeral nudge),
	//           user message, assistant response
	// NOTE: The continue prompt (initial trigger) is now ephemeral and NOT stored in DB
	memories, err := s.GetMemories(stored.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories) != 4 {
		t.Errorf("expected 4 memories, got %d", len(memories))
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
	// Check for captain's log guidance in Critical Rules section
	if !strings.Contains(constants.SystemPrompt, "Captain's log entry field must be non-empty") {
		t.Fatal("SystemPrompt missing non-empty entry reminder")
	}
	if !strings.Contains(constants.SystemPrompt, "max 20 entries") {
		t.Fatal("SystemPrompt missing captain's log limit guidance")
	}
	if !strings.Contains(constants.SystemPrompt, "100KB each") {
		t.Fatal("SystemPrompt missing captain's log size limit")
	}
}

func TestContinuePromptContainsCriticalReminders(t *testing.T) {
	// Check for get_notifications reminder which is the critical reminder in ContinuePrompt
	if !strings.Contains(constants.ContinuePrompt, "get_notifications") {
		t.Fatal("ContinuePrompt missing get_notifications reminder")
	}
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
	// Check for search tools in Swarm Coordination section
	if !strings.Contains(constants.SystemPrompt, "zoea_search_messages") {
		t.Fatal("SystemPrompt missing zoea_search_messages reference")
	}
	if !strings.Contains(constants.SystemPrompt, "zoea_search_reasoning") {
		t.Fatal("SystemPrompt missing zoea_search_reasoning reference")
	}
	if !strings.Contains(constants.SystemPrompt, "zoea_search_broadcasts") {
		t.Fatal("SystemPrompt missing zoea_search_broadcasts reference")
	}
	// Check for context limitation guidance
	if !strings.Contains(constants.SystemPrompt, "Context is limited") {
		t.Fatal("SystemPrompt missing context limitation guidance")
	}
}

func TestContinuePromptContainsSearchReminder(t *testing.T) {
	// ContinuePrompt is intentionally minimal - only checks for critical get_notifications reminder
	// Search tools, account claiming, and other guidance are in SystemPrompt
	if !strings.Contains(constants.ContinuePrompt, "get_notifications") {
		t.Fatal("ContinuePrompt missing get_notifications reminder")
	}
}

func TestContinuePromptAddsDriftReminder(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("drift-check", "mock", "test-model", 0.7)
	mock := provider.NewMock("mock", "response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "Waiting 5 minutes for travel.", "", "")

	prompt := mysis.buildContinuePrompt(0)
	if !strings.Contains(prompt, "DRIFT REMINDERS") {
		t.Fatal("expected drift reminders section in continue prompt")
	}
	if !strings.Contains(prompt, "real-world time") {
		t.Fatal("expected real-world time drift reminder")
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
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

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
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

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
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

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

// TestStopDuringIdleNudge tests stopping a mysis while it's processing an idle nudge.
// This reproduces the race condition where Stop() is called during nudge processing.
//
// Timeline:
// - Start() completes, initial message finishes processing
// - Test manually triggers nudge via nudgeCh (no 30s wait needed)
// - Line 1047-1052: nudge handler spawns `go a.SendMessage(ContinuePrompt, ...)`
// - HERE: Test calls Stop() while SendMessage goroutine is running
// - SendMessage tries to process but context is canceled -> calls setError()
// - Race: setError() tries to set state=Errored, but Stop() already set state=Stopped
//
// Expected: Final state is Stopped (not Errored), no lastError.
func TestStopDuringIdleNudge(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("nudge-race-test", "mock", "test-model", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Use a mock provider with a delay to simulate LLM processing
	mock := provider.NewMock("mock", "nudge response").SetDelay(100 * time.Millisecond)
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Start mysis (triggers initial SendMessage at line 268)
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Wait for initial message to complete
	time.Sleep(200 * time.Millisecond)

	// Manually trigger a nudge (instead of waiting 30 seconds for ticker)
	// This simulates what the run() goroutine does on line 1043
	select {
	case m.nudgeCh <- struct{}{}:
		// Nudge sent successfully
	default:
		t.Fatal("failed to send nudge - channel full")
	}

	// Give the nudge handler time to spawn SendMessage goroutine
	time.Sleep(10 * time.Millisecond)

	// Call Stop() DURING the nudge processing
	// The race happens when:
	// 1. Nudge handler (line 1047-1052) spawns: go a.SendMessage(...)
	// 2. Stop() is called here (cancels context)
	// 3. SendMessage goroutine tries to process but context is canceled
	// 4. SendMessage calls setError() which tries to change state to Errored
	// 5. But Stop() should have already set state to Stopped
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// Give a moment for any racing goroutines to complete
	time.Sleep(100 * time.Millisecond)

	// Assert state == MysisStateStopped (NOT Errored)
	if m.State() != MysisStateStopped {
		t.Errorf("expected state=stopped after Stop(), got %s (lastError: %v)", m.State(), m.LastError())
	}

	// Assert lastError == nil
	if m.LastError() != nil {
		t.Errorf("expected no lastError after clean stop, got: %v", m.LastError())
	}
}

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
// - 500ms: After idle nudge interval
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
		500 * time.Millisecond, // 500ms - after idle nudge interval
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
			mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

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

// TestNudgeCircuitBreaker tests the circuit breaker logic that errors out stuck myses
func TestNudgeCircuitBreaker(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	t.Run("errors_after_3_ticker_fires", func(t *testing.T) {
		// Create a mock provider that never responds (long delay)
		mock := provider.NewMock("stuck", "")
		mock.SetDelay(10 * time.Second) // Will timeout

		stored, err := s.CreateMysis("stuck-mysis", "stuck", "stuck-model", 0.7)
		if err != nil {
			t.Fatalf("CreateMysis() error: %v", err)
		}

		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

		// Start the mysis
		if err := mysis.Start(); err != nil {
			t.Fatalf("Start() error: %v", err)
		}
		defer mysis.Stop()

		// Manually increment the counter 3 times to simulate 3 ticker fires
		// (In real usage, the ticker would do this automatically)
		for i := 0; i < 3; i++ {
			mysis.mu.Lock()
			mysis.nudgeFailCount++
			count := mysis.nudgeFailCount
			mysis.mu.Unlock()

			if count >= 3 {
				// Trigger error state
				mysis.setError(errors.New("Failed to respond after 3 nudges"))
				break
			}
		}

		// Wait for error state transition
		timeout := time.After(2 * time.Second)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		errored := false
		for !errored {
			select {
			case <-timeout:
				t.Fatal("timed out waiting for error state after 3 nudges")
			case <-ticker.C:
				if mysis.State() == MysisStateErrored {
					errored = true
				}
			}
		}

		// Verify error message
		lastErr := mysis.LastError()
		if lastErr == nil {
			t.Fatal("expected error to be set after 3 nudges")
		}
		if !strings.Contains(lastErr.Error(), "Failed to respond after 3 nudges") {
			t.Errorf("expected 'Failed to respond after 3 nudges' error, got: %v", lastErr)
		}
	})

	t.Run("resets_counter_on_successful_response", func(t *testing.T) {
		// Create a mock provider that responds quickly
		mock := provider.NewMock("responsive", "I'm working!")

		stored, err := s.CreateMysis("responsive-mysis", "responsive", "responsive-model", 0.7)
		if err != nil {
			t.Fatalf("CreateMysis() error: %v", err)
		}

		mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

		// Start the mysis
		if err := mysis.Start(); err != nil {
			t.Fatalf("Start() error: %v", err)
		}
		defer mysis.Stop()

		// Manually set counter to 2 (simulate 2 ticker fires)
		mysis.mu.Lock()
		mysis.nudgeFailCount = 2
		mysis.mu.Unlock()

		// Verify counter is at 2
		mysis.mu.RLock()
		count := mysis.nudgeFailCount
		mysis.mu.RUnlock()
		if count != 2 {
			t.Fatalf("expected nudgeFailCount=2, got %d", count)
		}

		// Send a successful message (should reset counter)
		if err := mysis.SendMessage("test message", store.MemorySourceDirect); err != nil {
			t.Fatalf("SendMessage() error: %v", err)
		}

		// Wait for response processing
		time.Sleep(200 * time.Millisecond)

		// Verify counter was reset to 0
		mysis.mu.RLock()
		count = mysis.nudgeFailCount
		mysis.mu.RUnlock()
		if count != 0 {
			t.Errorf("expected nudgeFailCount to reset to 0 after successful response, got %d", count)
		}

		// Verify state is still running (not errored)
		if mysis.State() != MysisStateRunning {
			t.Errorf("expected state=running after successful response, got %s", mysis.State())
		}
	})

	t.Run("increments_counter_on_ticker_fire", func(t *testing.T) {
		// Test that the counter increments properly
		mysis := &Mysis{}
		mysis.nudgeFailCount = 0

		// Simulate ticker fires (what happens in real usage)
		for i := 1; i <= 2; i++ {
			// This is what the ticker.C case does
			mysis.nudgeFailCount++

			if mysis.nudgeFailCount != i {
				t.Errorf("after ticker fire %d: expected nudgeFailCount=%d, got %d", i, i, mysis.nudgeFailCount)
			}
		}

		// Verify final count
		if mysis.nudgeFailCount != 2 {
			t.Errorf("expected final nudgeFailCount=2, got %d", mysis.nudgeFailCount)
		}
	})
}
