package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

func setupTestModel(t *testing.T) (Model, func()) {
	t.Helper()

	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}

	bus := core.NewEventBus(100)
	eventCh := bus.Subscribe()

	reg := provider.NewRegistry()
	reg.Register(provider.NewMock("ollama", "mock response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxAgents: 16,
		},
		Providers: map[string]config.ProviderConfig{
			"ollama": {Endpoint: "http://mock", Model: "mock-model"},
		},
	}

	commander := core.NewCommander(s, reg, bus, cfg)

	model := New(commander, s, eventCh)
	model.width = 80
	model.height = 24

	cleanup := func() {
		commander.StopAll()
		bus.Close()
		s.Close()
	}

	return model, cleanup
}

func TestModelInit(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	cmd := m.Init()
	if cmd == nil {
		t.Error("expected Init to return a command")
	}
}

func TestModelView(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Initial view should render without panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModelViewWithZeroSize(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 0
	m.height = 0

	view := m.View()
	if view != "Loading..." {
		t.Errorf("expected 'Loading...', got %s", view)
	}
}

func TestModelWindowResize(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newModel.(Model)

	if m.width != 120 {
		t.Errorf("expected width=120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height=40, got %d", m.height)
	}
}

func TestModelQuit(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// Check if quit command was returned
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModelHelpToggle(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	if m.showHelp {
		t.Error("help should be hidden initially")
	}

	// Toggle help on
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newModel.(Model)

	if !m.showHelp {
		t.Error("help should be shown after pressing ?")
	}

	// Toggle help off
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newModel.(Model)

	if m.showHelp {
		t.Error("help should be hidden after pressing ? again")
	}
}

func TestModelNavigation(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create some agents
	m.commander.CreateAgent("agent-1", "ollama")
	m.commander.CreateAgent("agent-2", "ollama")
	m.refreshAgentList()

	if m.selectedIdx != 0 {
		t.Errorf("expected selectedIdx=0, got %d", m.selectedIdx)
	}

	// Navigate down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)

	if m.selectedIdx != 1 {
		t.Errorf("expected selectedIdx=1, got %d", m.selectedIdx)
	}

	// Navigate up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)

	if m.selectedIdx != 0 {
		t.Errorf("expected selectedIdx=0, got %d", m.selectedIdx)
	}
}

func TestModelFocusView(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create an agent
	agent, _ := m.commander.CreateAgent("test-agent", "ollama")
	m.refreshAgentList()

	// Press enter to focus
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if m.view != ViewFocus {
		t.Errorf("expected view=ViewFocus, got %d", m.view)
	}
	if m.focusID != agent.ID() {
		t.Errorf("expected focusID=%s, got %s", agent.ID(), m.focusID)
	}

	// Press escape to go back
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)

	if m.view != ViewDashboard {
		t.Errorf("expected view=ViewDashboard, got %d", m.view)
	}
}

func TestInputModel(t *testing.T) {
	input := NewInputModel()

	if input.IsActive() {
		t.Error("input should not be active initially")
	}

	input.SetMode(InputModeBroadcast, "")
	if !input.IsActive() {
		t.Error("input should be active after SetMode")
	}
	if input.Mode() != InputModeBroadcast {
		t.Errorf("expected mode=InputModeBroadcast, got %d", input.Mode())
	}

	input.Reset()
	if input.IsActive() {
		t.Error("input should not be active after Reset")
	}
}

func TestRenderHelp(t *testing.T) {
	help := RenderHelp(80, 24)
	if help == "" {
		t.Error("expected non-empty help")
	}
}

func TestRenderDashboard(t *testing.T) {
	agents := []AgentInfo{
		{ID: "1", Name: "agent-1", State: "running", Provider: "ollama"},
		{ID: "2", Name: "agent-2", State: "idle", Provider: "ollama"},
	}

	loadingSet := make(map[string]bool)
	swarmMsgs := []SwarmMessageInfo{}
	dashboard := RenderDashboard(agents, swarmMsgs, 0, 80, 24, loadingSet, "⠋")
	if dashboard == "" {
		t.Error("expected non-empty dashboard")
	}

	// Test with loading state
	loadingSet["1"] = true
	dashboardWithLoading := RenderDashboard(agents, swarmMsgs, 0, 80, 24, loadingSet, "⠋")
	if dashboardWithLoading == "" {
		t.Error("expected non-empty dashboard with loading")
	}
}

func TestRenderDashboardEmpty(t *testing.T) {
	dashboard := RenderDashboard([]AgentInfo{}, []SwarmMessageInfo{}, 0, 80, 24, make(map[string]bool), "⠋")
	if dashboard == "" {
		t.Error("expected non-empty dashboard even with no agents")
	}
}

func TestRenderFocusView(t *testing.T) {
	agent := AgentInfo{ID: "1", Name: "test-agent", State: "running", Provider: "ollama"}
	logs := []LogEntry{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there! This is a longer response that might span multiple lines when properly wrapped in the terminal window."},
	}

	view := RenderFocusView(agent, logs, 80, 24, false, "⠋")
	if view == "" {
		t.Error("expected non-empty focus view")
	}

	// Test with loading state
	viewLoading := RenderFocusView(agent, logs, 80, 24, true, "⠋")
	if viewLoading == "" {
		t.Error("expected non-empty focus view with loading")
	}
}

func TestStateStyle(t *testing.T) {
	// Just ensure these don't panic
	_ = StateStyle("running")
	_ = StateStyle("idle")
	_ = StateStyle("stopped")
	_ = StateStyle("errored")
	_ = StateStyle("unknown")
}

func TestRoleStyle(t *testing.T) {
	// Just ensure these don't panic
	_ = RoleStyle("user")
	_ = RoleStyle("assistant")
	_ = RoleStyle("system")
	_ = RoleStyle("unknown")
}
