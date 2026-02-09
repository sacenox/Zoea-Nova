package core

import (
	"fmt"
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
	reg.RegisterFactory("mock", provider.NewMockFactory("mock", "mock response"))
	reg.RegisterFactory("ollama", provider.NewMockFactory("ollama", "ollama response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses: 16,
		},
		Providers: map[string]config.ProviderConfig{
			"mock":   {Endpoint: "http://mock", Model: "mock-model", Temperature: 0.7},
			"ollama": {Endpoint: "http://ollama", Model: "llama3", Temperature: 0.7},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)

	// Set a dummy MCP proxy to avoid "no tools" error events
	proxy := mcp.NewProxy(nil)
	cmd.SetMCP(proxy)

	cleanup := func() {
		cmd.StopAll()
		bus.Close()
		s.Close()
	}

	return cmd, bus, cleanup
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

	// Give mysis time to process first turn (triggered by encouragement message)
	// (otherwise Stop cancels mid-turn and state becomes errored)
	time.Sleep(150 * time.Millisecond)

	// Stop
	if err := cmd.StopMysis(id); err != nil {
		t.Fatalf("StopMysis() error: %v", err)
	}
	// After stopping, state should be stopped (not errored)
	// Note: If stopped mid-turn, it may be errored with "context canceled"
	// which is acceptable behavior
	if mysis.State() != MysisStateStopped && mysis.State() != MysisStateErrored {
		t.Errorf("expected state=stopped or errored, got %s (lastError: %v)", mysis.State(), mysis.LastError())
	}

	// Start/stop non-existent
	if err := cmd.StartMysis("nonexistent"); err == nil {
		t.Error("expected error starting non-existent mysis")
	}
	if err := cmd.StopMysis("nonexistent"); err == nil {
		t.Error("expected error stopping non-existent mysis")
	}
}

func TestCommanderRestartErroredMysis(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	mysis, _ := cmd.CreateMysis("restart-test", "mock")
	id := mysis.ID()

	// Start mysis
	if err := cmd.StartMysis(id); err != nil {
		t.Fatalf("StartMysis() error: %v", err)
	}
	if mysis.State() != MysisStateRunning {
		t.Errorf("expected state=running, got %s", mysis.State())
	}

	// Simulate error by calling setError
	mysis.SetErrorState(fmt.Errorf("simulated error"))
	if mysis.State() != MysisStateErrored {
		t.Errorf("expected state=errored after SetErrorState, got %s", mysis.State())
	}

	// Wait a moment for any pending operations
	time.Sleep(100 * time.Millisecond)

	// Restart the errored mysis
	if err := cmd.StartMysis(id); err != nil {
		t.Fatalf("Restart errored mysis failed: %v", err)
	}

	// Should now be running
	if mysis.State() != MysisStateRunning {
		t.Errorf("expected state=running after restart, got %s (lastError: %v)", mysis.State(), mysis.LastError())
	}

	// Verify lastError was cleared
	if mysis.LastError() != nil {
		t.Errorf("expected lastError to be cleared, got: %v", mysis.LastError())
	}

	// Clean stop
	cmd.StopMysis(id)
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

	if err := cmd.ConfigureMysis(id, "ollama", "llama3"); err != nil {
		t.Fatalf("ConfigureMysis() error: %v", err)
	}

	// Should receive config changed event
	select {
	case e := <-events:
		if e.Type != EventMysisConfigChanged {
			t.Errorf("expected EventMysisConfigChanged, got %s", e.Type)
		}
		if e.Config == nil {
			t.Fatal("expected config change data")
		}
		if e.Config.Provider != "ollama" {
			t.Errorf("expected provider=ollama, got %s", e.Config.Provider)
		}
		if e.Config.Model != "llama3" {
			t.Errorf("expected model=llama3, got %s", e.Config.Model)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for config changed event")
	}

	stored, err := cmd.Store().GetMysis(id)
	if err != nil {
		t.Fatalf("GetMysis() error: %v", err)
	}
	if stored.Model != "llama3" {
		t.Errorf("expected stored model=llama3, got %s", stored.Model)
	}
	if stored.Temperature != 0.7 {
		t.Errorf("expected stored temperature=0.7, got %v", stored.Temperature)
	}

	// Configure with non-existent provider
	if err := cmd.ConfigureMysis(id, "nonexistent", "model"); err == nil {
		t.Error("expected error configuring with non-existent provider")
	}

	// Configure non-existent mysis
	if err := cmd.ConfigureMysis("nonexistent", "mock", "mock-model"); err == nil {
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

func TestCommanderBroadcastToIdleMyses(t *testing.T) {
	cmd, bus, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Subscribe first
	events := bus.Subscribe()

	// Create myses but don't start them (they'll be in idle state)
	mysis1, _ := cmd.CreateMysis("idle-1", "mock")
	mysis2, _ := cmd.CreateMysis("idle-2", "mock")

	// Verify they're idle
	if mysis1.State() != MysisStateIdle {
		t.Fatalf("mysis1 should be idle, got %s", mysis1.State())
	}
	if mysis2.State() != MysisStateIdle {
		t.Fatalf("mysis2 should be idle, got %s", mysis2.State())
	}

	// Drain created events with timeout
	for i := 0; i < 2; i++ {
		select {
		case <-events:
		case <-time.After(100 * time.Millisecond):
		}
	}

	// Should be able to broadcast to idle myses
	if err := cmd.Broadcast("Wake up!"); err != nil {
		t.Fatalf("Broadcast() should accept idle myses, got error: %v", err)
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

	// Verify messages were stored (even though myses were idle)
	memories1, err := cmd.Store().GetMemories(mysis1.ID())
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories1) == 0 {
		t.Error("broadcast should be stored for idle mysis1")
	}

	memories2, err := cmd.Store().GetMemories(mysis2.ID())
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories2) == 0 {
		t.Error("broadcast should be stored for idle mysis2")
	}

	// NEW: Verify myses auto-started (transitioned from idle to running)
	// Wait up to 1 second for state transition
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if mysis1.State() == MysisStateRunning && mysis2.State() == MysisStateRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if mysis1.State() != MysisStateRunning {
		t.Errorf("mysis1 should auto-start on broadcast, got state %s", mysis1.State())
	}
	if mysis2.State() != MysisStateRunning {
		t.Errorf("mysis2 should auto-start on broadcast, got state %s", mysis2.State())
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

func TestBroadcastExcludesSender(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	sender, _ := cmd.CreateMysis("sender", "mock")
	receiver, _ := cmd.CreateMysis("receiver", "mock")

	cmd.StartMysis(sender.ID())
	cmd.StartMysis(receiver.ID())

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if sender.State() == MysisStateRunning && receiver.State() == MysisStateRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sender.State() != MysisStateRunning || receiver.State() != MysisStateRunning {
		t.Fatal("myses failed to start within timeout")
	}

	if err := cmd.BroadcastFrom(sender.ID(), "test broadcast"); err != nil {
		t.Fatalf("BroadcastFrom() error: %v", err)
	}

	senderMemories, err := cmd.Store().GetRecentMemories(sender.ID(), 50)
	if err != nil {
		t.Fatalf("GetRecentMemories() error: %v", err)
	}
	for _, m := range senderMemories {
		if m.Source == store.MemorySourceBroadcast && m.Content == "test broadcast" {
			t.Error("sender received its own broadcast - should be excluded")
		}
	}

	receiverMemories, err := cmd.Store().GetRecentMemories(receiver.ID(), 50)
	if err != nil {
		t.Fatalf("GetRecentMemories() error: %v", err)
	}

	found := false
	for _, m := range receiverMemories {
		if m.Source == store.MemorySourceBroadcast && m.Content == "test broadcast" && m.SenderID == sender.ID() {
			found = true
			break
		}
	}
	if !found {
		t.Error("receiver did not receive broadcast with correct sender_id")
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

	// After StopAll(), myses should be either stopped or errored
	// (errored if an async processing error occurred before stop completed)
	if mysis1.State() != MysisStateStopped && mysis1.State() != MysisStateErrored {
		t.Errorf("expected mysis1 state=stopped or errored, got %s", mysis1.State())
	}
	if mysis2.State() != MysisStateStopped && mysis2.State() != MysisStateErrored {
		t.Errorf("expected mysis2 state=stopped or errored, got %s", mysis2.State())
	}
}

func TestCommanderLoadMyses(t *testing.T) {
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	defer s.Close()

	// Pre-populate store
	s.CreateMysis("existing-1", "mock", "mock-model", 0.7)
	s.CreateMysis("existing-2", "mock", "mock-model", 0.7)

	bus := NewEventBus(100)
	defer bus.Close()

	reg := provider.NewRegistry()
	reg.RegisterFactory("mock", provider.NewMockFactory("mock", "response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{MaxMyses: 16},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model", Temperature: 0.7},
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

// TestBroadcastDoesNotBlockOnBusyMysis verifies that broadcasting doesn't block
// when one or more myses are busy processing a previous message.
// This test ensures the parallel broadcast implementation works correctly.
func TestBroadcastDoesNotBlockOnBusyMysis(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer s.Close()

	bus := NewEventBus(100)
	defer bus.Close()

	reg := provider.NewRegistry()

	// Create a slow provider that will block for 2 seconds
	slowProvider := provider.NewMock("slow", "slow response").
		SetDelay(2 * time.Second)

	// Create a fast provider for the other mysis
	fastProvider := provider.NewMock("fast", "fast response")

	// Register custom factories that return our specific providers
	reg.RegisterFactory("slow", &customMockFactory{
		name:     "slow",
		provider: slowProvider,
	})
	reg.RegisterFactory("fast", &customMockFactory{
		name:     "fast",
		provider: fastProvider,
	})

	cfg := &config.Config{
		Swarm: config.SwarmConfig{MaxMyses: 16},
		Providers: map[string]config.ProviderConfig{
			"slow": {Endpoint: "http://slow", Model: "slow-model", Temperature: 0.7},
			"fast": {Endpoint: "http://fast", Model: "fast-model", Temperature: 0.7},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg)
	proxy := mcp.NewProxy(nil)
	cmd.SetMCP(proxy)

	// Create two myses: one slow, one fast
	slowMysis, err := cmd.CreateMysis("slow-mysis", "slow")
	if err != nil {
		t.Fatalf("CreateMysis(slow) error: %v", err)
	}

	fastMysis, err := cmd.CreateMysis("fast-mysis", "fast")
	if err != nil {
		t.Fatalf("CreateMysis(fast) error: %v", err)
	}

	// Start both myses
	if err := cmd.StartMysis(slowMysis.ID()); err != nil {
		t.Fatalf("StartMysis(slow) error: %v", err)
	}
	if err := cmd.StartMysis(fastMysis.ID()); err != nil {
		t.Fatalf("StartMysis(fast) error: %v", err)
	}

	// Wait for both to be running
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if slowMysis.State() == MysisStateRunning && fastMysis.State() == MysisStateRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if slowMysis.State() != MysisStateRunning || fastMysis.State() != MysisStateRunning {
		t.Fatal("myses failed to start within timeout")
	}

	// Send a direct message to the slow mysis to make it busy
	// This will block for 2 seconds due to the delay
	go func() {
		cmd.SendMessage(slowMysis.ID(), "make slow mysis busy")
	}()

	// Give the slow mysis time to start processing (acquire turnMu lock)
	time.Sleep(100 * time.Millisecond)

	// Now broadcast a message - this should NOT block waiting for the slow mysis
	// The broadcast should complete quickly (within 500ms) because it sends in parallel
	start := time.Now()
	err = cmd.Broadcast("broadcast message")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Broadcast() error: %v", err)
	}

	// The broadcast should complete much faster than the 2-second delay
	// We allow up to 1 second for the broadcast to complete (generous margin)
	// If it takes longer, it means the broadcast blocked waiting for the slow mysis
	if elapsed > 1*time.Second {
		t.Errorf("Broadcast took too long (%v), likely blocked on busy mysis", elapsed)
	}

	// Verify both myses received the broadcast
	// Wait a bit for the slow mysis to finish processing
	time.Sleep(2500 * time.Millisecond)

	slowMemories, err := s.GetRecentMemories(slowMysis.ID(), 50)
	if err != nil {
		t.Fatalf("GetRecentMemories(slow) error: %v", err)
	}

	fastMemories, err := s.GetRecentMemories(fastMysis.ID(), 50)
	if err != nil {
		t.Fatalf("GetRecentMemories(fast) error: %v", err)
	}

	// Check that both myses have the broadcast message
	slowHasBroadcast := false
	for _, m := range slowMemories {
		if m.Source == store.MemorySourceBroadcast && m.Content == "broadcast message" {
			slowHasBroadcast = true
			break
		}
	}

	fastHasBroadcast := false
	for _, m := range fastMemories {
		if m.Source == store.MemorySourceBroadcast && m.Content == "broadcast message" {
			fastHasBroadcast = true
			break
		}
	}

	if !slowHasBroadcast {
		t.Error("slow mysis did not receive broadcast message")
	}
	if !fastHasBroadcast {
		t.Error("fast mysis did not receive broadcast message")
	}

	cmd.StopAll()
}

// customMockFactory is a factory that returns a specific provider instance
type customMockFactory struct {
	name     string
	provider provider.Provider
}

func (f *customMockFactory) Name() string { return f.name }

func (f *customMockFactory) Create(model string, temperature float64) provider.Provider {
	return f.provider
}

// TestAggregateTick_MaxAcrossMyses verifies that AggregateTick returns the maximum
// lastServerTick across all myses in the swarm.
func TestAggregateTick_MaxAcrossMyses(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Empty swarm should return 0
	if tick := cmd.AggregateTick(); tick != 0 {
		t.Errorf("empty swarm: expected tick=0, got %d", tick)
	}

	// Create myses
	m1, _ := cmd.CreateMysis("mysis-1", "mock")
	m2, _ := cmd.CreateMysis("mysis-2", "mock")
	m3, _ := cmd.CreateMysis("mysis-3", "mock")

	// Set lastServerTick directly for testing
	m1.mu.Lock()
	m1.lastServerTick = 98
	m1.mu.Unlock()

	m2.mu.Lock()
	m2.lastServerTick = 120
	m2.mu.Unlock()

	m3.mu.Lock()
	m3.lastServerTick = 0 // No tick data yet
	m3.mu.Unlock()

	// Should return max tick (120)
	if tick := cmd.AggregateTick(); tick != 120 {
		t.Errorf("expected aggregate tick=120, got %d", tick)
	}
}

// TestAggregateTick_EdgeCases verifies edge cases for aggregate tick calculation.
func TestAggregateTick_EdgeCases(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	t.Run("all myses with zero tick", func(t *testing.T) {
		m1, _ := cmd.CreateMysis("zero-1", "mock")
		m2, _ := cmd.CreateMysis("zero-2", "mock")

		m1.mu.Lock()
		m1.lastServerTick = 0
		m1.mu.Unlock()

		m2.mu.Lock()
		m2.lastServerTick = 0
		m2.mu.Unlock()

		if tick := cmd.AggregateTick(); tick != 0 {
			t.Errorf("expected tick=0 when all myses have zero tick, got %d", tick)
		}
	})

	t.Run("single mysis", func(t *testing.T) {
		// Clean up previous myses
		for _, m := range cmd.ListMyses() {
			cmd.DeleteMysis(m.ID(), false)
		}

		m, _ := cmd.CreateMysis("single", "mock")
		m.mu.Lock()
		m.lastServerTick = 42
		m.mu.Unlock()

		if tick := cmd.AggregateTick(); tick != 42 {
			t.Errorf("expected tick=42 for single mysis, got %d", tick)
		}
	})
}

// TestMaxMyses verifies the MaxMyses getter returns the configured value.
func TestMaxMyses(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	expected := 16 // From setupCommanderTest config
	if got := cmd.MaxMyses(); got != expected {
		t.Errorf("MaxMyses() = %d, want %d", got, expected)
	}
}

// TestGetStateCounts verifies the GetStateCounts method returns accurate counts.
func TestGetStateCounts(t *testing.T) {
	cmd, _, cleanup := setupCommanderTest(t)
	defer cleanup()

	// Empty swarm - may have zero counts for all states
	counts := cmd.GetStateCounts()
	total := 0
	for _, count := range counts {
		total += count
	}
	if total != 0 {
		t.Errorf("empty swarm should have 0 total myses, got %d", total)
	}

	// Create myses in various states
	cmd.CreateMysis("idle-1", "mock")
	cmd.CreateMysis("idle-2", "mock")
	m3, _ := cmd.CreateMysis("running-1", "mock")

	// Start one mysis
	cmd.StartMysis(m3.ID())
	time.Sleep(50 * time.Millisecond) // Give it time to start

	counts = cmd.GetStateCounts()

	// Should have at least idle and running states
	if counts["idle"] < 2 {
		t.Errorf("expected at least 2 idle myses, got %d", counts["idle"])
	}
	if counts["running"] < 1 {
		t.Errorf("expected at least 1 running mysis, got %d", counts["running"])
	}

	// Total should match number of myses
	total = 0
	for _, count := range counts {
		total += count
	}
	if total != 3 {
		t.Errorf("total myses across states = %d, want 3", total)
	}
}
