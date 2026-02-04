package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

func setupCommanderTest(t *testing.T) (*Commander, *EventBus, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	bus := NewEventBus(100)

	reg := provider.NewRegistry()
	reg.Register(provider.NewMock("mock", "mock response"))
	reg.Register(provider.NewMock("ollama", "ollama response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxAgents: 16,
		},
		Providers: map[string]config.ProviderConfig{
			"mock":   {Endpoint: "http://mock", Model: "mock-model"},
			"ollama": {Endpoint: "http://ollama", Model: "llama3"},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)

	// Set a dummy MCP proxy to avoid "no tools" error events
	proxy := mcp.NewProxy("")
	cmd.SetMCP(proxy)

	cleanup := func() {
		cmd.StopAll()
		bus.Close()
		s.Close()
	}

	return cmd, bus, cleanup
}

func TestCommanderCreateAgent(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	events := bus.Subscribe()

	agent, err := cmd.CreateAgent("test-agent", "mock")
	if err != nil {
		t.Fatalf("CreateAgent() error: %v", err)
	}

	if agent.Name() != "test-agent" {
		t.Errorf("expected name=test-agent, got %s", agent.Name())
	}
	if agent.State() != AgentStateIdle {
		t.Errorf("expected state=idle, got %s", agent.State())
	}

	// Should receive created event
	select {
	case e := <-events:
		if e.Type != EventAgentCreated {
			t.Errorf("expected EventAgentCreated, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for created event")
	}

	if cmd.AgentCount() != 1 {
		t.Errorf("expected agent count=1, got %d", cmd.AgentCount())
	}
}

func TestCommanderDeleteAgent(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Subscribe before creating agent
	events := bus.Subscribe()

	agent, _ := cmd.CreateAgent("delete-me", "mock")
	id := agent.ID()

	// Drain the created event
	select {
	case <-events:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for created event")
	}

	if err := cmd.DeleteAgent(id, true); err != nil {
		t.Fatalf("DeleteAgent() error: %v", err)
	}

	// Should receive deleted event
	select {
	case e := <-events:
		if e.Type != EventAgentDeleted {
			t.Errorf("expected EventAgentDeleted, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for deleted event")
	}

	if cmd.AgentCount() != 0 {
		t.Errorf("expected agent count=0, got %d", cmd.AgentCount())
	}

	// Delete non-existent should error
	if err := cmd.DeleteAgent("nonexistent", false); err == nil {
		t.Error("expected error deleting non-existent agent")
	}
}

func TestCommanderMaxAgents(t *testing.T) {
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	defer s.Close()

	bus := NewEventBus(100)
	defer bus.Close()

	reg := provider.NewRegistry()
	reg.Register(provider.NewMock("mock", "response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxAgents: 2, // Low limit for testing
		},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model"},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)
	defer cmd.StopAll()

	// Create up to limit
	cmd.CreateAgent("agent-1", "mock")
	cmd.CreateAgent("agent-2", "mock")

	// Should fail at limit
	_, err = cmd.CreateAgent("agent-3", "mock")
	if err == nil {
		t.Error("expected error when exceeding max agents")
	}

	if cmd.MaxAgents() != 2 {
		t.Errorf("expected max agents=2, got %d", cmd.MaxAgents())
	}
}

func TestCommanderStartStopAgent(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	agent, _ := cmd.CreateAgent("lifecycle-test", "mock")
	id := agent.ID()

	// Start
	if err := cmd.StartAgent(id); err != nil {
		t.Fatalf("StartAgent() error: %v", err)
	}
	if agent.State() != AgentStateRunning {
		t.Errorf("expected state=running, got %s", agent.State())
	}

	// Stop
	if err := cmd.StopAgent(id); err != nil {
		t.Fatalf("StopAgent() error: %v", err)
	}
	if agent.State() != AgentStateStopped {
		t.Errorf("expected state=stopped, got %s", agent.State())
	}

	// Start/stop non-existent
	if err := cmd.StartAgent("nonexistent"); err == nil {
		t.Error("expected error starting non-existent agent")
	}
	if err := cmd.StopAgent("nonexistent"); err == nil {
		t.Error("expected error stopping non-existent agent")
	}
}

func TestCommanderConfigureAgent(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Subscribe before creating agent
	events := bus.Subscribe()

	agent, _ := cmd.CreateAgent("config-test", "mock")
	id := agent.ID()

	// Drain created event
	select {
	case <-events:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for created event")
	}

	if err := cmd.ConfigureAgent(id, "ollama"); err != nil {
		t.Fatalf("ConfigureAgent() error: %v", err)
	}

	// Should receive config changed event
	select {
	case e := <-events:
		if e.Type != EventAgentConfigChanged {
			t.Errorf("expected EventAgentConfigChanged, got %s", e.Type)
		}
		data := e.Data.(ConfigChangeData)
		if data.Provider != "ollama" {
			t.Errorf("expected provider=ollama, got %s", data.Provider)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for config changed event")
	}

	// Configure with non-existent provider
	if err := cmd.ConfigureAgent(id, "nonexistent"); err == nil {
		t.Error("expected error configuring with non-existent provider")
	}

	// Configure non-existent agent
	if err := cmd.ConfigureAgent("nonexistent", "mock"); err == nil {
		t.Error("expected error configuring non-existent agent")
	}
}

func TestCommanderListAgents(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	cmd.CreateAgent("agent-1", "mock")
	cmd.CreateAgent("agent-2", "mock")

	agents := cmd.ListAgents()
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestCommanderGetAgent(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	agent, _ := cmd.CreateAgent("get-test", "mock")

	fetched, err := cmd.GetAgent(agent.ID())
	if err != nil {
		t.Fatalf("GetAgent() error: %v", err)
	}
	if fetched.Name() != "get-test" {
		t.Errorf("expected name=get-test, got %s", fetched.Name())
	}

	// Get non-existent
	_, err = cmd.GetAgent("nonexistent")
	if err == nil {
		t.Error("expected error getting non-existent agent")
	}
}

func TestCommanderSendMessage(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	agent, _ := cmd.CreateAgent("msg-test", "mock")
	id := agent.ID()

	// Start agent
	cmd.StartAgent(id)

	// Send message
	if err := cmd.SendMessage(id, "Hello!"); err != nil {
		t.Fatalf("SendMessage() error: %v", err)
	}

	// Send to non-existent
	if err := cmd.SendMessage("nonexistent", "Hello!"); err == nil {
		t.Error("expected error sending to non-existent agent")
	}
}

func TestCommanderBroadcast(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Subscribe first
	events := bus.Subscribe()

	agent1, _ := cmd.CreateAgent("broadcast-1", "mock")
	agent2, _ := cmd.CreateAgent("broadcast-2", "mock")

	cmd.StartAgent(agent1.ID())
	cmd.StartAgent(agent2.ID())

	// Drain created/started events with timeout
	for i := 0; i < 4; i++ {
		select {
		case <-events:
		case <-time.After(100 * time.Millisecond):
			// Some events may not have been emitted yet
		}
	}

	if err := cmd.Broadcast("Hello everyone!"); err != nil {
		t.Fatalf("Broadcast() error: %v", err)
	}

	// Should receive broadcast event
	found := false
	timeout := time.After(2 * time.Second)
	for !found {
		select {
		case e := <-events:
			if e.Type == EventBroadcast {
				found = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for broadcast event")
		}
	}
}

func TestCommanderBroadcastSource(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	agent, _ := cmd.CreateAgent("broadcast-source-test", "mock")
	cmd.StartAgent(agent.ID())

	// Wait for agent to be running with timeout
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if agent.State() == AgentStateRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if agent.State() != AgentStateRunning {
		t.Fatal("agent failed to start within timeout")
	}

	// Send broadcast
	if err := cmd.Broadcast("Swarm command!"); err != nil {
		t.Fatalf("Broadcast() error: %v", err)
	}

	// Poll for broadcast message with timeout
	var found bool
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !found {
		broadcasts, err := cmd.Store().GetRecentBroadcasts(10)
		if err != nil {
			t.Fatalf("GetRecentBroadcasts() error: %v", err)
		}
		for _, b := range broadcasts {
			if b.Content == "Swarm command!" {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(10 * time.Millisecond)
		}
	}

	if !found {
		t.Error("broadcast message not found with source='broadcast' within timeout")
	}
}

func TestCommanderStopAll(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	agent1, _ := cmd.CreateAgent("stopall-1", "mock")
	agent2, _ := cmd.CreateAgent("stopall-2", "mock")

	cmd.StartAgent(agent1.ID())
	cmd.StartAgent(agent2.ID())

	cmd.StopAll()

	if agent1.State() != AgentStateStopped {
		t.Errorf("expected agent1 state=stopped, got %s", agent1.State())
	}
	if agent2.State() != AgentStateStopped {
		t.Errorf("expected agent2 state=stopped, got %s", agent2.State())
	}
}

func TestCommanderLoadAgents(t *testing.T) {
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	defer s.Close()

	// Pre-populate store
	s.CreateAgent("existing-1", "mock", "mock-model")
	s.CreateAgent("existing-2", "mock", "mock-model")

	bus := NewEventBus(100)
	defer bus.Close()

	reg := provider.NewRegistry()
	reg.Register(provider.NewMock("mock", "response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{MaxAgents: 16},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model"},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)

	if err := cmd.LoadAgents(); err != nil {
		t.Fatalf("LoadAgents() error: %v", err)
	}

	if cmd.AgentCount() != 2 {
		t.Errorf("expected 2 agents loaded, got %d", cmd.AgentCount())
	}
}
