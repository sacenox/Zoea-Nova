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
			MaxMyses: 16,
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

func TestCommanderCreateMysis(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	events := bus.Subscribe()

	mysis, err := cmd.CreateMysis("test-mysis", "mock")
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	if mysis.Name() != "test-mysis" {
		t.Errorf("expected name=test-mysis, got %s", mysis.Name())
	}
	if mysis.State() != MysisStateIdle {
		t.Errorf("expected state=idle, got %s", mysis.State())
	}

	// Should receive created event
	select {
	case e := <-events:
		if e.Type != EventMysisCreated {
			t.Errorf("expected EventMysisCreated, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for created event")
	}

	if cmd.MysisCount() != 1 {
		t.Errorf("expected mysis count=1, got %d", cmd.MysisCount())
	}
}

func TestCommanderDeleteMysis(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Subscribe before creating mysis
	events := bus.Subscribe()

	mysis, _ := cmd.CreateMysis("delete-me", "mock")
	id := mysis.ID()

	// Drain the created event
	select {
	case <-events:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for created event")
	}

	if err := cmd.DeleteMysis(id, true); err != nil {
		t.Fatalf("DeleteMysis() error: %v", err)
	}

	// Should receive deleted event
	select {
	case e := <-events:
		if e.Type != EventMysisDeleted {
			t.Errorf("expected EventMysisDeleted, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for deleted event")
	}

	if cmd.MysisCount() != 0 {
		t.Errorf("expected mysis count=0, got %d", cmd.MysisCount())
	}

	// Delete non-existent should error
	if err := cmd.DeleteMysis("nonexistent", false); err == nil {
		t.Error("expected error deleting non-existent mysis")
	}
}

func TestCommanderMaxMyses(t *testing.T) {
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
			MaxMyses: 2, // Low limit for testing
		},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model"},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)
	defer cmd.StopAll()

	// Create up to limit
	cmd.CreateMysis("mysis-1", "mock")
	cmd.CreateMysis("mysis-2", "mock")

	// Should fail at limit
	_, err = cmd.CreateMysis("mysis-3", "mock")
	if err == nil {
		t.Error("expected error when exceeding max myses")
	}

	if cmd.MaxMyses() != 2 {
		t.Errorf("expected max myses=2, got %d", cmd.MaxMyses())
	}
}

func TestCommanderStartStopMysis(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	mysis, _ := cmd.CreateMysis("lifecycle-test", "mock")
	id := mysis.ID()

	// Start
	if err := cmd.StartMysis(id); err != nil {
		t.Fatalf("StartMysis() error: %v", err)
	}
	if mysis.State() != MysisStateRunning {
		t.Errorf("expected state=running, got %s", mysis.State())
	}

	// Stop
	if err := cmd.StopMysis(id); err != nil {
		t.Fatalf("StopMysis() error: %v", err)
	}
	if mysis.State() != MysisStateStopped {
		t.Errorf("expected state=stopped, got %s", mysis.State())
	}

	// Start/stop non-existent
	if err := cmd.StartMysis("nonexistent"); err == nil {
		t.Error("expected error starting non-existent mysis")
	}
	if err := cmd.StopMysis("nonexistent"); err == nil {
		t.Error("expected error stopping non-existent mysis")
	}
}

func TestCommanderConfigureMysis(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Subscribe before creating mysis
	events := bus.Subscribe()

	mysis, _ := cmd.CreateMysis("config-test", "mock")
	id := mysis.ID()

	// Drain created event
	select {
	case <-events:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for created event")
	}

	if err := cmd.ConfigureMysis(id, "ollama"); err != nil {
		t.Fatalf("ConfigureMysis() error: %v", err)
	}

	// Should receive config changed event
	select {
	case e := <-events:
		if e.Type != EventMysisConfigChanged {
			t.Errorf("expected EventMysisConfigChanged, got %s", e.Type)
		}
		data := e.Data.(ConfigChangeData)
		if data.Provider != "ollama" {
			t.Errorf("expected provider=ollama, got %s", data.Provider)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for config changed event")
	}

	// Configure with non-existent provider
	if err := cmd.ConfigureMysis(id, "nonexistent"); err == nil {
		t.Error("expected error configuring with non-existent provider")
	}

	// Configure non-existent mysis
	if err := cmd.ConfigureMysis("nonexistent", "mock"); err == nil {
		t.Error("expected error configuring non-existent mysis")
	}
}

func TestCommanderListMyses(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	cmd.CreateMysis("mysis-1", "mock")
	cmd.CreateMysis("mysis-2", "mock")

	myses := cmd.ListMyses()
	if len(myses) != 2 {
		t.Errorf("expected 2 myses, got %d", len(myses))
	}
}

func TestCommanderGetMysis(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	mysis, _ := cmd.CreateMysis("get-test", "mock")

	fetched, err := cmd.GetMysis(mysis.ID())
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if fetched.Name() != "get-test" {
		t.Errorf("expected name=get-test, got %s", fetched.Name())
	}

	// Get non-existent
	_, err = cmd.GetMysis("nonexistent")
	if err == nil {
		t.Error("expected error getting non-existent mysis")
	}
}

func TestCommanderSendMessage(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	mysis, _ := cmd.CreateMysis("msg-test", "mock")
	id := mysis.ID()

	// Start mysis
	cmd.StartMysis(id)

	// Send message
	if err := cmd.SendMessage(id, "Hello!"); err != nil {
		t.Fatalf("SendMessage() error: %v", err)
	}

	// Send to non-existent
	if err := cmd.SendMessage("nonexistent", "Hello!"); err == nil {
		t.Error("expected error sending to non-existent mysis")
	}
}

func TestCommanderBroadcast(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Subscribe first
	events := bus.Subscribe()

	mysis1, _ := cmd.CreateMysis("broadcast-1", "mock")
	mysis2, _ := cmd.CreateMysis("broadcast-2", "mock")

	cmd.StartMysis(mysis1.ID())
	cmd.StartMysis(mysis2.ID())

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

	mysis, _ := cmd.CreateMysis("broadcast-source-test", "mock")
	cmd.StartMysis(mysis.ID())

	// Wait for mysis to be running with timeout
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if mysis.State() == MysisStateRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mysis.State() != MysisStateRunning {
		t.Fatal("mysis failed to start within timeout")
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

	mysis1, _ := cmd.CreateMysis("stopall-1", "mock")
	mysis2, _ := cmd.CreateMysis("stopall-2", "mock")

	cmd.StartMysis(mysis1.ID())
	cmd.StartMysis(mysis2.ID())

	cmd.StopAll()

	if mysis1.State() != MysisStateStopped {
		t.Errorf("expected mysis1 state=stopped, got %s", mysis1.State())
	}
	if mysis2.State() != MysisStateStopped {
		t.Errorf("expected mysis2 state=stopped, got %s", mysis2.State())
	}
}

func TestCommanderLoadMyses(t *testing.T) {
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	defer s.Close()

	// Pre-populate store
	s.CreateMysis("existing-1", "mock", "mock-model")
	s.CreateMysis("existing-2", "mock", "mock-model")

	bus := NewEventBus(100)
	defer bus.Close()

	reg := provider.NewRegistry()
	reg.Register(provider.NewMock("mock", "response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{MaxMyses: 16},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model"},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)

	if err := cmd.LoadMyses(); err != nil {
		t.Fatalf("LoadMyses() error: %v", err)
	}

	if cmd.MysisCount() != 2 {
		t.Errorf("expected 2 myses loaded, got %d", cmd.MysisCount())
	}
}
