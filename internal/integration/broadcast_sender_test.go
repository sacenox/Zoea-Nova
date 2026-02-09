package integration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

func setupIntegrationCommander(t *testing.T) (*core.Commander, *store.Store, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	bus := core.NewEventBus(100)

	reg := provider.NewRegistry()
	reg.RegisterFactory("mock", provider.NewMockFactory("mock", "mock response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses: 16,
		},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model", Temperature: 0.7},
		},
	}

	cmd := core.NewCommander(s, reg, bus, cfg, "")

	cleanup := func() {
		bus.Close()
		s.Close()
	}

	return cmd, s, cleanup
}

func TestBroadcastSenderTracking(t *testing.T) {
	cmd, s, cleanup := setupIntegrationCommander(t)
	defer cleanup()

	sender, _ := cmd.CreateMysis("sender", "mock")
	receiver1, _ := cmd.CreateMysis("receiver1", "mock")
	receiver2, _ := cmd.CreateMysis("receiver2", "mock")

	cmd.StartMysis(sender.ID())
	cmd.StartMysis(receiver1.ID())
	cmd.StartMysis(receiver2.ID())

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if sender.State() == core.MysisStateRunning && receiver1.State() == core.MysisStateRunning && receiver2.State() == core.MysisStateRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sender.State() != core.MysisStateRunning || receiver1.State() != core.MysisStateRunning || receiver2.State() != core.MysisStateRunning {
		t.Fatal("myses failed to start within timeout")
	}

	// Send broadcast from sender using BroadcastFrom
	err := cmd.BroadcastFrom(sender.ID(), "integration test broadcast")
	if err != nil {
		t.Fatalf("BroadcastFrom() error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	senderMems, err := s.GetRecentMemories(sender.ID(), 50)
	if err != nil {
		t.Fatalf("GetRecentMemories() error: %v", err)
	}
	for _, m := range senderMems {
		if m.Source == store.MemorySourceBroadcast && m.Content == "integration test broadcast" {
			t.Error("sender received its own broadcast")
		}
	}

	for _, receiver := range []*core.Mysis{receiver1, receiver2} {
		mems, err := s.GetRecentMemories(receiver.ID(), 50)
		if err != nil {
			t.Fatalf("GetRecentMemories() error: %v", err)
		}
		found := false
		for _, m := range mems {
			if m.Source == store.MemorySourceBroadcast && m.Content == "integration test broadcast" && m.SenderID == sender.ID() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s did not receive broadcast with sender tracking", receiver.Name())
		}
	}

	// Verify sender_id was tracked correctly by checking receiver memories
	foundWithSender := false
	for _, receiver := range []*core.Mysis{receiver1, receiver2} {
		mems, err := s.GetRecentMemories(receiver.ID(), 50)
		if err != nil {
			t.Fatalf("GetRecentMemories() error: %v", err)
		}
		for _, m := range mems {
			if m.Source == store.MemorySourceBroadcast && m.Content == "integration test broadcast" {
				if m.SenderID == sender.ID() {
					foundWithSender = true
					break
				}
			}
		}
		if foundWithSender {
			break
		}
	}
	if !foundWithSender {
		t.Error("broadcast sender_id not correctly tracked in receiver memories")
	}
}
