package core

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

func setupAgentTest(t *testing.T) (*store.Store, *EventBus, func()) {
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

func TestAgentLifecycle(t *testing.T) {
	s, bus, cleanup := setupAgentTest(t)
	defer cleanup()

	// Create stored agent
	stored, err := s.CreateAgent("test-agent", "mock", "test-model")
	if err != nil {
		t.Fatalf("CreateAgent() error: %v", err)
	}

	mock := provider.NewMock("mock", "Hello from agent!")
	agent := NewAgent(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Initial state
	if agent.State() != AgentStateIdle {
		t.Errorf("expected initial state=idle, got %s", agent.State())
	}
	if agent.ID() != stored.ID {
		t.Errorf("expected ID=%s, got %s", stored.ID, agent.ID())
	}
	if agent.Name() != "test-agent" {
		t.Errorf("expected name=test-agent, got %s", agent.Name())
	}

	// Start
	if err := agent.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if agent.State() != AgentStateRunning {
		t.Errorf("expected state=running, got %s", agent.State())
	}

	// Start again should error
	if err := agent.Start(); err == nil {
		t.Error("expected error starting already running agent")
	}

	// Stop
	if err := agent.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if agent.State() != AgentStateStopped {
		t.Errorf("expected state=stopped, got %s", agent.State())
	}

	// Stop again should error
	if err := agent.Stop(); err == nil {
		t.Error("expected error stopping already stopped agent")
	}
}

func TestAgentSendMessage(t *testing.T) {
	s, bus, cleanup := setupAgentTest(t)
	defer cleanup()

	stored, _ := s.CreateAgent("msg-agent", "mock", "test-model")
	mock := provider.NewMock("mock", "I received your message!")
	agent := NewAgent(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Can't send to non-running agent
	if err := agent.SendMessage("Hello", store.MemorySourceDirect); err == nil {
		t.Error("expected error sending to non-running agent")
	}

	// Start agent
	agent.Start()

	// Subscribe to events
	events := bus.Subscribe()

	// Wait for the initial autonomous turn to finish
	// It should emit a response event for the ContinuePrompt
	timeout := time.After(2 * time.Second)
initialTurnLoop:
	for {
		select {
		case e := <-events:
			if e.Type == EventAgentResponse {
				break initialTurnLoop
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial autonomous turn")
		}
	}

	// Send message
	if err := agent.SendMessage("Hello, agent!", store.MemorySourceDirect); err != nil {
		t.Fatalf("SendMessage() error: %v", err)
	}

	// Should receive message and response events for our message
	var messageEvent, responseEvent bool
	timeout = time.After(2 * time.Second)

eventLoop:
	for {
		select {
		case e := <-events:
			if e.Type == EventAgentMessage {
				data, ok := e.Data.(MessageData)
				if ok && data.Content == "Hello, agent!" {
					messageEvent = true
				}
			}
			if e.Type == EventAgentResponse {
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
		t.Error("expected message event for 'Hello, agent!'")
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

	agent.Stop()
}

func TestAgentSetErrorState(t *testing.T) {
	s, bus, cleanup := setupAgentTest(t)
	defer cleanup()

	stored, _ := s.CreateAgent("error-state-test", "mock", "test-model")
	mock := provider.NewMock("mock", "response")
	agent := NewAgent(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	events := bus.Subscribe()

	testErr := errors.New("test error")
	agent.setErrorState(testErr)

	if agent.State() != AgentStateErrored {
		t.Errorf("expected state=errored, got %s", agent.State())
	}
	if agent.LastError() != testErr {
		t.Errorf("expected last error %v, got %v", testErr, agent.LastError())
	}

	// Should receive state change event first
	select {
	case e := <-events:
		if e.Type != EventAgentStateChanged {
			t.Errorf("expected EventAgentStateChanged, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for state change event")
	}

	// Should receive error event
	select {
	case e := <-events:
		if e.Type != EventAgentError {
			t.Errorf("expected EventAgentError, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for error event")
	}
}

func TestAgentProviderName(t *testing.T) {
	s, bus, cleanup := setupAgentTest(t)
	defer cleanup()

	stored, _ := s.CreateAgent("provider-test", "mock", "test-model")
	mock := provider.NewMock("test-provider", "response")
	agent := NewAgent(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	if agent.ProviderName() != "test-provider" {
		t.Errorf("expected provider name=test-provider, got %s", agent.ProviderName())
	}

	// Test with nil provider
	agent.SetProvider(nil)
	if agent.ProviderName() != "" {
		t.Errorf("expected empty provider name for nil provider, got %s", agent.ProviderName())
	}
}

func TestAgentStateEvents(t *testing.T) {
	s, bus, cleanup := setupAgentTest(t)
	defer cleanup()

	stored, _ := s.CreateAgent("event-test", "mock", "test-model")
	mock := provider.NewMock("mock", "response")
	agent := NewAgent(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	events := bus.Subscribe()

	agent.Start()

	// Should get state change event
	select {
	case e := <-events:
		if e.Type != EventAgentStateChanged {
			t.Errorf("expected state change event, got %s", e.Type)
		}
		data := e.Data.(StateChangeData)
		if data.OldState != AgentStateIdle {
			t.Errorf("expected old state=idle, got %s", data.OldState)
		}
		if data.NewState != AgentStateRunning {
			t.Errorf("expected new state=running, got %s", data.NewState)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for state change event")
	}

	agent.Stop()
}

func TestAgentContextMemoryLimit(t *testing.T) {
	s, bus, cleanup := setupAgentTest(t)
	defer cleanup()

	stored, _ := s.CreateAgent("context-test", "mock", "test-model")
	mock := provider.NewMock("mock", "response")
	agent := NewAgent(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Add system prompt
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt")

	// Add more memories than MaxContextMessages
	for i := 0; i < MaxContextMessages+10; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "user message")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "assistant response")
	}

	// Get context memories
	memories, err := agent.getContextMemories()
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

func TestAgentContextMemoryWithRecentSystemPrompt(t *testing.T) {
	s, bus, cleanup := setupAgentTest(t)
	defer cleanup()

	stored, _ := s.CreateAgent("context-test-2", "mock", "test-model")
	mock := provider.NewMock("mock", "response")
	agent := NewAgent(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Add system prompt
	s.AddMemory(stored.ID, store.MemoryRoleSystem, store.MemorySourceSystem, "System prompt")

	// Add fewer memories than MaxContextMessages
	for i := 0; i < 5; i++ {
		s.AddMemory(stored.ID, store.MemoryRoleUser, store.MemorySourceDirect, "user message")
		s.AddMemory(stored.ID, store.MemoryRoleAssistant, store.MemorySourceLLM, "assistant response")
	}

	// Get context memories
	memories, err := agent.getContextMemories()
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
