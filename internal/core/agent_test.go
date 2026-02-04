package core

import (
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

func setupAgentTest(t *testing.T) (*store.Store, *EventBus, func()) {
	t.Helper()

	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
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
	agent := NewAgent(stored.ID, stored.Name, mock, s, bus)

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
	agent := NewAgent(stored.ID, stored.Name, mock, s, bus)

	// Can't send to non-running agent
	if err := agent.SendMessage("Hello"); err == nil {
		t.Error("expected error sending to non-running agent")
	}

	// Start agent
	agent.Start()

	// Subscribe to events
	events := bus.Subscribe()

	// Send message
	if err := agent.SendMessage("Hello, agent!"); err != nil {
		t.Fatalf("SendMessage() error: %v", err)
	}

	// Should receive message and response events
	var messageEvent, responseEvent bool
	timeout := time.After(500 * time.Millisecond)

	for i := 0; i < 3; i++ { // State change + message + response
		select {
		case e := <-events:
			if e.Type == EventAgentMessage {
				messageEvent = true
				data := e.Data.(MessageData)
				if data.Content != "Hello, agent!" {
					t.Errorf("expected message content 'Hello, agent!', got %s", data.Content)
				}
			}
			if e.Type == EventAgentResponse {
				responseEvent = true
				data := e.Data.(MessageData)
				if data.Content != "I received your message!" {
					t.Errorf("expected response 'I received your message!', got %s", data.Content)
				}
			}
		case <-timeout:
			break
		}
	}

	if !messageEvent {
		t.Error("expected message event")
	}
	if !responseEvent {
		t.Error("expected response event")
	}

	// Check memories were stored
	memories, _ := s.GetMemories(stored.ID)
	if len(memories) != 2 { // user + assistant
		t.Errorf("expected 2 memories, got %d", len(memories))
	}

	agent.Stop()
}

func TestAgentProviderName(t *testing.T) {
	s, bus, cleanup := setupAgentTest(t)
	defer cleanup()

	stored, _ := s.CreateAgent("provider-test", "mock", "test-model")
	mock := provider.NewMock("test-provider", "response")
	agent := NewAgent(stored.ID, stored.Name, mock, s, bus)

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
	agent := NewAgent(stored.ID, stored.Name, mock, s, bus)

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
