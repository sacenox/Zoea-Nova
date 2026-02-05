package integration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
	"golang.org/x/time/rate"
)

type orchestratorAdapter struct {
	commander *core.Commander
}

func (a *orchestratorAdapter) ListMyses() []mcp.MysisInfo {
	myses := a.commander.ListMyses()
	result := make([]mcp.MysisInfo, len(myses))
	for i, mysis := range myses {
		result[i] = mcp.MysisInfo{
			ID:        mysis.ID(),
			Name:      mysis.Name(),
			LastError: mysis.LastError(),
		}
	}
	return result
}

func (a *orchestratorAdapter) MysisCount() int {
	return a.commander.MysisCount()
}

func (a *orchestratorAdapter) MaxMyses() int {
	return a.commander.MaxMyses()
}

func (a *orchestratorAdapter) GetStateCounts() map[string]int {
	return a.commander.GetStateCounts()
}

func (a *orchestratorAdapter) SendMessageAsync(mysisID, message string) error {
	return a.commander.SendMessageAsync(mysisID, message)
}

func (a *orchestratorAdapter) BroadcastAsync(message string) error {
	return a.commander.BroadcastAsync(message)
}

func (a *orchestratorAdapter) BroadcastFrom(senderID, message string) error {
	return a.commander.BroadcastFrom(senderID, message)
}

func (a *orchestratorAdapter) SearchMessages(mysisID, query string, limit int) ([]mcp.SearchResult, error) {
	memories, err := a.commander.Store().SearchMemories(mysisID, query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]mcp.SearchResult, len(memories))
	for i, m := range memories {
		results[i] = mcp.SearchResult{
			Role:      string(m.Role),
			Source:    string(m.Source),
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	return results, nil
}

func (a *orchestratorAdapter) SearchReasoning(mysisID, query string, limit int) ([]mcp.ReasoningResult, error) {
	memories, err := a.commander.Store().SearchReasoning(mysisID, query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]mcp.ReasoningResult, len(memories))
	for i, m := range memories {
		results[i] = mcp.ReasoningResult{
			Role:      string(m.Role),
			Source:    string(m.Source),
			Content:   m.Content,
			Reasoning: m.Reasoning,
			CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	return results, nil
}

func (a *orchestratorAdapter) SearchBroadcasts(query string, limit int) ([]mcp.BroadcastResult, error) {
	broadcasts, err := a.commander.Store().SearchBroadcasts(query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]mcp.BroadcastResult, len(broadcasts))
	for i, b := range broadcasts {
		results[i] = mcp.BroadcastResult{
			Content:   b.Content,
			CreatedAt: b.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	return results, nil
}

func (a *orchestratorAdapter) ClaimAccount() (mcp.AccountInfo, error) {
	acc, err := a.commander.Store().ClaimAccount()
	if err != nil {
		return mcp.AccountInfo{}, err
	}

	return mcp.AccountInfo{
		Username: acc.Username,
		Password: acc.Password,
	}, nil
}

func setupIntegrationCommander(t *testing.T) (*core.Commander, *store.Store, *mcp.Proxy, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	bus := core.NewEventBus(100)

	reg := provider.NewRegistry()
	limiter := rate.NewLimiter(rate.Limit(1000), 1000)
	reg.RegisterFactory(provider.NewMockFactoryWithLimiter("mock", "mock response", limiter))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses: 16,
		},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model", Temperature: 0.7},
		},
	}

	cmd := core.NewCommander(s, reg, bus, cfg)
	proxy := mcp.NewProxy("")
	mcp.RegisterOrchestratorTools(proxy, &orchestratorAdapter{commander: cmd})
	cmd.SetMCP(proxy)

	cleanup := func() {
		bus.Close()
		s.Close()
	}

	return cmd, s, proxy, cleanup
}

func TestBroadcastSenderTracking(t *testing.T) {
	cmd, s, proxy, cleanup := setupIntegrationCommander(t)
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

	ctx := context.Background()
	caller := mcp.CallerContext{MysisID: sender.ID(), MysisName: sender.Name()}
	args := json.RawMessage(`{"message": "integration test broadcast"}`)
	result, err := proxy.CallTool(ctx, caller, "zoea_broadcast", args)
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content[0].Text)
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

	broadcasts, err := s.SearchBroadcasts("integration test", 10)
	if err != nil {
		t.Fatalf("SearchBroadcasts() error: %v", err)
	}
	if len(broadcasts) == 0 {
		t.Fatal("broadcast not found in search")
	}
	if broadcasts[0].SenderID != sender.ID() {
		t.Errorf("broadcast sender_id: got %q, want %q", broadcasts[0].SenderID, sender.ID())
	}
}
