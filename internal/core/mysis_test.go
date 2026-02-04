package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "")

	// Add more memories than MaxContextMessages
	for i := 0; i < MaxContextMessages+10; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "user message", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "assistant response", "")
	}

	// Get context memories
	memories, err := mysis.getContextMemories()
	if err != nil {
		t.Fatalf("getContextMemories() error: %v", err)
	}

	// Should have system prompt + MaxContextMessages recent messages
	expectedCount := MaxContextMessages + 1 // +1 for system prompt
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
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt", "")

	// Add fewer memories than MaxContextMessages
	for i := 0; i < 5; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "user message", "")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "assistant response", "")
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
	if !strings.Contains(SystemPrompt, "captains_log_add({\"entry\":") {
		t.Fatal("SystemPrompt missing captains_log_add example")
	}
	if !strings.Contains(SystemPrompt, "non-empty entry field") {
		t.Fatal("SystemPrompt missing non-empty entry reminder")
	}
}

func TestContinuePromptContainsCriticalReminders(t *testing.T) {
	if !strings.Contains(ContinuePrompt, "zoea_list_myses") {
		t.Fatal("ContinuePrompt missing mysis ID reminder")
	}
	if !strings.Contains(ContinuePrompt, "captains_log_add") {
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
