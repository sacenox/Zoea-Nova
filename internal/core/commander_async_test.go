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

func setupCommanderAsyncTest(t *testing.T) (*Commander, *store.Store, *EventBus, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	bus := NewEventBus(100)
	reg := provider.NewRegistry()
	cfg := config.DefaultConfig()
	cfg.Providers["mock"] = config.ProviderConfig{Model: "test-model"}
	cfg.Providers["mock1"] = config.ProviderConfig{Model: "test-model"}
	cfg.Providers["mock2"] = config.ProviderConfig{Model: "test-model"}

	c := NewCommander(s, reg, bus, cfg)

	// Set a dummy MCP proxy to avoid "no tools" error events
	proxy := mcp.NewProxy("")
	c.SetMCP(proxy)

	cleanup := func() {
		bus.Close()
		s.Close()
	}

	return c, s, bus, cleanup
}

func TestCommanderSendMessageAsync(t *testing.T) {
	c, _, bus, cleanup := setupCommanderAsyncTest(t)
	defer cleanup()

	mock := provider.NewMock("mock", "response")
	c.registry.Register(mock)

	mysis, _ := c.CreateMysis("async-mysis", "mock")

	events := bus.Subscribe()
	mysis.Start()

	// Wait for initial turn
	timeout := time.After(10 * time.Second)
	found := false
	for !found {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				found = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial turn")
		}
	}

	// Send async message
	if err := c.SendMessageAsync(mysis.ID(), "Hello async"); err != nil {
		t.Fatalf("SendMessageAsync() error: %v", err)
	}

	// Should receive response eventually
	found = false
	for !found {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				found = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for async response")
		}
	}
}

func TestCommanderBroadcastAsync(t *testing.T) {
	c, _, bus, cleanup := setupCommanderAsyncTest(t)
	defer cleanup()

	// Use separate mocks to avoid any potential contention
	m1 := provider.NewMock("mock1", "response1")
	m2 := provider.NewMock("mock2", "response2")
	c.registry.Register(m1)
	c.registry.Register(m2)

	a1, err := c.CreateMysis("m1", "mock1")
	if err != nil {
		t.Fatalf("CreateMysis(m1) error: %v", err)
	}
	a2, err := c.CreateMysis("m2", "mock2")
	if err != nil {
		t.Fatalf("CreateMysis(m2) error: %v", err)
	}

	events := bus.Subscribe()
	if err := a1.Start(); err != nil {
		t.Fatalf("a1.Start() error: %v", err)
	}
	// Wait for a1 initial turn
	timeout := time.After(10 * time.Second)
	found := false
	for !found {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse && e.MysisID == a1.ID() {
				found = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for a1 initial turn")
		}
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("a2.Start() error: %v", err)
	}
	// Wait for a2 initial turn
	timeout = time.After(10 * time.Second)
	found = false
	for !found {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse && e.MysisID == a2.ID() {
				found = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for a2 initial turn")
		}
	}

	// Broadcast async
	if err := c.BroadcastAsync("Hello swarm"); err != nil {
		t.Fatalf("BroadcastAsync() error: %v", err)
	}

	// Should receive 2 responses
	responses := make(map[string]bool)
	timeout = time.After(15 * time.Second)
	for len(responses) < 2 {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				responses[e.MysisID] = true
			}
		case <-timeout:
			t.Fatalf("timeout waiting for broadcast responses, got %d", len(responses))
		}
	}
}
